package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func runtimeTester(t *testing.T, language string) (*app.App, *ui.Tester, string, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-ready")
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
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
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	openRuntimePage(t, tt)
	return a, tt, home, log
}
func openRuntimePage(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.SearchWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RuntimeRecoveryTitle()); err != nil {
		t.Fatal(err)
	}
}
func runtimeAuthority(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.RuntimeRecoveryAuthority()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoveryAuthorityAdmin())
}
func runtimeFill(t *testing.T, tt *ui.Tester, label, value string) {
	t.Helper()
	runtimeReveal(t, tt, label)
	if err := tt.Click(label); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type(value)
}
func runtimeReveal(t *testing.T, tt *ui.Tester, label string) {
	t.Helper()
	for attempt := 0; attempt < 4; attempt++ {
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("missing control %q", label)
		}
		bottom := float32(475)
		if footer, ok := tt.Find(locales.RuntimeRecoveryFinish()); ok {
			bottom = footer.Y - 12
		}
		if r.Y >= 165 && r.Y+r.H <= bottom {
			return
		}
		tt.Scroll(650, 250, 0, r.Y-225)
	}
}
func runtimeInspect(t *testing.T, tt *ui.Tester, action string) {
	t.Helper()
	runtimeAuthority(t, tt)
	runtimeFill(t, tt, locales.RuntimeRecoveryAccount(), testutil.RuntimeAccountID)
	runtimeFill(t, tt, locales.RuntimeRecoveryOriginalTask(), testutil.RuntimeOriginalTask)
	runtimeReveal(t, tt, locales.RuntimeRecoveryAction(action))
	if err := tt.Click(locales.RuntimeRecoveryAction(action)); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RuntimeRecoveryInspect()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoveryObservation())
}
func runtimeApply(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.RuntimeRecoveryApply()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RuntimeRecoveryApplyConfirm()); err != nil {
		t.Fatal(err)
	}
}
func TestRuntimeRecoveryExplicitAuthorityAndThreeActions(t *testing.T) {
	for _, action := range []string{"verify_target", "verify_source", "repair_source"} {
		t.Run(action, func(t *testing.T) {
			_, tt, _, log := runtimeTester(t, "en")
			t.Setenv("DESKTOP_TEST_RUNTIME_JOURNAL", filepath.Join(t.TempDir(), "journal"))
			if calls, _ := os.ReadFile(log); len(calls) > 0 {
				t.Fatal("opening recovery performed I/O")
			}
			runtimeInspect(t, tt, action)
			if !tt.HasText(locales.RuntimeRecoveryActor(testutil.RuntimeActorID)) || !tt.HasText(locales.RuntimeRecoveryOwner(testutil.RuntimeOwnerID)) {
				t.Fatal("actor/owner missing or conflated")
			}
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.runtime_recovery.apply") {
				t.Fatal("inspection submitted recovery")
			}
			if err := tt.Click(locales.RuntimeRecoveryApply()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.RuntimeRecoveryConfirmHelp(action)) {
				t.Fatal("missing action-specific write confirmation")
			}
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.runtime_recovery.apply") {
				t.Fatal("review submitted recovery")
			}
			if err := tt.Click(locales.RuntimeRecoveryApplyConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.RuntimeRecoverySubmission("accepted"))
			if tt.HasText(locales.RuntimeRecoveryTargetConfirmed()) || tt.HasText(locales.RuntimeRecoverySourceConfirmed()) {
				t.Fatal("acceptance claimed success")
			}
			tt.Scroll(650, 350, 0, 10000)
			if err := tt.Click(locales.RuntimeRecoveryQuery()); err != nil {
				t.Fatal(err)
			}
			completion := locales.RuntimeRecoverySourceConfirmed()
			if action == "verify_target" {
				completion = locales.RuntimeRecoveryTargetConfirmed()
			}
			waitRemoteText(t, tt, completion)
			tt.Scroll(650, 350, 0, 10000)
			if err := tt.Click(locales.RuntimeRecoveryForget()); err != nil {
				t.Fatal(err)
			}
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.runtime_recovery.forget") {
				t.Fatal("forget before exact confirmation")
			}
			if !tt.HasText(locales.RuntimeRecoveryForgetHelp()) || !tt.HasText(locales.RuntimeRecoveryDigest("sha256:"+strings.Repeat("a", 64))) {
				t.Fatal("forget target/scope missing")
			}
			if err := tt.Click(locales.RuntimeRecoveryForgetConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.RuntimeRecoveryNoPending())
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts.runtime_recovery.apply") != 1 || strings.Count(string(calls), "accounts.runtime_recovery.forget") != 1 {
				t.Fatalf("replayed operation: %s", calls)
			}
		})
	}
}
func TestRuntimeRecoveryDeniedAndExpiredUserCannotPrepare(t *testing.T) {
	for _, mode := range []string{"runtime-denied", "runtime-expired"} {
		t.Run(mode, func(t *testing.T) {
			_, tt, _, log := runtimeTester(t, "zh-Hans")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := tt.Click(locales.RuntimeRecoveryAuthority()); err != nil {
				t.Fatal(err)
			}
			want := locales.RuntimeRecoveryAuthorityDenied()
			if mode == "runtime-expired" {
				want = locales.RuntimeRecoveryError("authentication_required")
			}
			waitRemoteText(t, tt, want)
			if tt.HasText(locales.RuntimeRecoveryInspect()) || tt.HasText(locales.RuntimeRecoveryApply()) {
				t.Fatal("unprivileged user offered recovery")
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "accounts.runtime_recovery.inspect") || strings.Contains(string(calls), "login.") {
				t.Fatal("automatic mutation/refresh")
			}
		})
	}
}
func TestRuntimeRecoveryManualKeyRemainsReadOnly(t *testing.T) {
	_, tt, _, log := runtimeTester(t, "en")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-manual-repair_source")
	runtimeAuthority(t, tt)
	tt.Scroll(650, 350, 0, 10000)
	runtimeFill(t, tt, locales.RuntimeRecoveryManualAccount(), testutil.RuntimeAccountID)
	runtimeFill(t, tt, locales.RuntimeRecoveryManualKey(), testutil.RuntimeRequestID)
	if err := tt.Click(locales.RuntimeRecoveryManualQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoverySourceConfirmed())
	if !tt.HasText(locales.RuntimeRecoveryAction("repair_source")) || tt.HasText(locales.RuntimeRecoveryApply()) || tt.HasText(locales.RuntimeRecoveryForget()) {
		t.Fatal("manual result became a plan/forgettable record")
	}
	tt.Scroll(650, 350, 0, 10000)
	if err := tt.Click(locales.RuntimeRecoveryManualQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoverySourceConfirmed())
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.runtime_recovery.apply") || strings.Contains(string(calls), "accounts.runtime_recovery.forget") {
		t.Fatalf("manual key mutated: %s", calls)
	}
}
func TestRuntimeRecoveryUnknownRestartRetainsOriginalKeyAndBlocksNew(t *testing.T) {
	a, tt, home, log := runtimeTester(t, "en")
	journal := filepath.Join(t.TempDir(), "journal")
	t.Setenv("DESKTOP_TEST_RUNTIME_JOURNAL", journal)
	runtimeInspect(t, tt, "repair_source")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-unknown")
	runtimeApply(t, tt)
	waitRemoteText(t, tt, locales.RuntimeRecoveryError("runtime_recovery_outcome_unknown"))
	raw, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		Key string `json:"request_id"`
	}
	if json.Unmarshal(raw, &ref) != nil || ref.Key == "" {
		t.Fatal("missing journal key")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	restored := ui.NewTester(b.View, 900, 600)
	openRuntimePage(t, restored)
	if err := restored.Click(locales.RuntimeRecoveryPending()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, restored, locales.RuntimeRecoveryRequest(ref.Key))
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-status-unknown")
	restored.Scroll(650, 350, 0, 10000)
	if err := restored.Click(locales.RuntimeRecoveryQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, restored, locales.RuntimeRecoveryError("runtime_recovery_status_unknown"))
	if restored.HasText(locales.RuntimeRecoveryForget()) {
		t.Fatal("404 permitted forgetting")
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-ready")
	restored.Scroll(650, 350, 0, -10000)
	runtimeInspect(t, restored, "repair_source")
	if !restored.HasText(locales.RuntimeRecoveryRetained()) {
		t.Fatal("new key not blocked")
	}
	_ = restored.Click(locales.RuntimeRecoveryApply())
	if restored.HasText(locales.RuntimeRecoveryApplyConfirm()) {
		t.Fatal("unknown original made new key possible")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.runtime_recovery.apply") != 1 {
		t.Fatal("unknown replayed")
	}
}
func TestRuntimeRecoveryLayouts(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			_, tt, _, _ := runtimeTester(t, language)
			runtimeInspect(t, tt, "repair_source")
			captureSyncSnapshot(t, tt, language+"-runtime-review")
			tt.Scroll(650, 350, 0, 10000)
			if err := tt.Click(locales.RuntimeRecoveryApply()); err != nil {
				t.Fatal(err)
			}
			captureSyncSnapshot(t, tt, language+"-runtime-confirm")
		})
	}
}

func TestRuntimeRecoveryUnknownBindingCanBeConfirmedAndForgotten(t *testing.T) {
	_, tt, _, log := runtimeTester(t, "en")
	t.Setenv("DESKTOP_TEST_RUNTIME_JOURNAL", filepath.Join(t.TempDir(), "journal"))
	runtimeInspect(t, tt, "verify_source")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-unknown")
	runtimeApply(t, tt)
	waitRemoteText(t, tt, locales.RuntimeRecoveryError("runtime_recovery_outcome_unknown"))
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-ready")
	runtimeReveal(t, tt, locales.RuntimeRecoveryQuery())
	if err := tt.Click(locales.RuntimeRecoveryQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoverySourceConfirmed())
	runtimeReveal(t, tt, locales.RuntimeRecoveryForget())
	if err := tt.Click(locales.RuntimeRecoveryForget()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RuntimeRecoveryForgetConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoveryFinish())
	if tt.HasText(locales.RuntimeRecoverySourceConfirmed()) {
		t.Fatal("forgotten record still shown")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.runtime_recovery.apply") != 1 {
		t.Fatal("unknown request replayed")
	}
}

func TestRuntimeRecoveryCancellationRetainsKeyWithoutReplay(t *testing.T) {
	_, tt, _, log := runtimeTester(t, "en")
	t.Setenv("DESKTOP_TEST_RUNTIME_JOURNAL", filepath.Join(t.TempDir(), "journal"))
	runtimeInspect(t, tt, "repair_source")
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	t.Setenv("DESKTOP_TEST_RUNTIME_STARTED", started)
	t.Setenv("DESKTOP_TEST_RUNTIME_RELEASE", filepath.Join(dir, "release"))
	runtimeApply(t, tt)
	waitAccountStarted(t, started)
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RuntimeRecoveryTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RuntimeRecoveryStopWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.RuntimeRecoveryError("runtime_recovery_outcome_unknown"))
	if tt.HasText(locales.RuntimeRecoveryApplyConfirm()) || tt.HasText(locales.RuntimeRecoveryForget()) {
		t.Fatal("cancelled wait lost uncertainty")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.runtime_recovery.apply") != 1 {
		t.Fatal("cancelled request replayed")
	}
}

func TestRuntimeRecoveryPreSubmitFailureDoesNotLeavePhantomRequest(t *testing.T) {
	_, tt, _, _ := runtimeTester(t, "en")
	runtimeInspect(t, tt, "verify_target")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-pre-submit")
	runtimeApply(t, tt)
	waitRemoteText(t, tt, locales.RuntimeRecoveryNotSubmitted())
	if err := tt.Click(locales.RuntimeRecoveryFinish()); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-ready")
	runtimeInspect(t, tt, "verify_target")
	if tt.HasText(locales.RuntimeRecoveryRetained()) {
		t.Fatal("unsubmitted request blocks fresh review")
	}
	runtimeApply(t, tt)
	waitRemoteText(t, tt, locales.RuntimeRecoverySubmission("accepted"))
}

func TestRuntimeRecoveryUnknownErrorCodeDoesNotExposeDiagnostics(t *testing.T) {
	_, tt, _, _ := runtimeTester(t, "en")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-error-raw")
	if err := tt.Click(locales.RuntimeRecoveryAuthority()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, presentation.Error(fault.CLIProtocol))
	if tt.HasText("private diagnostic") {
		t.Fatal("raw diagnostic reached UI")
	}
}
