package theme_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
)

func TestBackgroundDecorationIsBoundedAndReadable(t *testing.T) {
	for _, test := range []struct {
		name     string
		reduced  bool
		focus    bool
		enabled  bool
		animated bool
	}{
		{name: "normal", enabled: true, animated: true},
		{name: "reduced", reduced: true, enabled: true, animated: false},
		{name: "focus", focus: true, enabled: false, animated: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := theme.BackgroundDecor(test.reduced, test.focus)
			if got.Enabled != test.enabled || got.Animated != test.animated {
				t.Fatalf("policy = %+v, want enabled=%v animated=%v", got, test.enabled, test.animated)
			}
			if got.Enabled && (got.Opacity <= 0 || got.Opacity > 0.08) {
				t.Fatalf("decoration opacity = %v, want a low readable value", got.Opacity)
			}
		})
	}
	if theme.BackgroundMotionDuration <= 0 || theme.BackgroundMotionDuration > 200*time.Millisecond {
		t.Fatalf("background animation duration = %s, outside shared budget", theme.BackgroundMotionDuration)
	}
}
