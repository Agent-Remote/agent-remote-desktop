package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) accountView(c *ui.Context) {
	p := &a.account
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(12).Children(func() {
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			a.accountContextView(c)
			if p.bindingReview {
				a.accountBindingReviewView(c)
				return
			}
			if p.review {
				a.accountReviewView(c)
			}
			if p.busy {
				ui.Text(c, locales.AccountWorking())
				ui.Text(c, locales.AccountCancelHelp()).TextColor(c.Theme().TextMuted)
				return
			}
			if p.review && !p.submitted {
				return
			}
			if p.err != nil {
				a.errorText(c, p.err, presentation.Error(p.err))
			}
			if p.receipt != nil {
				a.accountReceiptView(c)
			}
			if p.loading {
				ui.Text(c, locales.LoadingRemote())
				return
			}
			if p.queryErr != nil {
				a.errorText(c, p.queryErr, presentation.Error(p.queryErr))
			}
			if p.status != nil {
				a.accountDetailsView(c, p.status.Account)
				a.accountBindingDetails(c, p.status.Binding)
				a.accountBindingConnection(c)
				return
			}
			if p.submitted || p.queryErr != nil || p.review {
				return
			}
			if p.action == "create" {
				a.accountCreateForm(c)
			}
		})
		a.accountFooter(c)
	})
}
func (a *App) accountContextView(c *ui.Context) {
	p := &a.account
	if p.expected.ServerURL != nil {
		ui.Text(c, *p.expected.ServerURL).FontSize(12)
	}
	if p.expected.DeviceID != nil {
		ui.Text(c, *p.expected.DeviceID).FontSize(12)
	}
}
func (a *App) accountReviewView(c *ui.Context) {
	p := &a.account
	if p.action == "create" {
		a.accountCreateReview(c)
	} else {
		ui.Text(c, locales.AccountExactID(p.accountID)).Bold()
		if p.status != nil {
			ui.Text(c, p.status.Account.Name)
		}
	}
	ui.Text(c, locales.AccountActionHelp(p.action)).TextColor(c.Theme().TextMuted)
	if p.action == "bind" && p.status != nil && p.status.Account.Status == "disabled" {
		ui.Text(c, locales.AccountRebindDisabledHelp()).TextColor(c.Theme().Text)
	}
}
func (a *App) accountReceiptView(c *ui.Context) {
	p := &a.account
	ui.Text(c, locales.AccountAccepted(p.action)).Bold()
	if receipt := p.receipt; receipt.Account != nil {
		a.accountDetailsView(c, *receipt.Account)
	} else if receipt.Binding != nil {
		a.accountBindingDetails(c, *receipt.Binding)
	}
	ui.Text(c, locales.AccountManualRefreshHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
}
func (a *App) accountFooter(c *ui.Context) {
	p := &a.account
	ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
		switch {
		case p.bindingReview:
			ui.Button(c, locales.Cancel()).OnClick(func() { p.bindingReview = false })
			ui.PrimaryButton(c, locales.AccountBindingConnect()).Label(locales.AccountBindingConfirmLabel()).Disabled(!a.accountBindingReviewValid()).OnClick(a.confirmAccountBinding)
		case p.busy:
			ui.Button(c, locales.AccountStopWaiting()).OnClick(p.cancel)
		case p.loading:
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(p.cancel)
		case p.submitted:
			if p.accountID != "" {
				ui.Button(c, locales.AccountRefreshStatus()).Disabled(a.accountBlocked()).OnClick(a.refreshAccountStatus)
			}
			ui.Button(c, locales.AccountDone()).OnClick(a.finishAccountOperation)
		case p.review:
			ui.Button(c, locales.Cancel()).OnClick(func() {
				p.review = false
				p.action = "status"
				if p.accountID == "" {
					p.action = "create"
				}
			})
			ui.PrimaryButton(c, locales.AccountConfirm()).Disabled(!a.accountOperationAvailable("accounts." + p.action)).OnClick(a.submitAccount)
		case p.action == "create" && p.queryErr == nil:
			ui.Button(c, locales.Cancel()).OnClick(a.finishAccountOperation)
			ui.PrimaryButton(c, locales.AccountReviewCreate()).Disabled(!a.accountOperationAvailable("accounts.create")).OnClick(a.reviewAccountCreate)
		default:
			ui.Button(c, locales.AccountDone()).OnClick(a.finishAccountOperation)
			ui.Button(c, locales.AccountRefreshStatus()).Disabled(a.accountBlocked()).OnClick(a.refreshAccountStatus)
		}
	})
}
