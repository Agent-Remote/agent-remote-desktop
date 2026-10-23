package app_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// A mutagen environment receipt is owned by the CLI, but its ordered public
// stages must remain explainable in the desktop operation center after a
// restart. The desktop snapshot must contain only stage names and states.
func TestMutagenOperationStagesSurviveDesktopRestart(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()

	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 900, 600)
	tester.SetLocale("en")
	if err := tester.Click(locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.DiagnosticsChecks())
	if err := tester.Click(locales.MutagenEnvironmentTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click(locales.MutagenEnvironmentInspect()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.MutagenEnvironmentReview())
	if err := tester.Click(locales.MutagenEnvironmentConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.MutagenEnvironmentOutcome("completed"))
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.Load()
	_ = store.Close()
	if err != nil {
		t.Fatal(err)
	}
	var found *storage.OperationRecord
	for i := range state.Operations {
		if state.Operations[i].ID == "mutagen" {
			found = &state.Operations[i]
			break
		}
	}
	if found == nil || len(found.Stages) != 4 {
		t.Fatalf("mutagen stage projection was not persisted: %+v", state.Operations)
	}
	for i, want := range []storage.OperationStage{
		{Name: "stop", State: "completed"},
		{Name: "start", State: "completed"},
		{Name: "verify", State: "completed"},
		{Name: "persist", State: "completed"},
	} {
		if found.Stages[i] != want {
			t.Fatalf("stage %d = %+v, want %+v", i, found.Stages[i], want)
		}
	}

	reopened, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := ui.NewTester(reopened.View, 900, 600)
	restarted.SetLocale("en")
	if err := restarted.Click(locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	if !restarted.HasText(locales.OperationMutagen()) || !restarted.HasText(locales.OperationCompleted()) {
		t.Fatalf("mutagen operation was not restored: %v", restarted.Texts())
	}
	// Keep this test deterministic if the UI scheduler has not rendered the
	// details on the first frame after opening the operation center.
	deadline := time.Now().Add(2 * time.Second)
	for !restarted.HasText(locales.MutagenEnvironmentStage("persist", "completed")) && time.Now().Before(deadline) {
		restarted.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !restarted.HasText(locales.MutagenEnvironmentStage("persist", "completed")) {
		t.Fatalf("restored stage details missing: %v", restarted.Texts())
	}
}
