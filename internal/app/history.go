package app

import (
	"slices"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) savedSession(id string) (workspace.Session, bool) {
	for _, saved := range a.state.Sessions {
		if saved.ID == id {
			return saved, true
		}
	}
	return workspace.Session{}, false
}

func (a *App) removeSaved(id string) {
	delete(a.bindingConnections, id)
	a.state.Sessions = slices.DeleteFunc(a.state.Sessions, func(s workspace.Session) bool { return s.ID == id })
}

func (a *App) forgetSession(id string) {
	a.state.Board.Hide(id)
	a.removeSaved(id)
	a.restoreFocus()
	a.persist()
}

func (a *App) beginRecovery(id string) {
	if saved, ok := a.savedSession(id); ok {
		if session.Tool(saved.Tool) == session.AccountBinding {
			account, err := session.BindingAccount(saved.Reference)
			if err != nil {
				a.message = presentation.Notice(err)
				return
			}
			a.openAccountStatus(account)
			return
		}
		a.recoverID = id
		a.recoverReference = saved.Reference
		a.recoverOpen = true
		a.message = nil
	}
}

func (a *App) recoverSession() {
	saved, ok := a.savedSession(a.recoverID)
	if !ok {
		a.message = presentation.Notice(fault.SessionMissing)
		return
	}
	if !a.toolAvailable(session.Tool(saved.Tool)) {
		a.message = presentation.Notice(fault.ExecutableMissing)
		return
	}
	if session.Tool(saved.Tool) == session.RemoteClaude {
		if _, err := a.findExecutable(session.RemoteClaude.Executable()); err != nil {
			a.refreshTools()
			a.message = presentation.Notice(err)
			return
		}
		a.beginRemoteRecovery(saved)
		return
	}
	reference := strings.TrimSpace(a.recoverReference)
	args, err := session.RecoveryArguments(session.Tool(saved.Tool), reference)
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	name := saved.Name
	if name == "" {
		name = saved.Tool
	}
	a.launch(launchRequest{projectID: saved.ProjectID, directory: saved.Directory, tool: session.Tool(saved.Tool), args: args, replaceID: saved.ID, reference: reference, name: name})
}

func (a *App) recoveryDialog(c *ui.Context) {
	ui.Modal(c, &a.recoverOpen, func() {
		saved, ok := a.savedSession(a.recoverID)
		if !ok {
			return
		}
		remote := saved.Tool == string(session.RemoteClaude)
		reviewingRemote := remote && saved.Remote != nil
		_, height := c.Size()
		available := a.toolAvailable(session.Tool(saved.Tool))
		ui.Column(c).Width(540).Height(min(480, height-100)).Gap(14).Children(func() {
			title := locales.ResumeSession()
			if reviewingRemote {
				title = locales.RemoteHistoryResumeTitle()
			}
			ui.Text(c, title).FontSize(24).Bold()
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(12).Children(func() {
				ui.Text(c, saved.Directory).TextColor(c.Theme().TextMuted)
				if remote {
					ui.Text(c, locales.RemoteHistoryReviewHelp()).TextColor(c.Theme().TextMuted)
					if saved.Remote == nil {
						ui.Text(c, locales.RemoteHistorySelectAgain()).TextColor(c.Theme().TextMuted)
					} else {
						ui.Text(c, saved.Remote.ServerURL)
						ui.Text(c, locales.RemoteAttachDevice(saved.Remote.DeviceID)).FontSize(12)
					}
					if saved.Reference != "" {
						ui.Text(c, saved.Reference).FontSize(12)
					}
				} else {
					ui.Text(c, locales.LocalResumeHelp()).TextColor(c.Theme().TextMuted)
					ui.TextInput(c, &a.recoverReference).Label(locales.LocalResumeReference()).Placeholder(locales.LocalResumeReference()).AutoFocus()
				}
				a.providerAvailability(c, session.Tool(saved.Tool))
				if a.message != nil {
					ui.Text(c, a.message()).TextColor(c.Theme().Danger)
				}
			})
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).OnClick(func() { a.recoverOpen = false })
				continueLabel := locales.ContinueSession()
				if reviewingRemote {
					continueLabel = locales.RemoteHistoryContinue()
				}
				ui.PrimaryButton(c, continueLabel).Disabled(a.starting || !available || (remote && (a.remote.busy || a.login.busy || a.logout.busy || a.remoteLifecycleBusy()))).OnClick(a.recoverSession)
			})
		})
	})
}

func (a *App) savedPane(c *ui.Context, saved workspace.Session) {
	ui.Column(c.Key(saved.ID)).Fill().MinWidth(0).MinHeight(0).Padding(20).Gap(14).Background(c.Theme().Surface).Radius(10).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			label := locales.LocalTool(saved.Tool)
			if session.Tool(saved.Tool).IsRemote() {
				label = locales.RemoteTool(saved.Tool)
			}
			ui.Button(c, label).OnClick(func() { a.focusPane(saved.ID, true) }).FocusBind(&a.focusValue, saved.ID)
			ui.Spacer(c)
			ui.Button(c, locales.Hide()).OnClick(func() { a.hideSession(saved.ID) })
		})
		ui.Scroll(c).FillWidth().Grow(1).Gap(14).Children(func() {
			if saved.Name != "" {
				ui.Text(c, saved.Name).Bold()
			}
			ui.Text(c, locales.SavedSession()).FontSize(20).Bold()
			help := locales.SavedSessionHelp()
			if session.Tool(saved.Tool) == session.AccountBinding {
				help = locales.AccountBindingReconnectHelp()
			}
			ui.Text(c, help).TextColor(c.Theme().TextMuted)
			if !a.toolAvailable(session.Tool(saved.Tool)) {
				ui.Text(c, locales.ProviderRequired(saved.Tool)).FontSize(12).TextColor(c.Theme().Danger)
			}
			ui.Text(c, saved.Directory).FontSize(12).TextColor(c.Theme().TextMuted)
			if saved.RemoteInfo != nil {
				// Keep the observed public placement visible after a restart. This
				// is metadata only; reconnect still refreshes the exact remote
				// session and re-confirms its current account/node/backend.
				ui.Text(c, locales.RemoteAttachAccount(saved.RemoteInfo.AccountID)).FontSize(12)
				ui.Text(c, locales.RemoteAttachNode(saved.RemoteInfo.NodeID)).FontSize(12)
				ui.Text(c, locales.RemoteBackend(saved.RemoteInfo.Backend)).FontSize(12)
			}
			if saved.ExitCode != nil {
				ui.Text(c, locales.LastExitCode(*saved.ExitCode)).FontSize(12)
			}
			ui.Row(c).Gap(10).Wrap().Children(func() {
				label := locales.ResumeSession()
				if session.Tool(saved.Tool) == session.AccountBinding {
					label = locales.AccountBindingReviewStatus()
				} else if session.Tool(saved.Tool) == session.RemoteClaude && saved.Remote != nil {
					label = locales.RemoteHistoryReviewAction()
				}
				ui.PrimaryButton(c, label).Disabled(a.starting).OnClick(func() { a.beginRecovery(saved.ID) })
				ui.Button(c, locales.ForgetSavedSession()).OnClick(func() { a.forgetSession(saved.ID) })
			})
		})
	})
}
