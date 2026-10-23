package app

import (
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) remoteView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(16).Children(func() {
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Button(c, locales.AccountImportRecoveryTitle()).OnClick(a.openAccountImportHistory)
			ui.Button(c, locales.AccountDefaultTitle()).OnClick(a.openAccountDefault)
			if a.remoteWorkflowAvailable(remote.CredentialsListMethod) {
				ui.Button(c, locales.CredentialsTitle()).OnClick(a.openCredentials)
			}
			ui.Button(c, locales.RefreshRemote()).Disabled(a.remote.busy).OnClick(a.refreshRemote)
		})
		if a.remote.loaded && a.remote.err == nil {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				if a.remoteWorkflowAvailable("login.password") || a.remoteWorkflowAvailable("login.device_code") {
					ui.Button(c, locales.SignInRemote()).Disabled(a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.wireguard.busy || a.setup.busy || a.login.busy || a.logout.busy || a.remoteLifecycleBusy() || a.remote.busy).OnClick(a.openLogin)
				}
				if a.remote.snapshot.Config.ServerURL != nil && a.remoteWorkflowAvailable("logout") {
					ui.Button(c, locales.SignOutRemote()).Disabled(a.synchronizationBusy() || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.wireguard.busy || a.setup.busy || a.login.busy || a.logout.busy || a.remoteLifecycleBusy() || a.remote.busy).OnClick(a.openLogout)
				}
			})
		}
		if a.remote.busy {
			ui.Text(c, locales.LoadingRemote()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.remote.cancel)
			return
		}
		if a.remote.err != nil {
			a.errorText(c, a.remote.err, presentation.Error(a.remote.err))
			return
		}
		if !a.remote.loaded {
			return
		}
		config := a.remote.snapshot.Config
		if config.ServerURL == nil || config.DeviceID == nil {
			ui.Text(c, locales.RemoteSetupRequired()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.RuntimeRecoveryTitle()).OnClick(a.openRuntimeRecovery)
			return
		}
		ui.Text(c, *config.ServerURL).TextColor(c.Theme().TextMuted)
		if a.remoteStopRecoveryPresent() && sameConfigContext(config, a.remote.stopContext) {
			ui.Button(c, locales.RemoteStopRecovery()).Disabled(a.remote.stopBusy || a.remote.stopOpen).OnClick(a.openRemoteStopRecovery)
		}
		if a.remoteCreateRecoveryPresent() && sameConfigContext(config, a.remote.createContext) && remote.SessionCreateQuerySupported(a.remote.snapshot.Capabilities) {
			ui.Button(c, locales.SessionCreateRecovery()).Disabled(a.remote.createBusy || a.remote.createOpen || a.remote.stopBusy || a.remote.deleteBusy).OnClick(a.openRemoteCreateRecovery)
		}
		if a.remoteDeleteRecoveryPresent() && sameConfigContext(config, a.remote.deleteContext) && remote.SessionDeleteQuerySupported(a.remote.snapshot.Capabilities) {
			ui.Button(c, locales.RemoteDeleteRecovery()).Disabled(a.remote.deleteBusy || a.remote.deleteOpen).OnClick(a.openRemoteDeleteRecovery)
		}
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Radio(c, &a.remote.tab, "", locales.RemoteSessions())
			ui.Radio(c, &a.remote.tab, "accounts", locales.RemoteAccounts())
			if a.remoteWorkflowAvailable(remote.SessionCreateMethod) {
				ui.Button(c, locales.CreateRemoteSession()).Disabled(a.remote.createBusy || a.remote.stopBusy || a.remote.deleteBusy).OnClick(a.openRemoteCreate)
			}
			if remote.SessionDeleteSupported(a.remote.snapshot.Capabilities) && len(a.remote.deleteSelected) > 0 {
				ui.Button(c, locales.RemoteDeleteReviewSelection()).Disabled(a.remote.deleteBusy || a.remote.busy || a.remote.deleteOpen).OnClick(a.beginRemoteDeleteCollection)
			}
			if a.remote.tab == "accounts" && a.remoteWorkflowAvailable("accounts.create") {
				ui.Button(c, locales.AccountActionTitle("create")).Disabled(a.accountBlocked() || a.account.busy).OnClick(a.openAccountCreate)
			}
		})
		if a.remote.tab != "accounts" {
			a.remoteSessionFiltersView(c)
		} else {
			ui.TextInput(c, &a.remote.filter).Label(locales.FilterRemote()).Placeholder(locales.FilterRemote())
		}
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			if a.remote.tab == "accounts" {
				a.remoteAccounts(c)
			} else {
				a.remoteSessions(c)
			}
			ui.Button(c, locales.RuntimeRecoveryTitle()).OnClick(a.openRuntimeRecovery)
		})
	})
}

func (a *App) remoteSessionFiltersView(c *ui.Context) {
	ui.Row(c).Gap(8).Wrap().Children(func() {
		ui.TextInput(c, &a.remote.filter).Label(locales.FilterRemote()).Placeholder(locales.FilterRemote()).Grow(1)
		label := locales.RemoteSessionServerFilter()
		if a.remote.sessionFiltersOpen {
			label = locales.RemoteSessionHideFilter()
		}
		ui.Button(c, label).OnClick(func() {
			a.remote.sessionFiltersOpen = !a.remote.sessionFiltersOpen
		})
		if a.remote.sessionToolFilter != "" || len(a.remote.sessionStatusFilters) > 0 {
			ui.Text(c, locales.RemoteSessionFilterActive()).FontSize(12).TextColor(c.Theme().TextMuted)
		}
	})
	if !a.remote.sessionFiltersOpen {
		return
	}
	a.card(c, ui.Column(c).FillWidth().Padding(10).Gap(6)).Children(func() {
		ui.Text(c, locales.RemoteSessionServerFilter()).Bold()
		ui.Text(c, locales.RemoteSessionServerFilterHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Text(c, locales.RemoteSessionToolFilter()).TextColor(c.Theme().TextMuted)
			ui.Radio(c, &a.remote.sessionToolDraft, "", locales.RemoteSessionFilterAll()).OnChange(func() {
				a.remote.sessionFilterDirty = true
			})
			for _, tool := range a.remote.sessionToolOptions {
				tool := tool
				ui.Radio(c, &a.remote.sessionToolDraft, tool, tool).OnChange(func() {
					a.remote.sessionFilterDirty = true
				})
			}
		})
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Text(c, locales.RemoteSessionStatusFilter()).TextColor(c.Theme().TextMuted)
			for _, status := range a.remote.sessionStatusOptions {
				status := status
				selected := a.remote.sessionStatusDraft[status]
				if ui.Checkbox(c.Key("remote-status-filter-"+status), &selected, presentation.RemoteStatus(status)).Changed() {
					if a.remote.sessionStatusDraft == nil {
						a.remote.sessionStatusDraft = map[string]bool{}
					}
					a.remote.sessionStatusDraft[status] = selected
					a.remote.sessionFilterDirty = true
				}
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			ui.Button(c, locales.RemoteSessionApplyFilter()).Disabled(!a.remote.sessionFilterDirty || a.remote.busy).OnClick(a.applyRemoteSessionFilter)
			ui.Button(c, locales.RemoteSessionClearFilter()).Disabled((a.remote.sessionToolDraft == "" && len(a.remote.sessionStatusDraft) == 0) || a.remote.busy).OnClick(a.clearRemoteSessionFilter)
		})
	})
}

func (a *App) remoteAccounts(c *ui.Context) {
	if err := a.remote.snapshot.AccountsError; err != nil {
		a.errorText(c, err, presentation.Error(err))
		return
	}
	count := 0
	for _, account := range a.remote.snapshot.Accounts {
		if !remoteMatches(a.remote.filter, account.ID, account.Name, account.Tool, account.Status, presentation.RemoteStatus(account.Status)) {
			continue
		}
		count++
		a.card(c, ui.Column(c.Key(account.ID)).FillWidth().Padding(14).Gap(6)).Children(func() {
			ui.Text(c, account.Name).Bold()
			ui.Text(c, account.ID).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, locales.RemoteAccountSummary(account.Tool, presentation.RemoteStatus(account.Status), account.Region))
			if a.remoteWorkflowAvailable("accounts.status") {
				ui.Button(c, locales.AccountDetailsTitle()).Disabled(a.accountBlocked() || a.account.busy).OnClick(func() { a.openAccountStatus(account.ID) })
			}
			if account.Backend != nil {
				ui.Text(c, locales.RemoteBackend(*account.Backend)).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
	}
	if count == 0 {
		ui.Text(c, locales.NoRemoteMatches()).TextColor(c.Theme().TextMuted)
	}
}

func (a *App) remoteSessions(c *ui.Context) {
	if a.message != nil {
		ui.Text(c, a.message()).TextColor(c.Theme().Danger)
	}
	if err := a.remote.snapshot.SessionsError; err != nil {
		a.errorText(c, err, presentation.Error(err))
		return
	}
	if !a.remote.snapshot.Capabilities.SupportsTerminal("sessions.attach") {
		ui.Text(c, locales.RemoteAttachUnsupported()).TextColor(c.Theme().TextMuted)
	}
	if !remote.SessionStopSupported(a.remote.snapshot.Capabilities) {
		ui.Text(c, locales.RemoteStopUnsupported()).TextColor(c.Theme().TextMuted)
	}
	if !remote.SessionDeleteSupported(a.remote.snapshot.Capabilities) {
		ui.Text(c, locales.RemoteDeleteUnsupported()).TextColor(c.Theme().TextMuted)
	}
	if a.remote.recoveryNotice != nil {
		ui.Text(c, a.remote.recoveryNotice()).TextColor(c.Theme().TextMuted)
	}
	if a.remote.recovery != nil {
		ui.Text(c, a.remote.recovery.Directory).FontSize(12).TextColor(c.Theme().TextMuted)
		if a.remote.recovery.Reference != "" {
			ui.Text(c, a.remote.recovery.Reference).FontSize(12)
		}
		ui.Button(c, locales.RemoteHistoryEndReview()).OnClick(func() {
			a.remote.recovery, a.remote.recoveryNotice = nil, nil
		})
	}
	if a.project() == nil {
		ui.Text(c, locales.RemoteProjectRequired()).TextColor(c.Theme().TextMuted)
	}
	count := 0
	for _, s := range a.remote.snapshot.Sessions {
		_, validTarget := a.remoteTarget(s.ID)
		_, runningTarget := a.currentRemoteSession(s.ID)
		path := ""
		if s.LocalPath != nil {
			path = *s.LocalPath
		} else if s.RemotePath != nil {
			path = *s.RemotePath
		}
		if !remoteMatches(a.remote.filter, s.ID, s.Tool, s.Status, presentation.RemoteStatus(s.Status), path) {
			continue
		}
		count++
		a.card(c, ui.Column(c.Key(s.ID)).FillWidth().Padding(14).Gap(6)).Children(func() {
			if remote.SessionDeleteSupported(a.remote.snapshot.Capabilities) && remoteSessionCanDelete(s) {
				selected := a.remote.deleteSelected[s.ID]
				if ui.Checkbox(c.Key("delete-select-"+s.ID), &selected, locales.RemoteDeleteSelect()).Changed() {
					if a.remote.deleteSelected == nil {
						a.remote.deleteSelected = map[string]bool{}
					}
					a.remote.deleteSelected[s.ID] = selected
					if !selected {
						delete(a.remote.deleteSelected, s.ID)
					}
				}
			}
			ui.Text(c, s.ID).Bold()
			ui.Text(c, path).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, locales.RemoteSessionSummary(s.Tool, presentation.RemoteStatus(s.Status)))
			ui.Text(c, locales.RemoteBackend(s.Backend)).FontSize(12).TextColor(c.Theme().TextMuted)
			if !validTarget || (s.Status == "running" && s.Tool == "claude" && !runningTarget) {
				ui.Text(c, locales.RemoteAttachContextInvalid()).TextColor(c.Theme().TextMuted)
			}
			ui.Button(c, locales.ConnectRemoteSession()).Disabled(a.project() == nil || !validTarget || !runningTarget || !a.remoteAttachAvailable()).OnClick(func() { a.beginRemoteAttach(s) })
			if remoteSessionCanStop(s) && remote.SessionStopSupported(a.remote.snapshot.Capabilities) {
				ui.Button(c, locales.StopRemoteSession()).Disabled(a.remote.stopBusy || a.remote.busy || a.remote.stopOpen).OnClick(func() { a.beginRemoteStop(s) })
			}
			if remote.SessionDeleteSupported(a.remote.snapshot.Capabilities) && remoteSessionCanDelete(s) {
				ui.Button(c, locales.DeleteRemoteSession()).Disabled(a.remote.deleteBusy || a.remote.busy || a.remote.deleteOpen).OnClick(func() { a.beginRemoteDelete(s) })
			}
		})
	}
	if count == 0 {
		ui.Text(c, locales.NoRemoteMatches()).TextColor(c.Theme().TextMuted)
	}
}

func remoteMatches(filter string, values ...string) bool {
	filter = strings.ToLower(strings.TrimSpace(filter))
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), filter) {
			return true
		}
	}
	return false
}
