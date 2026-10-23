package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func reopenImportAfterReview(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.AccountImportNew()); err != nil {
		t.Fatal(err)
	}
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountDetailsTitle())
	openAccountDetails(t, tt)
	if err := tt.Click(locales.AccountImportTitle()); err != nil {
		t.Fatal(err)
	}
}

func TestAccountImportPreSubmitFailureAllowsExplicitFreshReview(t *testing.T) {
	for _, scenario := range []string{"import-changed", "read-only-capabilities", "account_import_empty", "invalid_params", "confirmation_required"} {
		t.Run(scenario, func(t *testing.T) {
			tt, log := importTester(t, "en")
			t.Setenv("DESKTOP_TEST_IMPORT_JOURNAL", filepath.Join(t.TempDir(), "journal"))
			prepareImport(t, tt, "")
			if scenario == "import-changed" || scenario == "read-only-capabilities" {
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", scenario)
			} else {
				t.Setenv("DESKTOP_TEST_IMPORT_APPLY_ERROR", scenario)
			}
			applyImport(t, tt)
			waitRemoteText(t, tt, locales.AccountImportNotSubmitted())
			if tt.HasText(locales.AccountImportApply()) || tt.HasText(locales.AccountImportStatus()) || tt.HasText(locales.AccountImportForget()) {
				t.Fatal("unsubmitted review exposed upload or recovery actions")
			}
			calls, _ := os.ReadFile(log)
			expectedApply := 1
			if scenario == "read-only-capabilities" {
				expectedApply = 0
			}
			if strings.Count(string(calls), "accounts.import.apply") != expectedApply || strings.Count(string(calls), "accounts.import.pending") != 2 {
				t.Fatalf("missing absent-key proof or automatic retry: %s", calls)
			}
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
			t.Setenv("DESKTOP_TEST_IMPORT_APPLY_ERROR", "")
			reopenImportAfterReview(t, tt)
			prepareImport(t, tt, "")
			if tt.HasText(locales.AccountImportRetainedHelp()) {
				t.Fatal("phantom reference still blocks fresh upload")
			}
			applyImport(t, tt)
			waitRemoteText(t, tt, locales.AccountImportPhase("accepted"))
			calls, _ = os.ReadFile(log)
			if strings.Count(string(calls), "accounts.import.apply") != expectedApply+1 {
				t.Fatalf("fresh explicit upload missing or replayed: %s", calls)
			}
		})
	}
}

func TestAccountImportPreSubmitCleanupRequiresAbsentJournalProof(t *testing.T) {
	for _, scenario := range []string{"pending-unavailable", "import-changed-retained", "account_import_outcome_unknown", "protocol_error"} {
		t.Run(scenario, func(t *testing.T) {
			tt, log := importTester(t, "en")
			t.Setenv("DESKTOP_TEST_IMPORT_JOURNAL", filepath.Join(t.TempDir(), "journal"))
			prepareImport(t, tt, "")
			code := "account_import_source_changed"
			switch scenario {
			case "pending-unavailable":
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-changed")
				t.Setenv("DESKTOP_TEST_IMPORT_PENDING_ERROR", "yes")
			case "import-changed-retained":
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", scenario)
			default:
				code = scenario
				t.Setenv("DESKTOP_TEST_IMPORT_APPLY_ERROR", code)
			}
			applyImport(t, tt)
			waitRemoteText(t, tt, locales.AccountImportErrorCode(code))
			if tt.HasText(locales.AccountImportNotSubmitted()) {
				t.Fatal("uncertain or durable request claimed unsubmitted")
			}
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
			t.Setenv("DESKTOP_TEST_IMPORT_PENDING_ERROR", "")
			t.Setenv("DESKTOP_TEST_IMPORT_APPLY_ERROR", "")
			reopenImportAfterReview(t, tt)
			prepareImport(t, tt, "")
			if !tt.HasText(locales.AccountImportRetainedHelp()) {
				t.Fatal("unproven request lost its recovery reference")
			}
			_ = tt.Click(locales.AccountImportApply())
			if tt.HasText(locales.AccountImportApplyConfirm()) {
				t.Fatal("unproven request enabled a new upload key")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts.import.apply") != 1 {
				t.Fatalf("uncertain request replayed: %s", calls)
			}
		})
	}
}
