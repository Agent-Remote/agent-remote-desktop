package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type logoutPage struct {
	expected     remote.LoginContext
	revoke, busy bool
	err          error
	receipt      *remote.LogoutReceipt
	cancel       context.CancelFunc
	done         chan struct{}
	results      chan logoutResult
}

type logoutResult struct {
	receipt remote.LogoutReceipt
	err     error
}

func (page logoutPage) hasOperation() bool {
	return page.busy || page.err != nil || page.receipt != nil
}

func (a *App) openLogout() {
	if a.logout.hasOperation() {
		a.page = "logout"
		return
	}
	if a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.wireguard.busy || a.setup.busy || a.login.busy || a.remoteLifecycleBusy() || !a.remoteWorkflowAvailable("logout") {
		return
	}
	config := a.remote.snapshot.Config
	a.logout = logoutPage{expected: remote.LoginContext{ServerURL: config.ServerURL, DeviceID: config.DeviceID}, revoke: true}
	a.page = "logout"
}

func (a *App) submitLogout() {
	if a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.wireguard.busy || a.setup.busy || a.logout.busy || a.login.busy || a.remoteLifecycleBusy() || a.closing || a.logout.receipt != nil || a.logout.err != nil || !a.remoteWorkflowAvailable("logout") {
		return
	}
	input := remote.LogoutInput{Confirmed: true, Revoke: a.logout.revoke, Expected: a.logout.expected}
	ctx, cancel := context.WithCancel(context.Background())
	a.logout.cancel, a.logout.busy = cancel, true
	a.logout.results, a.logout.done = make(chan logoutResult, 1), make(chan struct{})
	results, done, home := a.logout.results, a.logout.done, a.home
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.LogoutReceipt
		if err == nil {
			receipt, err = remote.Logout(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- logoutResult{receipt, err}
	}()
}

func (a *App) pollLogout(c *ui.Context) {
	if !a.logout.busy {
		return
	}
	select {
	case result := <-a.logout.results:
		a.logout.busy, a.logout.err = false, result.err
		if result.err == nil {
			a.logout.receipt = &result.receipt
		}
		a.invalidateRemoteContext()
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) leaveLogout() {
	if a.logout.busy {
		return
	}
	a.logout = logoutPage{}
	a.openRemote()
}
