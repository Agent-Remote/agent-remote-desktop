package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type mutagenEnvironmentRecord struct {
	plan    remote.MutagenEnvironmentPlan
	receipt *remote.MutagenEnvironmentReceipt
	err     error
}
type mutagenEnvironmentPage struct {
	sessionPages             map[string]int
	plan                     *remote.MutagenEnvironmentPlan
	records                  []mutagenEnvironmentRecord
	err                      error
	loading, busy, submitted bool
	generation               uint64
	cancel                   context.CancelFunc
	done                     chan struct{}
	results                  chan mutagenEnvironmentResult
}
type mutagenEnvironmentResult struct {
	generation uint64
	plan       remote.MutagenEnvironmentPlan
	receipt    remote.MutagenEnvironmentReceipt
	err        error
}

func (p *mutagenEnvironmentPage) hasOperation() bool {
	return p.busy || p.submitted || len(p.records) > 0
}
func (a *App) synchronizationBusy() bool { return a.sync.busy || a.mutagenEnvironment.busy }
func (a *App) openMutagenEnvironment()   { a.page = "mutagen-environment" }
func (a *App) mutagenEnvironmentBlocked() bool {
	return a.sync.busy || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.closing || a.quitting.Load()
}
func (a *App) previewMutagenEnvironment() {
	p := &a.mutagenEnvironment
	if p.loading || p.busy || a.mutagenEnvironmentBlocked() {
		return
	}
	p.plan, p.err = nil, nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.loading = cancel, make(chan struct{}), make(chan mutagenEnvironmentResult, 1), true
	results, done, generation, home, find := p.results, p.done, p.generation, a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		result := mutagenEnvironmentResult{generation: generation}
		binary, err := find("agent-remote")
		if err == nil {
			result.plan, err = remote.PreviewMutagenEnvironment(ctx, cliadapter.Client{Binary: binary, Home: home})
		}
		result.err = err
		results <- result
	}()
}
func (a *App) applyMutagenEnvironment() {
	p := &a.mutagenEnvironment
	if p.plan == nil || !p.plan.Repairable || p.loading || p.busy || p.submitted || a.mutagenEnvironmentBlocked() {
		return
	}
	input := remote.MutagenEnvironmentApplyInput{Confirmed: true, ExpectedHome: a.home, ExpectedPlan: *p.plan}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.busy, p.submitted = cancel, make(chan struct{}), make(chan mutagenEnvironmentResult, 1), true, true
	p.err = nil
	// A global restart invalidates the current project observation, but retains
	// every earlier project/recovery receipt and never dispatches a project action.
	a.invalidateSyncContext()
	a.sync.queryErr = &cliadapter.OperationError{Code: "sync_mutagen_environment_changed"}
	results, done, home, find := p.results, p.done, a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		result := mutagenEnvironmentResult{plan: input.ExpectedPlan}
		binary, err := find("agent-remote")
		if err == nil {
			result.receipt, err = remote.ApplyMutagenEnvironment(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		result.err = err
		results <- result
	}()
}
func (a *App) pollMutagenEnvironment(c *ui.Context) {
	p := &a.mutagenEnvironment
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		applying := p.busy
		p.loading, p.busy = false, false
		if applying {
			record := mutagenEnvironmentRecord{plan: result.plan, err: result.err}
			if result.err == nil {
				record.receipt = &result.receipt
			}
			p.records = append(p.records, record)
			p.plan, p.err = nil, nil
			a.invalidateCapabilities()
		} else if result.generation == p.generation {
			p.err = result.err
			if result.err == nil {
				p.plan = &result.plan
				p.submitted = false
			}
		}
	default:
		c.After(50 * time.Millisecond)
	}
}
func (a *App) cancelMutagenEnvironmentPreview() {
	p := &a.mutagenEnvironment
	if p.loading {
		p.generation++
		p.cancel()
		p.plan, p.err = nil, context.Canceled
	}
}
func (a *App) finishMutagenEnvironment() {
	if a.mutagenEnvironment.busy || a.mutagenEnvironment.loading {
		return
	}
	a.mutagenEnvironment = mutagenEnvironmentPage{}
	a.page = ""
}
