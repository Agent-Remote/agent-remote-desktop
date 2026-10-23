package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func environmentTester(t *testing.T, language string) (*ui.Tester, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
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
	tt.SetLocale(language)
	if err := tt.Click(locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsChecks())
	if err := tt.Click(locales.MutagenEnvironmentTitle()); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return tt, log
}
func inspectEnvironment(t *testing.T, tt *ui.Tester, mode string) {
	t.Helper()
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	if err := tt.Click(locales.MutagenEnvironmentInspect()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.MutagenEnvironmentReview())
}
func TestMutagenEnvironmentNoProjectNoLoginAndExplicitGlobalConfirmation(t *testing.T) {
	tt, log := environmentTester(t, "en")
	if calls, _ := os.ReadFile(log); len(calls) > 0 {
		t.Fatalf("opening page executed CLI %s", calls)
	}
	inspectEnvironment(t, tt, "unconfigured")
	for _, text := range []string{locales.MutagenEnvironmentImpact(), locales.MutagenEnvironmentPrevious(), locales.MutagenEnvironmentDesired(), locales.MutagenEnvironmentSessionCount("2"), locales.MutagenEnvironmentField("identifier", "sync_project0"), locales.MutagenEnvironmentField("identifier", "fwrd_forward1"), locales.MutagenEnvironmentPaused("true")} {
		if !tt.HasText(text) {
			t.Fatalf("missing global scope %s", text)
		}
	}
	if calls, _ := os.ReadFile(log); string(calls) != "capabilities\nmutagen.environment.preview\n" {
		t.Fatalf("unexpected authentication or mutation %s", calls)
	}
	if err := tt.Click(locales.MutagenEnvironmentConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.MutagenEnvironmentOutcome("completed"))
	if !tt.HasText(locales.MutagenEnvironmentReady()) {
		t.Fatal("no verified environment result")
	}
	if tt.HasText(locales.MutagenEnvironmentConfirm()) {
		t.Fatal("unsafe replay confirmation")
	}
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.MutagenEnvironmentOperation()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.MutagenEnvironmentOutcome("completed")) {
		t.Fatal("lost global receipt")
	}
	if calls, _ := os.ReadFile(log); strings.Count(string(calls), "mutagen.environment.apply") != 1 {
		t.Fatalf("replayed %s", calls)
	}
}
func TestMutagenEnvironmentCompleteRosterIsPagedWithoutTruncation(t *testing.T) {
	tt, log := environmentTester(t, "en")
	inspectEnvironment(t, tt, "environment-paged")
	if !tt.HasText(locales.MutagenEnvironmentSessionCount("10")) || !tt.HasText(locales.MutagenEnvironmentPage("1", "2")) {
		t.Fatal("complete scope count missing")
	}
	if tt.HasText(locales.MutagenEnvironmentField("identifier", "sync_project8")) {
		t.Fatal("paging not active")
	}
	tt.Scroll(650, 350, 0, 10000)
	if err := tt.Click(locales.MutagenEnvironmentNextPage()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.MutagenEnvironmentField("identifier", "sync_project8")) || !tt.HasText(locales.MutagenEnvironmentField("identifier", "fwrd_forward9")) || !tt.HasText(locales.MutagenEnvironmentPage("2", "2")) {
		t.Fatal("omitted last page scope")
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "mutagen.environment.apply") {
		t.Fatal("page navigation restarted service")
	}
}
func TestMutagenEnvironmentBlockedPlanCannotRestart(t *testing.T) {
	for _, mode := range []string{"environment-external", "environment-blocked"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := environmentTester(t, "zh-Hans")
			inspectEnvironment(t, tt, mode)
			_ = tt.Click(locales.MutagenEnvironmentConfirm())
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "mutagen.environment.apply") {
				t.Fatal("blocked service restarted")
			}
		})
	}
}
func TestMutagenEnvironmentPartialAndCancellationKeepOriginalScope(t *testing.T) {
	for _, mode := range []string{"environment-partial", "environment-unknown", "environment-persist-unknown", "environment-signal-unknown", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := environmentTester(t, "en")
			inspectEnvironment(t, tt, "")
			if mode == "cancel" {
				barrier := t.TempDir()
				started := filepath.Join(barrier, "started")
				t.Setenv("DESKTOP_TEST_ENVIRONMENT_STARTED", started)
				t.Setenv("DESKTOP_TEST_ENVIRONMENT_RELEASE", filepath.Join(barrier, "release"))
				if err := tt.Click(locales.MutagenEnvironmentConfirm()); err != nil {
					t.Fatal(err)
				}
				waitAccountStarted(t, started)
				if err := tt.Click(locales.ReturnWorkbench()); err != nil {
					t.Fatal(err)
				}
				if err := tt.Click(locales.MutagenEnvironmentOperation()); err != nil {
					t.Fatal(err)
				}
				if err := tt.Click(locales.SyncStopWaiting()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, locales.MutagenEnvironmentUnknown())
			} else {
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				if err := tt.Click(locales.MutagenEnvironmentConfirm()); err != nil {
					t.Fatal(err)
				}
				outcome := "unknown"
				if mode == "environment-partial" {
					outcome = "partial"
				}
				if mode == "environment-signal-unknown" {
					waitRemoteText(t, tt, locales.MutagenEnvironmentUnknown())
				} else {
					waitRemoteText(t, tt, locales.MutagenEnvironmentOutcome(outcome))
				}
			}
			if tt.HasText(locales.MutagenEnvironmentReady()) || tt.HasText(locales.MutagenEnvironmentConfirm()) {
				t.Fatal("incomplete operation claimed readiness or replay")
			}
			if !tt.HasText(locales.MutagenEnvironmentReceiptScope()) || !tt.HasText(locales.MutagenEnvironmentField("identifier", "fwrd_forward1")) {
				t.Fatal("lost reviewed forwarding scope")
			}
			if calls, _ := os.ReadFile(log); strings.Count(string(calls), "mutagen.environment.apply") != 1 {
				t.Fatalf("replayed %s", calls)
			}
		})
	}
}
func TestMutagenEnvironmentRetainsProjectReceiptAndInvalidatesObservation(t *testing.T) {
	tt, _ := syncTester(t, "en")
	if err := tt.Click(locales.SyncAction("pause")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.SyncConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncOutcome("completed"))
	if err := tt.Click(locales.SyncRefresh()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncObservationScope())
	if err := tt.Click(locales.MutagenEnvironmentTitle()); err != nil {
		t.Fatal(err)
	}
	inspectEnvironment(t, tt, "")
	if err := tt.Click(locales.MutagenEnvironmentConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.MutagenEnvironmentOutcome("completed"))
	if err := tt.Click(locales.SyncOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SyncOutcome("completed")) || tt.HasText(locales.SyncObservationScope()) || !tt.HasText(locales.MutagenEnvironmentResultCode("sync_mutagen_environment_changed")) {
		t.Fatal("global restart lost receipt or kept stale observation")
	}
}
func TestMutagenEnvironmentLayouts(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := environmentTester(t, language)
			inspectEnvironment(t, tt, "")
			captureSyncSnapshot(t, tt, language+"-environment-preview")
			tt.Scroll(650, 350, 0, 10000)
			captureSyncSnapshot(t, tt, language+"-environment-sessions")
			if err := tt.Click(locales.MutagenEnvironmentConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.MutagenEnvironmentOutcome("completed"))
			tt.Scroll(650, 350, 0, -10000)
			captureSyncSnapshot(t, tt, language+"-environment-receipt")
		})
	}
}
