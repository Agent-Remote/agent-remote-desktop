package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// draggedTab is a tab moved by dragging its card in the agents dock.
type draggedTab struct{ ID, ProjectID string }

// tile is a Control Center destination. A wide tile carries the module's
// observed state as its subtitle — never a guess: "Not checked" until the
// page has read it (redesign §4.5).
type tile struct {
	label string
	open  func()
	wide  bool
	sub   string
}

type tileGroup struct {
	title string
	tiles []tile
}

// controlGroups are the Control Center's destinations (redesign §4.5).
// Entries the CLI does not offer on this host are left out; Capabilities
// explains them.
func (a *App) controlGroups() []tileGroup {
	remoteTiles := []tile{
		{label: locales.RemoteManagement(), open: a.openRemote, wide: true, sub: a.remoteTileState()},
		{label: locales.RemoteAccounts(), open: func() { a.pendingPage = "remote-accounts"; a.openRemote() }},
	}
	if a.remoteWorkflowAvailable(remote.CredentialsListMethod) {
		remoteTiles = append(remoteTiles, tile{label: locales.CredentialsTitle(), open: a.openCredentials})
	}
	remoteTiles = append(remoteTiles,
		tile{label: locales.SkillsTitle(), open: a.openSkills}, tile{label: locales.AccountDefaultTitle(), open: a.openAccountDefault},
		tile{label: locales.AccountImportRecoveryTitle(), open: a.openAccountImportHistory}, tile{label: locales.RuntimeRecoveryTitle(), open: a.openRuntimeRecovery},
	)
	groups := []tileGroup{
		{locales.ControlCenterProject(), []tile{
			{label: locales.SyncOpen(), open: a.openSync, wide: true, sub: a.syncTileState()},
			{label: locales.PortForwardTitle(), open: a.openPortForwards},
			{label: locales.ProjectDetails(), open: func() { a.page = "project" }},
		}},
		{locales.ControlCenterRemote(), remoteTiles},
		{locales.ControlCenterConnections(), []tile{
			{label: locales.WireGuardNetworkTitle(), open: a.openNetwork, wide: true, sub: a.networkTileState()},
			{label: locales.DeviceTitle(), open: a.openDevice}, {label: locales.EgoBrowserTitle(), open: a.openEgoBrowser},
			{label: locales.NodesTitle(), open: a.openNodes}, {label: locales.MutagenEnvironmentTitle(), open: a.openMutagenEnvironment},
		}},
		{locales.ControlCenterSystem(), []tile{
			{label: locales.OperationsSidebar(), open: a.openOperations, wide: true, sub: a.operationsTileState()},
			{label: locales.DiagnosticsTitle(), open: a.openDiagnostics}, {label: locales.DependenciesTitle(), open: a.openDependencies},
			{label: locales.InitWizardTitle(), open: a.openInitWizard}, {label: locales.CapabilitiesTitle(), open: a.openCapabilities},
			{label: locales.Settings(), open: func() { a.page = "settings" }},
		}},
	}
	var active []tile
	add := func(on bool, label string, open func()) {
		if on {
			active = append(active, tile{label: label, open: open})
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

func (a *App) syncTileState() string {
	switch {
	case a.sync.hasOperation():
		return locales.SyncOperationStatus()
	case a.project() != nil:
		return a.project().Name
	}
	return locales.BarNoProject()
}

func (a *App) remoteTileState() string {
	switch {
	case a.remote.busy:
		return locales.TileChecking()
	case !a.remote.loaded:
		return locales.TileNotChecked()
	case a.remote.err != nil:
		return locales.TileReadFailed()
	}
	return locales.TileRemoteSessions(len(a.remote.snapshot.Sessions))
}

func (a *App) networkTileState() string {
	switch status := a.network.status; {
	case status.busy || a.network.check.busy:
		return locales.TileChecking()
	case status.err != nil:
		return locales.TileReadFailed()
	case status.report != nil:
		return locales.TileStatusRead()
	}
	return locales.TileNotChecked()
}

func (a *App) operationsTileState() string {
	if n := a.runningOperations(); n > 0 {
		return locales.BarOperations(n)
	}
	return locales.TileNothingRunning()
}

// controlLayer is the Control Center (⌥S, or the breathing bar): a glass
// sheet rising from the bar, dragged down 1:1 to close (apple-design §2,
// §6, §9). Its grid and pages share the sheet: a page pushes in from the
// right and Back returns it the same way (redesign §4.5, §6.2). It is
// built while open and while it slides out from wherever a drag left it.
func (a *App) controlLayer(c *ui.Context, env chrome.Env, open bool) {
	l := &a.layers
	if !open && !l.controlClosing {
		return
	}
	width, height := c.Size()
	paged := l.hosted != ""
	w := min(1080, width-48)
	target := min(max(height*0.74, height-130), 760)
	if paged {
		// A page needs the room its sidebar layout had.
		w, target = min(1180, width-32), height-24
	}
	if open {
		fade := ui.ElementTransition{Animation: ptr(env.Fade(theme.Smooth)), Enter: &ui.Motion{}, Exit: &ui.Motion{}}
		scrim := env.Scrim(c).Key("control-scrim").Transition(fade).FocusBind(&a.focusValue, focusLayer).HandleInput(a.layerKey)
		// Keep layer chords available while a page control owns focus. The
		// scrim is an overlay, so it receives chords that focused controls leave
		// even when their native focus chain omits the page container.
		for key := range chordKeys {
			key := key
			for _, mods := range a.chordShortcutMods() {
				mods := mods
				if scrim.OverlayShortcut(mods, key) {
					a.chordPress(mods, key)
				}
			}
		}
		if scrim.Clicked() {
			a.closeLayer()
			return
		}
	}
	sheet := ui.Column(c.Key("control-sheet")).Absolute().Bottom(0).Left((width - w) / 2).Width(w)
	h := sheet.AnimateTo("height", target, env.Move(theme.Page))
	rising := l.controlRise
	if rising {
		// Opened from rest: this frame places it below the window, the
		// next springs it up from there.
		h = target
		l.controlRise, l.control.Offset.Y = false, h
		c.AnimationFrame()
		defer func() { l.control.Offset.Y = 0 }()
	}
	l.controlH = h
	sheet = sheet.Height(h)
	follow := env.Move(theme.Sheet)
	if l.pulling {
		// Pulled up from the bar, the pointer moves it.
		l.control.Offset.Y = max(0, h-l.pull)
		follow = theme.Instant
	}
	sheet = env.Glass(sheet, chrome.Dock, 28, false).Radius(28, 28, 0, 0).BorderWidth(1, 1, 0, 1).Clip()
	sheet = sheet.Draggable(&l.control.Offset, ui.DraggableOptions{
		Axis: ui.Vertical, Bounds: &ui.Rect{H: h},
		Snap:      ui.Nearest(ui.Point{}, ui.Point{Y: h}),
		Animation: follow,
	})
	dragged := float32(-1)
	if sheet.Dragging() {
		dragged = 1 - sheet.DragOffset().Y/max(h, 1)
	}
	if dragged != l.follow {
		// The workspace was built before the sheet: build it again with
		// this drag, so it moves in the same frame.
		l.follow = dragged
		c.AnimationFrame()
	}
	switch {
	case open && !rising && !l.pulling && l.control.Offset.Y >= h/2 && !sheet.Dragging():
		// Thrown or dragged past half: the layer closes at once and the
		// sheet goes on to rest below.
		a.closeLayer()
	case !open && l.controlClosing && sheet.DragOffset().Y >= h-1 && !sheet.Transitioning():
		l.controlClosing = false
	}
	sheet.Children(func() {
		ui.Row(c).Height(22).FillWidth().Center().Children(func() {
			ui.Box(c).Size(40, 5).Radius(2.5).Background(env.P.TextTer.Alpha(0.45))
		})
		ui.Box(c.Key("control-stage")).Grow(1).MinHeight(0).FillWidth().Clip().Children(func() {
			if paged {
				if p, ok := a.pages()[l.hosted]; ok {
					page := ui.Column(c.Key("control-page")).Absolute().Top(0).Right(0).Bottom(0).Left(0).Transition(a.push(env, w*0.3)).HandleInput(a.layerKey)
					// A page's buttons and checkboxes can own focus. Register the
					// layer chords on the page subtree so they remain available
					// before a focused control handles ordinary key input.
					for key := range chordKeys {
						key := key
						for _, mods := range a.chordShortcutMods() {
							mods := mods
							page = page.OnShortcut(mods, key, func() { a.chordPress(mods, key) })
						}
					}
					page.Children(func() { a.pageHost(c, env, l.hosted, p) })
				}
				return
			}
			ui.Column(c.Key("control-grid")).Absolute().Top(0).Right(0).Bottom(0).Left(0).Transition(a.push(env, -w*0.12)).
				Children(func() { a.controlGrid(c, env) })
		})
	})
}

// push is how the grid and a page trade places: the page from and to the
// right, the grid a little to the left, both fading; under reduced motion
// only the cross-fade stays (redesign §6.2, §6.3).
func (a *App) push(env chrome.Env, dx float32) ui.ElementTransition {
	if env.S.ReduceMotion {
		dx = 0
	}
	return ui.ElementTransition{Animation: ptr(env.Fade(theme.Page)), Position: true,
		Enter: &ui.Motion{X: dx}, Exit: &ui.Motion{X: dx}}
}

// controlGrid is the sheet's home: the header (title, account, search,
// appearance, close) over the tile groups, or over search results.
func (a *App) controlGrid(c *ui.Context, env chrome.Env) {
	l := &a.layers
	ui.Column(c).Fill().Children(func() {
		// Two groups, which wrap onto two lines when long labels need it.
		ui.Row(c).FillWidth().Padding(0, 28, 14).Gap(12).Wrap().Children(func() {
			ui.Row(c).Gap(12).Grow(1).Children(func() {
				env.Text(c, locales.ControlCenter(), theme.SheetTitle, env.P.TextPrimary).SingleLine()
				a.accountPill(c, env)
			})
			ui.Row(c).Gap(12).Children(func() {
				in := ui.TextInput(c.Key("control-search"), &l.ccQuery).Label(locales.ControlCenterSearch()).Placeholder(locales.SearchWorkbench()).
					Width(240).OnSubmit(func() {
					if items := a.controlSearch(); len(items) > 0 {
						a.runSearchItem(items[0])
					}
				})
				// ⌥ + letter types into a field (å, ß), so chords wait until
				// the focus leaves it; Esc clears the query first.
				in.OnShortcut(0, ui.KeyEscape, a.escapeLayer)
				a.appearanceSwitch(c, env)
				if env.Button(c, "×", chrome.Secondary).Padding(4, 10).Label(locales.CloseControlCenter()).Clicked() {
					a.closeLayer()
				}
			})
		})
		ui.Scroll(c.Key("control-scroll")).Grow(1).MinHeight(0).FillWidth().Padding(4, 28, 24).Gap(16).Children(func() {
			if l.ccQuery != "" {
				a.controlResults(c, env)
				return
			}
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

// accountPill is the header's sign-in state as last read; it opens Remote
// management, where sign-in and sign-out live.
func (a *App) accountPill(c *ui.Context, env chrome.Env) {
	label, dot := locales.AccountUnchecked(), (*chrome.Status)(nil)
	if a.remote.loaded && !a.remote.busy && a.remote.err == nil {
		if url := a.remote.snapshot.Config.ServerURL; url != nil {
			label, dot = *url, ptr(chrome.StatusConnected)
		} else {
			label, dot = locales.AccountSignedOut(), ptr(chrome.StatusDisconnected)
		}
	}
	b := ui.ButtonBase(c).Radius(999).Label(label).Shrink(1).MinWidth(0)
	if b.Clicked() {
		a.openRemote()
	}
	chrome.Pressable(b, env).Children(func() { env.Pill(c, dot, label).Shrink(1).MinWidth(0) })
}

func (a *App) controlTile(c *ui.Context, env chrome.Env, t tile) {
	// Compact tiles: every group fits the 1366×768 acceptance size without
	// scrolling; smaller windows scroll the sheet. Wide tiles carry state.
	width := float32(160)
	if t.wide {
		width = 330
	}
	b := ui.ButtonBase(c).Width(width).Height(56).Padding(8, 14).Justify(ui.Start).Label(t.label)
	b = env.Glass(b, chrome.Card, 18, b.Hovered())
	if b.Clicked() {
		t.open()
	}
	chrome.Pressable(b, env).Children(func() {
		ui.Column(c).Gap(2).Shrink(1).MinWidth(0).Children(func() {
			env.Text(c, t.label, theme.BodyStrong, env.P.TextPrimary).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			if t.sub != "" {
				env.Text(c, t.sub, theme.Caption, env.P.TextSecondary).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			}
		})
	})
}

// controlSearch is the palette's index (pages, commands, projects,
// sessions) filtered by the header field; the sheet itself is no result.
func (a *App) controlSearch() []paletteItem {
	items := a.searchIndex(a.layers.ccQuery)
	out := items[:0]
	for _, item := range items {
		if item.id != "control-center" {
			out = append(out, item)
		}
	}
	return out
}

// runSearchItem leaves the grid and does what the result says: a page
// result comes straight back into the sheet as a push.
func (a *App) runSearchItem(item paletteItem) {
	a.layers.ccQuery = ""
	a.closeLayer()
	item.run()
}

func (a *App) controlResults(c *ui.Context, env chrome.Env) {
	items := a.controlSearch()
	env.Text(c, locales.ControlCenterResults(), theme.Tag, env.P.TextTer).Margin(0, 4)
	if len(items) == 0 {
		env.Text(c, locales.NoSearchResults(), theme.Body, env.P.TextSecondary).Margin(0, 4)
		return
	}
	for _, item := range items {
		b := ui.ButtonBase(c.Key(item.id)).FillWidth().Padding(10, 14).Justify(ui.Start).Label(item.title)
		b = env.Well(b)
		if b.Hovered() {
			b = b.Background(env.P.GlassCardHover)
		}
		if b.Clicked() {
			a.runSearchItem(item)
		}
		chrome.Pressable(b, env).Children(func() {
			ui.Column(c).Gap(2).Shrink(1).MinWidth(0).Children(func() {
				env.Text(c, item.title, theme.BodyStrong, env.P.TextPrimary).SingleLine().Ellipsis("…")
				if item.detail != "" {
					env.Text(c, item.detail, theme.Caption, env.P.TextSecondary).SingleLine().Ellipsis("…")
				}
			})
		})
	}
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
