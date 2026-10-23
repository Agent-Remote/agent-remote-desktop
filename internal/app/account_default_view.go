package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) accountDefaultView(c *ui.Context) {
	p := &a.accountDefault
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.AccountDefaultTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = "" })
		})
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			ui.Text(c, locales.AccountDefaultScope()).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, a.home).FontSize(12)
			if p.loading {
				ui.Text(c, locales.LoadingRemote())
				return
			}
			if p.review != nil {
				a.accountDefaultReview(c)
			}
			if p.busy {
				ui.Text(c, locales.AccountDefaultWaiting())
				ui.Text(c, locales.AccountDefaultCancelHelp())
				return
			}
			if p.err != nil {
				ui.Text(c, presentation.Error(p.err)).TextColor(c.Theme().Danger)
				return
			}
			if p.receipt != nil {
				label := locales.AccountDefaultSaved()
				if !p.receipt.Changed {
					label = locales.AccountDefaultUnchanged()
				}
				ui.Text(c, label).Bold()
				ui.Text(c, locales.AccountDefaultCurrent(defaultAccountText(p.receipt.AccountID)))
				return
			}
			if p.review != nil {
				ui.Text(c, locales.AccountDefaultConfirmHelp())
				return
			}
			ui.Text(c, locales.AccountDefaultTool()).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.TextInput(c, &p.tool).Label(locales.AccountDefaultTool()).Placeholder(locales.AccountDefaultTool()).OnChange(func() { p.snapshot = nil })
			if p.snapshot == nil {
				return
			}
			snapshot := p.snapshot
			ui.Text(c, locales.AccountDefaultCurrent(defaultAccountText(snapshot.Default.AccountID))).Bold()
			ui.Button(c, locales.AccountDefaultClear()).Disabled(a.defaultBlocked() || !remote.AccountDefaultSupported(snapshot.Capabilities, "accounts.default.clear")).OnClick(func() { a.reviewAccountDefault(nil) })
			if snapshot.Default.Context.ServerURL != nil {
				ui.Text(c, *snapshot.Default.Context.ServerURL).FontSize(12)
			}
			if snapshot.AccountsError != nil {
				ui.Text(c, presentation.Error(snapshot.AccountsError)).TextColor(c.Theme().Danger)
				return
			}
			if snapshot.Default.Context.ServerURL == nil || snapshot.Default.Context.DeviceID == nil {
				ui.Text(c, locales.AccountDefaultSignIn())
				return
			}
			count := 0
			for _, account := range snapshot.Accounts {
				if account.Tool != snapshot.Default.Tool {
					continue
				}
				count++
				ui.Column(c.Key(account.ID)).FillWidth().Padding(12).Gap(5).Background(c.Theme().Surface).Radius(8).Children(func() {
					ui.Text(c, account.Name).Bold()
					ui.Text(c, account.ID).FontSize(12)
					ui.Text(c, locales.RemoteAccountSummary(account.Tool, presentation.RemoteStatus(account.Status), account.Region)).FontSize(12)
					ui.Button(c, locales.AccountDefaultSelect()).Disabled(a.defaultBlocked() || !remote.AccountDefaultSupported(snapshot.Capabilities, "accounts.default.set")).OnClick(func() { id := account.ID; a.reviewAccountDefault(&id) })
				})
			}
			if count == 0 {
				ui.Text(c, locales.NoRemoteMatches())
			}
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			switch {
			case p.loading:
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(p.cancel)
			case p.busy:
				ui.Button(c, locales.AccountDefaultStopWaiting()).OnClick(p.cancel)
			case p.submitted || p.err != nil:
				ui.Button(c, locales.AccountDefaultRefresh()).Disabled(a.defaultBlocked()).OnClick(a.loadAccountDefault)
			case p.review != nil:
				ui.Button(c, locales.Cancel()).OnClick(func() { p.review = nil })
				ui.PrimaryButton(c, locales.AccountDefaultConfirm()).Disabled(a.defaultBlocked()).OnClick(a.submitAccountDefault)
			default:
				ui.Button(c, locales.AccountDefaultRefresh()).Disabled(a.defaultBlocked()).OnClick(a.loadAccountDefault)
			}
		})
	})
}
func (a *App) accountDefaultReview(c *ui.Context) {
	review := a.accountDefault.review
	ui.Text(c, locales.AccountDefaultToolValue(review.Tool)).Bold()
	ui.Text(c, locales.AccountDefaultPrevious(defaultAccountText(review.Previous)))
	next := locales.AccountDefaultNone()
	if !a.accountDefault.clear {
		next = review.AccountID
	}
	ui.Text(c, locales.AccountDefaultNext(next)).Bold()
	if review.Expected.ServerURL != nil {
		ui.Text(c, *review.Expected.ServerURL).FontSize(12)
	}
	if review.Expected.DeviceID != nil {
		ui.Text(c, *review.Expected.DeviceID).FontSize(12)
	}
}
func defaultAccountText(value *string) string {
	if value == nil {
		return locales.AccountDefaultNone()
	}
	return *value
}
