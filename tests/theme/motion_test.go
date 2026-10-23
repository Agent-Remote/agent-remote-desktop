package theme_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

func TestMotionPolicyHasOneBoundedBudget(t *testing.T) {
	if theme.MotionDuration <= 0 || theme.MotionDuration > 200*time.Millisecond {
		t.Fatalf("decorative animation exceeds the 200 ms budget: %s", theme.MotionDuration)
	}
	if theme.FrameBudget != 16*time.Millisecond {
		t.Fatalf("unexpected 60 Hz frame budget: %s", theme.FrameBudget)
	}
	if got := theme.Animation(false); got.Duration != theme.MotionDuration {
		t.Fatalf("normal animation duration = %s, want %s", got.Duration, theme.MotionDuration)
	}
}

func TestReducedMotionDisablesDecorativeTransitions(t *testing.T) {
	// A zero Duration would be mygo's default 500 ms spring; reduced motion
	// must be a curve that arrives on the next frame.
	if got := theme.Animation(true); got.Duration <= 0 || got.Duration > time.Millisecond {
		t.Fatalf("reduced-motion animation = %+v, want an instant curve", got)
	}
	if transition := theme.Transition(true); transition != nil {
		t.Fatal("reduced-motion policy still returned a decorative transition")
	}
	if transition := theme.Transition(false); transition == nil || transition.Animation == nil {
		t.Fatal("normal motion policy did not return the shared animation")
	}
}

func TestBackgroundDecorationIsBoundedAndStopsWhenRequested(t *testing.T) {
	animated := theme.BackgroundDecor(false, false)
	if !animated.Enabled || !animated.Animated {
		t.Fatalf("normal background decoration = %+v, want enabled animation", animated)
	}
	if animated.Opacity <= 0 || animated.Opacity > 0.05 {
		t.Fatalf("background opacity = %f, want a low contrast value", animated.Opacity)
	}
	if theme.BackgroundMotionDuration > theme.MotionDuration {
		t.Fatalf("background duration = %s exceeds shared motion budget %s", theme.BackgroundMotionDuration, theme.MotionDuration)
	}
	if reduced := theme.BackgroundDecor(true, false); !reduced.Enabled || reduced.Animated {
		t.Fatalf("reduced-motion background decoration = %+v, want static decoration", reduced)
	}
	if focused := theme.BackgroundDecor(false, true); focused.Enabled {
		t.Fatalf("focus-mode background decoration = %+v, want disabled", focused)
	}
}

func TestFrameObserverCountsBudgetBreaches(t *testing.T) {
	var observer theme.FrameObserver
	observer.Observe(4 * time.Millisecond)
	observer.Observe(theme.FrameBudget + time.Millisecond)
	observer.Observe(-time.Millisecond)
	stats := observer.Snapshot()
	if stats.Frames != 2 || stats.SlowFrames != 1 {
		t.Fatalf("unexpected frame stats: %+v", stats)
	}
	if stats.Max != theme.FrameBudget+time.Millisecond {
		t.Fatalf("max frame = %s, want %s", stats.Max, theme.FrameBudget+time.Millisecond)
	}
	if stats.Total != 4*time.Millisecond+theme.FrameBudget+time.Millisecond {
		t.Fatalf("total frame time = %s", stats.Total)
	}
	if stats.SampleCount != 2 || stats.P95 != theme.FrameBudget+time.Millisecond {
		t.Fatalf("sample window statistics = %+v, want two samples and p95=%s", stats, theme.FrameBudget+time.Millisecond)
	}
}

func TestFrameObserverP95UsesRecentBoundedWindow(t *testing.T) {
	var observer theme.FrameObserver
	for i := 1; i <= theme.FrameSampleWindow+20; i++ {
		observer.Observe(time.Duration(i) * time.Millisecond)
	}
	stats := observer.Snapshot()
	if stats.Frames != theme.FrameSampleWindow+20 || stats.SampleCount != theme.FrameSampleWindow {
		t.Fatalf("unexpected rolling sample counts: %+v", stats)
	}
	// The retained window is 21ms..532ms. Nearest-rank P95 is the 487th
	// element (507ms), which proves old startup samples do not affect the
	// diagnostic percentile.
	if stats.P95 != 507*time.Millisecond {
		t.Fatalf("recent-window p95 = %s, want 507ms", stats.P95)
	}
}

// Doc §6.1: bounce only where a release carried momentum.
func TestSpringTokensOnlyFlickBounces(t *testing.T) {
	for name, a := range map[string]ui.Animation{"Smooth": theme.Smooth, "Snappy": theme.Snappy, "Press": theme.Press, "Sheet": theme.Sheet, "Page": theme.Page} {
		if a.Spring.Bounce != 0 || a.Spring.Duration <= 0 || a.Duration != 0 {
			t.Fatalf("%s must be a critically damped spring: %+v", name, a)
		}
	}
	if theme.Flick.Spring.Bounce <= 0 || theme.Flick.Spring.Bounce > 0.25 {
		t.Fatalf("Flick bounce = %v", theme.Flick.Spring.Bounce)
	}
}

func TestReducedMotionJumpsMovesAndCrossFadesFades(t *testing.T) {
	if got := theme.Move(theme.Smooth, true); got.Duration <= 0 || got.Duration > time.Millisecond {
		t.Fatalf("reduced Move = %+v, want an instant curve", got)
	}
	if got := theme.Fade(theme.Smooth, true); got.Duration != theme.ReducedFade {
		t.Fatalf("reduced Fade = %+v, want %s cross-fade", got, theme.ReducedFade)
	}
	if theme.Move(theme.Sheet, false).Spring != theme.Sheet.Spring || theme.Fade(theme.Snappy, false).Spring != theme.Snappy.Spring {
		t.Fatal("tokens must pass through unchanged without reduced motion")
	}
}

// Redesign §7.1: blur gives way to the scrim when paints run late.
func TestBlurBudget(t *testing.T) {
	paint := func(b *theme.BlurBudget, start time.Time, n int, every time.Duration) time.Time {
		for i := 0; i < n; i++ {
			start = start.Add(every)
			b.Paint(start)
		}
		return start
	}
	now := time.Unix(1000, 0)

	var smooth theme.BlurBudget
	paint(&smooth, now, 200, 16700*time.Microsecond)
	if smooth.Exceeded() {
		t.Fatal("60 Hz frames exceeded the blur budget")
	}

	var idle theme.BlurBudget
	at := now
	for i := 0; i < 40; i++ {
		at = paint(&idle, at, 3, 16*time.Millisecond).Add(2 * time.Second) // short bursts, long rests
	}
	if idle.Exceeded() {
		t.Fatal("idle gaps between animations counted as slow frames")
	}

	var hitch theme.BlurBudget
	at = paint(&hitch, now, 60, 16*time.Millisecond)
	paint(&hitch, at, 2, 40*time.Millisecond) // one hitch is no trend
	if hitch.Exceeded() {
		t.Fatal("a single hitch disabled blur")
	}

	var slow theme.BlurBudget
	paint(&slow, now, 40, 33*time.Millisecond)
	if !slow.Exceeded() {
		t.Fatal("30 Hz frames did not exceed the blur budget")
	}
	paint(&slow, now.Add(time.Hour), 200, 16*time.Millisecond)
	if !slow.Exceeded() {
		t.Fatal("the decision flapped back to blur")
	}
}
