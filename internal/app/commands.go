package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type workbenchCommand struct {
	id          string
	label       func() string
	accelerator string
	mods        ui.Modifiers
	key         ui.Key
	run         func()
}

func (a *App) commands() []workbenchCommand {
	return []workbenchCommand{
		{"palette", locales.CommandPalette, "CmdOrCtrl+Shift+P", ui.Cmd | ui.Shift, ui.KeyP, a.openPalette},
		{"project", locales.OpenProjectMenu, "CmdOrCtrl+O", ui.Cmd, ui.KeyO, func() { a.projectOpen = true }},
		{"session", locales.NewSession, "CmdOrCtrl+T", ui.Cmd, ui.KeyT, func() { a.newSession("", false) }},
		{"split-horizontal", locales.SplitHorizontal, "CmdOrCtrl+Shift+D", ui.Cmd | ui.Shift, ui.KeyD, func() {
			if a.active != "" {
				a.newSession(a.active, false)
			}
		}},
		{"split-vertical", locales.SplitVertical, "CmdOrCtrl+Shift+E", ui.Cmd | ui.Shift, ui.KeyE, func() {
			if a.active != "" {
				a.newSession(a.active, true)
			}
		}},
		{"tab-next", locales.NextTab, "Ctrl+Tab", ui.Ctrl, ui.KeyTab, func() { a.cycleTab(1) }},
		{"tab-previous", locales.PreviousTab, "Ctrl+Shift+Tab", ui.Ctrl | ui.Shift, ui.KeyTab, func() { a.cycleTab(-1) }},
		{"project-next", locales.NextProject, "CmdOrCtrl+Alt+Right", ui.Cmd | ui.Alt, ui.KeyRight, func() { a.cycleProject(1) }},
		{"project-previous", locales.PreviousProject, "CmdOrCtrl+Alt+Left", ui.Cmd | ui.Alt, ui.KeyLeft, func() { a.cycleProject(-1) }},
		{"pane-next", locales.NextPane, "F6", 0, ui.KeyF6, a.cyclePane},
		{"pane-hide", locales.HideActivePane, "CmdOrCtrl+Shift+W", ui.Cmd | ui.Shift, ui.KeyW, func() {
			if a.active != "" {
				a.hideSession(a.active)
			}
		}},
		{"focus", locales.Focus, "CmdOrCtrl+Shift+F", ui.Cmd | ui.Shift, ui.KeyF, func() { a.state.Preferences.FocusMode = !a.state.Preferences.FocusMode; a.persist() }},
		{"settings", locales.Settings, "CmdOrCtrl+,", ui.Cmd, ui.KeyComma, func() { a.page = "settings" }},
		{"remote", locales.RemoteManagement, "CmdOrCtrl+Shift+R", ui.Cmd | ui.Shift, ui.KeyR, a.openRemote},
	}
}

func (a *App) modalOpen() bool {
	return a.projectOpen || a.newOpen || a.stopOpen || a.quitOpen || a.recoverOpen || a.paletteOpen || a.remote.attachOpen || a.remote.createOpen || a.remote.deleteOpen || a.page == "login" || a.page == "logout"
}

func (a *App) shortcuts(c *ui.Context) {
	if a.modalOpen() {
		return
	}
	for _, command := range a.commands() {
		c.OnShortcut(command.mods, command.key, command.run)
	}
}
