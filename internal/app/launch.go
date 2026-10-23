package app

import (
	"strconv"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/attachment"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/egoist/mygo/plugins/terminal"
)

type launchRequest struct {
	projectID, directory string
	tool                 session.Tool
	args                 []string
	target               string
	vertical             bool
	replaceID, reference string
	name                 string
	operationID          string
	bindingKey           string
	remote               *workspace.RemoteContext
	remoteInfo           *workspace.RemoteSessionInfo
}

// launchResult crosses from the process worker to the UI frame. Keeping the
// result in a buffered channel lets headless mygo testers drive the same
// lifecycle as a native window, where Window.Invalidate wakes the next frame.
type launchResult struct {
	request launchRequest
	record  session.Record
	err     error
}

type exitResult struct{ record session.Record }

func (a *App) startSession() {
	p := a.project()
	if p == nil {
		return
	}
	if !a.toolAvailable(a.newTool) {
		a.message = presentation.Notice(fault.ExecutableMissing)
		return
	}
	args, err := session.ParseArguments(a.newArgs)
	if err != nil {
		a.message = presentation.Notice(err)
		return
	}
	name := strings.TrimSpace(a.newName)
	if name == "" {
		name = a.nextSessionName(a.newTool)
	}
	a.launch(launchRequest{projectID: p.ID, directory: p.Path, tool: a.newTool, args: args, target: a.splitTarget, vertical: a.splitVertical, name: name})
}

func (a *App) nextSessionName(tool session.Tool) string {
	names := map[string]bool{}
	for _, s := range a.state.Sessions {
		names[s.Name] = true
	}
	for i := 1; ; i++ {
		name := string(tool) + " " + strconv.Itoa(i)
		if !names[name] {
			return name
		}
	}
}

func (a *App) launch(request launchRequest) {
	if a.starting || a.quitting.Load() || a.closing {
		return
	}
	if _, err := a.findExecutable(request.tool.Executable()); err != nil {
		a.refreshTools()
		a.message = presentation.Notice(err)
		if operation := a.dedicatedOperation(request.operationID); operation != nil {
			operation.status = "failed"
			operation.err = err
			operation.updatedAt = time.Now().UTC()
			a.persist()
		}
		return
	}
	a.starting = true
	a.launches.Add(1)
	a.message = nil
	a.newOpen, a.recoverOpen = false, false
	home := a.home
	attachmentTransfer := false
	attachmentLimits := attachment.Limits{}
	if request.remote != nil {
		home = request.remote.Home
		status := a.remoteAttachmentStatus()
		attachmentTransfer = status.Validate() == nil && status.Files
		if attachmentTransfer {
			attachmentLimits, _ = status.Limits()
		}
	}
	go func() {
		defer a.launches.Done()
		cmd, err := session.PrepareWithResolver(session.Launch{Tool: request.tool, Directory: request.directory, Home: home, Arguments: request.args, AttachmentTransfer: attachmentTransfer, AttachmentLimits: attachmentLimits}, a.findExecutable)
		var record session.Record
		if err == nil {
			record, err = a.sessions.Start(request.projectID, request.tool, cmd)
		}
		a.launchResults <- launchResult{request: request, record: record, err: err}
		if a.win != nil {
			a.win.Invalidate()
		}
	}()
}

func (a *App) pollLaunch() {
	for {
		select {
		case result := <-a.launchResults:
			a.finishLaunch(result)
		default:
			return
		}
	}
}

func (a *App) finishLaunch(result launchResult) {
	a.starting = false
	// Close waits for the worker, then owns session cleanup and the final
	// snapshot. A result that arrives after teardown must not touch persistence.
	if a.closing {
		return
	}
	request, record, err := result.request, result.record, result.err
	if err != nil {
		if operation := a.dedicatedOperation(request.operationID); operation != nil {
			operation.status = "failed"
			operation.err = err
			operation.updatedAt = time.Now().UTC()
			a.persist()
		}
		a.message = presentation.Notice(err)
	} else {
		if operation := a.dedicatedOperation(request.operationID); operation != nil {
			operation.session = record.ID
			operation.status = "running"
			operation.updatedAt = time.Now().UTC()
		}
		if request.bindingKey != "" {
			if a.bindingConnections == nil {
				a.bindingConnections = map[string]string{}
			}
			a.bindingConnections[record.ID] = request.bindingKey
		}
		host := record.Process.(*terminalhost.Host)
		host.Terminal.SetTheme(theme.TerminalPair(a.state.Preferences.Theme, a.state.Preferences.FollowSystemTheme))
		host.Terminal.SetFont(terminal.Font{Size: a.state.Preferences.FontSize})
		if request.replaceID == "" || !a.state.Board.Replace(request.replaceID, record.ID) {
			a.state.Board.Add(request.projectID, record.ID, request.name, request.target, request.vertical)
		}
		a.removeSaved(request.replaceID)
		a.state.Sessions = append(a.state.Sessions, workspace.Session{ID: record.ID, ProjectID: record.ProjectID, Tool: string(record.Tool), Name: request.name, Directory: record.Directory, Started: record.Started, Reference: request.reference, Remote: request.remote.Clone(), RemoteInfo: request.remoteInfo.Clone()})
		if a.state.Preferences.SelectedProject == request.projectID {
			a.page = ""
			a.focusSession(record.ID)
		} else {
			a.persist()
		}
		go a.observeExit(record)
	}
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) observeExit(record session.Record) {
	<-record.Process.Done()
	// Exit observation must not synchronously wait for the native main loop.
	// Hidden windows and headless testers still need the worker to terminate;
	// the next UI frame consumes this bounded result and updates the projection.
	a.exitResults <- exitResult{record: record}
	if a.win != nil {
		a.win.Invalidate()
	}
}

func (a *App) pollExit() {
	for {
		select {
		case result := <-a.exitResults:
			if a.closing {
				continue
			}
			record := result.record
			for i := range a.dedicatedOperations {
				if a.dedicatedOperations[i].session != record.ID {
					continue
				}
				a.dedicatedOperations[i].exitCode = record.Process.ExitCode()
				a.dedicatedOperations[i].status = "completed"
				if record.Process.ExitCode() != 0 {
					a.dedicatedOperations[i].status = "failed"
				}
				a.dedicatedOperations[i].updatedAt = time.Now().UTC()
				break
			}
			a.persist()
		default:
			return
		}
	}
}
