package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func openSyncRecovery(t *testing.T, tt *ui.Tester, mode string) {
	t.Helper()
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	if err := tt.Click(locales.SyncRecoveryInspect()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncRecoveryTitle())
}
func TestSyncRecoveryKeepsOriginalResultAndSeparatelyReviewsNewRelation(t *testing.T) {
	tt, log := syncTester(t, "en")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "sync-partial")
	if err := tt.Click(locales.SyncAction("pause")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.SyncConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncOutcome("partial"))
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "recovery-stale")
	if err := tt.Click(locales.SyncRefresh()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncResultCode("sync_not_found"))
	openSyncRecovery(t, tt, "recovery-stale")
	for _, text := range []string{locales.SyncRecoveryEffects(), locales.SyncRecoveryScope("sync_not_found"), locales.SyncField("mutagen_identifier", "sync_exact_old"), locales.SyncField("configuration_fingerprint", "sha256:"+strings.Repeat("c", 64)), locales.SyncField("namespace_fingerprint", "sha256:"+strings.Repeat("b", 64))} {
		if !tt.HasText(text) {
			t.Fatalf("missing reviewed scope %s", text)
		}
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "sync.recovery.apply") {
		t.Fatalf("removed before confirmation: %s", calls)
	}
	request := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_SYNC_RECOVERY_REQUEST", request)
	if err := tt.Click(locales.SyncRecoveryConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncRecoveryFresh())
	if !tt.HasText(locales.SyncOutcome("partial")) || !tt.HasText(locales.SyncRecoveryReceipt()) {
		t.Fatal("recovery lost original result")
	}
	data, err := os.ReadFile(request)
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if json.Unmarshal(data, &input) != nil || input["confirmed"] != true {
		t.Fatalf("request %s", data)
	}
	plan, ok := input["expected_plan"].(map[string]any)
	if !ok || plan["mapping_fingerprint"] != "sha256:"+strings.Repeat("d", 64) {
		t.Fatalf("lost frozen plan: %s", data)
	}
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := openPage(tt, locales.SyncOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SyncRecoveryReceipt()) || !tt.HasText(locales.SyncOutcome("partial")) {
		t.Fatal("navigation lost receipts")
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "sync-no-session")
	if err := tt.Click(locales.SyncRecoveryFresh()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncNoSession())
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "sync.ensure") {
		t.Fatalf("fresh status automatically created relationship: %s", calls)
	}
	if err := tt.Click(locales.SyncAction("ensure")); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SyncActionHelp("ensure")) || !tt.HasText(locales.SyncConfirm()) {
		t.Fatal("new relationship missing separate review")
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	if err := tt.Click(locales.SyncConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SyncReceiptScope())
	// A previous completed recovery is already visible; wait for ensure's receipt.
	waitRemoteText(t, tt, locales.SyncAction("ensure"))
	if !tt.HasText(locales.SyncOutcome("partial")) || !tt.HasText(locales.SyncRecoveryReceipt()) {
		t.Fatal("new relationship overwrote old receipts")
	}
	calls, _ = os.ReadFile(log)
	if strings.Count(string(calls), "sync.recovery.apply") != 1 || strings.Count(string(calls), "sync.ensure") != 1 {
		t.Fatalf("replayed: %s", calls)
	}
}
func TestSyncRecoveryBlockedErrorsNeverShowExecutableConfirmation(t *testing.T) {
	for _, mode := range []string{"recovery-blocked", "recovery-denied", "recovery-unavailable", "recovery-not-required"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := syncTester(t, "zh-Hans")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := tt.Click(locales.SyncRecoveryInspect()); err != nil {
				t.Fatal(err)
			}
			if mode == "recovery-blocked" {
				waitRemoteText(t, tt, locales.SyncRecoveryBlocked())
				_ = tt.Click(locales.SyncRecoveryConfirm())
			} else {
				want := locales.SyncResultCode("sync_recovery_not_required")
				if mode == "recovery-denied" {
					want = locales.RemotePermissionDenied()
				}
				if mode == "recovery-unavailable" {
					want = locales.RemoteServerUnavailable()
				}
				waitRemoteText(t, tt, want)
				if tt.HasText(locales.SyncRecoveryConfirm()) {
					t.Fatal("Server error became recovery confirmation")
				}
			}
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "sync.recovery.apply") {
				t.Fatalf("unsafe removal: %s", calls)
			}
		})
	}
}
func TestSyncRecoveryCancellationAndPartialKeepScopeWithoutReplay(t *testing.T) {
	for _, mode := range []string{"recovery-partial", "recovery-unknown", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := syncTester(t, "en")
			openSyncRecovery(t, tt, "")
			if mode == "cancel" {
				barrier := t.TempDir()
				started := filepath.Join(barrier, "started")
				t.Setenv("DESKTOP_TEST_SYNC_RECOVERY_STARTED", started)
				t.Setenv("DESKTOP_TEST_SYNC_RECOVERY_RELEASE", filepath.Join(barrier, "release"))
				if err := tt.Click(locales.SyncRecoveryConfirm()); err != nil {
					t.Fatal(err)
				}
				waitAccountStarted(t, started)
				if err := tt.Click(locales.SyncStopWaiting()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, locales.SyncUnknown())
			} else {
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				if err := tt.Click(locales.SyncRecoveryConfirm()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, locales.SyncOutcome(strings.TrimPrefix(mode, "recovery-")))
			}
			if tt.HasText(locales.SyncRecoveryFresh()) || tt.HasText(locales.SyncRecoveryConfirm()) {
				t.Fatal("unverified result permits unreviewed continuation")
			}
			if err := tt.Click(locales.ReturnWorkbench()); err != nil {
				t.Fatal(err)
			}
			if err := openPage(tt, locales.SyncOperationStatus()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.SyncField("mutagen_identifier", "sync_exact_old")) {
				t.Fatal("lost original scope")
			}
			if calls, _ := os.ReadFile(log); strings.Count(string(calls), "sync.recovery.apply") != 1 {
				t.Fatalf("replayed: %s", calls)
			}
		})
	}
}
func TestSyncRecoveryReviewInvalidatesOnProjectChange(t *testing.T) {
	tt, log := syncTester(t, "en")
	openSyncRecovery(t, tt, "")
	if err := openProject(tt, "Second project"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if tt.HasText(locales.SyncRecoveryConfirm()) {
		t.Fatal("old review survived project change")
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "sync.recovery.apply") {
		t.Fatal("changed project mutated")
	}
}
func TestSyncRecoveryLayouts(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := syncTester(t, language)
			openSyncRecovery(t, tt, "")
			captureSyncSnapshot(t, tt, language+"-recovery-preview")
			if err := tt.Click(locales.SyncRecoveryConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.SyncRecoveryFresh())
			captureSyncSnapshot(t, tt, language+"-recovery-receipt")
		})
	}
}
