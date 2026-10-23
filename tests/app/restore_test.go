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
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: providers.resolve})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	if !tt.HasText("Saved session") || !tt.HasText("Not connected") || tt.HasText("Running") {
		t.Fatal("restored metadata was presented as a running Agent")
	}
	from, ok := tt.Find(locales.TabTerminals("Review", 2))
	if !ok {
		t.Fatal("split tab missing")
	}
	to, ok := tt.Find("Remote")
	if !ok {
		t.Fatal("second tab missing")
	}
	tt.Press(from.X+from.W/2, from.Y+from.H/2)
	tt.Move(from.X+from.W/2+12, from.Y+from.H/2)
	tt.Move(to.X+to.W/2, to.Y+to.H/2)
	tt.Release(to.X+to.W/2, to.Y+to.H/2)
	deadline := time.Now().Add(3 * time.Second)
	for {
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
	// The selected tab and its last focused pane survive reordering.
	tt.Key(0, ui.KeyF6)
	if err := tt.Click(locales.HideTab("Remote")); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	saved := testutil.LoadWorkspace(t, home)
	if len(saved.Sessions) != 5 || len(saved.Board.Tabs) != 2 {
		t.Fatal("hiding a tab deleted history")
	}
	if saved.Board.Current("one").FocusedSession != "a" {
		t.Fatalf("focus did not cycle in preserved split: %+v", saved.Board.Current("one"))
	}
	if saved.Board.Current("one").Root.Ratio != 64 {
		t.Fatal("drag changed split geometry")
	}
	// Reopening must keep the hidden remote session available in the sidebar.
	second, err := app.NewWithOptions(home, app.Options{FindExecutable: providers.resolve})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	view := ui.NewTester(second.View, 1366, 768)
	if !view.HasText("fclaude · Remote  Not connected") {
		t.Fatal("hidden history disappeared on restart")
	}
	if err := view.Click("fclaude · Remote  Not connected"); err != nil {
		t.Fatal(err)
	}
	view.Frame()
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

func TestPaletteNavigatesByQueryAndSidebarResizeIsSaved(t *testing.T) {
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
	divider, ok := tt.Find("Divider")
	if !ok {
		t.Fatal("sidebar divider missing")
	}
	x, y := divider.X+divider.W/2, divider.Y+divider.H/2
	tt.Press(x, y)
	tt.Move(x+30, y)
	tt.Release(x+30, y)
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
	if stored.Preferences.SelectedProject != "two" || stored.Preferences.SidebarWidth < 285 || stored.Preferences.SidebarWidth > 295 {
		t.Fatalf("navigation or resize was not saved: %+v", stored.Preferences)
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
			if err := tt.Click(locales.NewSessionAction()); err != nil {
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
