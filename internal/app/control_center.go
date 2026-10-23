package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// draggedTab is a tab moved by dragging its card in the agents dock.
type draggedTab struct{ ID, ProjectID string }

type tile struct {
	label string
	open  func()
}

type tileGroup struct {
	title string
	tiles []tile
}

// controlGroups are the Control Center's destinations (doc §4.5). R2 opens
// the existing pages over the workspace; R3 moves them into the sheet.
func (a *App) controlGroups() []tileGroup {
	groups := []tileGroup{
		{locales.ControlCenterProject(), []tile{{locales.SyncOpen(), a.openSync}}},
		{locales.ControlCenterRemote(), []tile{
			{locales.RemoteManagement(), a.openRemote}, {locales.SkillsTitle(), a.openSkills},
			{locales.AccountImportRecoveryTitle(), a.openAccountImportHistory}, {locales.RuntimeRecoveryTitle(), a.openRuntimeRecovery},
		}},
		{locales.ControlCenterConnections(), []tile{
			{locales.WireGuardNetworkTitle(), a.openNetwork}, {locales.DeviceTitle(), a.openDevice},
			{locales.EgoBrowserTitle(), a.openEgoBrowser}, {locales.NodesTitle(), a.openNodes},
			{locales.MutagenEnvironmentTitle(), a.openMutagenEnvironment},
		}},
		{locales.ControlCenterSystem(), []tile{
			{locales.OperationsSidebar(), a.openOperations}, {locales.DiagnosticsTitle(), a.openDiagnostics},
			{locales.DependenciesTitle(), a.openDependencies}, {locales.InitWizardTitle(), a.openInitWizard},
			{locales.CapabilitiesTitle(), a.openCapabilities}, {locales.Settings(), func() { a.page = "settings" }},
		}},
	}
	var active []tile
	add := func(on bool, label string, open func()) {
		if on {
			active = append(active, tile{label, open})
		}
	}
	add(a.mutagenEnvironment.hasOperation(), locales.MutagenEnvironmentOperation(), a.openMutagenEnvironment)
	add(a.sync.hasOperation(), locales.SyncOperationStatus(), func() { a.page = "sync" })
	add(a.account.submitted, locales.AccountOperationStatus(), a.openAccountOperation)
	add(a.accountDefault.submitted, locales.AccountDefaultStatus(), a.openAccountDefault)
	add(a.login.hasOperation(), locales.LoginOperationStatus(), a.openLogin)
	add(a.logout.hasOperation(), locales.LogoutOperationStatus(), a.openLogout)
	add(a.setup.hasOperation(), locales.LocalSetupOperationStatus(), a.openLocalSetup)
	add(a.wireguard.hasOperation(), locales.WireGuardOperationStatus(), a.openWireGuard)
	if len(active) > 0 {
		groups = append([]tileGroup{{locales.ControlCenterActive(), active}}, groups...)
	}
	return groups
}

// controlLayer is the Control Center (⌥S, or the breathing bar): a glass
// sheet rising from the bar, dragged down 1:1 to close (apple-design §2,
// §6, §9). It is built only while open and slides out along the way it
// came, from wherever a drag left it.
func (a *App) controlLayer(c *ui.Context, env chrome.Env, open bool) {
	l := &a.layers
	if !open && !l.controlClosing {
		return
	}
	width, height := c.Size()
	w, h := min(1080, width-48), min(max(height*0.74, height-130), 760)
	if open {
		fade := ui.ElementTransition{Animation: ptr(env.Fade(theme.Smooth)), Enter: &ui.Motion{}, Exit: &ui.Motion{}}
		scrim := env.Scrim(c).Key("control-scrim").Transition(fade).FocusBind(&a.focusValue, focusLayer).HandleInput(a.layerKey)
		if scrim.Clicked() {
			a.closeLayer()
			return
		}
	}
	slide := ui.ElementTransition{Animation: ptr(env.Move(theme.Sheet)), Position: true, Enter: &ui.Motion{Y: h + 40, Opacity: 1}, Exit: &ui.Motion{Y: h + 40, Opacity: 1}}
	sheet := ui.Column(c.Key("control-sheet")).Absolute().Bottom(0).Left((width - w) / 2).Width(w).Height(h)
	sheet = env.Glass(sheet, chrome.Dock, 28, false).Radius(28, 28, 0, 0).BorderWidth(1, 1, 0, 1).Clip().Transition(slide)
	sheet = sheet.Draggable(&l.control.Offset, ui.DraggableOptions{
		Axis: ui.Vertical, Bounds: &ui.Rect{H: h},
		Snap:      ui.Nearest(ui.Point{}, ui.Point{Y: h}),
		Animation: env.Move(theme.Sheet),
	})
	switch {
	case open && l.control.Offset.Y >= h/2 && !sheet.Dragging():
		// Thrown or dragged past half: the layer closes at once, while the
		// sheet keeps building until its spring brings it to rest below.
		a.closeLayer()
		l.controlClosing = true
	case !open && l.controlClosing && sheet.DragOffset().Y >= h-1 && !sheet.Transitioning():
		l.controlClosing = false
	}
	sheet.Children(func() {
		ui.Row(c).Height(22).FillWidth().Center().Children(func() {
			ui.Box(c).Size(40, 5).Radius(2.5).Background(env.P.TextTer.Alpha(0.45))
		})
		ui.Row(c).FillWidth().Padding(0, 28, 14).Gap(12).Children(func() {
			env.Text(c, locales.ControlCenter(), theme.SheetTitle, env.P.TextPrimary).Grow(1)
			if env.Button(c, locales.SearchWorkbench(), chrome.Secondary).Clicked() {
				a.closeLayer()
				a.openPalette()
			}
			a.appearanceSwitch(c, env)
			if env.Button(c, "×", chrome.Secondary).Padding(4, 10).Label(locales.CloseControlCenter()).Clicked() {
				a.closeLayer()
			}
		})
		ui.Scroll(c.Key("control-scroll")).Grow(1).MinHeight(0).FillWidth().Padding(4, 28, 24).Gap(16).Children(func() {
			for _, g := range a.controlGroups() {
				ui.Column(c.Key(g.title)).FillWidth().Gap(8).Children(func() {
					env.Text(c, g.title, theme.Tag, env.P.TextTer).Margin(0, 4)
					ui.Row(c).FillWidth().Gap(10).Wrap().Children(func() {
						for _, t := range g.tiles {
							a.controlTile(c.Key(g.title+"\x00"+t.label), env, t)
						}
					})
				})
			}
		})
	})
}

func (a *App) controlTile(c *ui.Context, env chrome.Env, t tile) {
	// Compact tiles: every group fits the 1366×768 acceptance size without
	// scrolling; smaller windows scroll the sheet.
	b := ui.ButtonBase(c).Width(160).Height(50).Padding(10, 14).Justify(ui.Start).Label(t.label)
	b = env.Glass(b, chrome.Card, 18, b.Hovered())
	if b.Clicked() {
		a.closeLayer()
		t.open()
	}
	chrome.Pressable(b, env).Children(func() {
		env.Text(c, t.label, theme.BodyStrong, env.P.TextPrimary).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	})
}

// appearanceSwitch is the four-way theme switch of Settings and the
// Control Center header: follow system, light, dark, kawaii.
func (a *App) appearanceSwitch(c *ui.Context, env chrome.Env) {
	choice := 0
	if !a.state.Preferences.FollowSystemTheme {
		choice = map[string]int{"daybreak": 1, "midnight": 2, "sakura": 3}[a.state.Preferences.Theme]
	}
	if env.Segmented(c, &choice, locales.AppearanceTheme(), locales.FollowSystem(), locales.ThemeDaybreak(), locales.ThemeMidnight(), locales.ThemeSakura()) {
		if choice == 0 {
			a.setTheme("", true)
		} else {
			a.setTheme([...]string{"", "daybreak", "midnight", "sakura"}[choice], false)
		}
	}
}
