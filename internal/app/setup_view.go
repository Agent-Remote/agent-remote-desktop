package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) localSetupView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			if a.setup.checking {
				ui.Text(c, locales.LocalSetupChecking())
				return
			}
			if a.setup.busy {
				ui.Text(c, locales.LocalSetupBusy())
				ui.Text(c, locales.LocalSetupCancelHelp()).TextColor(c.Theme().TextMuted)
				return
			}
			if a.setup.unsupported {
				ui.Text(c, locales.LocalSetupUnsupported()).TextColor(c.Theme().Danger)
				return
			}
			if a.setup.err != nil {
				a.errorText(c, a.setup.err, presentation.Error(a.setup.err))
				return
			}
			if receipt := a.setup.receipt; receipt != nil {
				if receipt.Complete {
					ui.Text(c, locales.LocalSetupComplete()).Bold()
				} else {
					ui.Text(c, locales.LocalSetupIncomplete()).TextColor(c.Theme().Danger)
				}
				for _, step := range receipt.Steps {
					a.card(c, ui.Column(c.Key(step.ID)).FillWidth().Gap(4).Padding(10)).Children(func() {
						ui.Row(c).Gap(10).Wrap().Children(func() {
							ui.Text(c, locales.LocalSetupStep(step.ID)).Bold()
							ui.Text(c, locales.LocalSetupStatus(step.Status)).TextColor(c.Theme().TextMuted)
						})
						ui.Text(c, step.Path).FontSize(12).TextColor(c.Theme().TextMuted)
						if step.Code != "none" {
							ui.Text(c, locales.LocalSetupCode(step.Code)).FontSize(12)
						}
					})
				}
				return
			}
			ui.Text(c, locales.LocalSetupReviewHelp())
			ui.Text(c, locales.LocalSetupScope()).TextColor(c.Theme().TextMuted)
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			if a.setup.busy {
				ui.Button(c, locales.StopLocalSetupWaiting()).OnClick(a.setup.cancel)
				return
			}
			if a.setup.hasOperation() || a.setup.err != nil || a.setup.unsupported {
				ui.Button(c, locales.ReturnDiagnostics()).OnClick(a.leaveLocalSetup)
				return
			}
			ui.Button(c, locales.Cancel()).OnClick(a.leaveLocalSetup)
			ui.PrimaryButton(c, locales.ConfirmLocalSetup()).Disabled(a.wireguard.busy || !a.setup.ready || a.setup.checking || a.login.busy || a.logout.busy).OnClick(a.submitLocalSetup)
		})
	})
}
