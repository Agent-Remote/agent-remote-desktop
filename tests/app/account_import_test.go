package app_test

import (
	"encoding/json"
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

func importTester(t *testing.T, language string) (*ui.Tester, string) {
	t.Helper()
	tt, log := accountsTester(t, language)
	openAccountDetails(t, tt)
	if err := tt.Click(locales.AccountImportTitle()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(log, nil, 0600); err != nil {
		t.Fatal(err)
	}
	return tt, log
}
func discoverImport(t *testing.T, tt *ui.Tester, mode string) {
	t.Helper()
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	if err := tt.Click(locales.AccountImportDiscover()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportPrepare())
}
func prepareImport(t *testing.T, tt *ui.Tester, mode string) {
	t.Helper()
	discoverImport(t, tt, mode)
	if err := tt.Click(locales.AccountImportPrepare()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportManifest())
}
func applyImport(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.AccountImportApply()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountImportApplyConfirm()); err != nil {
		t.Fatal(err)
	}
}
func TestAccountImportExplicitLocalReadsAndSeparateServerConfirmations(t *testing.T) {
	tt, log := importTester(t, "en")
	if calls, _ := os.ReadFile(log); len(calls) > 0 {
		t.Fatalf("open read local files %s", calls)
	}
	prepareImport(t, tt, "")
	if !tt.HasText(locales.AccountImportPrivacy()) || !tt.HasText(locales.AccountImportCount("2", "32")) {
		t.Fatal("missing privacy/full scope")
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.import.validate") || strings.Contains(string(calls), "accounts.import.apply") {
		t.Fatalf("implicit upload %s", calls)
	}
	if err := tt.Click(locales.AccountImportValidate()); err != nil {
		t.Fatal(err)
	}
	calls, _ = os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.import.validate") {
		t.Fatal("audit before confirmation")
	}
	if err := tt.Click(locales.AccountImportValidateConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportValidationCounts("2", "0"))
	if err := tt.Click(locales.AccountImportApply()); err != nil {
		t.Fatal(err)
	}
	calls, _ = os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.import.apply") {
		t.Fatal("upload before confirmation")
	}
	if err := tt.Click(locales.AccountImportApplyConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportPhase("accepted"))
	if tt.HasText(locales.AccountImportPhase("succeeded")) || tt.HasText(locales.AccountImportApplyConfirm()) {
		t.Fatal("acceptance claimed completion or allowed replay")
	}
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := openPage(tt, locales.AccountImportRecoveryTitle()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AccountImportPhase("accepted")) {
		t.Fatal("lost original receipt")
	}
	calls, _ = os.ReadFile(log)
	if strings.Count(string(calls), "accounts.import.apply") != 1 {
		t.Fatalf("replayed %s", calls)
	}
}
func TestAccountImportOptionalValidationAndCompletePagedManifest(t *testing.T) {
	tt, log := importTester(t, "en")
	prepareImport(t, tt, "import-paged")
	if !tt.HasText(locales.AccountImportCount("10", "160")) {
		t.Fatal("incomplete roster")
	}
	tt.Scroll(650, 350, 0, 10000)
	if err := tt.Click(locales.AccountImportNextPage()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AccountImportFile("~/.claude/rules/rule-0009.md", "16")) {
		t.Fatal("last manifest item missing")
	}
	applyImport(t, tt)
	waitRemoteText(t, tt, locales.AccountImportPhase("accepted"))
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.import.validate") {
		t.Fatal("optional audit auto dispatched")
	}
}
func TestAccountImportServerRejectDoesNotChangeSelection(t *testing.T) {
	tt, log := importTester(t, "zh-Hans")
	prepareImport(t, tt, "")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-reject-validation")
	if err := tt.Click(locales.AccountImportValidate()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountImportValidateConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportValidationCounts("1", "1"))
	_ = tt.Click(locales.AccountImportApply())
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.import.apply") {
		t.Fatal("rejected selection uploaded")
	}
	capture := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_IMPORT_REQUEST", capture)
	discoverImport(t, tt, "")
	data, _ := os.ReadFile(capture)
	var input struct {
		Selection struct {
			ExcludeSkills bool `json:"exclude_skills"`
		} `json:"selection"`
	}
	_ = json.Unmarshal(data, &input)
	if input.Selection.ExcludeSkills {
		t.Fatal("silently excluded skills")
	}
}
func TestAccountImportUnknownKeepsOriginalRequestAcrossRestart(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	journal := filepath.Join(t.TempDir(), "journal")
	t.Setenv("DESKTOP_TEST_IMPORT_JOURNAL", journal)
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tt := ui.NewTester(a.View, 900, 600)
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.RemoteAccounts()); err != nil {
		t.Fatal(err)
	}
	openAccountDetails(t, tt)
	if err := tt.Click(locales.AccountImportTitle()); err != nil {
		t.Fatal(err)
	}
	prepareImport(t, tt, "")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-unknown")
	applyImport(t, tt)
	waitRemoteText(t, tt, locales.AccountImportErrorCode("account_import_outcome_unknown"))
	data, err := os.ReadFile(journal)
	if err != nil {
		t.Fatal(err)
	}
	var ref struct {
		RequestID string `json:"request_id"`
	}
	if json.Unmarshal(data, &ref) != nil || ref.RequestID == "" {
		t.Fatal("no key")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.Close(); err != nil {
			t.Error(err)
		}
	})
	restored := ui.NewTester(b.View, 900, 600)
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	if err := openSearch(restored); err != nil {
		t.Fatal(err)
	}
	// Filter the palette before clicking: recovery may be below the initial
	// viewport once new commands are registered.
	restored.Type(locales.AccountImportRecoveryTitle())
	if err := restored.Click(locales.AccountImportRecoveryTitle()); err != nil { // the palette's item
		t.Fatal(err)
	}
	waitRemoteText(t, restored, locales.AccountImportRequest(ref.RequestID))
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.import.apply") || strings.Contains(string(calls), "accounts.import.prepare") {
		t.Fatalf("recovery reread/uploaded %s", calls)
	}
	if !strings.Contains(string(calls), "accounts.import.pending") {
		t.Fatalf("opening recovery did not query the CLI journal: %s", calls)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-status-unknown")
	restored.Scroll(650, 350, 0, 10000)
	if err := restored.Click(locales.AccountImportStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, restored, locales.AccountImportErrorCode("account_import_status_unknown"))
	if !restored.HasText(locales.AccountImportRequest(ref.RequestID)) || restored.HasText(locales.AccountImportForget()) {
		t.Fatal("404 erased/settled original request")
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.import.apply") || strings.Contains(string(calls), "accounts.import.prepare") {
		t.Fatalf("recovery reread/uploaded %s", calls)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	if err := openPage(restored, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, restored, "11111111-1111-4111-8111-111111111111")
	if err := restored.Click(locales.RemoteAccounts()); err != nil {
		t.Fatal(err)
	}
	openAccountDetails(t, restored)
	if err := restored.Click(locales.AccountImportTitle()); err != nil {
		t.Fatal(err)
	}
	prepareImport(t, restored, "")
	if !restored.HasText(locales.AccountImportRetainedHelp()) {
		t.Fatal("pending request did not block a new key")
	}
	_ = restored.Click(locales.AccountImportApply())
	if restored.HasText(locales.AccountImportApplyConfirm()) {
		t.Fatal("offered new request for uncertain account")
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.import.apply") {
		t.Fatal("unknown original retried")
	}

}
func TestAccountImportExactStatusAndExplicitForget(t *testing.T) {
	tt, log := importTester(t, "en")
	prepareImport(t, tt, "")
	applyImport(t, tt)
	waitRemoteText(t, tt, locales.AccountImportPhase("accepted"))
	tt.Scroll(650, 350, 0, 10000)
	if err := tt.Click(locales.AccountImportStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportPhase("succeeded"))
	if err := tt.Click(locales.AccountImportForget()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.import.forget") {
		t.Fatal("forgot without confirmation")
	}
	if err := tt.Click(locales.AccountImportForgetConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportNew())
	if tt.HasText(locales.AccountImportPhase("succeeded")) {
		t.Fatal("forgot record still shown")
	}
}
func TestAccountImportLayouts(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := importTester(t, language)
			prepareImport(t, tt, "import-paged")
			captureSyncSnapshot(t, tt, language+"-import-review")
			tt.Scroll(650, 350, 0, 10000)
			captureSyncSnapshot(t, tt, language+"-import-manifest")
			if err := tt.Click(locales.AccountImportApply()); err != nil {
				t.Fatal(err)
			}
			captureSyncSnapshot(t, tt, language+"-import-confirm")
			if err := tt.Click(locales.AccountImportApplyConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.AccountImportPhase("accepted"))
			tt.Scroll(650, 350, 0, 10000)
			captureSyncSnapshot(t, tt, language+"-import-receipt")
		})
	}
}

func TestAccountImportCancellationKeepsOriginalKeyWithoutReplay(t *testing.T) {
	tt, log := importTester(t, "en")
	prepareImport(t, tt, "")
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	t.Setenv("DESKTOP_TEST_IMPORT_STARTED", started)
	t.Setenv("DESKTOP_TEST_IMPORT_RELEASE", filepath.Join(dir, "release"))
	applyImport(t, tt)
	waitAccountStarted(t, started)
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := openPage(tt, locales.AccountImportRecoveryTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountImportStopWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportUnknown())
	if tt.HasText(locales.AccountImportApplyConfirm()) || tt.HasText(locales.AccountImportForget()) {
		t.Fatal("uncertain request became replayable/forgettable")
	}
	if calls, _ := os.ReadFile(log); strings.Count(string(calls), "accounts.import.apply") != 1 {
		t.Fatalf("replayed %s", calls)
	}
}

func TestAccountImportManagedSkillsRequiresExplicitForgetAndSelection(t *testing.T) {
	tt, log := importTester(t, "en")
	journal := filepath.Join(t.TempDir(), "journal")
	t.Setenv("DESKTOP_TEST_IMPORT_JOURNAL", journal)
	prepareImport(t, tt, "")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-skills-managed")
	applyImport(t, tt)
	waitRemoteText(t, tt, locales.AccountImportErrorCode("account_import_skills_managed"))
	if err := tt.Click(locales.AccountImportNew()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountImportPending()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportPhase("rejected"))
	tt.Scroll(650, 350, 0, 10000)
	if err := tt.Click(locales.AccountImportForget()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.import.forget") {
		t.Fatal("rejection forgotten without confirmation")
	}
	if err := tt.Click(locales.AccountImportForgetConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountImportNoPending())
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountDetailsTitle())
	openAccountDetails(t, tt)
	if err := tt.Click(locales.AccountImportTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountImportExcludeSkills()); err != nil {
		t.Fatal(err)
	}
	prepareImport(t, tt, "")
	capture := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_IMPORT_REQUEST", capture)
	applyImport(t, tt)
	waitRemoteText(t, tt, locales.AccountImportPhase("accepted"))
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Selection struct {
			ExcludeSkills bool `json:"exclude_skills"`
		} `json:"selection"`
	}
	if json.Unmarshal(raw, &input) != nil || !input.Selection.ExcludeSkills {
		t.Fatal("manual exclusion not bound to new plan")
	}
	if calls, _ := os.ReadFile(log); strings.Count(string(calls), "accounts.import.apply") != 2 || strings.Count(string(calls), "accounts.import.forget") != 1 {
		t.Fatalf("unexpected replay %s", calls)
	}
}
