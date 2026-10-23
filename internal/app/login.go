package app

import (
	"context"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type loginPage struct {
	server, username, password, totp string
	device, sshKey, wireguardKey     string
	skipDevice                       bool
	browser                          bool
	approval                         *remote.DeviceCodeApproval
	approvals                        chan loginApproval
	approvalDeadline                 time.Time
	browserOpenFailed                bool
	review, busy                     bool
	expected                         remote.LoginContext
	err                              error
	receipt                          *remote.LoginReceipt
	cancel                           context.CancelFunc
	done                             chan struct{}
	results                          chan loginResult
}
type loginApproval struct {
	event    remote.DeviceCodeApproval
	deadline time.Time
}

type loginResult struct {
	receipt remote.LoginReceipt
	err     error
}

func (a *App) selectLoginSSHKey() {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: a.win, Title: locales.LoginSSHKey()})
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	if len(paths) > 0 {
		a.login.sshKey = paths[0]
	}
}

func (page loginPage) hasOperation() bool { return page.busy || page.err != nil || page.receipt != nil }

func (a *App) openLogin() {
	if a.login.hasOperation() {
		a.page = "login"
		return
	}
	if a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.wireguard.busy || a.setup.busy || a.logout.busy || a.remoteLifecycleBusy() || (!a.remoteWorkflowAvailable("login.password") && !a.remoteWorkflowAvailable("login.device_code")) {
		return
	}
	config := a.remote.snapshot.Config
	a.login = loginPage{browser: !a.remote.snapshot.Capabilities.SupportsEffect("login.password", "write"), expected: remote.LoginContext{ServerURL: config.ServerURL, DeviceID: config.DeviceID}}
	if config.ServerURL != nil {
		a.login.server = *config.ServerURL
	}
	a.page = "login"
}

func (a *App) leaveLogin() {
	if a.login.busy {
		return
	}
	// This explicit dismissal acknowledges the receipt/error. Ordinary page
	// navigation preserves an operation so its result can still be inspected.
	a.login = loginPage{}
	a.openRemote()
}

func optionalLoginText(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func (page loginPage) method() string {
	if page.browser {
		return "login.device_code"
	}
	return "login.password"
}

func (a *App) submitLogin() {
	if a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.wireguard.busy || a.setup.busy || a.login.busy || a.logout.busy || a.remoteLifecycleBusy() || !a.login.review || a.closing || !a.remoteWorkflowAvailable(a.login.method()) {
		return
	}
	input := remote.PasswordLoginInput{
		ServerURL: strings.TrimSpace(a.login.server), Username: strings.TrimSpace(a.login.username), Password: a.login.password, TOTP: optionalLoginText(a.login.totp),
		DeviceName: optionalLoginText(a.login.device), SSHKey: optionalLoginText(a.login.sshKey), WireGuardKey: optionalLoginText(a.login.wireguardKey),
		SkipDevice: a.login.skipDevice, Confirmed: true, Expected: a.login.expected,
	}
	a.login.password, a.login.totp = "", ""
	a.login.busy = true
	a.login.err = nil
	ctx, cancel := context.WithCancel(context.Background())
	a.login.cancel = cancel
	a.login.done = make(chan struct{})
	a.login.results = make(chan loginResult, 1)
	a.login.approvals = make(chan loginApproval, 1)
	results, done, home := a.login.results, a.login.done, a.home
	approvals, browser := a.login.approvals, a.login.browser
	deviceInput := remote.DeviceCodeLoginInput{ServerURL: input.ServerURL, DeviceName: input.DeviceName, SSHKey: input.SSHKey, WireGuardKey: input.WireGuardKey, SkipDevice: input.SkipDevice, Confirmed: input.Confirmed, Expected: input.Expected}
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.LoginReceipt
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			if browser {
				receipt, err = remote.DeviceCodeLogin(ctx, client, deviceInput, func(event remote.DeviceCodeApproval) {
					approvals <- loginApproval{event: event, deadline: time.Now().Add(time.Duration(event.ExpiresIn) * time.Second)}
				})
			} else {
				receipt, err = remote.PasswordLogin(ctx, client, input)
			}
		}
		input.Password = ""
		input.TOTP = nil
		results <- loginResult{receipt, err}
	}()
}

func (a *App) pollLogin(c *ui.Context) {
	if a.page != "login" {
		a.login.password, a.login.totp = "", ""
	}
	if !a.login.busy {
		return
	}
	select {
	case approval := <-a.login.approvals:
		a.login.approval = &approval.event
		a.login.approvalDeadline = approval.deadline
	default:
	}
	select {
	case result := <-a.login.results:
		a.login.busy = false
		a.login.approval = nil
		a.login.review = false
		a.login.err = result.err
		a.invalidateRemoteContext()
		if result.err == nil {
			a.login.receipt = &result.receipt
		}
	default:
		c.After(50 * time.Millisecond)
	}
}
