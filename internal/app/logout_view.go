package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) logoutView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(16).Children(func() {
		ui.Scroll(c).Grow(1).FillWidth().Gap(14).Children(func() {
			if a.logout.expected.ServerURL != nil {
				ui.Text(c, *a.logout.expected.ServerURL).Bold()
			}
			if a.logout.expected.DeviceID != nil {
				ui.Text(c, *a.logout.expected.DeviceID)
			}
			if a.logout.busy {
				ui.Text(c, locales.LogoutInProgress())
				ui.Text(c, locales.LogoutCancelHelp()).TextColor(c.Theme().TextMuted)
				return
			}
			if a.logout.err != nil {
				a.errorText(c, a.logout.err, presentation.Error(a.logout.err))
				return
			}
			if receipt := a.logout.receipt; receipt != nil {
				if *receipt.LocalComplete {
					ui.Text(c, locales.LogoutLocalComplete()).Bold()
				} else {
					ui.Text(c, locales.LogoutLocalIncomplete()).TextColor(c.Theme().Danger)
				}
				for _, item := range []struct {
					label    string
					complete bool
				}{
					{locales.LogoutUserCredential(), *receipt.User.LocalRemoved},
					{locales.LogoutDeviceCredential(), *receipt.Device.LocalRemoved && *receipt.BrokerRemoved},
					{locales.LogoutConfiguration(), *receipt.ConfigCleared && *receipt.RefreshRemoved},
				} {
					status := locales.LogoutCleanupUnconfirmed()
					if item.complete {
						status = locales.LogoutCleanupConfirmed()
					}
					ui.Text(c, locales.LogoutResultItem(item.label, status))
				}
				ui.Text(c, locales.LogoutResultItem(locales.LogoutUserRevocation(), logoutRevocation(receipt.User.Remote)))
				ui.Text(c, locales.LogoutResultItem(locales.LogoutDeviceRevocation(), logoutRevocation(receipt.Device.Remote)))
				ui.Text(c, locales.LogoutSessionsHelp()).TextColor(c.Theme().TextMuted)
				return
			}
			ui.Text(c, locales.LogoutReviewHelp())
			ui.Checkbox(c, &a.logout.revoke, locales.LogoutRevokeRemote())
			if !a.logout.revoke {
				ui.Text(c, locales.LogoutLocalOnlyHelp()).TextColor(c.Theme().TextMuted)
			}
			ui.Text(c, locales.LogoutSessionsHelp()).TextColor(c.Theme().TextMuted)
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			if a.logout.busy {
				ui.Button(c, locales.StopLogoutWaiting()).OnClick(a.logout.cancel)
				return
			}
			if a.logout.err != nil || a.logout.receipt != nil {
				ui.Button(c, locales.ReturnRemoteManagement()).OnClick(a.leaveLogout)
				return
			}
			ui.Button(c, locales.Cancel()).OnClick(func() { a.page = "remote" })
			ui.PrimaryButton(c, locales.ConfirmLogout()).Disabled(!a.remoteWorkflowAvailable("logout")).OnClick(a.submitLogout)
		})
	})
}

func logoutRevocation(state string) string {
	switch state {
	case "confirmed":
		return locales.LogoutRevocationConfirmed()
	case "not_requested":
		return locales.LogoutRevocationSkipped()
	case "not_present":
		return locales.LogoutCredentialAbsent()
	default:
		return locales.LogoutRevocationUnconfirmed()
	}
}
