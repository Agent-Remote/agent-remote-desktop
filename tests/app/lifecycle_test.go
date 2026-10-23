package app_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
)

func TestWindowClosePolicyHidesUntilConfirmedQuit(t *testing.T) {
	tests := []struct {
		name     string
		quitting bool
		want     app.WindowCloseDecision
	}{
		{name: "ordinary close stays in background", want: app.WindowCloseHide},
		{name: "confirmed quit allows native close", quitting: true, want: app.WindowCloseAllow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := app.DecideWindowClose(tt.quitting); got != tt.want {
				t.Fatalf("DecideWindowClose(%t) = %v, want %v", tt.quitting, got, tt.want)
			}
		})
	}
}

func TestQuitRequestPolicyPromptsBeforeTeardown(t *testing.T) {
	if got := app.DecideQuitRequest(false); got != app.QuitRequestPrompt {
		t.Fatalf("first quit request = %v, want prompt", got)
	}
	if got := app.DecideQuitRequest(true); got != app.QuitRequestAllow {
		t.Fatalf("confirmed quit request = %v, want allow", got)
	}
}

func TestConfirmedQuitPlanStopsManagedWorkAndForwards(t *testing.T) {
	plan := app.PlanConfirmedQuit()
	if !plan.StopManagedSessions {
		t.Fatal("confirmed quit must stop managed sessions")
	}
	if !plan.StopLocalPortForwards {
		t.Fatal("confirmed quit must stop local port forwards")
	}
}
