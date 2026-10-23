package app

import (
	"runtime"
	"strings"
	"time"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// dockItem is one ⌥A card: a session of the current project, shown in a
// tab (with its place in a split) or hidden. Its state is what the PTY or
// connection shows — never "thinking" or "awaiting approval" (doc §4.2).
type dockItem struct {
	id     string
	saved  workspace.Session
	live   *session.Record
	host   *terminalhost.Host
	tab    *workspace.Tab
	index  int // place in the tab's split, from 0
	splits int
}

// dockItems lists the current project's sessions: tab order, then hidden.
func (a *App) dockItems() []dockItem {
	project := a.state.Preferences.SelectedProject
	live := map[string]session.Record{}
	for _, r := range a.sessions.Records() {
		live[r.ID] = r
	}
	var items []dockItem
	seen := map[string]bool{}
	add := func(saved workspace.Session, tab *workspace.Tab, index, splits int) {
		it := dockItem{id: saved.ID, saved: saved, tab: tab, index: index, splits: splits}
		if r, ok := live[saved.ID]; ok {
			it.live = &r
			it.host, _ = r.Process.(*terminalhost.Host)
		}
		items = append(items, it)
		seen[saved.ID] = true
	}
	for _, tab := range a.state.Board.Tabs {
		if tab.ProjectID != project {
			continue
		}
		leaves := tab.Root.Leaves()
		for i, id := range leaves {
			if saved, ok := a.savedSession(id); ok && !seen[id] {
				add(saved, tab, i, len(leaves))
			}
		}
	}
	for _, saved := range a.state.Sessions {
		if saved.ProjectID == project && !seen[saved.ID] {
			add(saved, nil, 0, 0)
		}
	}
	return items
}

func (it dockItem) status() chrome.Status {
	switch {
	case it.live == nil:
		return chrome.StatusDisconnected
	case it.live.Exited:
		return chrome.StatusExited
	case it.live.Remote:
		return chrome.StatusConnected
	case it.host != nil && time.Since(it.host.LastOutput()) < 2*time.Second:
		return chrome.StatusOutput
	}
	return chrome.StatusIdle
}

func statusText(it dockItem) string {
	switch it.status() {
	case chrome.StatusDisconnected:
		return locales.SavedSessionStatus()
	case chrome.StatusExited:
		return locales.ExitCode(it.live.ExitCode)
	case chrome.StatusConnected:
		return locales.StatusConnected()
	case chrome.StatusOutput:
		return locales.StatusOutput()
	}
	return locales.StatusIdle()
}

func sessionName(s workspace.Session) string {
	if s.Name != "" {
		return s.Name
	}
	return s.Tool
}

func toolLabel(tool string) string {
	if session.Tool(tool).IsRemote() {
		return locales.RemoteTool(tool)
	}
	return locales.LocalTool(tool)
}

// relativeActive is when the session last showed something.
func relativeActive(it dockItem) string {
	at := it.saved.Started
	if it.host != nil && !it.host.LastOutput().IsZero() {
		at = it.host.LastOutput()
	}
	if at.IsZero() {
		return locales.NoOutputYet()
	}
	d := time.Since(at)
	switch {
	case d < time.Minute:
		return locales.ActiveJustNow()
	case d < time.Hour:
		return locales.ActiveMinutesAgo(int(d.Minutes()))
	}
	return locales.ActiveHoursAgo(int(d.Hours()))
}

// previewOf is the last lines the program printed, control characters
// removed. It is sampled at most twice a second while the dock is open and
// kept only in memory: terminal output is never persisted.
func (a *App) previewOf(it dockItem) string {
	if it.host == nil {
		return it.saved.Directory
	}
	l := &a.layers
	if p, ok := l.previews[it.id]; ok && time.Since(p.at) < 500*time.Millisecond {
		return p.text
	}
	var lines []string
	for _, line := range strings.Split(it.host.Terminal.Text(), "\n") {
		line = strings.Map(func(r rune) rune {
			if unicode.IsControl(r) {
				return -1
			}
			return r
		}, strings.TrimRight(line, " "))
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > 4 {
		lines = lines[len(lines)-4:]
	}
	for i, line := range lines {
		if r := []rune(line); len(r) > 64 {
			lines[i] = string(r[:64]) + "…"
		}
	}
	text := strings.Join(lines, "\n")
	if l.previews == nil {
		l.previews = map[string]preview{}
	}
	l.previews[it.id] = preview{text: text, at: time.Now()}
	return text
}

// chordLabel is the platform's spelling of the chord modifier and a key.
func chordLabel(key string) string {
	if runtime.GOOS == "darwin" {
		return "⌥" + key
	}
	return "Alt+" + key
}

func modifierLabel() string {
	if runtime.GOOS == "darwin" {
		return "⌥"
	}
	return "Alt"
}

func shiftLabel() string {
	if runtime.GOOS == "darwin" {
		return "⇧"
	}
	return "Shift"
}

// dockLayer is Layer 1.1: a glass drawer from the top edge with a card per
// session; the workspace behind is pushed back (workspaceDepth).
func (a *App) dockLayer(c *ui.Context, env chrome.Env) {
	items := a.dockItems()
	l := &a.layers
	width, _ := c.Size()
	// The transparent stage takes the keyboard and closes on an outside click.
	stage := ui.Box(c.Key("dock-stage")).Absolute().Top(0).Right(0).Bottom(0).Left(0).
		FocusBind(&a.focusValue, focusLayer).HandleInput(a.layerKey)
	a.noteMovement(stage.PointerPosition())
	if stage.Clicked() {
		if _, y, ok := stage.PointerPosition(); ok && y > 332 {
			a.closeLayer()
		}
	}
	dock := ui.Column(c.Key("dock")).Absolute().Top(0).Left(0).Right(0).Height(332).Padding(26, max(24, width*0.07), 18).Gap(18)
	dock = env.Glass(dock, chrome.Dock, 28, false).Radius(0, 0, 28, 28).BorderWidth(0, 0, 1, 0).
		Transition(ui.ElementTransition{Animation: ptr(env.Move(theme.Sheet)), Enter: &ui.Motion{Y: -340, Opacity: 1}, Exit: &ui.Motion{Y: -340, Opacity: 1}})
	dock.Children(func() {
		ui.Row(c).FillWidth().Gap(12).Children(func() {
			env.Text(c, "✦ "+chordLabel("A"), theme.Caption, env.P.TextSecondary).FontWeight(600)
			project := ""
			if p := a.project(); p != nil {
				project = p.Name
			}
			env.Text(c, locales.DockTitle(project, len(items)), theme.Caption, env.P.TextPrimary).FontWeight(600).Grow(1)
			hint := locales.DockStickyHint()
			if l.rec.Held() {
				hint = locales.DockPeekHint(modifierLabel())
			}
			env.Text(c, hint, theme.Caption, env.P.TextSecondary)
		})
		ui.ScrollHorizontal(c).FillWidth().Height(220).Gap(22).Padding(6, 2).Children(func() {
			for i, it := range items {
				a.dockCard(c.Key(it.id), env, i, it)
			}
			a.dockGhost(c, env, len(items))
		})
	})
}

func ptr[T any](v T) *T { return &v }

func (a *App) dockCard(c *ui.Context, env chrome.Env, i int, it dockItem) {
	l := &a.layers
	selected := l.sel == i
	accent := env.P.ToolAccent(it.saved.Tool)
	card := ui.ButtonBase(c).Width(280).Height(188).Padding(16, 18).Gap(10).Label(sessionName(it.saved))
	card = env.Glass(card, chrome.Card, 22, selected)
	if selected {
		card = card.Border(1, env.P.SectorStrokeHover)
	}
	// Selection lifts the card (v3: −7 DIP, 1.03) on a critically damped
	// spring — no overshoot, nothing was thrown (apple-design §4).
	lift := ui.Pose{}
	if selected {
		lift = ui.Pose{Y: -7, Scale: 1.03}
	}
	card = card.AnimatePose(lift, env.Move(theme.Snappy))
	if l.moved && card.Hovered() {
		a.selectLayerItem(i)
	}
	if it.tab != nil {
		card = card.Drag(draggedTab{it.tab.ID, it.tab.ProjectID})
		if value, dropped := ui.Drop[draggedTab](card); dropped && value.ID != it.tab.ID && a.state.Board.Move(value.ID, it.tab.ID) {
			a.persist()
		}
	}
	if card.Clicked() {
		l.sel = i
		a.commitLayer(layerAgents)
	}
	card.Children(func() {
		ui.Column(c).Fill().Gap(10).Justify(ui.SpaceBetween).Children(func() { a.dockCardContent(c, env, it, accent) })
	})
}

func (a *App) dockCardContent(c *ui.Context, env chrome.Env, it dockItem, accent ui.Color) {
	{
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			env.Text(c, "✦ "+sessionName(it.saved), theme.BodyStrong, accent).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
			status := it.status()
			env.Pill(c, &status, statusText(it))
		})
		ui.Column(c).FillWidth().Height(92).Padding(10, 12).Radius(14).Background(env.P.CodePreviewBG).Border(1, env.P.Hairline).Clip().Children(func() {
			env.Text(c, a.previewOf(it), theme.MonoSmall, env.P.TextSecondary).FontSize(10.5 * c.Theme().FontSize / 14)
		})
		ui.Row(c).FillWidth().Gap(8).Children(func() {
			footer := toolLabel(it.saved.Tool)
			if it.splits > 1 {
				footer += " · " + locales.PaneOfSplit(it.index+1, it.splits)
			}
			env.Text(c, footer, theme.Caption, env.P.TextTer).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
			env.Text(c, relativeActive(it), theme.Caption, env.P.TextTer)
		})
	}
}

func (a *App) dockGhost(c *ui.Context, env chrome.Env, i int) {
	l := &a.layers
	ghost := ui.ButtonBase(c.Key("dock-new")).Width(150).Height(188).Radius(22).Gap(6).Center().
		Border(1.5, env.P.Hairline).BorderStyle(ui.BorderDashed).Label(locales.NewAgent())
	if l.sel == i {
		ghost = ghost.Background(env.P.Selected()).Border(1.5, env.P.SectorStrokeHover)
	}
	if l.moved && ghost.Hovered() {
		a.selectLayerItem(i)
	}
	if ghost.Clicked() {
		l.sel = i
		a.commitLayer(layerAgents)
	}
	chrome.Pressable(ghost, env).Children(func() {
		ui.Column(c).Gap(6).Center().Children(func() {
			env.Text(c, "＋", theme.HUDTitle, env.P.TextSecondary)
			env.Text(c, locales.NewAgent(), theme.BodyStrong, env.P.TextSecondary)
			env.Text(c, chordLabel("C"), theme.Caption, env.P.TextTer)
		})
	})
}
