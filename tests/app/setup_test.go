package app_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func setupTester(t *testing.T, mode, language string) (*ui.Tester, *app.App, string, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home, log := t.TempDir(), filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	tt := ui.NewTester(a.View, 900, 600)
	tt.SetLocale(language)
	for _, label := range []string{locales.DiagnosticsTitle(), locales.LocalSetupTitle()} {
		if err := navigate(tt, label); err != nil {
			t.Fatal(err)
		}
	}
	return tt, a, home, log
}

func TestLocalSetupReviewsExactHomeAndDoesNotMutateBeforeConfirmation(t *testing.T) {
	tt, _, home, log := setupTester(t, "", "en")
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	if !tt.HasText(home) || !tt.HasText(locales.LocalSetupScope()) {
		t.Fatal("confirmation omitted target or effect scope")
	}
	for _, name := range []string{"config.toml", "state.sqlite3", "dependencies/manifest.json"} {
		if _, err := os.Stat(filepath.Join(home, name)); !os.IsNotExist(err) {
			t.Fatalf("preflight created CLI state %s: %v", name, err)
		}
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "setup.local") {
		t.Fatal("cancel submitted mutation")
	}
	if err := tt.Click(locales.LocalSetupTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	capture := filepath.Join(t.TempDir(), "input")
	t.Setenv("DESKTOP_TEST_SETUP_RECEIPT", capture)
	if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LocalSetupComplete())
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Confirmed bool   `json:"confirmed"`
		Home      string `json:"expected_home"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		t.Fatal(err)
	}
	if !input.Confirmed || input.Home != home {
		t.Fatalf("incorrect target confirmation: %s", data)
	}
	calls, _ = os.ReadFile(log)
	if strings.Count(string(calls), "setup.local\n") != 1 {
		t.Fatalf("unexpected mutation count: %q", calls)
	}
	if tt.HasText(locales.ConfirmLocalSetup()) {
		t.Fatal("completed receipt offered duplicate submission")
	}
}

func TestLocalSetupRejectsUnsupportedCLIOrHostBeforeMutation(t *testing.T) {
	for _, mode := range []string{"setup-unsupported", "setup-readonly", "other-platform"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := setupTester(t, mode, "en")
			waitRemoteText(t, tt, locales.LocalSetupUnsupported())
			if tt.HasText(locales.ConfirmLocalSetup()) {
				t.Fatal("unsupported operation offered confirmation")
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "setup.local") {
				t.Fatal("unsupported operation mutated state")
			}
		})
	}
}

func TestLocalSetupUnknownAndPartialResultsAreNotRetried(t *testing.T) {
	for _, mode := range []string{"slow-setup", "setup-malformed", "setup-wrong-operation", "setup-partial"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := setupTester(t, mode, "en")
			waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
			started := filepath.Join(t.TempDir(), "started")
			t.Setenv("DESKTOP_TEST_SETUP_STARTED", started)
			if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
				t.Fatal(err)
			}
			if mode == "slow-setup" {
				waitFixtureFile(t, tt, started)
				if !tt.HasText(locales.LocalSetupCancelHelp()) {
					t.Fatal("missing cancellation consequence")
				}
				if err := tt.Click(locales.StopLocalSetupWaiting()); err != nil {
					t.Fatal(err)
				}
			}
			want := locales.LocalSetupUnknown()
			if mode == "setup-partial" {
				want = locales.LocalSetupIncomplete()
			}
			waitRemoteText(t, tt, want)
			if tt.HasText(locales.ConfirmLocalSetup()) || tt.HasText(locales.LocalSetupComplete()) {
				t.Fatal("unknown/partial result offered duplicate or complete outcome")
			}
			if err := openPage(tt, locales.Settings()); err != nil {
				t.Fatal(err)
			}
			if err := openPage(tt, locales.LocalSetupOperationStatus()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(want) {
				t.Fatal("navigation discarded receipt")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "setup.local\n") != 1 {
				t.Fatalf("automatically retried mutation: %q", calls)
			}
		})
	}
}

func TestLocalSetupContinuesAcrossPagesAndAcknowledgmentRefreshesDiagnostics(t *testing.T) {
	tt, _, _, log := setupTester(t, "", "en")
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	release, started := filepath.Join(t.TempDir(), "release"), filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_SETUP_RELEASE", release)
	t.Setenv("DESKTOP_TEST_SETUP_STARTED", started)
	if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	for _, label := range []string{locales.Settings(), locales.LocalSetupOperationStatus()} {
		if err := navigate(tt, label); err != nil {
			t.Fatal(err)
		}
	}
	if !tt.HasText(locales.LocalSetupBusy()) {
		t.Fatal("navigation lost operation")
	}
	if err := openPage(tt, locales.Settings()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{locales.Focus(), locales.SearchWorkbench()} {
		if err := navigate(tt, label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type(locales.LocalSetupOperationStatus())
	if err := tt.Click(locales.LocalSetupOperationStatus()); err != nil { // the palette's item
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LocalSetupComplete())
	if err := toggleFocusMode(tt); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ReturnDiagnostics()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if tt.HasText(locales.LocalSetupOperationStatus()) {
		t.Fatal("acknowledged operation remained pending")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "setup.local\n") != 1 || strings.Count(string(calls), "diagnostics.inspect\n") != 2 {
		t.Fatalf("incorrect mutation/diagnostic refresh count: %q", calls)
	}
}

func TestLocalSetupCloseCancelsOutstandingWork(t *testing.T) {
	tt, a, _, _ := setupTester(t, "slow-setup", "en")
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_SETUP_STARTED", started)
	if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	start := time.Now()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 5*time.Second {
		t.Fatal("closing left the setup subprocess running")
	}
}

func TestLocalSetupMinimumLocalizedLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _, _, _ := setupTester(t, "", language)
			waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
			for _, label := range []string{locales.LocalSetupTitle(), locales.ConfirmLocalSetup(), locales.Cancel()} {
				rect, ok := tt.Find(label)
				if !ok || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 901 || rect.Y+rect.H > 601 {
					t.Errorf("%q outside minimum window: %+v", label, rect)
				}
			}
			renderLocalSetup(t, tt, "setup-"+language)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "setup-partial")
			if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.LocalSetupIncomplete())
			for _, label := range []string{locales.LocalSetupIncomplete(), locales.ReturnDiagnostics()} {
				rect, ok := tt.Find(label)
				if !ok || rect.X < 0 || rect.Y < 0 || rect.X+rect.W > 901 || rect.Y+rect.H > 601 {
					t.Errorf("result %q outside minimum window: %+v", label, rect)
				}
			}
			renderLocalSetup(t, tt, "setup-result-"+language)
		})
	}
}

func TestLocalSetupDiscardsInFlightDiagnostics(t *testing.T) {
	tt, _, _, log := setupTester(t, "", "en")
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	complete, mutationStarted := filepath.Join(t.TempDir(), "complete"), filepath.Join(t.TempDir(), "mutation-started")
	t.Setenv("DESKTOP_TEST_SETUP_RELEASE", complete)
	t.Setenv("DESKTOP_TEST_SETUP_STARTED", mutationStarted)
	if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, mutationStarted)
	if err := openPage(tt, locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	release, started := filepath.Join(t.TempDir(), "release-query"), filepath.Join(t.TempDir(), "query-started")
	t.Setenv("DESKTOP_TEST_DIAGNOSTICS_RELEASE", release)
	t.Setenv("DESKTOP_TEST_DIAGNOSTICS_STARTED", started)
	if err := tt.Click(locales.DiagnosticsRefresh()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if err := openPage(tt, locales.LocalSetupOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(complete, nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LocalSetupComplete())
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ReturnDiagnostics()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if tt.HasText(locales.DiagnosticsCanceled()) {
		t.Fatal("old canceled query replaced refreshed diagnostics")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "diagnostics.inspect\n") != 3 {
		t.Fatalf("returned stale diagnostics instead of refreshing: %q", calls)
	}
}

func TestLocalSetupPreflightCancelFromSearchLoadsDiagnostics(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	release, started, log := filepath.Join(t.TempDir(), "release"), filepath.Join(t.TempDir(), "started"), filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
	t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	tt := ui.NewTester(a.View, 900, 600)
	if err := openSearch(tt); err != nil {
		t.Fatal(err)
	}
	tt.Type(locales.LocalSetupTitle())
	if err := tt.Click(locales.LocalSetupTitle()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if !tt.HasText(locales.LocalSetupChecking()) {
		t.Fatal("test did not pause the read-only preflight")
	}
	// Only the already-running setup preflight is held. The diagnostic query
	// starts its own read-only capability check after cancellation.
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", "")
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "capabilities\n") != 2 || strings.Count(string(calls), "diagnostics.inspect\n") != 1 || strings.Contains(string(calls), "setup.local") {
		t.Fatalf("preflight cancellation failed to load diagnostics without mutation: %q", calls)
	}
	if tt.HasText(locales.LocalSetupOperationStatus()) {
		t.Fatal("read-only preflight was reported as a pending mutation")
	}
}

func renderLocalSetup(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	output := os.Getenv("DESKTOP_RENDER_DIR")
	if output == "" {
		return
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(output, name+".png"))
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

func TestLocalSetupCompletionRefreshesAlreadyIdleDiagnostics(t *testing.T) {
	tt, _, _, log := setupTester(t, "", "en")
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	release, started := filepath.Join(t.TempDir(), "release"), filepath.Join(t.TempDir(), "started")
	params := filepath.Join(t.TempDir(), "params")
	t.Setenv("DESKTOP_TEST_SETUP_RELEASE", release)
	t.Setenv("DESKTOP_TEST_SETUP_STARTED", started)
	t.Setenv("DESKTOP_TEST_DIAGNOSTICS_PARAMS", params)
	if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if err := openPage(tt, locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	// The initial report is already displayed and no diagnostic query is active.
	if tt.HasText(locales.DiagnosticsBusy()) {
		t.Fatal("diagnostics were not idle before setup completion")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "diagnostics.inspect\n") != 1 {
		t.Fatalf("unexpected pre-completion queries: %q", calls)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		calls, _ = os.ReadFile(log)
		if strings.Count(string(calls), "diagnostics.inspect\n") == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if strings.Count(string(calls), "diagnostics.inspect\n") != 2 {
		t.Fatalf("setup completion left idle diagnostics empty: %q", calls)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	assertDiagnosticOptions(t, params, false, false, false)
	if strings.Count(string(calls), "setup.local\n") != 1 {
		t.Fatalf("setup was repeated: %q", calls)
	}
	if err := openPage(tt, locales.LocalSetupOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.LocalSetupComplete()) {
		t.Fatal("diagnostics refresh discarded setup receipt")
	}
}

func TestLocalSetupCapabilitiesChangedAfterReviewShowUnsupportedWithoutRetry(t *testing.T) {
	for _, mode := range []string{"setup-unsupported", "setup-readonly", "setup-capability-unsupported-version"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := setupTester(t, "", "en")
			waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.LocalSetupUnsupported())
			if tt.HasText(locales.ErrorCLIProtocol()) || tt.HasText(locales.ConfirmLocalSetup()) {
				t.Fatal("unsupported CLI was shown as a protocol failure or offered a retry")
			}
			for _, label := range []string{locales.Settings(), locales.LocalSetupOperationStatus()} {
				if err := navigate(tt, label); err != nil {
					t.Fatal(err)
				}
			}
			if !tt.HasText(locales.LocalSetupUnsupported()) {
				t.Fatal("navigation lost the capability rejection")
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "setup.local") {
				t.Fatalf("capability rejection submitted mutation: %q", calls)
			}
		})
	}
}
