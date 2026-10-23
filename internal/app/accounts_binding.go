package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) accountCanConnectBinding() bool {
	p := &a.account
	if p.status == nil || p.loading || p.busy || !p.capabilities.SupportsTerminal("accounts.binding.attach") {
		return false
	}
	binding := p.status.Binding
	if binding.BindingSessionID == nil {
		return false
	}
	if _, err := session.BindingReference(p.status.Account.ID, *binding.BindingSessionID); err != nil {
		return false
	}
	return binding.BindingSessionID != nil && (binding.Status == "binding_session_starting" || binding.Status == "binding_waiting_user_login") && p.status.Account.Status != "disabled"
}
func (a *App) accountBindingConnection(c *ui.Context) {
	if !a.accountCanConnectBinding() || a.account.review {
		return
	}
	ui.Button(c, locales.AccountBindingConnect()).Label(locales.AccountBindingConnectDetailsLabel()).Disabled(a.project() == nil || a.accountBlocked() || a.starting).OnClick(a.reviewAccountBinding)
	if a.project() == nil {
		ui.Text(c, locales.RemoteProjectRequired()).FontSize(12)
	}
}
func (a *App) reviewAccountBinding() {
	if !a.accountCanConnectBinding() || a.project() == nil || a.accountBlocked() || a.starting {
		return
	}
	p := &a.account
	p.bindingID = *p.status.Binding.BindingSessionID
	p.bindingProject, p.bindingProjectID = a.project().Path, a.project().ID
	p.bindingReview = true
}
func (a *App) accountBindingReviewValid() bool {
	p := &a.account
	return p.bindingReview && a.accountCanConnectBinding() && !a.accountBlocked() && !a.starting && a.project() != nil && a.project().ID == p.bindingProjectID && a.project().Path == p.bindingProject && *p.status.Binding.BindingSessionID == p.bindingID
}
func (a *App) accountBindingReviewView(c *ui.Context) {
	p := &a.account
	ui.Text(c, locales.AccountExactID(p.accountID)).Bold()
	ui.Text(c, locales.AccountBindingSession(p.bindingID))
	ui.Text(c, p.bindingProject)
	ui.Text(c, locales.AccountBindingConnectHelp()).TextColor(c.Theme().TextMuted)
}
func (a *App) confirmAccountBinding() {
	if !a.accountBindingReviewValid() {
		return
	}
	p := &a.account
	p.bindingReview = false
	a.launchAccountBinding(p.accountID, p.bindingID, p.expected)
}
