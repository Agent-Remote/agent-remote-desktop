package app_test

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// settle lets layer transitions finish. Tests that navigate run with the
// desktop's reduce-motion preference, so a frame or two is enough and every
// layer is where its layout says when a test clicks it.
func settle(tt *ui.Tester) {
	tt.SetPreferences(ui.Preferences{ReduceMotion: true, TextScale: 1})
	// A click can move focus and rebuild the layer in separate headless
	// frames.  Run one frame after the short clock advance so page-level
	// shortcuts and focus bindings are registered before the next key event.
	tt.Frame()
	tt.Advance(20 * time.Millisecond)
	tt.Frame()
}

// openPage reaches a management page the way the redesign does: the
// breathing bar opens the Control Center, whose tile pushes the page into
// the sheet; from another page, Back returns to the tiles first. It
// replaces clicking the removed sidebar.
func openPage(tt *ui.Tester, label string) error {
	settle(tt)
	switch {
	case tt.HasText(locales.BackToControlCenter()):
		if err := tt.Click(locales.BackToControlCenter()); err != nil {
			return err
		}
	case !tt.HasText(locales.CloseControlCenter()):
		if err := tt.Click(locales.OpenControlCenter()); err != nil {
			return err
		}
	}
	settle(tt)
	// Small windows scroll the sheet: scroll until the tile shows, as a
	// user would, then click it.
	for i := 0; i < 8; i++ {
		if r, ok := tt.Find(label); ok && r.W > 0 && r.H > 0 {
			break
		}
		sheet, ok := tt.Find(locales.ControlCenter())
		if !ok {
			break
		}
		tt.Scroll(sheet.X+20, sheet.Y+120, 0, 160)
		settle(tt)
	}
	if err := tt.Click(label); err != nil {
		return err
	}
	settle(tt)
	return nil
}

// openSearch opens the command palette (⌘⇧P), which the removed toolbar's
// Search button opened.
func openSearch(tt *ui.Tester) error {
	settle(tt)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	settle(tt)
	return nil
}

// toggleFocusMode is ⌘⇧F, which the removed toolbar's Focus button toggled.
func toggleFocusMode(tt *ui.Tester) error {
	settle(tt)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyF)
	settle(tt)
	return nil
}

// openNewSessionDialog opens the full new-session dialog: ⌘T opens the ⌥C
// wheel, whose "Advanced…" sector opens the dialog.
func openNewSessionDialog(tt *ui.Tester) error {
	settle(tt)
	tt.Key(ui.Cmd, ui.KeyT)
	settle(tt)
	if err := tt.Click(locales.CreateAdvanced()); err != nil {
		return err
	}
	settle(tt)
	return nil
}

func mustOpenPage(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	if err := openPage(tt, label); err != nil {
		t.Fatal(err)
	}
}

// navigate replaces clicking a label of the removed sidebar or toolbar:
// destinations go through the Control Center, Focus and Search through
// their shortcuts; any other label (a page's own button) is clicked.
func navigate(tt *ui.Tester, label string) error {
	switch label {
	case locales.Focus():
		return toggleFocusMode(tt)
	case locales.SearchWorkbench():
		return openSearch(tt)
	}
	for _, page := range []string{locales.Settings(), locales.RemoteManagement(), locales.CapabilitiesTitle(), locales.DiagnosticsTitle(),
		locales.SkillsTitle(), locales.SyncOpen(), locales.OperationsSidebar(), locales.InitWizardTitle(), locales.NodesTitle(), locales.EgoBrowserTitle(),
		locales.LoginOperationStatus(), locales.LogoutOperationStatus(), locales.LocalSetupOperationStatus(), locales.WireGuardOperationStatus()} {
		if label == page {
			return openPage(tt, label)
		}
	}
	return tt.Click(label)
}

func find(t *testing.T, tt *ui.Tester, s string) ui.Rect {
	t.Helper()
	r, ok := tt.Find(s)
	if !ok {
		t.Fatalf("%q not shown; texts %q", s, tt.Texts())
	}
	return r
}

// openDockCard switches to a session by its card in the ⌥A dock, which
// replaces the removed sidebar rows and tab strip.
func openDockCard(tt *ui.Tester, label string) error {
	settle(tt)
	tt.Key(ui.Alt, ui.KeyA)
	settle(tt)
	r, ok := tt.Find(label)
	if !ok {
		return tt.Click(label)
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	settle(tt)
	if err := tt.Click(label); err != nil {
		return err
	}
	settle(tt)
	return nil
}

// openProject switches project on the ⌥X wheel, which replaces the removed
// sidebar's project list: the project's sector commits on click.
func openProject(tt *ui.Tester, name string) error {
	// Hosted headless renderers can deliver the wheel's pointer action one
	// frame after the sector is painted. Retry the user-visible action until
	// the breathing bar confirms the selected project, keeping this helper
	// deterministic across Xvfb and native backends.
	for attempt := 0; attempt < 3; attempt++ {
		settle(tt)
		if tt.HasText(locales.BarProject(name)) {
			return nil
		}
		tt.Key(ui.Alt, ui.KeyX)
		settle(tt)
		r, ok := tt.Find(name)
		if !ok {
			return tt.Click(name)
		}
		tt.ClickAt(r.X+r.W/2, r.Y+r.H/2)
		settle(tt)
	}
	if tt.HasText(locales.BarProject(name)) {
		return nil
	}
	return tt.Click(name)
}
