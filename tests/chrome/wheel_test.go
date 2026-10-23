package chrome_test

import (
	"math"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
)

func bezier(c chrome.Cubic, t float32) chrome.Point {
	u := 1 - t
	a, b, d, e := u*u*u, 3*u*u*t, 3*u*t*t, t*t*t
	return chrome.Point{
		X: a*c.P0.X + b*c.C1.X + d*c.C2.X + e*c.P3.X,
		Y: a*c.P0.Y + b*c.C1.Y + d*c.C2.Y + e*c.P3.Y,
	}
}

// The design doc promises < 0.1 DIP radial error for the drawn sectors.
func TestArcRadialErrorBelowTenthOfDIP(t *testing.T) {
	for _, w := range []chrome.Wheel{chrome.CreateWheel, chrome.ProjectWheel} {
		for i := 0; i < w.Count; i++ {
			outer, inner := w.SectorOutline(i)
			for _, part := range []struct {
				r    float32
				segs []chrome.Cubic
			}{{w.Outer, outer}, {w.Inner, inner}} {
				worst := 0.0
				for _, seg := range part.segs {
					for s := 0; s <= 200; s++ {
						p := bezier(seg, float32(s)/200)
						worst = math.Max(worst, math.Abs(math.Hypot(float64(p.X), float64(p.Y))-float64(part.r)))
					}
				}
				if worst >= 0.1 {
					t.Fatalf("wheel %d sector %d r=%v: radial error %.4f DIP", w.Count, i, part.r, worst)
				}
			}
		}
	}
}

func TestArcEndpointsMatchSectorSpan(t *testing.T) {
	w := chrome.ProjectWheel
	start, end := w.SectorSpan(3)
	outer, inner := w.SectorOutline(3)
	near := func(a, b chrome.Point) bool { return math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)) < 1e-3 }
	if !near(outer[0].P0, chrome.Polar(w.Outer, start)) || !near(outer[len(outer)-1].P3, chrome.Polar(w.Outer, end)) {
		t.Fatal("outer arc does not span the sector clockwise")
	}
	if !near(inner[0].P0, chrome.Polar(w.Inner, end)) || !near(inner[len(inner)-1].P3, chrome.Polar(w.Inner, start)) {
		t.Fatal("inner arc does not return counter-clockwise")
	}
}

func TestArcSplitsLongSpans(t *testing.T) {
	if n := len(chrome.Arc(100, 0, 270)); n != 3 {
		t.Fatalf("270° arc: %d segments, want 3", n)
	}
	if n := len(chrome.Arc(100, 10, 10)); n != 1 {
		t.Fatalf("zero arc: %d segments, want 1", n)
	}
}

func TestHitMatchesPrototypeLayout(t *testing.T) {
	w := chrome.ProjectWheel
	for i := 0; i < w.Count; i++ {
		start, end := w.SectorSpan(i)
		p := chrome.Polar((w.Inner+w.Outer)/2, (start+end)/2)
		if got := w.Hit(p.X, p.Y, false); got != i {
			t.Fatalf("centre of sector %d hit %d", i, got)
		}
	}
	// 12 o'clock is "··· 更多" (index 0), 3 o'clock is index 2 on the 8-wheel.
	if got := w.Hit(0, -180, false); got != 0 {
		t.Fatalf("12 o'clock = %d", got)
	}
	if got := w.Hit(180, 0, false); got != 2 {
		t.Fatalf("3 o'clock = %d", got)
	}
	if got := chrome.CreateWheel.Hit(-170, 0, false); got != 3 {
		t.Fatalf("create wheel 9 o'clock = %d", got)
	}
}

func TestHitDeadZoneAndReach(t *testing.T) {
	w := chrome.CreateWheel
	if got := w.Hit(0, -50, true); got != -1 {
		t.Fatalf("inside HUD selected %d", got)
	}
	if got := w.Hit(0, -400, false); got != -1 {
		t.Fatalf("far click selected %d", got)
	}
	if got := w.Hit(0, -400, true); got != 0 {
		t.Fatalf("held flick toward 12 o'clock selected %d", got)
	}
	if got := w.Hit(0, -w.Outer*1.11, false); got != 0 {
		t.Fatalf("click within rim padding selected %d", got)
	}
}

func TestHitWrapsBoundaryAngles(t *testing.T) {
	w := chrome.ProjectWheel
	// Just left of 12 o'clock is still sector 0 (it spans −22.5°…22.5°).
	p := chrome.Polar(180, 359)
	if got := w.Hit(p.X, p.Y, false); got != 0 {
		t.Fatalf("359° = %d", got)
	}
	p = chrome.Polar(180, 337)
	if got := w.Hit(p.X, p.Y, false); got != 7 {
		t.Fatalf("337° = %d", got)
	}
}
