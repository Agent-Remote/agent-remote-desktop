package theme

import "time"

// BackgroundPolicy controls the inexpensive workbench-only decoration layer.
// It deliberately has no terminal rendering state: terminal panes keep their
// own opaque background and never participate in this animation.
type BackgroundPolicy struct {
	Enabled  bool
	Animated bool
	Opacity  float32
}

// BackgroundDecor returns the policy for the current workbench state. Focus
// mode removes decoration entirely; reduced motion leaves a quiet, static
// accent so the setting does not make the workbench lose its visual anchor.
func BackgroundDecor(reducedMotion, focusMode bool) BackgroundPolicy {
	if focusMode {
		return BackgroundPolicy{}
	}
	return BackgroundPolicy{
		Enabled:  true,
		Animated: !reducedMotion,
		// Keep contrast safely below UI text and terminal content. This is an
		// upper bound for both dark and light themes, not a paint instruction.
		Opacity: 0.045,
	}
}

// BackgroundMotionDuration uses the shared motion budget. The decoration is
// a one-shot intro, so an idle workbench does not keep scheduling frames.
const BackgroundMotionDuration = MotionDuration

// BackgroundMotionPeriod is retained as a named policy value for diagnostics
// and tests that need to describe the decoration's bounded cadence.
const BackgroundMotionPeriod = 180 * time.Millisecond
