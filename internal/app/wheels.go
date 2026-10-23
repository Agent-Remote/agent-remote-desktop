package app

import (
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// wheelLabel is one sector label: a title and a second line or a badge.
type wheelLabel struct {
	title, sub string
	badge      bool
	color      *ui.Color
	dot        *ui.Color
	dim        bool
}

// wheelStage is the shared stage of Layers 1.2 and 2: a vignette scrim,
// the wheel springing from 0.92 (never from scale 0, design.md §3), and the
// HUD at its centre. A click outside the ring closes; inside, it commits.
func (a *App) wheelStage(c *ui.Context, env chrome.Env, key string, w chrome.Wheel, sectors []chrome.Sector, labels []wheelLabel, hud float32, center func(), below func()) {
	l := &a.layers
	_, height := c.Size()
	scale := min(1, max(0.6, (height-110)/chrome.WheelSize))
	if height < compactHeight {
		// Short windows (900×600 minimum): the whole wheel at .85, so the
		// HUD and labels keep their room (redesign §9).
		scale = min(scale, 0.85)
	}
	scrim := env.P.VignetteEdge
	if env.S.ReduceTransparency {
		scrim = scrim.Alpha(1.25)
	}
	fade := ui.ElementTransition{Animation: ptr(env.Fade(theme.Smooth)), Enter: &ui.Motion{}, Exit: &ui.Motion{}}
	stage := ui.Column(c.Key(key+"-stage")).Absolute().Top(0).Right(0).Bottom(0).Left(0).Center().
		Background(scrim).Transition(fade).
		FocusBind(&a.focusValue, focusLayer).HandleInput(a.layerKey)
	a.noteMovement(stage.PointerPosition())
	stage.Children(func() {
		var box ui.Element
		under := -1
		holder := ui.Box(c.Key(key+"-wheel")).Size(chrome.WheelSize, chrome.WheelSize).Scale(scale, scale).
			Transition(ui.ElementTransition{Animation: ptr(env.Move(theme.Smooth)), Enter: &ui.Motion{Scale: 0.92}, Exit: &ui.Motion{Scale: 0.92}})
		holder.Children(func() {
			box, under = env.WheelView(c, w, sectors, l.sel, l.rec.Held(), func(i int) { placeLabel(c, env, w, i, labels[i]) }, hud, center)
			if env.P.Decorations && !env.S.ReduceMotion {
				orbit(c, env, hud+34)
			}
			if below != nil {
				below()
			}
		})
		if l.moved && under >= 0 {
			a.selectLayerItem(under)
		}
		if box.Clicked() {
			x, y, _ := box.PointerPosition()
			dx, dy := x-chrome.WheelSize/2, y-chrome.WheelSize/2
			switch {
			case under >= 0:
				l.sel = under
				a.commitLayer(a.openLayer())
			case dx*dx+dy*dy > w.DeadR*w.DeadR:
				a.closeLayer()
			}
		}
	})
	if stage.Clicked() {
		a.closeLayer() // outside the wheel's box
	}
}

// placeLabel centres a label at a sector's LabelCenter.
func placeLabel(c *ui.Context, env chrome.Env, w chrome.Wheel, i int, label wheelLabel) {
	pt := w.LabelCenter(i)
	ui.Column(c.Key(i)).Absolute().Left(chrome.WheelSize/2+pt.X-60).Top(chrome.WheelSize/2+pt.Y-24).Size(120, 48).
		PassThrough().Center().Gap(4).Children(func() {
		color := env.P.TextPrimary
		if label.color != nil {
			color = *label.color
		}
		if label.dim {
			color = env.P.TextTer
		}
		env.Text(c, label.title, theme.BodyStrong, color).SingleLine().Ellipsis("…").MaxWidth(116).TextAlign(ui.Center)
		if label.sub == "" {
			return
		}
		if label.badge {
			ui.Row(c).Gap(5).Padding(2, 8).Radius(999).Background(env.P.PillBG).Border(1, env.P.Hairline).Children(func() {
				if label.dot != nil {
					ui.Box(c).Size(6, 6).Radius(3).Background(*label.dot)
				}
				env.Text(c, label.sub, theme.Caption, env.P.TextSecondary).FontSize(10 * c.Theme().FontSize / 14).FontWeight(600)
			})
			return
		}
		env.Text(c, label.sub, theme.Caption, env.P.TextSecondary).SingleLine().Ellipsis("…").MaxWidth(116).TextAlign(ui.Center)
	})
}

// orbit is the kawaii dashed ring, turning once per 28 s only while a wheel
// is open and motion is not reduced (no idle loop, apple-design §14).
func orbit(c *ui.Context, env chrome.Env, d float32) {
	ring := ui.Box(c.Key("orbit")).Absolute().Left(chrome.WheelSize/2-d/2).Top(chrome.WheelSize/2-d/2).Size(d, d).
		Radius(d/2).Border(1.5, env.P.Accent.Alpha(0.45)).BorderStyle(ui.BorderDashed).PassThrough()
	turn := ring.Keyframes("turn", []ui.Keyframe{{Value: 0}, {At: 28 * time.Second, Value: 360, Ease: ui.Linear}}, ui.KeyframeOptions{Repeat: ui.RepeatForever})
	ring.Rotate(turn)
}

// createLayer is Layer 1.2 (⌥C): Claude / Codex / fclaude / Advanced….
func (a *App) createLayer(c *ui.Context, env chrome.Env) {
	accents := []ui.Color{env.P.AccentClaude, env.P.AccentCodex, env.P.AccentGemini, env.P.AccentCustom}
	titles := []string{locales.ToolNameClaude(), locales.ToolNameCodex(), locales.ToolNameFclaude(), locales.CreateAdvanced()}
	subs := []string{locales.CreateLocalSub(), locales.CreateLocalSub(), locales.CreateRemoteSub(), locales.CreateAdvancedSub()}
	sectors := make([]chrome.Sector, 4)
	labels := make([]wheelLabel, 4)
	for i := range sectors {
		sectors[i] = chrome.Sector{Enabled: true, Accent: &accents[i]}
		labels[i] = wheelLabel{title: titles[i], sub: subs[i], color: &accents[i]}
	}
	a.wheelStage(c, env, "create", chrome.CreateWheel, sectors, labels, 214, func() { a.createHUD(c, env, accents) }, nil)
}

func (a *App) toolPath(t session.Tool) string {
	if a.tools == nil {
		a.refreshTools()
	}
	for _, tool := range a.tools {
		if tool.name == t.Executable() {
			return tool.path
		}
	}
	return ""
}

func (a *App) createHUD(c *ui.Context, env chrome.Env, accents []ui.Color) {
	l := &a.layers
	env.HUD(c, 214, func() {
		sel := l.sel
		if sel < 0 {
			env.HUDTag(c, "✦ "+chordLabel("C")+" ✦")
			env.HUDTitle(c, locales.NewAgent(), nil)
			hint := locales.WheelStickyHint()
			if l.rec.Held() {
				hint = locales.CreateWheelPeekHint(modifierLabel())
			}
			env.Text(c, hint, theme.Caption, env.P.TextSecondary).TextAlign(ui.Center).MaxWidth(170)
			if p := a.project(); p != nil {
				env.Text(c, locales.WheelProject(p.Name), theme.Caption, env.P.TextTer).SingleLine().Ellipsis("…").MaxWidth(170)
			} else {
				env.Text(c, locales.ChooseProjectFirst(), theme.Caption, env.P.Warning)
			}
			return
		}
		if sel == len(createTools) {
			env.HUDTag(c, locales.TagAdvanced())
			env.HUDTitle(c, locales.CreateAdvanced(), &accents[sel])
			env.Text(c, locales.CreateAdvancedSub(), theme.Caption, env.P.TextSecondary)
			return
		}
		tool := createTools[sel]
		tag := locales.TagLocalAgent()
		if tool.IsRemote() {
			tag = locales.TagRemoteAgent()
		}
		env.HUDTag(c, tag)
		env.HUDTitle(c, []string{locales.ToolNameClaude(), locales.ToolNameCodex(), locales.ToolNameFclaude()}[sel], &accents[sel])
		if path := a.toolPath(tool); path != "" {
			env.HUDPath(c, path)
			env.Text(c, locales.ToolReady(), theme.Caption, env.P.TextSecondary)
			if !tool.IsRemote() {
				env.Text(c, locales.CreateSplitHint(shiftLabel()), theme.Caption, env.P.TextTer).TextAlign(ui.Center).MaxWidth(170)
			}
		} else {
			env.Text(c, locales.ToolNotInstalled(), theme.Caption, env.P.Warning).TextAlign(ui.Center).MaxWidth(170)
		}
	})
}

// languageName and languageDot map workspace.Probe IDs to what the wheel
// shows; dots use GitHub linguist colours so they read the same everywhere.
func languageName(id string) string {
	switch id {
	case "go":
		return locales.LanguageGo()
	case "rust":
		return locales.LanguageRust()
	case "typescript":
		return locales.LanguageTypeScript()
	case "javascript":
		return locales.LanguageJavaScript()
	case "python":
		return locales.LanguagePython()
	case "swift":
		return locales.LanguageSwift()
	case "cpp":
		return locales.LanguageCpp()
	}
	return locales.LanguageOther()
}

var languageDots = map[string]ui.Color{
	"go": ui.Hex("#00ADD8"), "rust": ui.Hex("#DEA584"), "typescript": ui.Hex("#3178C6"), "javascript": ui.Hex("#F1E05A"),
	"python": ui.Hex("#3572A5"), "swift": ui.Hex("#F05138"), "cpp": ui.Hex("#F34B7D"),
}

func (a *App) projectAgentCount(id string) int {
	n := 0
	for _, s := range a.state.Sessions {
		if s.ProjectID == id {
			n++
		}
	}
	return n
}

func (a *App) lastAgent(id string) string {
	var last *workspace.Session
	for i, s := range a.state.Sessions {
		if s.ProjectID == id && (last == nil || s.Started.After(last.Started)) {
			last = &a.state.Sessions[i]
		}
	}
	if last == nil {
		return ""
	}
	return sessionName(*last)
}

// projectsLayer is Layer 2 (⌥X): "··· more" at 12 o'clock, then seven
// stable project slots clockwise; empty slots keep the geometry (doc §4.4).
func (a *App) projectsLayer(c *ui.Context, env chrome.Env) {
	sectors := make([]chrome.Sector, 8)
	labels := make([]wheelLabel, 8)
	sectors[0] = chrome.Sector{Enabled: true}
	labels[0] = wheelLabel{title: locales.ProjectMore(), sub: locales.ProjectMoreBadge(), badge: true}
	for i := 1; i < 8; i++ {
		p := a.slotProject(i)
		if p == nil {
			labels[i] = wheelLabel{title: "—", sub: locales.ProjectEmptySlot(), dim: true}
			continue
		}
		facts, _ := a.layers.probes.Get(p.Path)
		sectors[i] = chrome.Sector{Enabled: true}
		label := wheelLabel{title: p.Name, sub: languageName(facts.Language) + " · " + locales.ProjectAgents(a.projectAgentCount(p.ID)), badge: true}
		if dot, ok := languageDots[facts.Language]; ok {
			label.dot = &dot
		}
		labels[i] = label
	}
	var below func()
	if a.layers.search {
		below = func() { a.projectResults(c, env) }
	}
	a.wheelStage(c, env, "projects", chrome.ProjectWheel, sectors, labels, 240, func() { a.projectsHUD(c, env) }, below)
}

func (a *App) projectsHUD(c *ui.Context, env chrome.Env) {
	l := &a.layers
	env.HUD(c, 240, func() {
		switch {
		case l.search || l.sel == 0:
			env.HUDTag(c, "✦ "+chordLabel("X")+" ✦")
			env.HUDTitle(c, locales.ProjectSearchTitle(), nil).FontSize(15 * c.Theme().FontSize / 14)
			if l.search {
				in := ui.TextInput(c.Key("project-search"), &l.query).Label(locales.ProjectSearchPlaceholder()).
					Placeholder(locales.ProjectSearchPlaceholder()).AutoFocus().Width(180).OnSubmit(a.submitProjectSearch)
				if in.Changed() {
					l.searchHot = 0
				}
				in.OnShortcut(0, ui.KeyEscape, a.closeLayer)
				in.OnShortcut(0, ui.KeyDown, func() { l.searchHot++ })
				in.OnShortcut(0, ui.KeyUp, func() { l.searchHot = max(0, l.searchHot-1) })
				for key := range chordKeys {
					in.OnShortcut(chordMods(), key, func() { a.chordPress(chordMods(), key) })
				}
			}
			if env.Button(c, locales.ProjectOpenFolder(), chrome.Secondary).Clicked() {
				a.closeLayer()
				a.page = ""
				a.selectDirectory()
			}
		case l.sel > 0 && a.slotProject(l.sel) != nil:
			p := a.slotProject(l.sel)
			facts, _ := l.probes.Get(p.Path)
			env.HUDTag(c, locales.ProjectLanguageTag(languageName(facts.Language)))
			env.HUDTitle(c, p.Name, nil)
			env.HUDPath(c, p.Path)
			branch := locales.ProjectNotRepository()
			switch {
			case facts.Detached:
				branch = locales.ProjectDetached(facts.Branch)
			case facts.Branch != "":
				branch = "⎇ " + facts.Branch
			}
			ui.Row(c).Padding(2, 10).Radius(999).Background(env.P.PillBG).Border(1, env.P.Hairline).MaxWidth(190).Children(func() {
				env.Text(c, branch, theme.Caption, env.P.AccentClaude).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
			})
			if last := a.lastAgent(p.ID); last != "" {
				env.Text(c, locales.ProjectLastAgent(last), theme.Caption, env.P.TextTer).SingleLine().Ellipsis("…").MaxWidth(180)
			} else {
				env.Text(c, locales.ProjectLastAgentNone(), theme.Caption, env.P.TextTer)
			}
		default:
			env.HUDTag(c, "✦ "+chordLabel("X")+" ✦")
			env.HUDTitle(c, locales.SwitchProject(), nil)
			hint := locales.WheelStickyHint()
			if l.rec.Held() {
				hint = locales.ProjectWheelPeekHint(modifierLabel())
			}
			env.Text(c, hint, theme.Caption, env.P.TextSecondary).TextAlign(ui.Center).MaxWidth(180)
			if p := a.project(); p != nil {
				env.Text(c, locales.ProjectCurrent(p.Name), theme.Caption, env.P.TextTer).SingleLine().Ellipsis("…").MaxWidth(180)
			}
		}
	})
}

// projectResults grows down from the HUD it belongs to (doc §4.4).
func (a *App) projectResults(c *ui.Context, env chrome.Env) {
	l := &a.layers
	matches := a.projectMatches()
	l.searchHot = min(l.searchHot, max(len(matches)-1, 0))
	panel := ui.Column(c.Key("project-results")).Absolute().Left(chrome.WheelSize/2 - 150).Top(chrome.WheelSize/2 + 128).Width(300).Padding(6).Gap(2).
		Transition(ui.ElementTransition{Animation: ptr(env.Move(theme.Snappy)), Enter: &ui.Motion{Y: -8, Scale: 0.96}, Exit: &ui.Motion{Y: -8, Scale: 0.96}})
	env.Glass(panel, chrome.HUD, 18, false).Children(func() {
		if len(matches) == 0 {
			env.Text(c, locales.ProjectSearchNoMatch(), theme.Caption, env.P.TextSecondary).Padding(7, 10)
			return
		}
		for i, p := range matches {
			row := ui.ButtonBase(c.Key(p.ID)).FillWidth().Padding(7, 10).Radius(11).Gap(10).Label(p.Name)
			if i == l.searchHot {
				row = row.Background(env.P.Selected())
			}
			if row.Clicked() {
				l.searchHot = i
				a.submitProjectSearch()
			}
			row.Children(func() {
				env.Text(c, p.Name, theme.Body, env.P.TextPrimary).SingleLine()
				env.Text(c, p.Path, theme.MonoSmall, env.P.TextTer).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0).TextAlign(ui.End)
			})
		}
	})
}
