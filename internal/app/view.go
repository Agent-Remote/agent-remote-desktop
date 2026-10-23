package app

import (
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// View renders the workbench: Layer 0 (Zen) is the full-window workspace
// with the breathing bar; ⌥A/⌥C/⌥X/⌥S layers sit above it (redesign §3).
func (a *App) View(c *ui.Context) {
	frameStarted := time.Now()
	defer func() { a.motionFrames.Observe(time.Since(frameStarted)) }()
	a.pollLaunch()
	a.pollExit()
	a.pollSync(c)
	a.pollMutagenEnvironment(c)
	a.pollAccount(c)
	a.pollAccountImport(c)
	a.pollRuntimeRecovery(c)
	a.pollAccountDefault(c)
	a.pollRemote(c)
	a.pollRemoteCreate(c)
	a.pollLogin(c)
	a.pollLogout(c)
	a.pollLocalSetup(c)
	a.pollInitWizard(c)
	a.pollWireGuard(c)
	a.pollNetwork(c)
	a.pollDiagnostics(c)
	a.pollDependencies(c)
	a.pollCommandDiagnostics(c)
	a.pollCapabilities(c)
	a.pollDevice(c)
	a.pollEgoBrowser(c)
	a.pollNodes(c)
	a.pollCredentials(c)
	a.pollSkills(c)
	// Persist safe lifecycle projections after workers publish their latest
	// result. This never performs a CLI query or retries a mutation.
	a.syncOperationJournal()
	// Keep one final frame scheduled while a background save is outstanding.
	// A failed save then appears without continuously polling an idle workbench.
	if a.savePending.Load() > 0 {
		c.After(50 * time.Millisecond)
	}
	a.resolveTheme(c.Theme().Dark)
	t := theme.UI(a.shownTheme)
	c.SetTheme(&t)
	env := a.chromeEnv(c)
	if a.dialogOpen() && a.openLayer() != "" && (a.openLayer() != layerControl || a.page == "") {
		// A dialog opened over a layer (the new-session dialog from ⌥C) owns
		// the keyboard; the layer has already committed. A page's own dialog
		// (a remote attach, a delete review) sits over its sheet instead.
		a.closeLayer()
	}
	a.syncPageHost()
	a.watchChordRelease(c)
	ui.Box(c).Fill().Background(t.Background).Children(func() {
		a.backgroundDecoration(c, env)
		a.workspace(c, env)
		a.overlays(c, env)
		a.gpuProbe(c)
	})
	if focused := ui.FocusedValue(c, &a.focusValue); isSessionFocus(focused) && focused == a.focusValue {
		a.focusPane(focused, false)
	}
	a.shortcuts(c)
	a.chordFallback(c)
	if a.openLayer() != "" && !a.dialogOpen() {
		// Esc leaves a layer wherever its keyboard focus went (a tile, the
		// appearance switch, a page's field): never trap the user
		// (apple-design §16). A page goes back to the grid first.
		c.OnShortcut(0, ui.KeyEscape, a.escapeLayer)
	}
	a.dialogs(c)
	a.recoveryDialog(c)
	a.remoteAttachDialog(c)
	a.remoteCreateDialog(c)
	a.remoteDeleteDialog(c)
	a.palette(c)
	a.saveLayoutWhenIdle(c)
}

// chordFallback registers ⌥A/C/X/S as window shortcuts only while nothing
// that handles keys has the focus (a button, say, also on a Control Center
// page). A window shortcut does not stop the text the key types, so it
// must never cover a terminal or the root, which take chords themselves
// (R0); a text field keeps ⌥ + letter as the text it types.
func (a *App) chordFallback(c *ui.Context) {
	if a.dialogOpen() || ui.FocusedValue(c, &a.focusValue) != "" || a.layers.search {
		return
	}
	for key := range chordKeys {
		c.OnShortcut(ui.Alt, key, func() { a.chordPress(ui.Alt, key) })
	}
}

// gpuProbe learns whether the window paints on a GPU (Painter.Accelerated
// is only known while painting): full-window blur is cheap there and over
// 16 ms on the CPU (R0), where a scrim replaces it.
func (a *App) gpuProbe(c *ui.Context) {
	ui.Box(c.Key("gpu-probe")).Absolute().Top(0).Left(0).Size(1, 1).PassThrough().Draw(func(p *ui.Painter, _ ui.Rect) {
		if on := p.Accelerated(); a.layers.gpu.Swap(on) != on {
			a.invalidate()
		}
	})
}

// backgroundDecoration paints the v3 mesh glow behind the workspace and,
// in kawaii, four static sparkles that twinkle once when the theme arrives.
// Nothing here loops: an idle workbench draws no frames.
func (a *App) backgroundDecoration(c *ui.Context, env chrome.Env) {
	policy := theme.BackgroundDecor(env.S.ReduceMotion, a.state.Preferences.FocusMode)
	if !policy.Enabled {
		return
	}
	p := env.P
	ui.Box(c).Key("mesh-a").Absolute().Right(-120).Top(-160).Size(520, 520).Radius(260).Background(p.Mesh1).PassThrough()
	ui.Box(c).Key("mesh-b").Absolute().Left(-160).Bottom(-200).Size(560, 560).Radius(280).Background(p.Mesh2).PassThrough()
	if p.Mesh3.A > 0 {
		ui.Box(c).Key("mesh-c").Absolute().Left(360).Top(220).Size(420, 420).Radius(210).Background(p.Mesh3).PassThrough()
	}
	if !p.Decorations {
		return
	}
	colors := []ui.Color{ui.Hex("#FF85B3"), ui.Hex("#60C5FF"), ui.Hex("#B888FF"), ui.Hex("#FF85B3")}
	for i, glyph := range []string{"✦", "✧", "⋆", "✦"} {
		s := ui.Text(c.Key([2]any{"sparkle", i}), glyph).FontSize(float32(14 + i%2*4)).TextColor(colors[i].Alpha(0.35)).PassThrough().Absolute()
		switch i {
		case 0:
			s = s.LeftPercent(10).TopPercent(14)
		case 1:
			s = s.RightPercent(14).TopPercent(22)
		case 2:
			s = s.LeftPercent(16).BottomPercent(20)
		default:
			s = s.RightPercent(12).BottomPercent(26)
		}
		delay := time.Duration(i) * 70 * time.Millisecond
		twinkle := s.Keyframes("twinkle", []ui.Keyframe{{Value: 0.6}, {At: delay + time.Millisecond, Value: 0.6}, {At: delay + 500*time.Millisecond, Value: 1, Ease: ui.EaseOut}},
			ui.KeyframeOptions{Run: uint64(len(a.shownTheme))})
		s.Scale(twinkle, twinkle)
	}
}

// workspaceDepth is the workspace's place behind the open layer: v3's
// values, and one level further back while a dialog sits on top.
func (a *App) workspaceDepth(env chrome.Env) (ui.Pose, float32) {
	var pose ui.Pose
	blur := float32(0)
	if a.layers.pulling {
		// The bar under the pointer stays put while it pulls the sheet.
		return pose, blur
	}
	switch a.openLayer() {
	case layerAgents:
		pose, blur = ui.Pose{Y: 36, Scale: 0.935, Fade: 0.25}, 18
	case layerCreate, layerProjects:
		pose, blur = ui.Pose{Scale: 0.965, Fade: 0.35}, 30
	case layerControl:
		pose, blur = ui.Pose{Y: -18, Scale: 0.94, Fade: 0.3}, 14
	}
	if a.dialogOpen() && pose.Scale == 0 {
		pose, blur = ui.Pose{Scale: 0.98, Fade: 0.1}, 0
	}
	if env.S.ReduceTransparency || !a.layers.gpu.Load() {
		// Without blur the scrim and a deeper dim keep the layer readable.
		blur = 0
		if pose.Fade > 0 {
			pose.Fade = 0.6
		}
	}
	return pose, blur
}

func (a *App) workspace(c *ui.Context, env chrome.Env) {
	pose, blur := a.workspaceDepth(env)
	ws := ui.Column(c.Key("workspace")).Absolute().Top(0).Right(0).Bottom(0).Left(0).Padding(14, 14, 0).
		FocusBind(&a.focusValue, focusZen).HandleInput(a.rootKey)
	ws = ws.AnimatePose(pose, env.Move(theme.Smooth))
	if sigma := ws.AnimateTo("blur", blur, env.Fade(theme.Smooth)); sigma > 0.2 {
		ws = ws.Blur(sigma)
	}
	ws.Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).FillWidth().Children(func() { a.content(c, env) })
		a.breathingBar(c, env)
	})
}

// overlays builds every layer's slot every frame, so a layer coming or
// going plays its Enter or Exit transition.
func (a *App) overlays(c *ui.Context, env chrome.Env) {
	open := a.openLayer()
	for _, layer := range []string{layerAgents, layerCreate, layerProjects, layerControl} {
		ui.Box(c.Key("layer-" + layer)).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough().Children(func() {
			switch {
			case layer == layerControl:
				a.controlLayer(c, env, open == layerControl)
			case layer == layerAgents:
				a.dockLayer(c, env, open == layerAgents)
			case open != layer:
			case layer == layerCreate:
				a.createLayer(c, env)
			case layer == layerProjects:
				a.projectsLayer(c, env)
			}
		})
	}
}

// breathingBar is Layer 0's only chrome (v3 .zen-footer): context, running
// operations and the chords; it opens the Control Center. Messages (a save
// failure, a launch error) take its line while present.
func (a *App) breathingBar(c *ui.Context, env chrome.Env) {
	bar := ui.ButtonBase(c.Key("breathing-bar")).FillWidth().Height(44).Gap(7).Center().Label(locales.OpenControlCenter())
	a.pullSheet(c, bar)
	if bar.Clicked() && !a.layers.pulled {
		a.showLayer(layerControl)
	}
	bar.Children(func() { ui.Column(c).Gap(7).Center().Children(func() { a.barContent(c, env) }) })
}

// pullSheet raises the Control Center from the bar 1:1 with the pointer
// (redesign §4.5); letting go settles it open or closed by where the
// flick would carry it (apple-design §5, §9).
func (a *App) pullSheet(c *ui.Context, bar ui.Element) {
	l := &a.layers
	if bar.Pressed() && !l.pulling && l.pull == 0 {
		l.pulled = false
	}
	_, dy, dragging := bar.Dragged()
	if dragging && (l.pulling || a.openLayer() == "") {
		l.pull = max(0, l.pull-dy)
		if !l.pulling && l.pull > 6 {
			l.pulling, l.pulled = true, true
			a.showLayer(layerControl)
		}
	}
	_, vy, ended := bar.DragEnded()
	if !ended || !l.pulling {
		if !dragging && !l.pulling {
			l.pull = 0
		}
		return
	}
	_, height := c.Size()
	h := min(max(height*0.74, height-130), 760)
	l.pulling = false
	if l.pull+ui.Project(0, -vy) >= h/2 {
		l.control.Offset.Y = 0
	} else {
		l.control.Offset.Y = h
	}
	l.pull = 0
}

func (a *App) barContent(c *ui.Context, env chrome.Env) {
	p := env.P
	{
		ui.Row(c).Gap(10).Center().Children(func() {
			text := ""
			if failure := a.saveError.Load(); failure != nil {
				text = locales.ErrorSavePreferences(presentation.Error(failure.err))
			}
			if a.message != nil {
				text = a.message()
			}
			if text != "" {
				env.Text(c, text, theme.Caption, p.TextSecondary).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
				return
			}
			if a.state.Preferences.FocusMode {
				return
			}
			if project := a.project(); project != nil {
				env.Text(c, locales.BarProject(project.Name), theme.Caption, p.TextSecondary).SingleLine()
			} else {
				env.Text(c, locales.BarNoProject(), theme.Caption, p.TextSecondary)
			}
			env.Text(c, "·", theme.Caption, p.TextTer)
			if saved, ok := a.savedSession(a.active); ok {
				env.Text(c, locales.BarAgent(sessionName(saved)), theme.Caption, p.TextSecondary).SingleLine()
			} else {
				env.Text(c, locales.BarNoAgent(), theme.Caption, p.TextSecondary)
			}
			if running := a.runningOperations(); running > 0 {
				env.Text(c, "·", theme.Caption, p.TextTer)
				env.Dot(c, chrome.StatusConnected)
				env.Text(c, locales.BarOperations(running), theme.Caption, p.AccentInk).FontWeight(600)
			}
			env.Text(c, "·", theme.Caption, p.TextTer)
			for _, k := range []string{"A", "C", "X", "S"} {
				ui.Row(c.Key("kbd"+k)).Padding(1, 6).Radius(6).Background(p.PillBG).Border(1, p.Hairline).Children(func() {
					env.Text(c, chordLabel(k), theme.Caption, p.TextSecondary).FontWeight(600).FontSize(10.5 * c.Theme().FontSize / 14)
				})
			}
		})
		indicator := ui.Box(c).Size(134, 4.5).Radius(2.25).Background(p.TextTer.Alpha(0.4))
		if p.Decorations {
			indicator.LinearGradient(ui.LinearGradient{Angle: 90, From: ui.Hex("#FF85B3").Alpha(0.65), To: ui.Hex("#60C5FF").Alpha(0.65)})
		}
	}
}

func (a *App) runningOperations() int {
	n := 0
	for _, entry := range a.operationEntries() {
		if entry.status == "running" {
			n++
		}
	}
	return n
}

// content is the workspace: the welcome, a project's empty state, or the
// selected tab's panes. Management pages live in the Control Center.
func (a *App) content(c *ui.Context, env chrome.Env) {
	project := a.project()
	if project == nil {
		a.welcome(c, env)
		return
	}
	tab := a.state.Board.Current(project.ID)
	if tab == nil {
		a.projectEmpty(c, env, *project)
		return
	}
	ui.Column(c).Fill().MinWidth(0).MinHeight(0).Children(func() { a.pane(c, env, tab.Root) })
}

func (a *App) welcome(c *ui.Context, env chrome.Env) {
	p := env.P
	ui.Column(c).Fill().Padding(64).Justify(ui.Center).Gap(18).Children(func() {
		env.Text(c, locales.WelcomeEyebrow(), theme.Caption, p.AccentInk).FontWeight(600)
		env.Text(c, locales.WelcomeTitle(), theme.Display, p.TextPrimary)
		env.Text(c, locales.WelcomeHelp(), theme.Body, p.TextSecondary).FontSize(16 * c.Theme().FontSize / 14)
		ui.Row(c).Margin(16, 0).Gap(12).Wrap().Children(func() {
			if env.Button(c, locales.OpenProjectDirectory(), chrome.Primary).Padding(10, 22).Clicked() {
				a.projectOpen = true
			}
			if env.Button(c, locales.ViewEnvironment(), chrome.Secondary).Padding(10, 22).Clicked() {
				a.page = "settings"
			}
		})
		env.Text(c, locales.SupportedAgents(), theme.Caption, p.TextTer)
	})
}

func (a *App) projectEmpty(c *ui.Context, env chrome.Env, project workspace.Project) {
	p := env.P
	ui.Column(c).Fill().Center().Gap(14).Children(func() {
		env.Text(c, locales.ChooseAgent(), theme.PageTitle, p.TextPrimary).FontSize(26 * c.Theme().FontSize / 14)
		env.Text(c, project.Path, theme.MonoSmall, p.TextSecondary)
		if env.Button(c, locales.NewProjectSession(), chrome.Primary).Padding(9, 20).Clicked() {
			a.showLayer(layerCreate)
		}
		env.Text(c, locales.HiddenSessionsHelp(), theme.Caption, p.TextTer)
	})
}

func (a *App) pane(c *ui.Context, env chrome.Env, p *workspace.Pane) {
	if p == nil {
		return
	}
	if p.First != nil {
		first := func() { a.pane(c, env, p.First) }
		second := func() { a.pane(c, env, p.Second) }
		if p.Vertical {
			if ui.SplitVerticalPercent(c.Key(p), &p.Ratio, first, second).Fill().Changed() {
				a.layoutChanged()
			}
		} else {
			if ui.SplitPercent(c.Key(p), &p.Ratio, first, second).Fill().Changed() {
				a.layoutChanged()
			}
		}
		return
	}
	var found *session.Record
	for _, r := range a.sessions.Records() {
		if r.ID == p.SessionID {
			copy := r
			found = &copy
			break
		}
	}
	if found == nil {
		if saved, ok := a.savedSession(p.SessionID); ok {
			a.savedPane(c, saved)
		}
		return
	}
	r := *found
	tokens := env.P
	host, _ := r.Process.(*terminalhost.Host)
	border := tokens.Hairline
	if r.ID == a.active {
		border = tokens.ToolAccent(string(r.Tool)).Alpha(0.55)
	}
	// The pane matches its opaque terminal so no canvas seam shows round it.
	ui.Column(c.Key(r.ID)).Fill().MinWidth(0).MinHeight(0).Radius(16).Clip().Background(tokens.TerminalBG).Border(1, border).Children(func() {
		if tokens.Decorations {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(3).PassThrough().
				LinearGradient(ui.LinearGradient{Angle: 90, Stops: []ui.GradientStop{{Color: ui.Hex("#FF85B3"), At: 0}, {Color: ui.Hex("#60C5FF"), At: 0.5}, {Color: ui.Hex("#B888FF"), At: 1}}})
		}
		a.paneChip(c, env, r, host)
		if host != nil {
			terminal.View(c.Key("terminal-"+r.ID), host.Terminal).Grow(1).FillWidth().Margin(0, 6, 6).FocusBind(&a.focusValue, r.ID)
		}
	})
	// Update exit metadata while visible, without busy polling in the background.
	if !r.Exited {
		c.After(time.Second)
	}
}

// paneChip replaces the 40-DIP pane header with a 36-DIP band holding the
// session: state, name and title, exit code, Hide and Close.
func (a *App) paneChip(c *ui.Context, env chrome.Env, r session.Record, host *terminalhost.Host) {
	p := env.P
	ui.Row(c).Height(36).FillWidth().Padding(6, 10, 0).Gap(8).Children(func() {
		label := locales.LocalTool(string(r.Tool))
		if r.Remote {
			label = locales.RemoteTool(string(r.Tool))
		}
		title := ""
		if host != nil {
			title = host.Title()
		}
		primary, secondary := label, ""
		if saved, ok := a.savedSession(r.ID); ok && saved.Name != "" {
			primary, secondary = saved.Name, title
			if secondary == "" {
				secondary = label
			}
		} else if title != "" {
			primary, secondary = title, label
		}
		status := chrome.StatusIdle
		switch {
		case r.Exited:
			status = chrome.StatusExited
		case r.Remote:
			status = chrome.StatusConnected
		case host != nil && time.Since(host.LastOutput()) < 2*time.Second:
			status = chrome.StatusOutput
		}
		ui.ButtonBase(c).Gap(7).Grow(1).MinWidth(0).OnClick(func() { a.focusPane(r.ID, true) }).Children(func() {
			env.Dot(c, status)
			env.Text(c, primary, theme.Caption, p.TextPrimary).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			if secondary != "" {
				env.Text(c, secondary, theme.Caption, p.TextSecondary).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			}
		})
		if r.Exited {
			env.Text(c, locales.ExitCode(r.ExitCode), theme.Caption, p.TextSecondary)
		}
		if env.Button(c, locales.Hide(), chrome.Secondary).Padding(3, 10).Clicked() {
			a.hideSession(r.ID)
		}
		if env.Button(c, "×", chrome.Secondary).Padding(3, 9).Label(locales.CloseSessionLabel(r.ID)).Clicked() {
			a.stopID, a.stopForce, a.stopOpen = r.ID, false, true
		}
	})
}
