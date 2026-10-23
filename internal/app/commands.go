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
		// ⌘O and ⌘T open the ⌥X and ⌥C layers sticky (doc §3.2): the wheel
		// is the one way to switch projects and create Agents.
		{"project", locales.OpenProjectMenu, "CmdOrCtrl+O", ui.Cmd, ui.KeyO, a.openProjectSearch},
		{"session", locales.NewSession, "CmdOrCtrl+T", ui.Cmd, ui.KeyT, func() { a.showLayer(layerCreate) }},
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

// openProjectSearch opens the ⌥X wheel with its "more" search field.
func (a *App) openProjectSearch() {
	a.showLayer(layerProjects)
	a.layers.sel = 0
	a.commitProject()
}

// modalOpen is true while keyboard shortcuts must stay off: a dialog is
// open, or a credential page (login, logout) takes typed secrets.
func (a *App) modalOpen() bool { return a.dialogOpen() || a.page == "login" || a.page == "logout" }

// dialogOpen is true while a dialog owns the window. Layers stay off then;
// on the credential pages the breathing bar still opens the Control Center,
// as the sidebar used to stay usable there.
func (a *App) dialogOpen() bool {
	return a.projectOpen || a.newOpen || a.stopOpen || a.quitOpen || a.recoverOpen || a.paletteOpen || a.remote.attachOpen || a.remote.createOpen || a.remote.deleteOpen
}

func (a *App) shortcuts(c *ui.Context) {
	if a.modalOpen() {
		// The credential pages keep the palette reachable, as the toolbar's
		// Search was; every other command waits until they are left.
		if !a.dialogOpen() {
			c.OnShortcut(ui.Cmd|ui.Shift, ui.KeyP, a.openPalette)
		}
		return
	}
	for _, command := range a.commands() {
		c.OnShortcut(command.mods, command.key, command.run)
	}
}
