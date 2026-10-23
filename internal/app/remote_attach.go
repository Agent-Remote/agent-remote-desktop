package app

import (
	"path/filepath"
	"strconv"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) clearRemoteAttach() {
	a.remote.attachOpen = false
	a.remote.attach = launchRequest{}
	a.remote.attachSession = cliadapter.RemoteSession{}
}

func (a *App) remoteAttachAvailable() bool {
	return a.remote.loaded && !a.remote.busy && a.remote.err == nil && a.remote.snapshot.SessionsError == nil && !a.starting && !a.login.busy && !a.logout.busy && !a.remoteLifecycleBusy() && !a.closing && !a.quitting.Load() && a.remote.snapshot.Capabilities.SupportsTerminal("sessions.attach")
}

func (a *App) remoteTarget(id string) (workspace.RemoteContext, bool) {
	config := a.remote.snapshot.Config
	if config.ServerURL == nil || config.DeviceID == nil || filepath.Clean(config.Home) != filepath.Clean(a.home) {
		return workspace.RemoteContext{}, false
	}
	target := workspace.RemoteContext{Home: config.Home, ServerURL: *config.ServerURL, DeviceID: *config.DeviceID, SessionID: id}
	return target, target.Valid()
}

func (a *App) currentRemoteSession(id string) (cliadapter.RemoteSession, bool) {
	var found cliadapter.RemoteSession
	count := 0
	for _, candidate := range a.remote.snapshot.Sessions {
		if candidate.ID == id {
			found, count = candidate, count+1
		}
	}
	// The Server/Node attachment contract treats both `running` and `active`
	// as attachable. `starting` is intentionally excluded until the runtime
	// has published a stable tmux target.
	return found, count == 1 && found.Tool == "claude" && (found.Status == "running" || found.Status == "active")
}

func (a *App) beginRemoteAttach(selected cliadapter.RemoteSession) {
	if !a.remoteAttachAvailable() {
		return
	}
	s, ok := a.currentRemoteSession(selected.ID)
	target, valid := a.remoteTarget(selected.ID)
	project := a.project()
	if !ok || !valid || project == nil {
		a.message = presentation.Notice(fault.ResumeReference)
		return
	}
	request := launchRequest{
		projectID: project.ID, directory: project.Path, tool: session.RemoteClaude,
		reference: s.ID, name: "fclaude " + s.ID, remote: &target,
		remoteInfo: &workspace.RemoteSessionInfo{AccountID: s.AccountID, NodeID: s.NodeID, Backend: s.Backend},
	}
	if saved := a.remote.recovery; saved != nil {
		if saved.Remote != nil && *saved.Remote != target {
			a.remote.recoveryNotice = locales.RemoteHistoryContextChanged
			return
		}
		request.projectID, request.directory, request.replaceID = saved.ProjectID, saved.Directory, saved.ID
		request.name = saved.Name
		if request.name == "" {
			request.name = "fclaude " + s.ID
		}
	}
	if !a.remoteProjectMatches(request) {
		a.remote.recoveryNotice = locales.RemoteHistoryContextChanged
		return
	}
	for _, saved := range a.state.Sessions {
		if saved.Tool != string(session.RemoteClaude) || saved.ProjectID != request.projectID || saved.Remote == nil || *saved.Remote != target {
			continue
		}
		for _, live := range a.sessions.Records() {
			if live.ID == saved.ID && !live.Exited {
				a.remote.recovery = nil
				a.remote.recoveryNotice = nil
				a.focusSession(saved.ID)
				return
			}
		}
	}
	args, err := session.RemoteAttachArguments(target)
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	request.args = args
	a.message = nil
	a.remote.attach, a.remote.attachSession = request, s
	a.remote.attachOpen = true
}

func (a *App) remoteProjectMatches(request launchRequest) bool {
	for _, project := range a.state.Projects {
		if project.ID == request.projectID {
			if request.replaceID == "" {
				return project.Path == request.directory && a.state.Preferences.SelectedProject == project.ID
			}
			saved, ok := a.savedSession(request.replaceID)
			return ok && saved.ProjectID == project.ID && saved.Directory == request.directory
		}
	}
	return false
}

func (a *App) remoteConfirmationCurrent() bool {
	r := a.remote.attach
	if !a.remoteAttachAvailable() || r.remote == nil || !a.remoteProjectMatches(r) {
		return false
	}
	target, valid := a.remoteTarget(r.reference)
	s, ok := a.currentRemoteSession(r.reference)
	return valid && target == *r.remote && ok && s.AccountID == a.remote.attachSession.AccountID && s.NodeID == a.remote.attachSession.NodeID && s.Backend == a.remote.attachSession.Backend
}

func (a *App) confirmRemoteAttach() {
	if !a.remoteConfirmationCurrent() {
		a.clearRemoteAttach()
		a.remote.recoveryNotice = locales.RemoteHistoryContextChanged
		return
	}
	if _, err := a.findExecutable(session.RemoteClaude.Executable()); err != nil {
		a.refreshTools()
		a.message = presentation.Notice(err)
		return
	}
	request := a.remote.attach
	a.clearRemoteAttach()
	a.remote.recovery = nil
	a.remote.recoveryNotice = nil
	a.launch(request)
}

func (a *App) dismissRemoteAttach() {
	a.clearRemoteAttach()
	a.remote.recovery = nil
	a.remote.recoveryNotice = nil
}

func (a *App) remoteAttachDialog(c *ui.Context) {
	// Escape and clicking the backdrop close the modal through mygo's binding.
	// Discard a canceled history review before a later window-focus refresh.
	if !a.remote.attachOpen && a.remote.attach.remote != nil {
		a.dismissRemoteAttach()
	}
	ui.Modal(c, &a.remote.attachOpen, func() {
		_, height := c.Size()
		// Keep the confirmation footer inside the viewport even in pseudo-
		// localized layouts. The body remains scrollable, while the larger
		// usable modal height prevents long translated labels from pushing the
		// confirm action below a 900x600 window.
		ui.Column(c).Width(560).Height(min(560, height-40)).Gap(12).Children(func() {
			ui.Text(c, locales.ConnectRemoteSession()).FontSize(22).Bold()
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(10).Children(func() {
				r := a.remote.attach
				ui.Text(c, r.reference).Bold()
				if r.remote != nil {
					ui.Text(c, r.remote.ServerURL)
					ui.Text(c, locales.RemoteAttachDevice(r.remote.DeviceID)).FontSize(12)
					ui.Text(c, r.remote.Home).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				ui.Text(c, locales.RemoteAttachConfirmation()).TextColor(c.Theme().TextMuted)
				ui.Text(c, r.directory).TextColor(c.Theme().TextMuted)
				ui.Text(c, locales.RemoteAttachLocalProject(r.projectID)).FontSize(12).TextColor(c.Theme().TextMuted)
				ui.Text(c, locales.RemoteAttachAccount(a.remote.attachSession.AccountID)).FontSize(12)
				ui.Text(c, locales.RemoteAttachNode(a.remote.attachSession.NodeID)).FontSize(12)
				ui.Text(c, locales.RemoteBackend(a.remote.attachSession.Backend)).FontSize(12)
				ui.Text(c, locales.RemoteAttachClipboardHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
				attachmentStatus := a.remoteAttachmentStatus()
				if attachmentStatus.Validate() == nil {
					statusText := locales.RemoteAttachAttachmentStatusText(strconv.Itoa(attachmentStatus.Version))
					if attachmentStatus.Files {
						statusText = locales.RemoteAttachAttachmentStatusTransfer(strconv.Itoa(attachmentStatus.Version))
					}
					ui.Text(c, statusText).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				a.providerAvailability(c, session.RemoteClaude)
				if a.message != nil {
					ui.Text(c, a.message()).TextColor(c.Theme().Danger)
				}
			})
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).OnClick(a.dismissRemoteAttach)
				ui.PrimaryButton(c, locales.ConfirmRemoteConnection()).Disabled(!a.remoteConfirmationCurrent() || !a.toolAvailable(session.RemoteClaude)).OnClick(a.confirmRemoteAttach)
			})
		})
	})
}
