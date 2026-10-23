package app

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chord"
	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/egoist/mygo/ui"
)

// The layers of the redesign (agent-remote docs/agent-remote-desktop-
// redesign.zh-CN.md §3): Zen is no layer; ⌥A agents dock, ⌥C create wheel,
// ⌥X project wheel, ⌥S control center.
const (
	layerAgents   = "agents"
	layerCreate   = "create"
	layerProjects = "projects"
	layerControl  = "control"
)

// Focus values bound besides session IDs (32 hex digits, so no collision).
// The workbench root takes chords while nothing else has the keyboard; the
// open layer takes every key so nothing types into a terminal behind it.
const (
	focusZen   = "workbench:zen"
	focusLayer = "workbench:layer"
)

func isSessionFocus(v string) bool { return v != "" && v != focusZen && v != focusLayer }

var chordKeys = map[ui.Key]string{ui.KeyA: layerAgents, ui.KeyC: layerCreate, ui.KeyX: layerProjects, ui.KeyS: layerControl}

// createTools are the ⌥C sectors 0–2 at 12, 3 and 6 o'clock; sector 3 is
// "Advanced…", which opens the full new-session dialog.
var createTools = []session.Tool{session.Claude, session.Codex, session.RemoteClaude}

type layerState struct {
	rec      chord.Recognizer
	released time.Time // when a key-up showed ⌥ gone, for the tap window
	sel      int       // dock card or wheel sector; -1 none
	shift    bool
	returnTo string // focus value to restore on close
	recent   []string

	search    bool // ⌥X "more": the HUD is a search field
	query     string
	searchHot int

	control        chrome.SheetState // only its Offset: the sheet is built while open
	controlClosing bool              // released past half: keep building until at rest
	hosted         string            // the page the sheet shows ("" its grid)
	ccQuery        string            // the Control Center's search
	dock           ui.Point          // the dock's drag offset: 0 open, −height pushed away
	dockClosing    bool              // pushed past half: keep building until at rest
	pull           float32           // how far a press on the bar has pulled the sheet up
	pulling        bool              // the sheet follows that press 1:1
	pulled         bool              // the press pulled: its release is no click

	pointer      ui.Point // last pointer seen over the open layer
	pointerKnown bool
	moved        bool // pointer moved since the layer opened

	previews map[string]preview
	probes   *workspace.ProbeCache
	gpu      atomic.Bool // the window paints on a GPU: blur is affordable
}

type preview struct {
	text string
	at   time.Time
}

// openLayer is the layer on screen logically ("" for Zen).
func (a *App) openLayer() string {
	if a.layers.rec.Phase == chord.Idle {
		return ""
	}
	return a.layers.rec.Layer
}

func (a *App) invalidate() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

// chordPress handles ⌥/Alt + A/C/X/S, from a terminal's OnKey, the open
// layer or the workbench root. It reports whether it took the key, so a
// taken chord sends nothing to the program (R0).
func (a *App) chordPress(mods ui.Modifiers, key ui.Key) bool {
	layer, ok := chordKeys[key]
	if !ok || mods&^ui.Shift != ui.Alt || a.dialogOpen() || a.closing {
		return false
	}
	if layer == layerControl {
		if a.openLayer() == layerControl {
			a.closeLayer()
		} else {
			a.showLayer(layerControl)
		}
		a.invalidate()
		return true
	}
	was := a.openLayer()
	a.layers.released = time.Time{}
	switch a.layers.rec.Press(layer, time.Now()) {
	case chord.Opened, chord.Retargeted:
		a.enterLayer(layer, was)
	case chord.Cycled:
		a.cycleLayer(1)
	case chord.Closed:
		a.closeLayer()
	}
	a.invalidate()
	return true
}

// showLayer opens layer sticky, as ⌘T, ⌘O, a click or the bar does.
func (a *App) showLayer(layer string) {
	if a.dialogOpen() || a.closing {
		return
	}
	was := a.openLayer()
	a.layers.rec.Show(layer)
	a.enterLayer(layer, was)
}

func (a *App) enterLayer(layer, was string) {
	l := &a.layers
	if was == "" && a.focusValue != focusLayer {
		l.returnTo = a.focusValue
	}
	if was == layerControl && layer != layerControl {
		// Another layer replaces the Control Center and its page.
		a.page, a.pendingPage, l.hosted = "", "", ""
	}

	l.search, l.query, l.searchHot = false, "", 0
	l.sel, l.shift, l.moved, l.pointerKnown = -1, false, false, false
	switch layer {
	case layerAgents:
		l.sel = a.defaultAgent()
		l.dock, l.dockClosing = ui.Point{}, false
	case layerProjects:
		a.ensureSlots()
	case layerControl:
		l.controlClosing = false
		l.control.Offset = ui.Point{}
		l.ccQuery = ""
	}
	a.focusValue = focusLayer
}

// closeLayer returns to Zen and gives the keyboard back. Closing the
// Control Center leaves its page; the sheet keeps drawing it on the way out.
func (a *App) closeLayer() {
	l := &a.layers
	if a.openLayer() == layerControl {
		a.page, a.pendingPage = "", ""
	}
	l.rec.Cancel()
	l.search = false
	back := l.returnTo
	l.returnTo = ""
	if isSessionFocus(back) {
		a.focusValue = back
	} else {
		a.restoreFocus()
	}
}

// watchChordRelease commits a held peek when ⌥/Alt goes up: every backend
// reports modifier changes (ModifiersChanged) and draws a frame for them.
func (a *App) watchChordRelease(c *ui.Context) {
	l := &a.layers
	mods := c.Modifiers()
	if !l.rec.Held() || mods&ui.Alt != 0 {
		return
	}
	layer := l.rec.Layer
	l.shift = mods&ui.Shift != 0
	// A key-up that let go of ⌥ timed the release as it happened; otherwise
	// (⌥ released on its own) this frame follows the change at once.
	at := time.Now()
	if !l.released.IsZero() {
		at, l.released = l.released, time.Time{}
	}
	if l.rec.ModifierReleased(at) == chord.Committed {
		a.commitLayer(layer)
	}
}

// noteKeyUp times a release of ⌥ that came with a key-up.
func (a *App) noteKeyUp(ev ui.InputEvent) {
	if ev.Kind == ui.InputKeyUp && ev.Mods&ui.Alt == 0 && a.layers.rec.Held() && a.layers.released.IsZero() {
		a.layers.released = time.Now()
	}
}

// noteMovement records pointer movement over the open layer: hover selects
// only after the pointer moves (≈4 DIP hysteresis), so a pointer resting
// where a wheel opens does not override the default selection.
func (a *App) noteMovement(x, y float32, ok bool) {
	l := &a.layers
	if !ok {
		return
	}
	p := ui.Point{X: x, Y: y}
	if l.pointerKnown && (abs32(p.X-l.pointer.X) > 4 || abs32(p.Y-l.pointer.Y) > 4) {
		l.moved = true
	}
	if !l.pointerKnown || l.moved {
		l.pointer, l.pointerKnown = p, true
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (a *App) selectLayerItem(i int) {
	if i != a.layers.sel {
		a.layers.sel = i
		a.layers.rec.Moved()
	}
}

// cycleLayer moves the selection, as arrows, Tab or the chord held do.
func (a *App) cycleLayer(dir int) {
	l := &a.layers
	switch a.openLayer() {
	case layerAgents:
		n := len(a.dockItems()) + 1 // + "New Agent"
		a.selectLayerItem(((max(l.sel, -1)+dir)%n + n) % n)
	case layerCreate:
		a.selectLayerItem(nextSector(4, l.sel, dir, func(int) bool { return true }))
	case layerProjects:
		a.selectLayerItem(nextSector(8, l.sel, dir, func(i int) bool { return i == 0 || a.slotProject(i) != nil }))
	}
}

func nextSector(n, from, dir int, enabled func(int) bool) int {
	i := from
	if i < 0 && dir < 0 {
		i = 0
	}
	for range n {
		i = ((i+dir)%n + n) % n
		if enabled(i) {
			return i
		}
	}
	return from
}

// layerKey is the open layer's keyboard: chords, Escape, Enter, arrows, Tab
// and 1–8. It swallows text so nothing types behind the layer.
func (a *App) layerKey(ev ui.InputEvent) bool {
	a.noteKeyUp(ev)
	switch ev.Kind {
	case ui.InputKeyDown:
		if a.chordPress(ev.Mods, ev.Key) {
			return true
		}
		if ev.Mods&^ui.Shift != 0 {
			return false
		}
		layer := a.openLayer()
		switch ev.Key {
		case ui.KeyEscape:
			a.closeLayer()
			return true
		case ui.KeyEnter:
			a.layers.shift = ev.Mods&ui.Shift != 0
			a.commitLayer(layer)
			return true
		}
		if layer == layerControl {
			return false // Tab and arrows move among the tiles
		}
		switch ev.Key {
		case ui.KeyRight, ui.KeyDown:
			a.cycleLayer(1)
			return true
		case ui.KeyLeft, ui.KeyUp:
			a.cycleLayer(-1)
			return true
		case ui.KeyTab:
			if ev.Mods&ui.Shift != 0 {
				a.cycleLayer(-1)
			} else {
				a.cycleLayer(1)
			}
			return true
		}
		if ev.Key >= ui.Key1 && ev.Key <= ui.Key8 && layer != layerAgents {
			a.selectLayerItem(int(ev.Key - ui.Key1))
			return true
		}
	case ui.InputText:
		return a.openLayer() != layerControl
	}
	return false
}

// rootKey is the workbench root's keyboard while it has the focus.
func (a *App) rootKey(ev ui.InputEvent) bool {
	a.noteKeyUp(ev)
	return ev.Kind == ui.InputKeyDown && a.chordPress(ev.Mods, ev.Key)
}

// commitLayer acts on the selection of layer.
func (a *App) commitLayer(layer string) {
	l := &a.layers
	switch layer {
	case layerAgents:
		items := a.dockItems()
		sel := l.sel
		a.closeLayer()
		if sel == len(items) {
			a.showLayer(layerCreate)
			return
		}
		if sel >= 0 && sel < len(items) {
			a.page = ""
			a.focusSession(items[sel].id)
		}
	case layerCreate:
		a.commitCreate()
	case layerProjects:
		a.commitProject()
	case layerControl:
		// Tiles act on their own click; Enter on a focused tile clicks it.
	}
}

// commitCreate starts a local Agent at once; fclaude and Advanced open the
// new-session dialog, which reviews remote and custom launches (doc §4.3).
// A missing tool also opens the dialog, which shows its setup help.
func (a *App) commitCreate() {
	sel, shift := a.layers.sel, a.layers.shift
	a.closeLayer()
	if sel < 0 {
		return
	}
	if a.project() == nil {
		a.showLayer(layerProjects)
		return
	}
	a.page = ""
	target := ""
	if shift {
		target = a.active
	}
	if sel < len(createTools) {
		a.newTool = createTools[sel]
	}
	a.newSession(target, false)
	if a.newOpen && sel < 2 && a.toolAvailable(a.newTool) {
		a.newOpen = false
		a.startSession()
	}
}

func (a *App) commitProject() {
	l := &a.layers
	if l.sel == 0 { // "··· more" becomes a search field and stays open
		l.rec.Show(layerProjects)
		l.search, l.query, l.searchHot = true, "", 0
		a.focusValue = focusLayer
		return
	}
	p := a.slotProject(l.sel)
	a.closeLayer()
	if p != nil {
		a.page = ""
		a.selectProject(p.ID)
	}
}

// slotProject is the project in wheel slot i (1–7), or nil.
func (a *App) slotProject(i int) *workspace.Project {
	if i < 1 {
		return nil
	}
	for j := range a.state.Projects {
		if a.state.Projects[j].Slot == i {
			return &a.state.Projects[j]
		}
	}
	return nil
}

// ensureSlots gives projects saved before wheel slots existed a free slot,
// in their list order, without evicting anyone.
func (a *App) ensureSlots() {
	changed := false
	for i := range a.state.Projects {
		used := 0
		for _, p := range a.state.Projects {
			if p.Slot != 0 {
				used++
			}
		}
		if used >= workspace.WheelSlots {
			break
		}
		if a.state.Projects[i].Slot == 0 {
			workspace.AssignSlot(a.state.Projects, i)
			changed = true
		}
	}
	if changed {
		a.persist()
	}
}

func (a *App) projectMatches() []workspace.Project {
	q := strings.ToLower(strings.TrimSpace(a.layers.query))
	var out []workspace.Project
	for _, p := range a.state.Projects {
		if q == "" || strings.Contains(strings.ToLower(p.Name+" "+p.Path), q) {
			out = append(out, p)
			if len(out) == 6 {
				break
			}
		}
	}
	return out
}

// submitProjectSearch opens the highlighted match, or the typed path.
func (a *App) submitProjectSearch() {
	matches := a.projectMatches()
	query := strings.TrimSpace(a.layers.query)
	hot := min(max(a.layers.searchHot, 0), max(len(matches)-1, 0))
	a.closeLayer()
	a.page = ""
	if len(matches) > 0 {
		a.selectProject(matches[hot].ID)
	} else if query != "" {
		a.addProject(query)
	}
}

// defaultAgent pre-selects the previously used Agent, as ⌘Tab does.
func (a *App) defaultAgent() int {
	items := a.dockItems()
	index := func(id string) int {
		for i, it := range items {
			if it.id == id {
				return i
			}
		}
		return -1
	}
	for _, id := range a.layers.recent {
		if id != a.active {
			if i := index(id); i >= 0 {
				return i
			}
		}
	}
	if i := index(a.active); i >= 0 {
		return i
	}
	return 0
}

// noteFocus keeps the most-recently-used order of focused sessions.
func (a *App) noteFocus(previous string) {
	if !isSessionFocus(previous) {
		return
	}
	l := &a.layers
	out := []string{previous}
	for _, id := range l.recent {
		if id != previous && len(out) < 16 {
			out = append(out, id)
		}
	}
	l.recent = out
}
