package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestDependenciesPageUsesReadOnlyStructuredInspection(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	params := filepath.Join(t.TempDir(), "params.json")
	t.Setenv("DESKTOP_TEST_DIAGNOSTICS_PARAMS", params)
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 900, 600)
	if err := tt.Click(locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if !tt.HasText(locales.DependenciesTitle()) {
		t.Fatalf("diagnostics page did not expose dependencies entry")
	}
	if err := tt.Click(locales.DependenciesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DependenciesPlatform(runtime.GOOS))
	if !tt.HasText(locales.DependenciesReadOnly()) || !tt.HasText(locales.DiagnosticsStatus("missing")) {
		t.Fatalf("dependency page did not show read-only platform/status projection")
	}
	assertDependenciesOptions(t, params, false)
	if err := tt.Click(locales.DependenciesVersions()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsVersionStatus("not_checked"))
	assertDependenciesOptions(t, params, true)
}

func assertDependenciesOptions(t *testing.T, path string, versions bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var options struct {
		Online   bool `json:"online"`
		Deep     bool `json:"deep_check"`
		Versions bool `json:"verify_versions"`
	}
	if err := json.Unmarshal(data, &options); err != nil {
		t.Fatal(err)
	}
	if options.Online || options.Deep || options.Versions != versions {
		t.Fatalf("dependencies.inspect received unexpected options: %+v", options)
	}
}
