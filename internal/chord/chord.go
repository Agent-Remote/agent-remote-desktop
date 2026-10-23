// Package chord recognizes the workbench's held-modifier layer gestures
// (⌥A / ⌥C / ⌥X): hold to peek, release the modifier to commit, tap to keep
// the layer open. It is pure logic driven by key and modifier observations so
// every platform backend and the native probes share one state machine.
package chord

import "time"

// TapWindow is the longest press that still counts as a tap.
const TapWindow = 250 * time.Millisecond

// Phase is where a gesture is.
type Phase uint8

const (
	Idle   Phase = iota
	Peek         // opened by a held chord; releasing the modifier commits
	Sticky       // stays open until an explicit commit, Escape or the chord again
)

// Outcome is what an observation means for the view.
type Outcome uint8

const (
	None       Outcome = iota
	Opened             // layer opened in Peek
	Cycled             // same chord again while held: advance the selection
	Retargeted         // different chord while held: switch layer mid-flight
	Committed          // modifier released after a hold: commit the selection
	MadeSticky         // modifier released after a quick tap: keep it open
	Closed             // chord pressed again on a sticky layer, or Cancel
)

// Recognizer tracks one gesture. Layer is an opaque caller value (for example
// "agents"); the zero Recognizer is Idle.
type Recognizer struct {
	Phase Phase
	Layer string
	held  bool
	start time.Time
	moved bool
}

// Press reports a chord key-down for layer at now. Auto-repeat must be
// filtered by the caller.
func (r *Recognizer) Press(layer string, now time.Time) Outcome {
	switch {
	case r.held && r.Layer == layer:
		r.moved = true
		return Cycled
	case r.held:
		r.Layer, r.Phase, r.start, r.moved = layer, Peek, now, false
		return Retargeted
	case r.Phase == Sticky && r.Layer == layer:
		r.reset()
		return Closed
	}
	r.Layer, r.Phase, r.held, r.start, r.moved = layer, Peek, true, now, false
	return Opened
}

// Moved records that the selection changed while held (pointer or keys), so
// a fast release still commits rather than becoming sticky.
func (r *Recognizer) Moved() {
	if r.held {
		r.moved = true
	}
}

// ModifierReleased reports that the chord modifier is no longer held at now.
// It is idempotent: a second release, or one while nothing is held, is None.
func (r *Recognizer) ModifierReleased(now time.Time) Outcome {
	if !r.held {
		return None
	}
	r.held = false
	if r.Phase != Peek {
		return None
	}
	if now.Sub(r.start) < TapWindow && !r.moved {
		r.Phase = Sticky
		return MadeSticky
	}
	r.reset()
	return Committed
}

// FocusLost converts a held peek to sticky: losing the window must never
// commit on the user's behalf.
func (r *Recognizer) FocusLost() Outcome {
	if !r.held {
		return None
	}
	r.held = false
	if r.Phase == Peek {
		r.Phase = Sticky
		return MadeSticky
	}
	return None
}

// Show opens layer sticky without a held chord, as ⌘T, a click or a menu
// does. It replaces any gesture in progress.
func (r *Recognizer) Show(layer string) {
	*r = Recognizer{Phase: Sticky, Layer: layer}
}

// Cancel closes the gesture (Escape, click outside, explicit commit).
func (r *Recognizer) Cancel() Outcome {
	if r.Phase == Idle {
		return None
	}
	r.reset()
	return Closed
}

// Held reports whether the chord modifier is still held for this gesture.
func (r *Recognizer) Held() bool { return r.held }

func (r *Recognizer) reset() { *r = Recognizer{} }
