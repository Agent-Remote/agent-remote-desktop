package app_test

import (
	"encoding/json"
	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func syncTester(t *testing.T, language string) (*ui.Tester, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	testutil.StoreWorkspace(t, home, testutil.SavedWorkspace(t))
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
	if err := tt.Click(locales.SyncOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncMutagenState("available"))
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return tt, log
}
func TestSyncReviewExactTargetDryRunAndNoReplay(t *testing.T) {
	for _, action := range []string{"ensure", "pause", "resume", "resolve", "reset"} {
		t.Run(action, func(t *testing.T) {
			tt, log := syncTester(t, "en")
			if err := tt.Click(locales.SyncSkipLocal()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.SyncAction(action)); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{locales.SyncActionHelp(action), locales.SyncSkipLocalHelp(), "https://test.invalid", "device-1", locales.SyncField("sync_session_id", "sync-1"), locales.SyncField("fingerprint", "sha256:"+strings.Repeat("a", 64))} {
				if !tt.HasText(want) {
					t.Fatalf("missing exact review %s: %v", want, tt.Texts())
				}
			}
			if data, _ := os.ReadFile(log); strings.Contains(string(data), "sync."+action) {
				t.Fatalf("submitted before review: %s", data)
			}
			capture := filepath.Join(t.TempDir(), "request")
			t.Setenv("DESKTOP_TEST_SYNC_REQUEST", capture)
			if err := tt.Click(locales.SyncConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.SyncOutcome("completed"))
			data, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			var input map[string]any
			if json.Unmarshal(data, &input) != nil || input["confirmed"] != true || input["dry_run"] != true {
				t.Fatalf("request %s", data)
			}
			if err := tt.Click(locales.ReturnWorkbench()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.SyncOperationStatus()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.SyncOutcome("completed")) {
				t.Fatal("navigation lost receipt")
			}
			if err := tt.Click(locales.SyncRefresh()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.SyncMutagenState("available"))
			if tt.HasText(locales.SyncConfirm()) || !tt.HasText(locales.SyncOutcome("completed")) {
				t.Fatal("refresh reset receipt/replay guard")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "sync."+action) != 1 {
				t.Fatalf("replayed: %s", calls)
			}
		})
	}
}

func TestSyncStageProjectionSurvivesDesktopRestart(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	testutil.StoreWorkspace(t, home, testutil.SavedWorkspace(t))
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 900, 600)
	if err := tester.Click(locales.SyncOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.SyncMutagenState("available"))
	if err := tester.Click(locales.SyncAction("pause")); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click(locales.SyncConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.SyncOutcome("completed"))
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reopenedTester := ui.NewTester(reopened.View, 900, 600)
	if err := reopenedTester.Click(locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	if !reopenedTester.HasText(locales.OperationSync()) || !reopenedTester.HasText(locales.OperationCompleted()) {
		t.Fatalf("sync lifecycle was not restored for review: %v", reopenedTester.Texts())
	}
	for _, stage := range []string{"server", "mutagen", "persist"} {
		if !reopenedTester.HasText(locales.SyncStage(stage, "completed")) {
			t.Fatalf("persisted sync stage %q missing: %v", stage, reopenedTester.Texts())
		}
	}
}
func TestSyncPartialUnknownAndLocaleKeepReceipts(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		for _, mode := range []string{"sync-partial", "sync-unknown"} {
			t.Run(language+mode, func(t *testing.T) {
				tt, log := syncTester(t, language)
				if err := tt.Click(locales.SyncAction("reset")); err != nil {
					t.Fatal(err)
				}
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				if err := tt.Click(locales.SyncConfirm()); err != nil {
					t.Fatal(err)
				}
				outcome := strings.TrimPrefix(mode, "sync-")
				waitRemoteText(t, tt, locales.SyncOutcome(outcome))
				if tt.HasText(locales.SyncConfirm()) {
					t.Fatal("unsafe replay available")
				}
				if err := tt.Click(locales.ReturnWorkbench()); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if err := tt.Click("Second project"); err != nil {
					t.Fatal(err)
				}
				tt.Frame()
				if err := tt.Click(locales.SyncOpen()); err != nil {
					t.Fatal(err)
				}
				if !tt.HasText(locales.SyncOutcome(outcome)) || !tt.HasText(locales.SyncField("project", "First project")) {
					t.Fatal("project switch lost operation target")
				}
				calls, _ := os.ReadFile(log)
				if strings.Count(string(calls), "sync.reset") != 1 {
					t.Fatalf("mutation replay: %s", calls)
				}
			})
		}
	}
}
func TestSyncProjectSwitchInvalidatesUnsubmittedReview(t *testing.T) {
	tt, log := syncTester(t, "en")
	if err := tt.Click(locales.SyncAction("reset")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Second project"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if err := tt.Click(locales.SyncOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncMutagenState("available"))
	if tt.HasText(locales.SyncConfirm()) || !tt.HasText(locales.SyncField("project", "Second project")) {
		t.Fatal("old review retained")
	}
	if data, _ := os.ReadFile(log); strings.Contains(string(data), "sync.reset") {
		t.Fatalf("mutated: %s", data)
	}
}

func TestSyncCancellationAfterSubmissionIsUnknown(t *testing.T) {
	tt, log := syncTester(t, "en")
	barrier := t.TempDir()
	started, release := filepath.Join(barrier, "started"), filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_SYNC_STARTED", started)
	t.Setenv("DESKTOP_TEST_SYNC_RELEASE", release)
	if err := tt.Click(locales.SyncAction("pause")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.SyncConfirm()); err != nil {
		t.Fatal(err)
	}
	waitAccountStarted(t, started)
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.SyncOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.SyncStopWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncUnknown())
	if tt.HasText(locales.SyncConfirm()) || tt.HasText(locales.RemoteQueryCanceled()) {
		t.Fatal("unsafe mutation cancellation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "sync.pause") != 1 {
		t.Fatalf("replayed: %s", calls)
	}
}

// TestSyncClosePersistsAttentionForInFlightMutation verifies that an explicit
// desktop shutdown cannot silently discard a submitted sync mutation when the
// CLI response has not arrived yet. The desktop must restore only a bounded
// attention summary after restart; it must never offer the old confirmation as
// an implicit retry.
func TestSyncClosePersistsAttentionForInFlightMutation(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	testutil.StoreWorkspace(t, home, testutil.SavedWorkspace(t))
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 900, 600)
	if err := tester.Click(locales.SyncOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.SyncMutagenState("available"))
	barrier := t.TempDir()
	started, release := filepath.Join(barrier, "started"), filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_SYNC_STARTED", started)
	t.Setenv("DESKTOP_TEST_SYNC_RELEASE", release)
	if err := tester.Click(locales.SyncAction("pause")); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click(locales.SyncConfirm()); err != nil {
		t.Fatal(err)
	}
	waitAccountStarted(t, started)
	// Quit while the mutating CLI request is still blocked. Close cancels and
	// reaps the helper; no release marker is written, so a fixture response
	// cannot be mistaken for a completed mutation.
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	saved := testutil.LoadWorkspace(t, home)
	if len(saved.Operations) != 1 || saved.Operations[0].ID != "sync" || saved.Operations[0].Status != "running" {
		t.Fatalf("in-flight sync operation was not persisted as running: %+v", saved.Operations)
	}

	reopened, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tester = ui.NewTester(reopened.View, 900, 600)
	if err := tester.Click(locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText(locales.OperationSync()) || !tester.HasText(locales.OperationAttention()) {
		t.Fatalf("in-flight sync operation was not recovered as attention: %v", tester.Texts())
	}
	if err := tester.Click(locales.OperationsOpen()); err != nil {
		t.Fatal(err)
	}
	if tester.HasText(locales.SyncConfirm()) {
		t.Fatal("restart exposed an unsafe sync retry confirmation")
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestSyncLogoutInvalidatesObservationKeepsReceipt(t *testing.T) {
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
	waitRemoteText(t, tt, locales.SyncMutagenState("available"))
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SignOutRemote()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ConfirmLogout()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LogoutLocalComplete())
	if err := tt.Click(locales.SyncOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SyncOutcome("completed")) || !tt.HasText(locales.RemoteContextChanged()) || tt.HasText(locales.SyncObservationScope()) {
		t.Fatal("context invalidation lost receipt or retained stale observation")
	}
}
func TestSyncLayoutSnapshots(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := syncTester(t, language)
			captureSyncSnapshot(t, tt, language+"-status")
			if err := tt.Click(locales.SyncAction("reset")); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.SyncConfirm()) {
				t.Fatal("missing review")
			}
			captureSyncSnapshot(t, tt, language+"-review")
		})
	}
}
func captureSyncSnapshot(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	dir := os.Getenv("DESKTOP_SYNC_SCREENSHOT_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(dir, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}

func TestSyncConflictCountKeepsFullIntegerAndLocalizedState(t *testing.T) {
	tt, _ := syncTester(t, "zh-Hans")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "sync-high-conflicts")
	if err := tt.Click(locales.SyncRefresh()); err != nil {
		t.Fatal(err)
	}
	expected := i18n.FormatNumber(i18n.Locale(), uint64(18446744073709551615), i18n.NumberOptions{Integer: true})
	waitRemoteText(t, tt, locales.SyncConflictCount(expected))
	if !tt.HasText(locales.SyncField("status", locales.SyncServerState("active"))) || !tt.HasText(locales.SyncField("sync_mode", locales.SyncMode("two_way"))) {
		t.Fatal("unlocalized Server state")
	}
}

func TestSyncServerConflictsAreLocalizedAndRemainActionable(t *testing.T) {
	tt, _ := syncTester(t, "zh-Hans")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "sync-conflict")
	if err := tt.Click(locales.SyncRefresh()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncField("status", locales.SyncServerState("conflicted")))
	if !tt.HasText(locales.SyncField("conflict_status", locales.SyncServerConflicts("has_conflicts"))) || !tt.HasText(locales.SyncConflicts("detected")) {
		t.Fatal("conflict state not localized")
	}
	if err := tt.Click(locales.SyncAction("resolve")); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SyncConfirm()) || !tt.HasText(locales.SyncActionHelp("resolve")) {
		t.Fatal("conflicted state blocked explicit resolution review")
	}
}
