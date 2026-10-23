package theme

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo/ui"
)

// MotionDuration is the budget of the legacy curve transitions (sidebar,
// details, hover colour) that the R2 workbench replaces with the spring
// tokens below. It stays until those views are gone.
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

// Spring tokens of the redesign (doc §6.1), in mygo's SwiftUI-style
// perceptual parameters, identical to prototype-v4/physics.js. Bounce is
// reserved for releases that carry momentum (Flick); everything opened by a
// key, hovered or pressed settles without overshoot (apple-design §4).
var (
	Smooth = ui.Animation{Spring: ui.SpringOptions{Duration: 400 * time.Millisecond}}
	Snappy = ui.Animation{Spring: ui.SpringOptions{Duration: 280 * time.Millisecond}}
	Press  = ui.Animation{Spring: ui.SpringOptions{Duration: 120 * time.Millisecond}}
	Sheet  = ui.Animation{Spring: ui.SpringOptions{Duration: 360 * time.Millisecond}}
	Flick  = ui.Animation{Spring: ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.2}}
	Page   = ui.Animation{Spring: ui.SpringOptions{Duration: 340 * time.Millisecond}}
)

// ReducedFade is the cross-fade that replaces movement under reduced motion.
const ReducedFade = 160 * time.Millisecond

// Instant arrives on the next frame. mygo reads a zero Duration as its
// default 500 ms spring, not as "no animation", so an instant change needs
// a curve of its own.
var Instant = ui.Animation{Duration: time.Nanosecond, Ease: ui.Linear}

// Move returns a for something that moves (position, scale), or an instant
// change under reduced motion: a vestibular-safe jump (apple-design §14).
func Move(a ui.Animation, reduced bool) ui.Animation {
	if reduced {
		return Instant
	}
	return a
}

// Fade returns a for opacity or colour, or a short linear cross-fade under
// reduced motion: comprehension-aiding changes stay, overshoot goes.
func Fade(a ui.Animation, reduced bool) ui.Animation {
	if reduced {
		return ui.Animation{Duration: ReducedFade, Ease: ui.Linear}
	}
	return a
}

// Surfaces is the material policy of a frame, from the desktop's settings
// and the user's own (doc §6.3). Transparency off means solid surfaces and
// no blur; high contrast also drops the specular highlight and strengthens
// hairlines. mygo reports no system "reduce transparency" signal, so only
// the user setting and high contrast turn it on.
type Surfaces struct {
	ReduceMotion       bool
	ReduceTransparency bool
	HighContrast       bool
}

// SurfacePolicy reads the frame's surface policy.
func SurfacePolicy(c *ui.Context, reduceMotion, reduceTransparency bool) Surfaces {
	prefs := c.Preferences()
	return Surfaces{
		ReduceMotion:       reduceMotion || prefs.ReduceMotion,
		ReduceTransparency: reduceTransparency || prefs.HighContrast,
		HighContrast:       prefs.HighContrast,
	}
}

// Reduced reports whether either the user preference or the native desktop
// accessibility setting asks for less motion. mygo supplies ReduceMotion from
// macOS Reduce Motion, Windows animation settings, and Linux desktop settings.
func Reduced(c *ui.Context, userSetting bool) bool {
	return userSetting || c.Preferences().ReduceMotion
}

// Animation returns the workbench animation policy. mygo's native
// ReduceMotion handling intentionally keeps colour changes smooth, while this
// product setting must also stop decorative colour interpolation, so reduced
// motion is Instant (a zero Duration would be mygo's default spring).
func Animation(reduced bool) ui.Animation {
	if reduced {
		return Instant
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

// BlurBudget decides whether workspace blur is affordable on this window
// (redesign §7.1: frame-time P95 over budget → scrim instead of blur). It
// sees the intervals between paints while blur shows: at 60 Hz a frame
// that misses its vsync arrives ~33 ms after the last, so a P95 interval
// over BlurFrameLimit means more than one frame in twenty was dropped.
// Once over, it stays over for the window's life: switching back and
// forth would be worse than either.
type BlurBudget struct {
	mu       sync.Mutex
	last     time.Time
	samples  [64]time.Duration
	n        int
	exceeded atomic.Bool
}

const (
	// BlurFrameLimit is between one refresh (16.7 ms) and two (33 ms).
	BlurFrameLimit = 24 * time.Millisecond
	// blurMinSamples is enough intervals to judge an animation, not a hitch.
	blurMinSamples = 30
)

// Paint records a paint at now while blur is on screen. Paints within
// 2 ms are one frame painted twice; gaps over 100 ms are idle, not slow.
func (b *BlurBudget) Paint(now time.Time) {
	if b.exceeded.Load() {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	d := now.Sub(b.last)
	b.last = now
	if d < 2*time.Millisecond || d > 100*time.Millisecond {
		return
	}
	b.samples[b.n%len(b.samples)] = d
	b.n++
	if b.n < blurMinSamples {
		return
	}
	sorted := b.samples // a copy on the stack: Paint allocates nothing
	window := sorted[:min(b.n, len(sorted))]
	slices.Sort(window)
	if window[(len(window)*95+99)/100-1] > BlurFrameLimit {
		b.exceeded.Store(true)
	}
}

// Idle forgets the last paint, so the gap to the next blurred frame is not
// counted as one interval.
func (b *BlurBudget) Idle() {
	b.mu.Lock()
	b.last = time.Time{}
	b.mu.Unlock()
}

// Exceeded reports whether blur has proved too slow here.
func (b *BlurBudget) Exceeded() bool { return b.exceeded.Load() }
