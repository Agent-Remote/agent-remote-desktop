package app

import (
	"context"
	"errors"
	"path/filepath"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type wireguardPage struct {
	outputMode, output        string
	planning, busy, submitted bool
	plan                      *remote.WireGuardConfigPlan
	receipt                   *remote.WireGuardConfigReceipt
	err                       error
	generation                uint64
	cancel                    context.CancelFunc
	done                      chan struct{}
	results                   chan wireguardResult
}

type wireguardResult struct {
	generation uint64
	plan       remote.WireGuardConfigPlan
	receipt    remote.WireGuardConfigReceipt
	err        error
}

func (page wireguardPage) hasOperation() bool { return page.submitted }

func (a *App) openWireGuard() { a.page = "wireguard" }

func (a *App) wireguardBlocked() bool {
	return a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.login.busy || a.logout.busy || a.setup.busy
}

func (a *App) selectWireGuardOutput() {
	if a.wireguard.planning || a.wireguard.hasOperation() {
		return
	}
	path, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: a.win, Title: locales.WireGuardOutputPath(), DefaultPath: a.wireguard.output})
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	if path != "" {
		a.wireguard.output = path
		a.wireguard.outputMode = "custom"
	}
}

func (a *App) reviewWireGuard() {
	if a.wireguard.planning || a.wireguard.hasOperation() || a.wireguardBlocked() || a.closing || a.quitting.Load() {
		return
	}
	input := remote.WireGuardPlanInput{ExpectedHome: a.home}
	if a.wireguard.outputMode == "custom" {
		if !filepath.IsAbs(a.wireguard.output) {
			return
		}
		output := a.wireguard.output
		input.Output = &output
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.wireguard.cancel, a.wireguard.planning = cancel, true
	a.wireguard.plan, a.wireguard.err = nil, nil
	a.wireguard.results, a.wireguard.done = make(chan wireguardResult, 1), make(chan struct{})
	results, done, home, find, generation := a.wireguard.results, a.wireguard.done, a.home, a.findExecutable, a.wireguard.generation
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var plan remote.WireGuardConfigPlan
		if err == nil {
			plan, err = remote.PlanWireGuardConfig(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- wireguardResult{generation: generation, plan: plan, err: err}
	}()
}

func (a *App) submitWireGuard() {
	if a.wireguard.plan == nil || a.wireguard.planning || a.wireguard.hasOperation() || a.wireguardBlocked() || a.closing || a.quitting.Load() {
		return
	}
	input := remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: *a.wireguard.plan}
	a.invalidateNetwork()
	ctx, cancel := context.WithCancel(context.Background())
	a.wireguard.cancel, a.wireguard.busy, a.wireguard.submitted = cancel, true, true
	a.wireguard.results, a.wireguard.done = make(chan wireguardResult, 1), make(chan struct{})
	results, done, home, find := a.wireguard.results, a.wireguard.done, a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.WireGuardConfigReceipt
		if err == nil {
			receipt, err = remote.ApplyWireGuardConfig(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- wireguardResult{receipt: receipt, err: err}
	}()
}

func (a *App) pollWireGuard(c *ui.Context) {
	if !a.wireguard.planning && !a.wireguard.busy {
		return
	}
	select {
	case result := <-a.wireguard.results:
		applying := a.wireguard.busy
		a.wireguard.planning, a.wireguard.busy = false, false
		if !applying && result.generation != a.wireguard.generation {
			return
		}
		a.wireguard.err = result.err
		if result.err == nil {
			if applying {
				a.wireguard.receipt = &result.receipt
			} else {
				a.wireguard.plan = &result.plan
			}
		}
		if applying {
			a.invalidateNetwork()
			a.invalidateDiagnostics()
			a.invalidateCapabilities()
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) editWireGuard() {
	if a.wireguard.planning || a.wireguard.hasOperation() {
		return
	}
	a.wireguard.plan, a.wireguard.err = nil, nil
}

func (a *App) leaveWireGuard() {
	if a.wireguard.busy {
		return
	}
	if a.wireguard.planning {
		a.wireguard.generation++
		a.wireguard.cancel()
	} else {
		a.wireguard = wireguardPage{}
	}
	a.openDiagnostics()
}

// Authentication can change while a read-only plan is in flight. Keep the
// worker handles for cleanup, but never publish its obsolete confirmation.
func (a *App) invalidateWireGuardPlan() {
	if a.wireguard.hasOperation() || (a.wireguard.plan == nil && !a.wireguard.planning) {
		return
	}
	a.wireguard.generation++
	a.wireguard.plan = nil
	a.wireguard.err = fault.CLIContextChanged
	if a.wireguard.planning {
		a.wireguard.cancel()
	}
}

func wireguardUnsupported(err error) bool {
	var operation *cliadapter.OperationError
	return errors.As(err, &operation) && (operation.Code == "unsupported_method" || operation.Code == "unsupported_version")
}
