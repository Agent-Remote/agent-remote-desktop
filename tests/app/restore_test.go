package app_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestRestoredWorkspaceKeepsHistoryWhileReorderingAndHidingTabs(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	testutil.StoreWorkspace(t, home, state)
	providers := &providerFixture{available: map[string]bool{"codex": true, "claude": true, "fclaude": true}, directory: t.TempDir()}
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: providers.resolve, ChordTapWindow: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	settle(tt)
	if !tt.HasText("Saved session") || tt.HasText("Running") {
		t.Fatal("restored metadata was presented as a running Agent")
	}
	// ⌥A: every session of the project is a card, with facts only.
	tt.Key(ui.Alt, ui.KeyA)
	settle(tt)
	if !tt.HasText(locales.SavedSessionStatus()) || !tt.HasText(locales.PaneOfSplit(1, 2)) {
		t.Fatal("dock does not show restored sessions as not connected, in their split")
	}
	// Dragging the split's card onto the remote card reorders their tabs.
	from := find(t, tt, "codex")
	to := find(t, tt, "fclaude")
	tt.Press(from.X+from.W/2, from.Y+from.H/2)
	tt.Move(from.X+from.W/2+12, from.Y+from.H/2)
	tt.Move(to.X+to.W/2, to.Y+to.H/2)
	tt.Release(to.X+to.W/2, to.Y+to.H/2)
	deadline := time.Now().Add(3 * time.Second)
	for {
		// Persist is queued from the drag callback; advance the headless main
		// loop while waiting so the save worker's completion frame can publish.
		tt.Frame()
		stored := testutil.LoadWorkspace(t, home)
		if stored.Board.Tabs[0].Title == "Remote" {
			if stored.Board.Current("one").ID != state.Board.Current("one").ID || stored.Board.Current("one").FocusedSession != "b" {
				t.Fatal("drag changed selection or focus")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("drag did not persist reordered tabs")
		}
		time.Sleep(10 * time.Millisecond)
	}
	tt.Key(0, ui.KeyEscape)
	settle(tt)
	// The selected tab and its last focused pane survive reordering.
	tt.Key(0, ui.KeyF6)
	settle(tt)
	// Switch to the remote session's tab from the dock and hide its pane.
	tt.Key(ui.Alt, ui.KeyA)
	settle(tt)
	card := find(t, tt, "fclaude")
	tt.Move(card.X+card.W/2, card.Y+card.H/2) // a pointer comes to a card before it clicks
	settle(tt)
	if err := tt.Click("fclaude"); err != nil {
		t.Fatal(err)
	}
	settle(tt)
	if err := tt.Click(locales.Hide()); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	saved := testutil.LoadWorkspace(t, home)
	if len(saved.Sessions) != 5 || len(saved.Board.Tabs) != 2 {
		t.Fatal("hiding a pane deleted history")
	}
	if saved.Board.Current("one").FocusedSession != "a" {
		t.Fatalf("focus did not cycle in preserved split: %+v", saved.Board.Current("one"))
	}
	if saved.Board.Current("one").Root.Ratio != 64 {
		t.Fatal("drag changed split geometry")
	}
	// Reopening must keep the hidden remote session available in the dock.
	second, err := app.NewWithOptions(home, app.Options{FindExecutable: providers.resolve, ChordTapWindow: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	view := ui.NewTester(second.View, 1366, 768)
	settle(view)
	view.Key(ui.Alt, ui.KeyA)
	settle(view)
	if _, ok := view.Find("fclaude"); !ok {
		t.Fatal("hidden history disappeared on restart")
	}
	if err := view.Click("fclaude"); err != nil {
		t.Fatal(err)
	}
	settle(view)
	if err := view.Click(locales.ResumeSession()); err != nil {
		t.Fatal(err)
	}
	view.Frame()
	if !view.HasText(locales.RemoteHistorySelectAgain()) || !view.HasText(locales.RemoteHistoryReviewHelp()) {
		t.Fatal("legacy recovery did not require refreshed selection")
	}
	if view.HasText(locales.RemoteResumeReference()) {
		t.Fatal("legacy recovery exposed direct UUID attachment")
	}
	if err := view.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
		if err := os.MkdirAll(output, 0755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(filepath.Join(output, "restored-workspace.png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, view.Image()); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPaletteNavigatesByQueryAndIsSaved(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	testutil.StoreWorkspace(t, home, state)
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	if err := tt.Click(locales.PaletteSearch()); err != nil {
		t.Fatal(err)
	}
	tt.Type("Second project")
	tt.Key(0, ui.KeyEnter)
	if !tt.HasText(state.Projects[1].Path) {
		t.Fatal("palette did not navigate to the matching project")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.Preferences.SelectedProject != "two" {
		t.Fatalf("navigation was not saved: %+v", stored.Preferences)
	}
}

func TestSessionCreationActionsFitMinimumWindowInEveryLanguage(t *testing.T) {
	for _, lang := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(lang, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return lang })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			home := t.TempDir()
			testutil.StoreWorkspace(t, home, testutil.SavedWorkspace(t))
			a, err := app.New(home)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			tt := ui.NewTester(a.View, 900, 600)
			tt.SetLocale(lang)
			if err := openNewSessionDialog(tt); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{locales.SessionName(), locales.LaunchArgs(), locales.StartSession(), locales.Cancel()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 900 || r.Y+r.H > 600 {
					t.Errorf("%q outside minimum window: %+v", label, r)
				}
			}
		})
	}
}

func TestWorkspaceKeyboardNavigationAndModalIsolation(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	testutil.StoreWorkspace(t, home, state)
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	tt.Key(ui.Ctrl, ui.KeyTab)
	if err := tt.Click(locales.ResumeSession()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.RemoteHistoryReviewHelp()) {
		t.Fatal("next tab shortcut did not select remote tab")
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyRight)
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("fclaude · Remote") {
		t.Fatal("project changed behind a modal")
	}
	tt.Key(ui.Cmd|ui.Alt, ui.KeyRight)
	if !tt.HasText(state.Projects[1].Path) {
		t.Fatal("next project shortcut did not switch project")
	}
	if err := tt.Click(locales.ResumeSession()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.LocalResumeHelp()) {
		t.Fatal("local history picker unavailable")
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.LoadWorkspace(t, home); got.Preferences.SelectedProject != "two" {
		t.Fatal("project navigation not persisted")
	}
}
