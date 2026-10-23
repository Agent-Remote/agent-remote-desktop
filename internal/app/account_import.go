package app

import (
	"context"
	"slices"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type accountImportRecord struct {
	reference    remote.AccountImportReference
	receipt      *remote.AccountImportReceipt
	err          error
	notSubmitted bool
}
type accountImportPage struct {
	expected                                         cliadapter.ConfigContext
	accountID                                        string
	selection                                        remote.AccountImportSelection
	discovery                                        *remote.AccountImportDiscovery
	plan                                             *remote.AccountImportPlan
	validation                                       *remote.AccountImportValidation
	records                                          []accountImportRecord
	history, pendingLoaded, loading, busy, submitted bool
	review, operation                                string
	activeReference                                  remote.AccountImportReference
	forget                                           *remote.AccountImportReference
	manifestPage                                     int
	err                                              error
	cancel                                           context.CancelFunc
	done                                             chan struct{}
	results                                          chan accountImportResult
	generation                                       uint64
}
type accountImportResult struct {
	kind         string
	generation   uint64
	discovery    remote.AccountImportDiscovery
	plan         remote.AccountImportPlan
	validation   remote.AccountImportValidation
	receipt      remote.AccountImportReceipt
	pending      []remote.AccountImportReference
	reference    remote.AccountImportReference
	forgotten    remote.AccountImportForgetReceipt
	err          error
	notSubmitted bool
}

func (a *App) importBlocked() bool {
	return a.runtimeRecovery.busy || a.synchronizationBusy() || a.account.busy || a.accountDefault.busy || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy || a.closing || a.quitting.Load()
}
func (a *App) openAccountImport() {
	a.page = "account-import"
	p := &a.accountImport
	if p.loading || p.busy || p.submitted || p.plan != nil {
		p.history = false
		return
	}
	if a.account.status == nil || a.account.status.Account.Tool != "claude" || !remote.AccountImportSupported(a.account.capabilities, "accounts.import.prepare") {
		return
	}
	a.changeImportSelection()
	p.expected = a.account.status.Context
	p.accountID = a.account.status.Account.ID
	p.selection = remote.AccountImportSelection{Tool: "claude"}
	p.history = false
	p.err = nil
}
func (a *App) openAccountImportHistory() {
	a.page = "account-import"
	p := &a.accountImport
	p.history = true
	// Rehydrate only from the CLI-owned journal. The desktop never persists a
	// second request record and never reconstructs an operation from a latest
	// snapshot. A failed read remains retryable through the explicit Pending
	// action; a successful empty read is marked loaded to avoid polling loops.
	if !p.pendingLoaded && !p.loading && !p.busy && !a.importBlocked() {
		a.loadAccountImportPending()
	}
}
func (a *App) changeImportSelection() {
	p := &a.accountImport
	p.discovery, p.plan, p.validation, p.err = nil, nil, nil, nil
	p.review = ""
	p.manifestPage = 0
}
func (a *App) startAccountImport(kind string, work func(context.Context, cliadapter.Client) accountImportResult) {
	p := &a.accountImport
	if p.loading || p.busy || a.importBlocked() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results = cancel, make(chan struct{}), make(chan accountImportResult, 1)
	p.busy = kind == "validate" || kind == "apply" || kind == "forget"
	p.loading = !p.busy
	p.err = nil
	p.operation = kind
	results, done, home, find, generation, reference := p.results, p.done, a.home, a.findExecutable, p.generation, p.activeReference
	go func() {
		defer close(done)
		defer cancel()
		result := accountImportResult{kind: kind, generation: generation, reference: reference}
		binary, err := find("agent-remote")
		if err == nil {
			result = work(ctx, cliadapter.Client{Binary: binary, Home: home})
			result.kind, result.generation = kind, generation
		} else {
			result.err = err
		}
		results <- result
	}()
}
func (a *App) discoverAccountImport() {
	if a.accountImport.loading || a.accountImport.busy || a.accountImport.submitted || a.importBlocked() {
		return
	}
	selection := a.accountImport.selection
	a.changeImportSelection()
	a.startAccountImport("discover", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		value, err := remote.DiscoverAccountImport(ctx, client, selection)
		return accountImportResult{discovery: value, err: err}
	})
}
func (a *App) prepareAccountImport() {
	p := &a.accountImport
	if p.discovery == nil || p.submitted {
		return
	}
	input := remote.AccountImportPrepareInput{Expected: p.expected, AccountID: p.accountID, Selection: p.selection, ExpectedSourceHome: p.discovery.SourceHome}
	p.plan, p.validation = nil, nil
	p.review = ""
	p.manifestPage = 0
	a.startAccountImport("prepare", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		pending, err := remote.ListAccountImportPending(ctx, client)
		if err != nil {
			return accountImportResult{err: err}
		}
		value, err := remote.PrepareAccountImport(ctx, client, input)
		return accountImportResult{plan: value, pending: pending, err: err}
	})
}
func (a *App) validateAccountImport() {
	p := &a.accountImport
	if p.review != "validate" || p.discovery == nil || p.submitted {
		return
	}
	input := remote.AccountImportValidateInput{Confirmed: true, Expected: p.expected, AccountID: p.accountID, Selection: p.selection, ExpectedSourceHome: p.discovery.SourceHome, ExpectedInclude: append([]string{}, p.discovery.Include...)}
	a.startAccountImport("validate", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		value, err := remote.ValidateAccountImport(ctx, client, input)
		return accountImportResult{validation: value, err: err}
	})
}
func (a *App) applyAccountImport() {
	p := &a.accountImport
	if p.review != "apply" || p.plan == nil || len(p.plan.Manifest) == 0 || p.loading || p.busy || p.submitted || p.err != nil || a.importBlocked() || a.accountImportHasRetainedRequest() {
		return
	}
	key, err := remote.NewAccountImportRequestID()
	if err != nil {
		p.err = err
		return
	}
	input := remote.AccountImportApplyInput{Confirmed: true, ExpectedPlan: *p.plan, RequestID: key}
	reference := remote.AccountImportReference{Context: p.plan.Context, AccountID: p.plan.AccountID, RequestID: key, Digest: p.plan.Digest, IntentFingerprint: p.plan.IntentFingerprint, Phase: "pending"}
	// Keep the original key in memory even if dispatch/cancellation loses the reply.
	// Rust durably journals it before any upload; pending restores it after restart.
	p.records = append(p.records, accountImportRecord{reference: reference})
	p.submitted = true
	p.activeReference = reference
	a.startAccountImport("apply", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		value, err := remote.ApplyAccountImport(ctx, client, input)
		result := accountImportResult{receipt: value, reference: reference, err: err}
		if accountImportPreSubmitError(err) {
			pending, pendingErr := remote.ListAccountImportPending(ctx, client)
			result.notSubmitted = pendingErr == nil && !slices.ContainsFunc(pending, func(item remote.AccountImportReference) bool { return item.RequestID == reference.RequestID })
		}
		return result
	})
}

func (a *App) pollAccountImport(c *ui.Context) {
	p := &a.accountImport
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		p.loading, p.busy = false, false
		p.review = ""
		if result.kind == "forget" {
			if result.err == nil {
				p.records = slices.DeleteFunc(p.records, func(record accountImportRecord) bool { return record.reference.RequestID == result.reference.RequestID })
				p.forget = nil
			} else {
				p.err = result.err
			}
			return
		}
		if result.kind == "apply" || result.kind == "status" {
			a.updateAccountImportRecord(result)
			if result.kind == "apply" {
				a.invalidateRemoteAccountList()
			}
			return
		}
		if result.generation != p.generation {
			return
		}
		p.err = result.err
		if result.err != nil {
			return
		}
		switch result.kind {
		case "discover":
			p.discovery = &result.discovery
		case "prepare":
			p.plan = &result.plan
			p.discovery = &remote.AccountImportDiscovery{SourceHome: result.plan.SourceHome, Selection: result.plan.Selection, Include: result.plan.Include, Exclude: result.plan.Exclude}
			a.mergeAccountImportPending(result.pending)
		case "validate":
			p.validation = &result.validation
		case "pending":
			a.mergeAccountImportPending(result.pending)
		}

	default:
		c.After(50 * time.Millisecond)
	}
}
func (a *App) invalidateAccountImport() {
	p := &a.accountImport
	p.generation++
	if !p.submitted {
		p.discovery, p.plan, p.validation = nil, nil, nil
		p.review = ""
		p.pendingLoaded = false
		p.err = fault.CLIContextChanged
	}
	if p.loading && p.operation != "status" && p.operation != "pending" {
		p.cancel()
	}
}
func (a *App) finishAccountImport() {
	p := &a.accountImport
	if p.loading || p.busy {
		return
	}
	// A completed pre-submit failure plus an absent durable key is not a request
	// to recover. Release that speculative reference only on explicit review end.
	records := slices.DeleteFunc(p.records, func(record accountImportRecord) bool { return record.notSubmitted })
	*p = accountImportPage{records: records, history: true}
}
