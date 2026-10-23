package cliadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func importInputs(t *testing.T, mode string) (cliadapter.Client, remote.AccountImportApplyInput) {
	t.Helper()
	client, expected := defaultClient(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	selection := remote.AccountImportSelection{Tool: "claude"}
	discovery, err := remote.DiscoverAccountImport(context.Background(), client, selection)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := remote.PrepareAccountImport(context.Background(), client, remote.AccountImportPrepareInput{Expected: expected.Expected, AccountID: "account-1", Selection: selection, ExpectedSourceHome: discovery.SourceHome})
	if err != nil {
		t.Fatal(err)
	}
	key, err := remote.NewAccountImportRequestID()
	if err != nil {
		t.Fatal(err)
	}
	return client, remote.AccountImportApplyInput{Confirmed: true, ExpectedPlan: plan, RequestID: key}
}
func TestAccountImportRejectsIncompleteOrUnsafePlan(t *testing.T) {
	for _, mode := range []string{"import-missing", "import-raw", "import-bad-size", "import-duplicate", "import-path-escape", "import-bad-mode", "import-bad-hash", "import-null-selection", "import-wrong-account"} {
		t.Run(mode, func(t *testing.T) {
			client, expected := defaultClient(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.PrepareAccountImport(context.Background(), client, remote.AccountImportPrepareInput{Expected: expected.Expected, AccountID: "account-1", Selection: remote.AccountImportSelection{Tool: "claude"}, ExpectedSourceHome: filepath.Join(client.Home, "source")})
			if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("accepted invalid plan %v", err)
			}
		})
	}
}
func TestAccountImportLargeManifestUsesDigestOnlyWire(t *testing.T) {
	client, input := importInputs(t, "import-large")
	full, _ := json.Marshal(input.ExpectedPlan)
	if len(full) < 64*1024 {
		t.Fatal("small fixture")
	}
	capture := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_IMPORT_REQUEST", capture)
	receipt, err := remote.ApplyAccountImport(context.Background(), client, input)
	if err != nil || receipt.FileCount != 500 {
		t.Fatalf("%+v %v", receipt, err)
	}
	raw, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if json.Unmarshal(raw, &wire) != nil || len(wire) != 7 || wire["expected_digest"] != input.ExpectedPlan.Digest || wire["request_id"] != input.RequestID || strings.Contains(string(raw), "manifest") || len(raw) >= 64*1024 {
		t.Fatalf("unsafe wire %s", raw)
	}
}
func TestAccountImportRequiresConfirmationAndNonemptyPlan(t *testing.T) {
	for _, mode := range []string{"unconfirmed", "import-empty"} {
		t.Run(mode, func(t *testing.T) {
			client, input := importInputs(t, mode)
			if mode == "unconfirmed" {
				input.Confirmed = false
			}
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			if _, err := remote.ApplyAccountImport(context.Background(), client, input); err == nil {
				t.Fatal("accepted unconfirmed/empty")
			}
			if calls, _ := os.ReadFile(log); len(calls) > 0 {
				t.Fatalf("dispatched before confirmation %s", calls)
			}
		})
	}
}
func TestAccountImportReceiptBindsExactReviewedRequest(t *testing.T) {
	for _, mode := range []string{"import-receipt-wrong-key", "import-receipt-wrong-digest", "import-receipt-wrong-intent", "import-receipt-wrong-count", "import-receipt-no-task", "import-receipt-bad-phase", "import-receipt-bad-opid", "import-bad-warning"} {
		t.Run(mode, func(t *testing.T) {
			client, input := importInputs(t, "")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if _, err := remote.ApplyAccountImport(context.Background(), client, input); !errors.Is(err, fault.AccountOutcomeUnknown) {
				t.Fatalf("not uncertain: %v", err)
			}
		})
	}
}
func TestAccountImportValidationRequiresExplicitConfirmationAndKnownWarnings(t *testing.T) {
	client, input := importInputs(t, "")
	plan := input.ExpectedPlan
	validation := remote.AccountImportValidateInput{Expected: plan.Context, AccountID: plan.AccountID, Selection: plan.Selection, ExpectedSourceHome: plan.SourceHome, ExpectedInclude: plan.Include}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	if _, err := remote.ValidateAccountImport(context.Background(), client, validation); err == nil {
		t.Fatal("no explicit audit confirmation")
	}
	if calls, _ := os.ReadFile(log); len(calls) > 0 {
		t.Fatalf("sent validation %s", calls)
	}
	validation.Confirmed = true
	value, err := remote.ValidateAccountImport(context.Background(), client, validation)
	if err != nil || !value.AuditWritten || len(value.Accepted) != 2 {
		t.Fatalf("%+v %v", value, err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-bad-warning")
	if _, err := remote.ValidateAccountImport(context.Background(), client, validation); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("raw server prose passed %v", err)
	}
}
func TestAccountImportPendingAndStatusPreserveOriginalReference(t *testing.T) {
	client, input := importInputs(t, "")
	journal := filepath.Join(t.TempDir(), "journal")
	t.Setenv("DESKTOP_TEST_IMPORT_JOURNAL", journal)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-unknown")
	if _, err := remote.ApplyAccountImport(context.Background(), client, input); err == nil {
		t.Fatal("missing uncertain outcome")
	}
	refs, err := remote.ListAccountImportPending(context.Background(), client)
	if err != nil || len(refs) != 1 || refs[0].RequestID != input.RequestID || refs[0].TaskID != nil {
		t.Fatalf("%+v %v", refs, err)
	}
	before, _ := os.ReadFile(journal)
	query := remote.AccountImportStatusInput{Expected: refs[0].Context, AccountID: refs[0].AccountID, RequestID: refs[0].RequestID, ExpectedDigest: refs[0].Digest}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "import-status-unknown")
	if _, err := remote.GetAccountImportStatus(context.Background(), client, query); err == nil {
		t.Fatal("status404 falsely settled")
	}
	after, _ := os.ReadFile(journal)
	if string(before) != string(after) {
		t.Fatal("status rewrote intent")
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	value, err := remote.GetAccountImportStatus(context.Background(), client, query)
	if err != nil || value.Phase != "succeeded" {
		t.Fatalf("%+v %v", value, err)
	}
	wrong := "import_tool_account_config:account-1:44444444-4444-4444-8444-444444444444"
	query.ExpectedTaskID = &wrong
	if _, err := remote.GetAccountImportStatus(context.Background(), client, query); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("accepted other task %v", err)
	}
}
func TestAccountImportForgetRequiresConfirmation(t *testing.T) {
	client, input := importInputs(t, "")
	value := remote.AccountImportForgetInput{Expected: input.ExpectedPlan.Context, AccountID: input.ExpectedPlan.AccountID, RequestID: input.RequestID, ExpectedDigest: input.ExpectedPlan.Digest}
	if _, err := remote.ForgetAccountImport(context.Background(), client, value); err == nil {
		t.Fatal("forgot without confirmation")
	}
	value.Confirmed = true
	receipt, err := remote.ForgetAccountImport(context.Background(), client, value)
	if err != nil || !receipt.Forgotten {
		t.Fatalf("%+v %v", receipt, err)
	}
}
