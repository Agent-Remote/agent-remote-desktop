package app_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// layersTester restores the shared workspace (project one: split a|b,
// remote c, hidden codex; project two) with every Agent "installed".
func layersTester(t *testing.T) (*app.App, *ui.Tester, string) {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	testutil.StoreWorkspace(t, home, testutil.SavedWorkspace(t))
	providers := &providerFixture{available: map[string]bool{"codex": true, "claude": true, "fclaude": true}, directory: t.TempDir()}
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: providers.resolve})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1366, 768)
	settle(tt)
	return a, tt, home
}

func dockOpen(tt *ui.Tester) bool { _, ok := tt.Find(locales.NewAgent()); return ok }

// Doc §3.2: hold ⌥ and press A to peek; A again while held cycles; letting
// go of ⌥ switches to the selection.
func TestHeldChordCyclesAndReleaseCommits(t *testing.T) {
	_, tt, _ := layersTester(t)
	if !tt.HasText(locales.BarAgent("claude")) {
		t.Fatalf("restored focus is not b (claude): %q", tt.Texts())
	}
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyA) // peek: pre-selects the previously used Agent
	settle(tt)
	if !dockOpen(tt) || !tt.HasText(locales.DockPeekHint("⌥")) && !tt.HasText(locales.DockPeekHint("Alt")) {
		t.Fatal("held ⌥A did not peek the dock")
	}
	tt.Key(ui.Alt, ui.KeyA) // b → c
	settle(tt)
	tt.HoldModifiers(0)
	settle(tt)
	if dockOpen(tt) {
		t.Fatal("releasing ⌥ did not close the dock")
	}
	if !tt.HasText(locales.BarAgent("fclaude")) {
		t.Fatalf("release did not switch to the cycled Agent: %q", tt.Texts())
	}
}

func TestQuickChordTapStaysOpenAndEscapeLeaves(t *testing.T) {
	_, tt, _ := layersTester(t)
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyA)
	tt.HoldModifiers(0) // well within the 250 ms tap window
	settle(tt)
	if !dockOpen(tt) || !tt.HasText(locales.DockStickyHint()) {
		t.Fatal("a quick tap did not leave the dock open")
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if dockOpen(tt) || !tt.HasText(locales.BarAgent("claude")) {
		t.Fatal("Escape did not return to the same Agent")
	}
	// The same chord again closes a sticky layer.
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyX)
	tt.HoldModifiers(0)
	settle(tt)
	if !tt.HasText(locales.SwitchProject()) {
		t.Fatal("⌥X tap did not open the project wheel")
	}
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyX)
	tt.HoldModifiers(0)
	settle(tt)
	if tt.HasText(locales.SwitchProject()) {
		t.Fatal("⌥X again did not close the wheel")
	}
}

func TestHeldChordReleasedOnNothingCancels(t *testing.T) {
	_, tt, _ := layersTester(t)
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyC)
	settle(tt)
	time.Sleep(300 * time.Millisecond) // a hold, not a tap
	tt.HoldModifiers(0)
	settle(tt)
	if tt.HasText(locales.NewAgent()) || tt.HasText(locales.NewAgentSession()) {
		t.Fatal("releasing ⌥C over no sector created something")
	}
}

func TestChordRetargetsWhileHeld(t *testing.T) {
	_, tt, _ := layersTester(t)
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyA)
	tt.Key(ui.Alt, ui.KeyX)
	settle(tt)
	if dockOpen(tt) || !tt.HasText(locales.SwitchProject()) {
		t.Fatal("⌥A then X did not move to the project wheel")
	}
	tt.HoldModifiers(0)
	settle(tt)
}

func TestProjectWheelSwitchesAndKeepsSlots(t *testing.T) {
	_, tt, home := layersTester(t)
	if err := openProject(tt, "Second project"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.BarProject("Second project")) {
		t.Fatal("the wheel did not switch project")
	}
	before := testutil.LoadWorkspace(t, home)
	if err := openProject(tt, "First project"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	settle(tt)
	after := testutil.LoadWorkspace(t, home)
	for i := range before.Projects {
		if before.Projects[i].Slot == 0 || before.Projects[i].Slot != after.Projects[i].Slot {
			t.Fatalf("wheel slots moved: %+v → %+v", before.Projects, after.Projects)
		}
	}
}

func TestProjectWheelSearchOpensMatch(t *testing.T) {
	_, tt, _ := layersTester(t)
	settle(tt)
	tt.Key(ui.Cmd, ui.KeyO) // ⌘O opens the wheel's search
	settle(tt)
	if !tt.HasText(locales.ProjectSearchTitle()) {
		t.Fatal("⌘O did not open project search")
	}
	tt.Type("second")
	settle(tt)
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if !tt.HasText(locales.BarProject("Second project")) {
		t.Fatalf("search did not open the match: %q", tt.Texts())
	}
}

func TestControlCenterOpensFromBarAndClosesWithEscape(t *testing.T) {
	_, tt, _ := layersTester(t)
	if err := tt.Click(locales.OpenControlCenter()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if !tt.HasText(locales.ControlCenter()) || !tt.HasText(locales.ControlCenterSystem()) {
		t.Fatal("the breathing bar did not open the Control Center")
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if tt.HasText(locales.ControlCenterSystem()) {
		t.Fatal("Escape did not close the Control Center")
	}
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyS)
	tt.HoldModifiers(0)
	settle(tt)
	if !tt.HasText(locales.ControlCenterSystem()) {
		t.Fatal("⌥S did not open the Control Center")
	}
	if err := tt.Click(locales.CloseControlCenter()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if tt.HasText(locales.ControlCenterSystem()) {
		t.Fatal("close did not close the Control Center")
	}
}

// A chord pressed while a live terminal has the keyboard is taken by the
// terminal's OnKey — the window shortcut is not registered then (R0).
func TestChordFromFocusedTerminalOpensLayer(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.Sessions, state.Board.Tabs, state.Board.Active = nil, nil, nil
	testutil.StoreWorkspace(t, home, state)
	provider := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(provider, []byte("#!/bin/sh\nwhile :; do sleep 1; done\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: func(name string) (string, error) {
		if name == "codex" {
			return provider, nil
		}
		return "", fault.ExecutableMissing
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1366, 768)
	settle(tt)
	tt.Key(ui.Cmd, ui.KeyT) // ⌘T opens the ⌥C wheel sticky
	settle(tt)
	if err := tt.Click(locales.ToolNameCodex()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !tt.HasText(locales.LocalTool("codex")) && time.Now().Before(deadline) {
		tt.Frame()
		time.Sleep(20 * time.Millisecond)
	}
	if !tt.HasText(locales.LocalTool("codex")) {
		t.Fatal("the Codex sector did not start a session")
	}
	term, ok := tt.Find("Terminal")
	if !ok {
		t.Fatal("terminal view missing")
	}
	tt.ClickAt(term.X+term.W/2, term.Y+term.H/2)
	settle(tt)
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyA)
	tt.HoldModifiers(0)
	settle(tt)
	if !dockOpen(tt) {
		t.Fatal("⌥A from a focused terminal did not open the dock")
	}
}
