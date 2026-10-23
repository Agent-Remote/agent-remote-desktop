package theme

import (
	"slices"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

// MotionDuration is the single budget for decorative workbench transitions.
// Keeping the budget here makes it possible to audit every animation without
// affecting terminal rendering, which must remain stable and readable.
const MotionDuration = 180 * time.Millisecond

// FrameBudget is the target time for a smooth 60 Hz frame. It is intentionally
// a documented target rather than a hard runtime deadline: terminal output and
// native window scheduling can legitimately exceed it for an individual frame.
const FrameBudget = 16 * time.Millisecond

// FrameSampleWindow is the number of most-recent view-build samples retained
// for percentile diagnostics. The samples are kept in a fixed atomic ring so
// Observe never allocates on the UI thread. A 512-frame window is roughly
// eight seconds at 60 Hz and is long enough to include a transition without
// allowing an old startup spike to hide current input latency.
const FrameSampleWindow = 512

// FrameStats is a read-only snapshot of UI build timing. It intentionally
// measures the Go/mygo view-build phase only; GPU paint and the window manager
// remain platform responsibilities and require native host validation.
type FrameStats struct {
	Frames      uint64
	SlowFrames  uint64
	SampleCount uint64
	Total       time.Duration
	Max         time.Duration
	P95         time.Duration
}

// FrameObserver records lightweight frame timing without allocating on the UI
// thread. It is useful for diagnostics and tests while keeping the terminal
// rendering path independent from animation state.
type FrameObserver struct {
	frames     atomic.Uint64
	slowFrames atomic.Uint64
	totalNanos atomic.Uint64
	maxNanos   atomic.Uint64
	samples    [FrameSampleWindow]atomic.Uint64
}

// Observe records one view-build duration. Negative durations are ignored so
// callers using an injected clock cannot corrupt the counters.
func (o *FrameObserver) Observe(duration time.Duration) {
	if duration < 0 {
		return
	}
	nanos := uint64(duration)
	frame := o.frames.Add(1)
	o.totalNanos.Add(nanos)
	o.samples[(frame-1)%FrameSampleWindow].Store(nanos)
	if duration > FrameBudget {
		o.slowFrames.Add(1)
	}
	for {
		old := o.maxNanos.Load()
		if nanos <= old || o.maxNanos.CompareAndSwap(old, nanos) {
			return
		}
	}
}

// Snapshot returns a consistent-enough point-in-time view for diagnostics;
// individual counters may advance between the atomic reads.
func (o *FrameObserver) Snapshot() FrameStats {
	frames := o.frames.Load()
	sampleCount := frames
	if sampleCount > FrameSampleWindow {
		sampleCount = FrameSampleWindow
	}
	var p95 time.Duration
	if sampleCount > 0 {
		samples := make([]uint64, sampleCount)
		start := uint64(0)
		if frames > FrameSampleWindow {
			start = frames - FrameSampleWindow
		}
		for i := range samples {
			samples[i] = o.samples[(start+uint64(i))%FrameSampleWindow].Load()
		}
		slices.Sort(samples)
		// Use the nearest-rank definition: P95 is the smallest sample
		// greater than or equal to 95% of the observations. It is an exact
		// percentile for the retained window, subject only to concurrent
		// writes while a diagnostic snapshot is being collected.
		index := (sampleCount*95+99)/100 - 1
		p95 = time.Duration(samples[index])
	}
	return FrameStats{
		Frames:      frames,
		SlowFrames:  o.slowFrames.Load(),
		SampleCount: sampleCount,
		Total:       time.Duration(o.totalNanos.Load()),
		Max:         time.Duration(o.maxNanos.Load()),
		P95:         p95,
	}
}

// Reduced reports whether either the user preference or the native desktop
// accessibility setting asks for less motion. mygo supplies ReduceMotion from
// macOS Reduce Motion, Windows animation settings, and Linux desktop settings.
func Reduced(c *ui.Context, userSetting bool) bool {
	return userSetting || c.Preferences().ReduceMotion
}

// Animation returns the workbench animation policy. A zero-duration animation
// is important here: mygo's native ReduceMotion handling intentionally keeps
// color changes smooth, while this product setting must also stop decorative
// color interpolation immediately.
func Animation(reduced bool) ui.Animation {
	if reduced {
		return ui.Animation{Duration: 0, Ease: ui.Linear}
	}
	return ui.Animation{Duration: MotionDuration, Ease: ui.CubicBezier(.22, 1, .36, 1)}
}

// Transition returns the standard sidebar/detail transition. A nil result is
// an explicit signal to skip Transition entirely in reduced-motion mode; an
// empty ElementTransition would still use mygo's default 200 ms duration.
func Transition(reduced bool) *ui.ElementTransition {
	if reduced {
		return nil
	}
	a := Animation(false)
	return &ui.ElementTransition{Animation: &a, Enter: &ui.Motion{X: 8}}
}
