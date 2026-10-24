package app_test

import (
	"image"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// tapChord opens a layer sticky, as a quick ⌥ + letter does.
func tapChord(tt *ui.Tester, key ui.Key) {
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, key)
	tt.HoldModifiers(0)
	settle(tt)
}

// assertInside fails for any text shown outside the window. Text clipped
// away (a scroll container's hidden rows) reports no size and is skipped.
func assertInside(t *testing.T, tt *ui.Tester, state string, w, h float32) {
	t.Helper()
	for _, s := range tt.Texts() {
		r, ok := tt.Find(s)
		if !ok || r.W <= 0 || r.H <= 0 {
			continue
		}
		if r.X < -1 || r.Y < -1 || r.X+r.W > w+1 || r.Y+r.H > h+1 {
			t.Errorf("%s: %q is outside the %v×%v window: %+v", state, s, w, h, r)
		}
	}
}

// Redesign §10.1/§R4: at the 900×600 minimum, in English, Chinese and the
// en-XA long pseudo-locale, every layer keeps its text inside the window.
func TestLayersFitTheMinimumWindowInEveryLanguage(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			_, tt, _ := layersTester(t)
			i18n.SetLocaleFunc(func() string { return language })
			tt.SetLocale(language)
			tt.SetSize(900, 600)
			settle(tt)
			if !tt.HasText(locales.OpenControlCenter()) {
				t.Fatalf("the %s locale did not apply: %q", language, tt.Texts())
			}
			assertInside(t, tt, "zen", 900, 600)
			for _, layer := range []struct {
				name  string
				key   ui.Key
				shows string
			}{{"dock", ui.KeyA, locales.NewAgent()}, {"create wheel", ui.KeyC, locales.CreateAdvanced()}, {"project wheel", ui.KeyX, locales.ProjectMore()}, {"control center", ui.KeyS, locales.ControlCenterSystem()}} {
				tapChord(tt, layer.key)
				if !tt.HasText(layer.shows) {
					t.Fatalf("%s did not open: %q", layer.name, tt.Texts())
				}
				assertInside(t, tt, layer.name, 900, 600)
				tt.Key(0, ui.KeyEscape)
				settle(tt)
			}
			tt.Key(ui.Cmd, ui.KeyO) // the project wheel's search
			settle(tt)
			assertInside(t, tt, "project search", 900, 600)
			tt.Key(0, ui.KeyEscape)
			settle(tt)
			for _, page := range []string{locales.DiagnosticsTitle(), locales.Settings(), locales.ProjectDetails(), locales.OperationsSidebar()} {
				if err := openPage(tt, page); err != nil {
					t.Fatal(err)
				}
				if !pageShown(tt) {
					t.Fatalf("%s did not open", page)
				}
				assertInside(t, tt, page, 900, 600)
			}
		})
	}
}

// Redesign §10.1, §6.3: every layer and the Control Center work from the
// keyboard alone.
func TestEveryLayerWorksFromTheKeyboard(t *testing.T) {
	_, tt, _ := layersTester(t)

	// ⌥A: arrows choose, Enter switches.
	before := tt.HasText(locales.BarAgent("claude"))
	tapChord(tt, ui.KeyA)
	tt.Key(0, ui.KeyRight)
	settle(tt)
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if dockOpen(tt) || !before || tt.HasText(locales.BarAgent("claude")) {
		t.Fatalf("the dock did not switch Agent from the keyboard: %q", tt.Texts())
	}

	// ⌥C: 4 picks "Advanced…", Enter opens the full dialog, Esc leaves it.
	tapChord(tt, ui.KeyC)
	tt.Key(0, ui.Key4)
	settle(tt)
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if !tt.HasText(locales.NewAgentSession()) {
		t.Fatalf("⌥C 4 Enter did not open the new-session dialog: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)

	// ⌥X: 1 is "more", typing searches, Enter opens the match.
	tapChord(tt, ui.KeyX)
	tt.Key(0, ui.Key1)
	settle(tt)
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	tt.Type("Second")
	settle(tt)
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if !tt.HasText(locales.BarProject("Second project")) {
		t.Fatalf("⌥X search did not switch project from the keyboard: %q", tt.Texts())
	}

	// ⌥S: Tab reaches a tile, Enter pushes its page, Esc goes back.
	tapChord(tt, ui.KeyS)
	for i := 0; i < 60 && !tt.Focused(locales.DiagnosticsTitle()); i++ {
		tt.Key(0, ui.KeyTab)
		settle(tt)
	}
	if !tt.Focused(locales.DiagnosticsTitle()) {
		t.Fatal("Tab never reached the Diagnostics tile")
	}
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if !pageShown(tt) || !tt.HasText(locales.DiagnosticsHelp()) {
		t.Fatalf("Enter on a tile did not push its page: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if pageShown(tt) || !controlCenterOpen(tt) {
		t.Fatal("Esc did not return from the page")
	}
}

// solidTester is layersTester with reduced transparency and no panes
// open, so a sheet is the only surface over the canvas and its painted
// edge is exact.
func solidTester(t *testing.T) *ui.Tester {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.Preferences.ReduceTransparency = true
	state.Board.Tabs, state.Board.Active = nil, nil // the project's empty state: no panes
	testutil.StoreWorkspace(t, home, state)
	providers := &providerFixture{available: map[string]bool{"codex": true, "claude": true, "fclaude": true}, directory: t.TempDir()}
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: providers.resolve, ChordTapWindow: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return ui.NewTester(a.View, 1366, 768)
}

// Redesign §10.2: a sheet closed and reopened half way moves on from where
// it shows, frame after frame — no jump back to an entrance and no frame
// without it (the hand-over from a leaving copy to a new one). Run at full
// motion in dark with solid surfaces, reading each sheet's painted edge
// every 16 ms inside its padding (no cards, no text).
func TestSheetsReverseMidFlightWithoutJumping(t *testing.T) {
	surface := func(img *image.RGBA, x, y int) bool { return img.RGBAAt(x, y).R >= 20 }
	sheetTop := func(img *image.RGBA) (float32, bool) {
		for y := 0; y < 768; y++ {
			if surface(img, 150, y) {
				return float32(y), true
			}
		}
		return 0, false
	}
	dockBottom := func(img *image.RGBA) (float32, bool) {
		y := 0
		for y < 768 && surface(img, 30, y) { // the dock's side padding
			y++
		}
		return float32(y), y > 0
	}
	for _, layer := range []struct {
		name string
		key  ui.Key
		edge func(*image.RGBA) (float32, bool)
	}{{"control center", ui.KeyS, sheetTop}, {"dock", ui.KeyA, dockBottom}} {
		t.Run(layer.name, func(t *testing.T) {
			tt := solidTester(t)
			tt.SetPreferences(ui.Preferences{TextScale: 1}) // springs on
			tt.SetDark(true)
			tt.Frame()
			var ys []float32
			reversing := false
			sample := func(frames int) {
				for i := 0; i < frames; i++ {
					tt.Advance(16 * time.Millisecond)
					y, ok := layer.edge(tt.Image())
					if !ok && reversing {
						t.Fatalf("%s vanished for a frame mid-flight: %v", layer.name, ys)
					}
					if ok {
						ys = append(ys, y)
					}
				}
			}
			tap := func() { // as tapChord, without settle's reduce-motion
				tt.HoldModifiers(ui.Alt)
				tt.Key(ui.Alt, layer.key)
				tt.HoldModifiers(0)
			}
			tap()
			sample(40) // open and at rest
			if len(ys) < 2 || ys[0] == ys[len(ys)-1] {
				t.Fatalf("%s did not move in: %v", layer.name, ys)
			}
			open := ys[len(ys)-1]
			reversing = true // from here the sheet is on screen in every frame
			tt.Key(0, ui.KeyEscape)
			sample(5) // part-way out
			if mid := ys[len(ys)-1]; abs(mid-open) < 20 {
				t.Fatalf("%s did not start leaving: %v", layer.name, ys)
			}
			tap() // reverse
			sample(40)
			if last := ys[len(ys)-1]; abs(last-open) > 1 {
				t.Fatalf("%s did not come back to rest open: %v", layer.name, ys)
			}
			for i := 1; i < len(ys); i++ {
				if d := abs(ys[i] - ys[i-1]); d > 90 {
					t.Fatalf("%s jumped %v DIP in one frame: %v", layer.name, d, ys)
				}
			}
		})
	}
}

func abs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// Redesign §6.2: while the Control Center is dragged, the workspace's
// depth follows the drag (one frame behind: it is built first) rather
// than staying pushed back until the sheet closes. The left pane edge
// moves in as the workspace scales back (14 DIP in Zen, 54 open).
func TestWorkspaceDepthFollowsTheDrag(t *testing.T) {
	_, tt, _ := layersTester(t)
	tt.SetPreferences(ui.Preferences{TextScale: 1})
	tt.SetDark(true)
	paneEdge := func() int {
		img := tt.Image()
		base := img.RGBAAt(2, 400).R
		for x := 0; x < 200; x++ {
			if d := int(img.RGBAAt(x, 400).R) - int(base); d > 3 || d < -3 {
				return x
			}
		}
		return -1
	}
	tt.Advance(time.Second)
	zen := paneEdge()
	if err := tt.Click(locales.OpenControlCenter()); err != nil {
		t.Fatal(err)
	}
	tt.Advance(time.Second)
	open := paneEdge()
	if open <= zen+20 {
		t.Fatalf("opening did not push the workspace back: edge %d → %d", zen, open)
	}
	x, y := float32(683), float32(141) // the sheet's grabber
	tt.Press(x, y)
	tt.Move(x, y+10)
	tt.Advance(16 * time.Millisecond)
	tt.Move(x, y+250)
	tt.Advance(16 * time.Millisecond)
	tt.Advance(16 * time.Millisecond)
	if mid := paneEdge(); mid <= zen+2 || mid >= open-2 {
		t.Fatalf("mid-drag depth did not follow: zen %d, mid %d, open %d", zen, mid, open)
	}
	tt.Move(x, y+10)
	tt.Advance(16 * time.Millisecond)
	tt.Advance(16 * time.Millisecond)
	if back := paneEdge(); back < open-3 {
		t.Fatalf("dragging back did not push the workspace back again: %d, open %d", back, open)
	}
	tt.Release(x, y+10)
}
