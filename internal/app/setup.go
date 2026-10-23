package app

import (
	"context"
	"errors"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type setupPage struct {
	checking, ready, unsupported, busy, submitted bool
	err                                           error
	receipt                                       *remote.LocalSetupReceipt
	cancel                                        context.CancelFunc
	done                                          chan struct{}
	results                                       chan setupResult
}

type setupResult struct {
	ready, unsupported bool
	receipt            remote.LocalSetupReceipt
	err                error
}

func (page setupPage) hasOperation() bool { return page.submitted }

func (a *App) openLocalSetup() {
	a.page = "setup"
	if a.setup.checking || a.setup.hasOperation() || a.closing || a.quitting.Load() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.setup = setupPage{checking: true, cancel: cancel, done: make(chan struct{}), results: make(chan setupResult, 1)}
	results, done, home, find := a.setup.results, a.setup.done, a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var capabilities cliadapter.Capabilities
		if err == nil {
			err = (cliadapter.Client{Binary: binary, Home: home}).Call(ctx, "capabilities", struct{}{}, &capabilities)
		}
		unsupported := setupUnsupported(err)
		if err == nil {
			err = capabilities.Validate()
		}
		if err == nil {
			unsupported = !capabilities.MatchesHostPlatform() || !capabilities.SupportsEffect("setup.local", "write")
		}
		results <- setupResult{ready: err == nil && !unsupported, unsupported: unsupported, err: err}
	}()
}

func (a *App) submitLocalSetup() {
	if a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.wireguard.busy || !a.setup.ready || a.setup.checking || a.setup.hasOperation() || a.login.busy || a.logout.busy || a.closing || a.quitting.Load() {
		return
	}
	input := remote.LocalSetupInput{Confirmed: true, ExpectedHome: a.home}
	ctx, cancel := context.WithCancel(context.Background())
	a.setup.cancel, a.setup.busy, a.setup.submitted = cancel, true, true
	a.setup.results, a.setup.done = make(chan setupResult, 1), make(chan struct{})
	results, done, home, find := a.setup.results, a.setup.done, a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.LocalSetupReceipt
		if err == nil {
			receipt, err = remote.SetupLocal(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- setupResult{receipt: receipt, err: err}
	}()
}

func (a *App) pollLocalSetup(c *ui.Context) {
	if !a.setup.checking && !a.setup.busy {
		return
	}
	select {
	case result := <-a.setup.results:
		wasMutation := a.setup.busy
		a.setup.busy, a.setup.checking, a.setup.err = false, false, result.err
		a.setup.unsupported = setupUnsupported(result.err)
		if !wasMutation {
			a.setup.ready, a.setup.unsupported = result.ready, result.unsupported
			return
		}
		if result.err == nil {
			a.setup.receipt = &result.receipt
		}
		a.invalidateDiagnostics()
		a.invalidateCapabilities()
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) leaveLocalSetup() {
	if a.setup.busy {
		return
	}
	// A preflight cancellation is read-only; retain its worker until it exits.
	if a.setup.checking {
		a.setup.cancel()
		a.openDiagnostics()
		return
	}
	a.setup = setupPage{}
	a.openDiagnostics()
}

func setupUnsupported(err error) bool {
	var operation *cliadapter.OperationError
	return errors.As(err, &operation) && (operation.Code == "unsupported_method" || operation.Code == "unsupported_version")
}
