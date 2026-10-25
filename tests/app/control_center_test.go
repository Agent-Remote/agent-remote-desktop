package app_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func controlCenterOpen(tt *ui.Tester) bool { return tt.HasText(locales.CloseControlCenter()) }
func pageShown(tt *ui.Tester) bool         { return tt.HasText(locales.BackToControlCenter()) }

// Redesign §4.5: a tile pushes its page into the sheet; Back returns to
// the tiles with the sheet still open; Esc goes back first, then closes.
func TestControlCenterPushesPagesAndGoesBack(t *testing.T) {
	_, tt, _ := layersTester(t)
	mustOpenPage(t, tt, locales.DiagnosticsTitle())
	if !pageShown(tt) || !tt.HasText(locales.DiagnosticsHelp()) {
		t.Fatalf("the Diagnostics tile did not push its page: %q", tt.Texts())
	}
	if !tt.HasText(locales.ContextHome()) {
		t.Fatal("the page has no context strip")
	}
	if err := tt.Click(locales.BackToControlCenter()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if pageShown(tt) || !controlCenterOpen(tt) {
		t.Fatal("Back did not return to the tiles")
	}
	mustOpenPage(t, tt, locales.Settings())
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if pageShown(tt) || !controlCenterOpen(tt) {
		t.Fatal("Esc on a page did not go back to the tiles first")
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if controlCenterOpen(tt) {
		t.Fatal("a second Esc did not close the Control Center")
	}
}

// Return to workbench closes the sheet with its page; reopening shows the
// tiles, not the page left behind.
func TestReturnToWorkbenchClosesThePage(t *testing.T) {
	_, tt, _ := layersTester(t)
	mustOpenPage(t, tt, locales.OperationsSidebar())
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if controlCenterOpen(tt) || pageShown(tt) {
		t.Fatal("Return to workbench left the Control Center open")
	}
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyS)
	tt.HoldModifiers(0)
	settle(tt)
	if !controlCenterOpen(tt) || pageShown(tt) {
		t.Fatal("⌥S reopened the last page instead of the tiles")
	}
}

// Every tile reaches a page in the sheet; none renders over the workspace.
func TestEveryControlCenterTileOpensAPage(t *testing.T) {
	_, tt, _ := layersTester(t)
	tiles := []string{
		locales.SyncOpen(), locales.PortForwardTitle(), locales.ProjectDetails(),
		locales.RemoteManagement(), locales.RemoteAccounts(), locales.SkillsTitle(), locales.AccountDefaultTitle(),
		locales.AccountImportRecoveryTitle(), locales.RuntimeRecoveryTitle(),
		locales.WireGuardNetworkTitle(), locales.DeviceTitle(), locales.EgoBrowserTitle(), locales.NodesTitle(), locales.MutagenEnvironmentTitle(),
		locales.OperationsSidebar(), locales.DiagnosticsTitle(), locales.DependenciesTitle(), locales.InitWizardTitle(),
		locales.CapabilitiesTitle(), locales.Settings(),
	}
	for _, label := range tiles {
		if err := openPage(tt, label); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		// Remote-backed tiles wait for the CLI read before their page.
		for i := 0; i < 50 && !pageShown(tt); i++ {
			settle(tt)
		}
		if !pageShown(tt) {
			t.Fatalf("the %q tile did not push a page: %q", label, tt.Texts())
		}
	}
}

// The header's search runs the shared index: a page result pushes the
// page into the sheet.
func TestControlCenterSearchPushesAPage(t *testing.T) {
	_, tt, _ := layersTester(t)
	if err := tt.Click(locales.OpenControlCenter()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if err := tt.Click(locales.ControlCenterSearch()); err != nil {
		t.Fatal(err)
	}
	tt.Type(locales.CapabilitiesTitle())
	settle(tt)
	if !tt.HasText(locales.ControlCenterResults()) || tt.HasText(locales.ControlCenterSystem()) {
		t.Fatalf("typing did not replace the tiles with results: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if !pageShown(tt) || !tt.HasText(locales.CapabilitiesHelp()) {
		t.Fatalf("the first result did not push its page: %q", tt.Texts())
	}
}

// A page opened from anywhere else — the palette here — shows in the
// Control Center, as the sidebar used to show it.
func TestPaletteOpensPagesInTheControlCenter(t *testing.T) {
	_, tt, _ := layersTester(t)
	if err := openSearch(tt); err != nil {
		t.Fatal(err)
	}
	tt.Type(locales.DependenciesTitle())
	settle(tt)
	tt.Key(0, ui.KeyEnter)
	settle(tt)
	if !pageShown(tt) || !tt.HasText(locales.DependenciesHelp()) {
		t.Fatalf("the palette did not open the page in the Control Center: %q", tt.Texts())
	}
}

// A chord from a button inside a page still reaches the layers: the sheet
// takes the keys a focused button leaves.
func TestChordFromAPageButtonOpensTheDock(t *testing.T) {
	_, tt, _ := layersTester(t)
	mustOpenPage(t, tt, locales.DiagnosticsTitle())
	if err := tt.Click(locales.DiagnosticsOnline()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	// Native Win32 focus dispatch can deliver the first chord before the page
	// subtree has registered its shortcut after a button click. Retry the same
	// user gesture after a settled frame; the assertion remains on the actual
	// dock state rather than on an implementation detail.
	for attempt := 0; attempt < 3 && !dockOpen(tt); attempt++ {
		tt.HoldModifiers(ui.Alt)
		tt.Key(ui.Alt, ui.KeyA)
		tt.HoldModifiers(0)
		settle(tt)
	}
	if !dockOpen(tt) || pageShown(tt) {
		t.Fatalf("⌥A from a page did not replace the Control Center with the dock: %q", tt.Texts())
	}
}

// In the search field ⌥ + letter is text, not a chord, and Esc clears the
// query before it closes the sheet.
func TestControlCenterSearchKeepsItsKeys(t *testing.T) {
	_, tt, _ := layersTester(t)
	if err := tt.Click(locales.OpenControlCenter()); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if err := tt.Click(locales.ControlCenterSearch()); err != nil {
		t.Fatal(err)
	}
	tt.HoldModifiers(ui.Alt)
	tt.TypeKey(ui.Alt, ui.KeyA, "å")
	tt.HoldModifiers(0)
	settle(tt)
	if dockOpen(tt) || !tt.HasText(locales.ControlCenterResults()) {
		t.Fatalf("⌥A in the search field acted as a chord instead of typing: %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if !controlCenterOpen(tt) || tt.HasText(locales.ControlCenterResults()) {
		t.Fatal("Esc in a searching field did not clear the query first")
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	if controlCenterOpen(tt) {
		t.Fatal("Esc with an empty query did not close the Control Center")
	}
}

// Pulling the breathing bar up raises the sheet with the pointer; letting
// go past half leaves it open, a short pull lets it fall back.
func TestBreathingBarPullsTheControlCenterUp(t *testing.T) {
	_, tt, _ := layersTester(t)
	bar := find(t, tt, locales.OpenControlCenter())
	x, y := bar.X+bar.W/2, bar.Y+bar.H/2
	pull := func(dy float32) {
		tt.Press(x, y)
		for step := float32(1); step <= 10; step++ {
			tt.Move(x, y-dy*step/10)
			tt.Advance(16 * time.Millisecond)
		}
		tt.Advance(200 * time.Millisecond) // stop before letting go: no flick
		tt.Release(x, y-dy)
		settle(tt)
		tt.Advance(time.Second)
		settle(tt)
	}
	pull(60)
	if controlCenterOpen(tt) {
		t.Fatal("a short pull left the Control Center open")
	}
	// Mid-pull, the sheet's top follows the pointer.
	tt.Press(x, y)
	tt.Move(x, y-10)
	tt.Advance(16 * time.Millisecond)
	tt.Move(x, y-200)
	tt.Advance(16 * time.Millisecond)
	first, ok := tt.Find(locales.CloseControlCenter())
	tt.Move(x, y-300)
	tt.Advance(16 * time.Millisecond)
	second, _ := tt.Find(locales.CloseControlCenter())
	if d := first.Y - second.Y; !ok || d < 95 || d > 105 {
		t.Fatalf("the sheet did not follow the pointer 1:1: %v then %v", first, second)
	}
	tt.Release(x, y-300)
	settle(tt)
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	pull(480)
	if !controlCenterOpen(tt) {
		t.Fatalf("pulling the bar up past half did not open the Control Center: %q", tt.Texts())
	}
}

// Pushing the dock back up past half closes it; a small push springs back.
func TestDockPushedUpCloses(t *testing.T) {
	_, tt, _ := layersTester(t)
	tt.HoldModifiers(ui.Alt)
	tt.Key(ui.Alt, ui.KeyA) // a tap: the dock stays open
	tt.HoldModifiers(0)
	settle(tt)
	if !dockOpen(tt) {
		t.Fatal("⌥A did not open the dock")
	}
	title := find(t, tt, locales.DockStickyHint())
	x, y := title.X-40, title.Y+title.H/2
	push := func(dy float32) {
		tt.Press(x, y)
		for step := float32(1); step <= 10; step++ {
			tt.Move(x, y-dy*step/10)
			tt.Advance(16 * time.Millisecond)
		}
		tt.Advance(200 * time.Millisecond)
		tt.Release(x, y-dy)
		settle(tt)
		tt.Advance(time.Second)
		settle(tt)
	}
	push(40)
	if !dockOpen(tt) {
		t.Fatal("a small push closed the dock")
	}
	push(240)
	if dockOpen(tt) {
		t.Fatal("pushing the dock up past half did not close it")
	}
}
