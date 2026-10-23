package chrome_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

func find(t *testing.T, tt *ui.Tester, s string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q not shown; texts %q", s, tt.Texts())
	}
	return r
}

func env(reduced bool) chrome.Env {
	return chrome.Env{P: theme.PaletteFor("midnight"), S: theme.Surfaces{ReduceMotion: reduced}}
}

func TestButtonClicksAndPressesOnPointerDown(t *testing.T) {
	e := env(false)
	clicks := 0
	view := func(c *ui.Context) {
		ui.Column(c).Padding(40).Children(func() {
			if e.Button(c, "Confirm pause", chrome.Primary).Clicked() {
				clicks++
			}
		})
	}
	tt := ui.NewTester(view, 300, 200)
	if err := tt.Click("Confirm pause"); err != nil {
		t.Fatal(err)
	}
	if clicks != 1 {
		t.Fatalf("clicks = %d", clicks)
	}
	// Feedback starts on the press, before release (apple-design §1). The
	// press is a paint-only pose, so measure the painted fill, not layout.
	r := find(t, tt, "Confirm pause")
	fill := e.P.AccentFill
	edge := func() int {
		img, y := tt.Image(), int(r.Y+r.H/2)
		for x := 0; x < img.Bounds().Dx(); x++ {
			if c := img.RGBAAt(x, y); c.R == fill.R && c.G == fill.G && c.B == fill.B {
				return x
			}
		}
		return -1
	}
	rest := edge()
	tt.Press(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(200 * time.Millisecond)
	if held := edge(); held <= rest {
		t.Fatalf("pressed button did not scale down: edge %d → %d", rest, held)
	}
	tt.Release(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(time.Second)
	if back := edge(); back != rest {
		t.Fatalf("button did not return: edge %d → %d", rest, back)
	}
}

func TestSegmentedChoosesAndReportsChange(t *testing.T) {
	e := env(false)
	choice, changes := 0, 0
	view := func(c *ui.Context) {
		if e.Segmented(c, &choice, "Theme", "System", "Light", "Dark", "Kawaii") {
			changes++
		}
	}
	tt := ui.NewTester(view, 400, 100)
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if choice != 2 || changes != 1 {
		t.Fatalf("choice %d changes %d", choice, changes)
	}
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Fatalf("choosing the chosen segment reported a change")
	}
}

func sheetView(e chrome.Env, s *chrome.SheetState, progress *float32) func(c *ui.Context) {
	return func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			_, p := e.Sheet(c, s, 320, 300, nil, func() { ui.Text(c, "Control Center") })
			*progress = p
		})
	}
}

func TestSheetOpensClosesAndReportsProgress(t *testing.T) {
	var s chrome.SheetState
	var progress float32
	tt := ui.NewTester(sheetView(env(false), &s, &progress), 360, 340)
	tt.Frame()
	if s.IsOpen() || progress != 0 {
		t.Fatalf("new sheet open=%v progress=%v", s.IsOpen(), progress)
	}
	s.Open()
	tt.Frame()
	tt.Advance(100 * time.Millisecond)
	if progress <= 0 || progress >= 1 {
		t.Fatalf("100 ms into opening, progress = %v (want part-way: a spring, not a jump)", progress)
	}
	tt.Advance(900 * time.Millisecond)
	if !s.IsOpen() || progress < 0.999 {
		t.Fatalf("open sheet progress = %v", progress)
	}
	s.Close()
	tt.Frame()
	tt.Advance(900 * time.Millisecond)
	if s.IsOpen() || progress > 0.001 {
		t.Fatalf("closed sheet progress = %v", progress)
	}
}

func TestSheetUnderReducedMotionArrivesAtOnce(t *testing.T) {
	var s chrome.SheetState
	var progress float32
	tt := ui.NewTester(sheetView(env(true), &s, &progress), 360, 340)
	tt.Frame()
	s.Open()
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	if progress < 0.999 {
		t.Fatalf("reduced-motion sheet progress after one frame = %v", progress)
	}
}

func TestSheetDragDownClosesAndShortPullSnapsBack(t *testing.T) {
	var s chrome.SheetState
	var progress float32
	tt := ui.NewTester(sheetView(env(false), &s, &progress), 360, 340)
	tt.Frame()
	s.Open()
	tt.Frame()
	tt.Advance(900 * time.Millisecond)
	title := find(t, tt, "Control Center")
	x, y := title.X+title.W/2, title.Y+title.H/2
	// A short, slow pull does not reach half way: it snaps back open.
	tt.Press(x, y)
	for i := 1; i <= 6; i++ {
		tt.Move(x, y+float32(i)*10)
		tt.Advance(60 * time.Millisecond)
	}
	tt.Advance(300 * time.Millisecond) // rest, so release carries no velocity
	tt.Release(x, y+60)
	tt.Advance(900 * time.Millisecond)
	if !s.IsOpen() {
		t.Fatal("short pull closed the sheet")
	}
	// A long pull past half closes it, following the pointer 1:1 on the way.
	tt.Press(x, y)
	tt.Move(x, y+100)
	tt.Advance(16 * time.Millisecond)
	if moved := find(t, tt, "Control Center"); moved.Y-title.Y < 90 || moved.Y-title.Y > 110 {
		t.Fatalf("sheet does not track the pointer 1:1: moved %v for 100", moved.Y-title.Y)
	}
	tt.Move(x, y+200)
	tt.Advance(300 * time.Millisecond)
	tt.Release(x, y+200)
	tt.Advance(900 * time.Millisecond)
	if s.IsOpen() || progress > 0.001 {
		t.Fatalf("long pull left the sheet open (progress %v)", progress)
	}
}

func TestWheelViewReportsSectorUnderPointer(t *testing.T) {
	e := env(false)
	under := -1
	sectors := []chrome.Sector{{Enabled: true}, {Enabled: true}, {Enabled: true}, {Enabled: false}}
	view := func(c *ui.Context) {
		ui.Box(c).Fill().Children(func() {
			_, under = e.WheelView(c, chrome.CreateWheel, sectors, -1, false, func(int) {}, 0, nil)
		})
	}
	tt := ui.NewTester(view, 490, 490)
	c := chrome.WheelSize / 2
	for want, p := range map[int]chrome.Point{0: {X: 0, Y: -170}, 1: {X: 170, Y: 0}, 2: {X: 0, Y: 170}} {
		tt.Move(c+p.X, c+p.Y)
		tt.Frame()
		if under != want {
			t.Fatalf("pointer at %v: under = %d, want %d", p, under, want)
		}
	}
	tt.Move(c-170, c) // the empty slot never takes the pointer
	tt.Frame()
	if under != -1 {
		t.Fatalf("empty sector selected: %d", under)
	}
	tt.Move(c, c) // the HUD owns the centre
	tt.Frame()
	if under != -1 {
		t.Fatalf("centre selected: %d", under)
	}
}
