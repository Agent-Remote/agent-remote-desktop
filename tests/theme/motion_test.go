package theme_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
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
	if got := theme.Animation(true); got.Duration != 0 {
		t.Fatalf("reduced-motion animation duration = %s, want zero", got.Duration)
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
