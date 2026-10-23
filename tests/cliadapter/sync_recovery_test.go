package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func recoveryInputs(t *testing.T) (cliadapter.Client, remote.SyncRecoveryApplyInput) {
	t.Helper()
	client, input := syncInputs(t)
	plan, err := remote.PreviewSyncRecovery(context.Background(), client, remote.SyncRecoveryPreviewInput{Expected: input.Expected, LocalPath: input.ExpectedTarget.LocalPath})
	if err != nil {
		t.Fatal(err)
	}
	return client, remote.SyncRecoveryApplyInput{Confirmed: true, Expected: input.Expected, ExpectedPlan: plan}
}
func TestSyncRecoveryValidatesEveryReviewScope(t *testing.T) {
	for _, mode := range []string{"recovery-bad-scope", "recovery-missing-field", "recovery-unknown-field", "recovery-bad-digest", "recovery-bad-target", "recovery-bad-context", "recovery-null-namespace", "recovery-contradictory", "recovery-wrong-endpoint"} {
		t.Run(mode, func(t *testing.T) {
			client, input := syncInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.PreviewSyncRecovery(context.Background(), client, remote.SyncRecoveryPreviewInput{Expected: input.Expected, LocalPath: input.ExpectedTarget.LocalPath})
			if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("accepted invalid review: %v", err)
			}
		})
	}
}
func TestSyncRecoveryBlockedAndServerErrorsNeverPermitRemoval(t *testing.T) {
	for _, mode := range []string{"recovery-blocked", "recovery-denied", "recovery-unavailable", "recovery-not-required"} {
		t.Run(mode, func(t *testing.T) {
			client, input := syncInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			plan, err := remote.PreviewSyncRecovery(context.Background(), client, remote.SyncRecoveryPreviewInput{Expected: input.Expected, LocalPath: input.ExpectedTarget.LocalPath})
			if mode == "recovery-blocked" {
				if err != nil || plan.Recoverable || plan.ErrorCode == nil {
					t.Fatalf("blocked %+v %v", plan, err)
				}
			} else if err == nil {
				t.Fatal("Server failure became a recovery plan")
			}
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			_, err = remote.ApplySyncRecovery(context.Background(), client, remote.SyncRecoveryApplyInput{Confirmed: true, Expected: input.Expected, ExpectedPlan: plan})
			if err == nil {
				t.Fatal("unverified plan submitted")
			}
			if calls, _ := os.ReadFile(log); len(calls) > 0 {
				t.Fatalf("spawned with blocked plan %s", calls)
			}
		})
	}
}
func TestSyncRecoveryReceiptsPreservePartialUnknownAndResultScope(t *testing.T) {
	for _, mode := range []string{"", "recovery-workspace-missing", "recovery-absent", "recovery-partial", "recovery-unknown", "recovery-persist-partial", "recovery-persist-unknown"} {
		t.Run(mode, func(t *testing.T) {
			client, input := recoveryInputs(t)
			if mode == "recovery-workspace-missing" {
				input.ExpectedPlan.ServerState = "workspace_not_found"
			}
			if mode == "recovery-absent" {
				input.ExpectedPlan.Mutagen.State, input.ExpectedPlan.Mutagen.Target = "session_missing", nil
			}
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			receipt, err := remote.ApplySyncRecovery(context.Background(), client, input)
			if err != nil || receipt.Method != "sync.recovery.apply" {
				t.Fatalf("receipt %+v %v", receipt, err)
			}
			if strings.Contains(mode, "partial") || strings.Contains(mode, "unknown") {
				if receipt.Outcome == "completed" || receipt.ResultTarget.SyncSessionID == nil {
					t.Fatal("lost partial effect")
				}
			} else if receipt.Outcome != "completed" || receipt.ResultTarget.SyncSessionID != nil {
				t.Fatal("wrong completed result")
			}
			if mode == "recovery-workspace-missing" && receipt.ResultTarget.WorkspaceID != nil {
				t.Fatal("kept obsolete workspace")
			}
		})
	}
}
func TestSyncRecoveryMalformedMutationIsUnknown(t *testing.T) {
	for _, mode := range []string{"recovery-bad-scope", "recovery-missing-field", "recovery-unknown-field", "recovery-bad-digest", "recovery-bad-target", "recovery-bad-context", "recovery-null-namespace", "recovery-contradictory", "recovery-wrong-endpoint", "recovery-wrong-identifier", "recovery-wrong-result", "recovery-bad-stage", "recovery-wrong-operation", "recovery-wrong-namespace", "recovery-no-observation", "recovery-still-available", "recovery-partial-false-observation"} {
		t.Run(mode, func(t *testing.T) {
			client, input := recoveryInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.ApplySyncRecovery(context.Background(), client, input)
			var op *cliadapter.OperationError
			if !errors.As(err, &op) || op.Code != "sync_outcome_unknown" {
				t.Fatalf("not unknown: %v", err)
			}
		})
	}
}
func TestSyncRecoveryUnconfirmedDoesNotStartCLI(t *testing.T) {
	client, input := recoveryInputs(t)
	input.Confirmed = false
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	_, err := remote.ApplySyncRecovery(context.Background(), client, input)
	var op *cliadapter.OperationError
	if !errors.As(err, &op) || op.Code != "confirmation_required" {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(log); len(data) > 0 {
		t.Fatalf("spawned: %s", data)
	}
}
