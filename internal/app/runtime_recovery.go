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

type runtimeRecoveryRecord struct {
	reference    remote.RuntimeRecoveryRecord
	receipt      *remote.RuntimeRecoveryReceipt
	err          error
	notSubmitted bool
}
type runtimeRecoveryPage struct {
	authority                               *remote.RuntimeRecoveryAuthority
	accountID, originalTask, action         string
	manualAccount, manualKey                string
	manualReceipt                           *remote.RuntimeRecoveryReceipt
	plan                                    *remote.RuntimeRecoveryPlan
	records                                 []runtimeRecoveryRecord
	forget                                  *remote.RuntimeRecoveryForgetInput
	activeReference                         remote.RuntimeRecoveryRecord
	review, operation                       string
	loading, busy, submitted, pendingLoaded bool
	err                                     error
	cancel                                  context.CancelFunc
	done                                    chan struct{}
	results                                 chan runtimeRecoveryResult
	generation                              uint64
}
type runtimeRecoveryResult struct {
	kind         string
	generation   uint64
	authority    remote.RuntimeRecoveryAuthority
	plan         remote.RuntimeRecoveryPlan
	receipt      remote.RuntimeRecoveryReceipt
	reference    remote.RuntimeRecoveryRecord
	pending      []remote.RuntimeRecoveryRecord
	notSubmitted bool
	err          error
}

func (a *App) runtimeRecoveryBlocked() bool {
	return a.synchronizationBusy() || a.account.busy || a.accountImport.busy || a.accountDefault.busy || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy || a.closing || a.quitting.Load()
}
func (a *App) openRuntimeRecovery() {
	a.page = "runtime-recovery"
	if a.runtimeRecovery.action == "" {
		a.runtimeRecovery.action = "verify_target"
	}
}
func (a *App) startRuntimeRecovery(kind string, work func(context.Context, cliadapter.Client) runtimeRecoveryResult) {
	p := &a.runtimeRecovery
	if p.loading || p.busy || a.runtimeRecoveryBlocked() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results = cancel, make(chan struct{}), make(chan runtimeRecoveryResult, 1)
	p.busy = kind == "apply" || kind == "forget"
	p.loading = !p.busy
	p.operation = kind
	p.err = nil
	results, done, home, find, generation, reference := p.results, p.done, a.home, a.findExecutable, p.generation, p.activeReference
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		result := runtimeRecoveryResult{err: err, reference: reference}
		if err == nil {
			result = work(ctx, cliadapter.Client{Binary: binary, Home: home})
		}
		result.kind, result.generation = kind, generation
		results <- result
	}()
}
func (a *App) checkRuntimeAuthority() {
	p := &a.runtimeRecovery
	if p.loading || p.busy || a.runtimeRecoveryBlocked() {
		return
	}
	p.authority = nil
	p.plan = nil
	p.review = ""
	p.manualReceipt = nil
	a.startRuntimeRecovery("authority", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		var config cliadapter.ConfigContext
		if err := client.Call(ctx, "context", struct{}{}, &config); err != nil {
			return runtimeRecoveryResult{err: err}
		}
		if config.Home != client.Home || config.ServerURL == nil {
			return runtimeRecoveryResult{err: fault.CLIContextChanged}
		}
		value, err := remote.GetRuntimeRecoveryAuthority(ctx, client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: *config.ServerURL})
		return runtimeRecoveryResult{authority: value, err: err}
	})
}
func (a *App) clearRuntimeReview() { p := &a.runtimeRecovery; p.plan = nil; p.review = ""; p.err = nil }
func (a *App) inspectRuntimeRecovery() {
	p := &a.runtimeRecovery
	if p.authority == nil || !p.authority.Admin || p.submitted {
		return
	}
	input := remote.RuntimeRecoveryInspectInput{Expected: p.authority.Context, AccountID: p.accountID, OriginalTaskID: p.originalTask, Action: p.action}
	a.clearRuntimeReview()
	a.startRuntimeRecovery("inspect", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		pending, err := remote.ListRuntimeRecoveryPending(ctx, client)
		if err != nil {
			return runtimeRecoveryResult{err: err}
		}
		value, err := remote.InspectRuntimeRecovery(ctx, client, input)
		return runtimeRecoveryResult{plan: value, pending: pending, err: err}
	})
}
func (a *App) applyRuntimeRecovery() {
	p := &a.runtimeRecovery
	if p.review != "apply" || p.plan == nil || p.submitted || p.loading || p.busy || p.err != nil || a.runtimeRecoveryBlocked() || a.runtimeHasRetainedRequest() {
		return
	}
	key, err := remote.NewRuntimeRecoveryRequestID()
	if err != nil {
		p.err = err
		return
	}
	plan := *p.plan
	ref := remote.RuntimeRecoveryRecord{Context: plan.Context, AccountID: plan.AccountID, RequestID: key, Digest: plan.Digest, Original: plan.Original, Action: plan.Action, Phase: "unknown"}
	p.records = append(p.records, runtimeRecoveryRecord{reference: ref})
	p.activeReference = ref
	p.submitted = true
	a.startRuntimeRecovery("apply", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		value, err := remote.ApplyRuntimeRecovery(ctx, client, remote.RuntimeRecoveryApplyInput{Confirmed: true, ExpectedPlan: plan, RequestID: key})
		result := runtimeRecoveryResult{receipt: value, reference: ref, err: err}
		if runtimeRecoveryPreSubmitError(err) {
			pending, pendingErr := remote.ListRuntimeRecoveryPending(ctx, client)
			result.notSubmitted = pendingErr == nil && !slices.ContainsFunc(pending, func(item remote.RuntimeRecoveryRecord) bool { return item.RequestID == key })
			if pendingErr == nil {
				result.pending = pending
			}
		}
		return result
	})
}
func (a *App) pollRuntimeRecovery(c *ui.Context) {
	p := &a.runtimeRecovery
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		p.loading, p.busy = false, false
		p.review = ""
		if result.kind == "apply" || result.kind == "status" {
			a.updateRuntimeRecord(result)
			if result.pending != nil {
				a.mergeRuntimePending(result.pending)
			}
			if result.kind == "apply" {
				a.invalidateRemoteAccountList()
			}
			return
		}
		if result.kind == "forget" {
			if result.err == nil {
				p.records = slices.DeleteFunc(p.records, func(r runtimeRecoveryRecord) bool { return sameRuntimeRecord(r.reference, result.reference) })
				p.forget = nil
			} else {
				p.err = result.err
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
		case "authority":
			p.authority = &result.authority
		case "inspect":
			p.plan = &result.plan
			a.mergeRuntimePending(result.pending)
		case "pending":
			a.mergeRuntimePending(result.pending)
		case "manual":
			p.manualReceipt = &result.receipt
		}
	default:
		c.After(50 * time.Millisecond)
	}
}
func (a *App) finishRuntimeReview() {
	p := &a.runtimeRecovery
	if p.loading || p.busy {
		return
	}
	records := slices.DeleteFunc(p.records, func(r runtimeRecoveryRecord) bool { return r.notSubmitted })
	*p = runtimeRecoveryPage{records: records, action: "verify_target"}
}
func (a *App) invalidateRuntimeRecovery() {
	p := &a.runtimeRecovery
	p.generation++
	p.authority = nil
	p.manualReceipt = nil
	p.review = ""
	if !p.submitted {
		p.plan = nil
	}
	p.err = fault.CLIContextChanged
	if p.loading && p.operation != "status" && p.operation != "pending" {
		p.cancel()
	}
}
