package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type accountDefaultPage struct {
	tool                     string
	snapshot                 *remote.AccountDefaultOverview
	review                   *remote.AccountDefaultInput
	clear                    bool
	loading, busy, submitted bool
	generation               uint64
	err                      error
	receipt                  *remote.AccountDefaultReceipt
	cancel                   context.CancelFunc
	done                     chan struct{}
	results                  chan accountDefaultResult
}
type accountDefaultResult struct {
	generation uint64
	snapshot   remote.AccountDefaultOverview
	receipt    remote.AccountDefaultReceipt
	err        error
}

func (a *App) defaultBlocked() bool {
	return a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy || a.closing || a.quitting.Load()
}
func (a *App) openAccountDefault() {
	a.page = "account-default"
	if a.accountDefault.tool == "" {
		a.accountDefault.tool = "claude"
	}
	if a.accountDefault.snapshot == nil && !a.accountDefault.loading && !a.accountDefault.submitted && a.accountDefault.err == nil {
		a.loadAccountDefault()
	}
}
func (a *App) loadAccountDefault() {
	p := &a.accountDefault
	if p.loading || p.busy || a.defaultBlocked() {
		return
	}
	p.snapshot, p.review, p.receipt, p.err = nil, nil, nil, nil
	p.submitted = false
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.loading = cancel, make(chan struct{}), make(chan accountDefaultResult, 1), true
	results, done, home, tool, generation, find := p.results, p.done, a.home, p.tool, p.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var snapshot remote.AccountDefaultOverview
		if err == nil {
			snapshot, err = remote.LoadAccountDefault(ctx, cliadapter.Client{Binary: binary, Home: home}, tool)
		}
		results <- accountDefaultResult{generation: generation, snapshot: snapshot, err: err}
	}()
}
func (a *App) reviewAccountDefault(id *string) {
	p := &a.accountDefault
	if p.snapshot == nil || p.loading || p.submitted || a.defaultBlocked() {
		return
	}
	method := "accounts.default.clear"
	input := remote.AccountDefaultInput{Expected: p.snapshot.Default.Context, Previous: p.snapshot.Default.AccountID, Tool: p.snapshot.Default.Tool}
	if id != nil {
		method = "accounts.default.set"
		found := false
		for _, account := range p.snapshot.Accounts {
			if account.ID == *id && account.Tool == input.Tool {
				found = true
				break
			}
		}
		if !found || p.snapshot.AccountsError != nil {
			return
		}
		input.AccountID = *id
	}
	if !remote.AccountDefaultSupported(p.snapshot.Capabilities, method) {
		return
	}
	p.clear = id == nil
	p.review = &input
}
func (a *App) submitAccountDefault() {
	p := &a.accountDefault
	if p.review == nil || p.loading || p.submitted || a.defaultBlocked() {
		return
	}
	input := *p.review
	input.Confirmed = true
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.busy, p.submitted = cancel, make(chan struct{}), make(chan accountDefaultResult, 1), true, true
	results, done, home, clearDefault, find := p.results, p.done, a.home, p.clear, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.AccountDefaultReceipt
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			if clearDefault {
				receipt, err = remote.ClearAccountDefault(ctx, client, input)
			} else {
				receipt, err = remote.SetAccountDefault(ctx, client, input)
			}
		}
		results <- accountDefaultResult{receipt: receipt, err: err}
	}()
}
func (a *App) pollAccountDefault(c *ui.Context) {
	p := &a.accountDefault
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		mutation := p.busy
		p.loading, p.busy = false, false
		if !mutation && result.generation != p.generation {
			return
		}
		p.err = result.err
		if mutation {
			if result.err == nil {
				p.receipt = &result.receipt
			}
			a.invalidateCapabilities()
		} else if result.err == nil {
			p.snapshot = &result.snapshot
		}
	default:
		c.After(50 * time.Millisecond)
	}
}
func (a *App) invalidateAccountDefault() {
	p := &a.accountDefault
	if p.submitted {
		return
	}
	p.generation++
	p.snapshot, p.review = nil, nil
	p.err = fault.CLIContextChanged
	if p.loading {
		p.cancel()
	}
}
