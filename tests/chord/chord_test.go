package chord_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chord"
)

var t0 = time.Unix(1_700_000_000, 0)

func at(ms int) time.Time { return t0.Add(time.Duration(ms) * time.Millisecond) }

func TestHoldThenReleaseCommits(t *testing.T) {
	var r chord.Recognizer
	if got := r.Press("agents", at(0)); got != chord.Opened || r.Phase != chord.Peek {
		t.Fatalf("press = %v phase %v", got, r.Phase)
	}
	if got := r.ModifierReleased(at(400)); got != chord.Committed || r.Phase != chord.Idle {
		t.Fatalf("release = %v phase %v", got, r.Phase)
	}
}

func TestQuickTapBecomesSticky(t *testing.T) {
	var r chord.Recognizer
	r.Press("create", at(0))
	if got := r.ModifierReleased(at(120)); got != chord.MadeSticky || r.Phase != chord.Sticky {
		t.Fatalf("tap = %v phase %v", got, r.Phase)
	}
	if got := r.Press("create", at(900)); got != chord.Closed || r.Phase != chord.Idle {
		t.Fatalf("same chord on sticky = %v", got)
	}
}

func TestQuickReleaseAfterMoveStillCommits(t *testing.T) {
	var r chord.Recognizer
	r.Press("projects", at(0))
	r.Moved()
	if got := r.ModifierReleased(at(100)); got != chord.Committed {
		t.Fatalf("fast flick release = %v", got)
	}
}

func TestSameChordWhileHeldCycles(t *testing.T) {
	var r chord.Recognizer
	r.Press("agents", at(0))
	if got := r.Press("agents", at(80)); got != chord.Cycled {
		t.Fatalf("second press = %v", got)
	}
	// Cycling is a deliberate selection, so even a short gesture commits.
	if got := r.ModifierReleased(at(150)); got != chord.Committed {
		t.Fatalf("release after cycle = %v", got)
	}
}

func TestDifferentChordWhileHeldRetargets(t *testing.T) {
	var r chord.Recognizer
	r.Press("agents", at(0))
	if got := r.Press("projects", at(90)); got != chord.Retargeted || r.Layer != "projects" || !r.Held() {
		t.Fatalf("retarget = %v layer %q", got, r.Layer)
	}
	// The tap window restarts at the retarget.
	if got := r.ModifierReleased(at(200)); got != chord.MadeSticky {
		t.Fatalf("quick release after retarget = %v", got)
	}
}

func TestReleaseIsIdempotent(t *testing.T) {
	var r chord.Recognizer
	if got := r.ModifierReleased(at(0)); got != chord.None {
		t.Fatalf("release while idle = %v", got)
	}
	r.Press("agents", at(0))
	r.ModifierReleased(at(500))
	if got := r.ModifierReleased(at(600)); got != chord.None {
		t.Fatalf("second release = %v", got)
	}
}

func TestFocusLossNeverCommits(t *testing.T) {
	var r chord.Recognizer
	r.Press("create", at(0))
	if got := r.FocusLost(); got != chord.MadeSticky || r.Phase != chord.Sticky || r.Held() {
		t.Fatalf("focus lost = %v phase %v", got, r.Phase)
	}
	if got := r.ModifierReleased(at(800)); got != chord.None {
		t.Fatalf("release after focus loss = %v", got)
	}
}

func TestCancel(t *testing.T) {
	var r chord.Recognizer
	if got := r.Cancel(); got != chord.None {
		t.Fatalf("cancel idle = %v", got)
	}
	r.Press("agents", at(0))
	if got := r.Cancel(); got != chord.Closed || r.Phase != chord.Idle || r.Held() {
		t.Fatalf("cancel = %v", got)
	}
}
