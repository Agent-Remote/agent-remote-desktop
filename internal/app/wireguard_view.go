package app

import (
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) wireguardView(c *ui.Context) {
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Text(c, locales.WireGuardConfigTitle()).FontSize(24).Bold()
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			if receipt := a.wireguard.receipt; receipt != nil {
				if receipt.Complete {
					ui.Text(c, locales.WireGuardConfigComplete()).Bold()
				} else {
					ui.Text(c, locales.WireGuardConfigIncomplete()).TextColor(c.Theme().Danger)
				}
				ui.Text(c, locales.WireGuardKeyState(receipt.KeyState))
				ui.Text(c, locales.WireGuardEnrollment(receipt.Enrollment))
				ui.Text(c, locales.WireGuardPublication(receipt.Publication))
				if receipt.Code != "none" {
					ui.Text(c, locales.WireGuardResultCode(receipt.Code)).TextColor(c.Theme().TextMuted)
				}
				if receipt.PeerCount != nil {
					ui.Text(c, locales.WireGuardPeerCount(int(*receipt.PeerCount)))
				}
				a.wireguardScope(c)
				return
			}
			if a.wireguard.plan != nil {
				a.wireguardScope(c)
			} else {
				ui.Text(c, a.home).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			if a.wireguard.planning {
				ui.Text(c, locales.WireGuardPlanning())
				return
			}
			if a.wireguard.busy {
				ui.Text(c, locales.WireGuardConfigBusy()).Bold()
				ui.Text(c, locales.WireGuardCancelHelp()).TextColor(c.Theme().TextMuted)
				return
			}
			if a.wireguard.err != nil {
				message := presentation.Error(a.wireguard.err)
				if wireguardUnsupported(a.wireguard.err) {
					message = locales.WireGuardUnsupported()
				}
				ui.Text(c, message).TextColor(c.Theme().Danger)
				return
			}

			if a.wireguard.plan != nil {
				ui.Text(c, locales.WireGuardReviewHelp()).TextColor(c.Theme().TextMuted)
				return
			}
			ui.Text(c, locales.WireGuardConfigHelp())
			ui.Row(c).Gap(12).Wrap().Children(func() {
				ui.Radio(c, &a.wireguard.outputMode, "", locales.WireGuardDefaultOutput())
				ui.Radio(c, &a.wireguard.outputMode, "custom", locales.WireGuardCustomOutput())
			})
			if a.wireguard.outputMode == "custom" {
				ui.Row(c).Gap(10).Children(func() {
					ui.TextInput(c, &a.wireguard.output).Label(locales.WireGuardOutputPath()).Placeholder(locales.WireGuardOutputPath()).Grow(1)
					ui.Button(c, locales.Browse()).OnClick(a.selectWireGuardOutput)
				})
				ui.Text(c, locales.WireGuardAbsolutePathHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			if a.wireguard.busy {
				ui.Button(c, locales.StopWireGuardWaiting()).OnClick(a.wireguard.cancel)
				return
			}
			if a.wireguard.hasOperation() {
				ui.Button(c, locales.ReturnDiagnostics()).OnClick(a.leaveWireGuard)
				return
			}
			ui.Button(c, locales.Cancel()).OnClick(a.leaveWireGuard)
			if a.wireguard.planning {
				return
			}
			if a.wireguard.err != nil {
				ui.Button(c, locales.EditWireGuardConfig()).OnClick(a.editWireGuard)
			} else if a.wireguard.plan != nil {
				ui.Button(c, locales.EditWireGuardConfig()).OnClick(a.editWireGuard)
				ui.PrimaryButton(c, locales.ConfirmWireGuardConfig()).Disabled(a.wireguardBlocked()).OnClick(a.submitWireGuard)
			} else {
				invalid := a.wireguard.outputMode == "custom" && !filepath.IsAbs(a.wireguard.output)
				ui.PrimaryButton(c, locales.ReviewWireGuardConfig()).Disabled(invalid || a.wireguardBlocked()).OnClick(a.reviewWireGuard)
			}
		})
	})
}

func (a *App) wireguardScope(c *ui.Context) {
	plan := a.wireguard.plan
	for _, field := range []struct{ label, value string }{
		{locales.WireGuardHome(), plan.Home},
		{locales.WireGuardServer(), plan.Context.ServerURL},
		{locales.WireGuardDevice(), plan.Context.DeviceID},
		{locales.WireGuardOutputPath(), plan.Output.Path},
	} {
		ui.Column(c).FillWidth().Gap(3).Children(func() {
			ui.Text(c, field.label).FontSize(11).TextColor(c.Theme().TextMuted)
			ui.Text(c, field.value).FontSize(13)
		})
	}
	ui.Text(c, locales.WireGuardOutputAction(plan.Output.Action)).Bold()
}
