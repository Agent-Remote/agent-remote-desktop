package app

import (
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) loginView(c *ui.Context) {
	ui.Column(c).Fill().Padding(24).Gap(16).Children(func() {
		ui.Text(c, locales.SignInRemote()).FontSize(24).Bold()
		ui.Text(c, a.home).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Scroll(c).Grow(1).FillWidth().Gap(14).Children(func() {
			if a.login.busy {
				if a.login.approval != nil {
					a.loginApprovalView(c)
				} else {
					ui.Text(c, locales.LoginInProgress())
				}
				ui.Text(c, locales.LoginCancelHelp()).TextColor(c.Theme().TextMuted)
				ui.Button(c, locales.StopLoginWaiting()).OnClick(a.login.cancel)
				return
			}
			if a.login.err != nil {
				ui.Text(c, presentation.Error(a.login.err)).TextColor(c.Theme().Danger)
				ui.Button(c, locales.ReturnRemoteManagement()).OnClick(a.leaveLogin)
				return
			}
			if a.login.receipt != nil {
				ui.Text(c, locales.LoginCompleted()).FontSize(20).Bold()
				ui.Text(c, a.login.receipt.ServerURL)
				if a.login.receipt.DeviceID != nil {
					ui.Text(c, *a.login.receipt.DeviceID)
				} else {
					ui.Text(c, locales.LoginWithoutDevice())
				}
				ui.Text(c, locales.LoginSharedCredentialHelp()).TextColor(c.Theme().TextMuted)
				ui.PrimaryButton(c, locales.ReturnRemoteManagement()).OnClick(a.leaveLogin)
				return
			}
			if a.login.review {
				a.loginReview(c)
				return
			}
			ui.Row(c).Gap(10).Wrap().Children(func() {
				if a.remoteWorkflowAvailable("login.password") {
					ui.Button(c, locales.LoginWithPassword()).Disabled(!a.login.browser).OnClick(func() { a.login.browser = false })
				}
				if a.remoteWorkflowAvailable("login.device_code") {
					ui.Button(c, locales.LoginWithBrowser()).Disabled(a.login.browser).OnClick(func() { a.login.browser = true; a.login.password, a.login.totp = "", "" })
				}
			})
			ui.TextInput(c, &a.login.server).Label(locales.LoginServer()).Placeholder(locales.LoginServer()).AutoFocus()
			if !a.login.browser {
				ui.TextInput(c, &a.login.username).Label(locales.LoginUsername()).Placeholder(locales.LoginUsername())
				ui.TextInput(c, &a.login.password).Label(locales.LoginPassword()).Placeholder(locales.LoginPassword()).Password()
				ui.TextInput(c, &a.login.totp).Label(locales.LoginTOTP()).Placeholder(locales.LoginTOTP()).Password()
			} else {
				ui.Text(c, locales.LoginBrowserHelp()).TextColor(c.Theme().TextMuted)
			}
			ui.Checkbox(c, &a.login.skipDevice, locales.LoginUserOnly())
			if !a.login.skipDevice {
				ui.TextInput(c, &a.login.device).Label(locales.LoginDeviceName()).Placeholder(locales.LoginDeviceName())
				ui.Row(c).Gap(10).Children(func() {
					ui.TextInput(c, &a.login.sshKey).Label(locales.LoginSSHKey()).Placeholder(locales.LoginSSHKey()).Grow(1)
					ui.Button(c, locales.Browse()).OnClick(a.selectLoginSSHKey)
				})
				ui.Text(c, locales.LoginDefaultKeyHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
				ui.TextInput(c, &a.login.wireguardKey).Label(locales.LoginWireGuardKey()).Placeholder(locales.LoginWireGuardKey())
			}
			ui.Text(c, locales.LoginSharedCredentialHelp()).TextColor(c.Theme().TextMuted)
		})
		if !a.login.busy && a.login.err == nil && a.login.receipt == nil {
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).OnClick(a.leaveLogin)
				if a.login.review {
					ui.Button(c, locales.EditLogin()).OnClick(func() { a.login.review = false })
					ui.PrimaryButton(c, locales.ConfirmLogin()).Disabled(!a.remoteWorkflowAvailable(a.login.method())).OnClick(a.submitLogin)
				} else {
					ui.PrimaryButton(c, locales.ReviewLogin()).Disabled(!a.remoteWorkflowAvailable(a.login.method()) || strings.TrimSpace(a.login.server) == "" || (!a.login.browser && (strings.TrimSpace(a.login.username) == "" || a.login.password == ""))).OnClick(func() { a.login.review = true })
				}
			})
		}
	})
}

func (a *App) loginReview(c *ui.Context) {
	ui.Text(c, locales.LoginReviewHelp()).TextColor(c.Theme().TextMuted)
	ui.Text(c, a.login.server).Bold()
	if a.login.browser {
		ui.Text(c, locales.LoginWithBrowser())
	} else {
		ui.Text(c, a.login.username)
	}
	if a.login.expected.ServerURL != nil {
		ui.Text(c, locales.LoginPreviousServer(*a.login.expected.ServerURL))
	}
	if a.login.skipDevice {
		ui.Text(c, locales.LoginWithoutDevice())
		return
	}
	ui.Text(c, locales.LoginDeviceRegistrationHelp())
	if a.login.expected.DeviceID != nil && a.login.expected.ServerURL != nil && strings.TrimRight(a.login.server, "/") == strings.TrimRight(*a.login.expected.ServerURL, "/") {
		ui.Text(c, *a.login.expected.DeviceID)
	}
	if a.login.device != "" {
		ui.Text(c, a.login.device)
	}
	if a.login.sshKey != "" {
		ui.Text(c, a.login.sshKey)
	} else {
		ui.Text(c, locales.LoginDefaultKeyHelp())
	}
	if a.login.wireguardKey != "" {
		ui.Text(c, a.login.wireguardKey)
	}
}

func (a *App) loginApprovalView(c *ui.Context) {
	approval := a.login.approval
	remaining := int(time.Until(a.login.approvalDeadline).Seconds())
	ui.Text(c, locales.LoginAwaitingApproval()).FontSize(20).Bold()
	ui.Text(c, approval.VerificationURL)
	ui.Text(c, approval.UserCode).FontSize(26).Bold()
	if remaining > 0 {
		ui.Text(c, locales.LoginCodeExpires(remaining)).TextColor(c.Theme().TextMuted)
	} else {
		ui.Text(c, locales.LoginCodeElapsed()).TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Gap(10).Wrap().Children(func() {
		ui.Button(c, locales.LoginCopyCode()).Disabled(remaining <= 0).OnClick(func() { c.WriteClipboard(approval.UserCode) })
		ui.PrimaryButton(c, locales.LoginOpenBrowser()).Disabled(remaining <= 0).OnClick(func() {
			c.OpenURLThen(approval.VerificationURL, func(err error) { a.login.browserOpenFailed = err != nil })
		})
	})
	if a.login.browserOpenFailed {
		ui.Text(c, locales.LoginBrowserOpenFailed()).TextColor(c.Theme().Danger)
	}
}
