package app_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestDiagnosticsPageHasLocalizedActionsAtMinimumSize(t *testing.T) {
	testutil.InstallControlHelper(t)
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return language })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			params := filepath.Join(t.TempDir(), "params.json")
			t.Setenv("DESKTOP_TEST_DIAGNOSTICS_PARAMS", params)
			a, err := app.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			tt := ui.NewTester(a.View, 900, 600)
			tt.SetLocale(language)
			if err := openPage(tt, locales.DiagnosticsTitle()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
			assertDiagnosticOptions(t, params, false, false, false)
			for _, label := range []string{locales.DiagnosticsRefresh(), locales.DiagnosticsVersions(), locales.ReturnWorkbench(), locales.DiagnosticsOnline()} {
				rect, ok := tt.Find(label)
				if !ok || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 901 || rect.Y+rect.H > 601 {
					t.Errorf("control %q outside minimum window: %+v", label, rect)
				}
			}
			if err := tt.Click(locales.DiagnosticsVersions()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
			assertDiagnosticOptions(t, params, true, true, true)
			if err := tt.Click(locales.DiagnosticsRefresh()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
			assertDiagnosticOptions(t, params, true, false, true)

			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "diagnostics-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					f.Close()
					t.Fatal(err)
				}
				if err := f.Close(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestDiagnosticsCancellationIsDistinctFromRemoteSessionCancellation(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "slow")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := openPage(tt, locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsCanceled())
}

func assertDiagnosticOptions(t *testing.T, path string, online, versions, deep bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var options struct {
		Deep     bool `json:"deep_check"`
		Online   bool `json:"online"`
		Versions bool `json:"verify_versions"`
	}
	if err := json.Unmarshal(data, &options); err != nil {
		t.Fatal(err)
	}
	if options.Online != online || options.Versions != versions || options.Deep != deep {
		t.Fatalf("unexpected explicit probe options: %+v", options)
	}
}
