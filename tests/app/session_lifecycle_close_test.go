package app_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// TestClosePersistsFinalLocalExitCode verifies that explicit application
// shutdown stops managed PTYs before the last desktop snapshot. This catches
// a regression where the process was stopped successfully but its exit code
// was only available after persistence had already completed.
func TestClosePersistsFinalLocalExitCode(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home, projectDir := t.TempDir(), t.TempDir()
	provider := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\ntrap 'exit 42' TERM\nwhile :; do sleep 1; done\n"), 0700); err != nil {
		t.Fatal(err)
	}
	state := storage.State{
		Preferences: workspace.Defaults(),
		Projects:    []workspace.Project{{ID: "project", Name: "Project", Path: projectDir}},
	}
	state.Preferences.SelectedProject = "project"
	testutil.StoreWorkspace(t, home, state)
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: func(name string) (string, error) {
		if name == "codex" {
			return provider, nil
		}
		return "", fault.ExecutableMissing
	}})
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 900, 600)
	if err := tester.Click(locales.NewProjectSession()); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click(locales.StartSession()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !tester.HasText(locales.Running()) && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(20 * time.Millisecond)
	}
	if !tester.HasText(locales.Running()) {
		t.Fatalf("local Agent did not reach running state (ready=%v required=%v starting=%v)", tester.HasText(locales.ProviderReady("codex")), tester.HasText(locales.ProviderRequired("codex")), tester.HasText(locales.Starting()))
	}
	// Ensure the fixture has installed its TERM trap before explicit quit. The
	// full repository suite starts several native process fixtures in parallel;
	// allow a little scheduling headroom so this assertion tests the shutdown
	// contract rather than racing the shell before it has parsed the trap.
	time.Sleep(time.Second)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	saved := testutil.LoadWorkspace(t, home)
	if len(saved.Sessions) != 1 || saved.Sessions[0].ExitCode == nil || *saved.Sessions[0].ExitCode != 42 {
		if len(saved.Sessions) == 1 && saved.Sessions[0].ExitCode != nil {
			t.Fatalf("final local exit code was not persisted: got %d, sessions=%+v", *saved.Sessions[0].ExitCode, saved.Sessions)
		}
		t.Fatalf("final local exit code was not persisted: %+v", saved.Sessions)
	}
}
