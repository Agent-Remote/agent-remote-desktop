package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func (a *App) beginRemoteRecovery(saved workspace.Session) {
	if a.remote.busy || a.login.busy || a.logout.busy || a.remoteLifecycleBusy() || a.starting {
		return
	}
	saved.Remote = saved.Remote.Clone()
	a.remote.recovery = &saved
	a.remote.recoveryNotice = nil
	a.remote.filter, a.remote.tab = "", ""
	a.recoverOpen = false
	a.page = "remote"
	a.refreshRemote()
}

func (a *App) reviewRemoteRecovery() {
	saved := a.remote.recovery
	if saved == nil || !a.remote.loaded || a.remote.busy || a.remote.err != nil || a.remote.snapshot.SessionsError != nil {
		return
	}
	if saved.Remote == nil {
		a.remote.recoveryNotice = locales.RemoteHistorySelectAgain
		return
	}
	target, valid := a.remoteTarget(saved.Reference)
	if !valid || target != *saved.Remote {
		a.remote.recoveryNotice = locales.RemoteHistoryContextChanged
		return
	}
	s, ok := a.currentRemoteSession(saved.Reference)
	if !ok {
		a.remote.recoveryNotice = locales.RemoteHistoryUnavailable
		return
	}
	a.beginRemoteAttach(s)
}
