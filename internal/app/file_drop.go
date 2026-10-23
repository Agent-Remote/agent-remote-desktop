package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/transfer"
)

func (a *App) remoteAttachmentStatus() remote.AttachmentStatus {
	if a == nil || !a.remote.loaded {
		return remote.ExactSessionAttachmentStatus()
	}
	return remote.ExactSessionAttachmentStatusForCapabilities(a.remote.snapshot.Capabilities)
}

// handleFileDrop keeps the file-transfer boundary explicit. Local paths can
// be inserted into a local Agent's interactive shell as data. An exact
// fclaude terminal receives a native file offer only after the CLI has
// declared the v1 transfer bridge; otherwise the event is rejected without
// inserting a misleading local path into the remote shell.
func (a *App) handleFileDrop(event *mygo.FileDropEvent) {
	if event == nil || len(event.Paths) == 0 {
		return
	}
	if a.page != "" || a.active == "" {
		a.message = locales.FileDropNoSession
		a.invalidateWindow()
		return
	}
	var selected *terminalhost.Host
	isRemote := false
	exited := false
	for _, record := range a.sessions.Records() {
		if record.ID != a.active {
			continue
		}
		isRemote = record.Remote
		exited = record.Exited
		if host, ok := record.Process.(*terminalhost.Host); ok {
			selected = host
		}
		break
	}
	if isRemote {
		status := a.remoteAttachmentStatus()
		if status.Validate() != nil || !status.Files {
			a.message = locales.RemoteAttachmentDropUnsupported
			a.invalidateWindow()
			return
		}
		limits, err := status.Limits()
		if err != nil || limits.ValidatePaths(event.Paths) != nil {
			a.message = locales.RemoteAttachmentDropUnsupported
			a.invalidateWindow()
			return
		}
		data, err := transfer.FileData(event.Paths...)
		if err != nil {
			a.message = locales.FileDropInvalid
			a.invalidateWindow()
			return
		}
		if selected == nil || exited || !selected.AttachmentTransferEnabled() {
			a.message = locales.RemoteAttachmentDropUnsupported
			a.invalidateWindow()
			return
		}
		if err := selected.PasteData(data); err != nil {
			a.message = locales.FileDropInvalid
		} else {
			count := len(event.Paths)
			a.message = func() string { return locales.FileDropInserted(count) }
		}
		a.invalidateWindow()
		return
	}
	if selected == nil || exited {
		a.message = locales.FileDropExited
		a.invalidateWindow()
		return
	}
	if err := selected.InsertDroppedPaths(event.Paths); err != nil {
		_ = err
		a.message = locales.FileDropInvalid
		a.invalidateWindow()
		return
	}
	count := len(event.Paths)
	a.message = func() string { return locales.FileDropInserted(count) }
	a.invalidateWindow()
}

func (a *App) invalidateWindow() {
	if a.win != nil {
		a.win.Invalidate()
	}
}
