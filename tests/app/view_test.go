package app_test

import (
	"database/sql"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestPreferencesSaveFailureIsVisibleAndReportedOnClose(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	// Simulate a newer application writing a schema this running version cannot save.
	if _, err := db.Exec("INSERT INTO desktop_state VALUES(1,99,'{}')"); err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.View, 1366, 768)
	if err := tt.Click("Theme"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !tt.HasText("Desktop preferences could not be saved") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText("Desktop preferences could not be saved") {
		t.Fatal("save failure was hidden")
	}
	if err := a.Close(); !errors.Is(err, fault.StorageVersion) {
		t.Fatalf("close hid save failure: %v", err)
	}
}

func TestWorkbenchProjectFlowAndTheme(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	if !tt.HasText("打开项目目录") {
		t.Fatal("welcome action is missing")
	}
	if err = tt.Click("打开项目目录"); err != nil {
		t.Fatal(err)
	}
	if err = tt.Click("项目目录"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	tt.Type(dir)
	if err = tt.Click("打开"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(dir) {
		t.Fatal("project not added")
	}
	if !tt.HasText("新建项目会话") {
		t.Fatal("project workspace did not open")
	}
	// A new install follows the system appearance; with a dark system the
	// quick switch starts from midnight, so one click chooses sakura and the
	// explicit choice stops following.
	tt.SetDark(true)
	if err = tt.Click("主题"); err != nil {
		t.Fatal(err)
	}

	if err = tt.Click("新建项目会话"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("新建 Agent 会话") {
		t.Fatal("agent picker missing")
	}
	if err = tt.Click("取消"); err != nil {
		t.Fatal(err)
	}
	tt.SetSize(900, 600)
	if !tt.HasText("＋ 新建会话") {
		t.Fatal("primary action lost at minimum window size")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	saved, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer saved.Close()
	state, err := saved.Load()
	if err != nil || state.Preferences.Theme != "sakura" || state.Preferences.FollowSystemTheme || len(state.Projects) != 1 {
		t.Fatalf("UI changes were not persisted: %+v %v", state, err)
	}
}

func TestRenderWorkbench(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "zh-Hans" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	output := os.Getenv("DESKTOP_RENDER_DIR")
	if output == "" {
		t.Skip("set DESKTOP_RENDER_DIR to render visual artifacts")
	}
	home := t.TempDir()
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err = os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	previousIndex := 0
	for index, name := range theme.ThemeNames {
		tt := ui.NewTester(a.View, 1366, 768)
		tt.SetDark(true) // the cycle starts from midnight, as ThemeNames does
		// The toolbar follows the same built-in order as the production
		// quick-switch action. Advance from the previous capture so each
		// rendered artifact names the palette it actually captures (including
		// Sakura).
		for click := previousIndex; click < index; click++ {
			if err := tt.Click("主题"); err != nil {
				t.Fatal(err)
			}
		}
		previousIndex = index
		f, err := os.Create(filepath.Join(output, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err = png.Encode(f, tt.Image()); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()
	}
}

func TestChangingLanguagePreservesProjectInputAndErrors(t *testing.T) {
	language := "en"
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1366, 768)
	if err := tt.Click("Open project directory"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Project directory"); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(t.TempDir(), "missing")
	tt.Type(dir)
	if err := tt.Click("Open"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("file or directory no longer exists") {
		t.Fatal("localized error missing")
	}
	language = "zh-Hans"
	tt.SetLocale(language)
	if !tt.HasText("文件或目录不存在") {
		t.Fatalf("previous error was left untranslated: %v", tt.Texts())
	}
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("打开"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("新建项目会话") {
		t.Fatal("could not continue project flow after switching language")
	}
	language = "en"
	tt.SetLocale(language)
	if !tt.HasText("New project session") || !tt.HasText(dir) {
		t.Fatal("project was lost during language switch")
	}
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{"Language", "Follow system", "English", "简体中文"} {
		if !tt.HasText(label) {
			t.Errorf("language setting missing: %s", label)
		}
	}
}

func TestLocalizedPrimaryActionsAtMinimumWindowSize(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return language })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			a, err := app.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			tt := ui.NewTester(a.View, 900, 600)
			tt.SetLocale(language)
			for _, action := range []string{locales.NewSessionAction(), locales.OpenProjectDirectory(), locales.ViewEnvironment(), locales.Settings()} {
				r, ok := tt.Find(action)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q action is outside window: %+v", action, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "minimum-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
