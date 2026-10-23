package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func runtimeFixturePlan(t *testing.T, action string) (cliadapter.Client, remote.RuntimeRecoveryPlan) {
	t.Helper()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-ready")
	authority, err := remote.GetRuntimeRecoveryAuthority(context.Background(), client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: "https://test.invalid"})
	if err != nil || !authority.Admin {
		t.Fatalf("authority: %+v %v", authority, err)
	}
	plan, err := remote.InspectRuntimeRecovery(context.Background(), client, remote.RuntimeRecoveryInspectInput{Expected: authority.Context, AccountID: testutil.RuntimeAccountID, OriginalTaskID: testutil.RuntimeOriginalTask, Action: action})
	if err != nil {
		t.Fatal(err)
	}
	return client, plan
}
func TestRuntimeRecoveryTypedThreeActionsAndExactStatus(t *testing.T) {
	for _, action := range []string{"verify_target", "verify_source", "repair_source"} {
		t.Run(action, func(t *testing.T) {
			client, plan := runtimeFixturePlan(t, action)
			t.Setenv("DESKTOP_TEST_RUNTIME_JOURNAL", filepath.Join(t.TempDir(), "journal"))
			if plan.Context.ActorUserID == plan.Original.UserID {
				t.Fatal("actor conflated with owner")
			}
			receipt, err := remote.ApplyRuntimeRecovery(context.Background(), client, remote.RuntimeRecoveryApplyInput{Confirmed: true, ExpectedPlan: plan, RequestID: testutil.RuntimeRequestID})
			if err != nil {
				t.Fatal(err)
			}
			if receipt.TargetCompletionConfirmed || receipt.SourceRestorationConfirmed {
				t.Fatal("acceptance claimed success")
			}
			pending, err := remote.ListRuntimeRecoveryPending(context.Background(), client)
			if err != nil || len(pending) != 1 || pending[0].Binding == nil {
				t.Fatalf("pending: %+v %v", pending, err)
			}
			query := remote.RuntimeRecoveryStatusInput{Expected: plan.Context, AccountID: plan.AccountID, RequestID: testutil.RuntimeRequestID, ExpectedDigest: &plan.Digest, ExpectedBinding: &receipt.Recovery.Binding}
			result, err := remote.GetRuntimeRecoveryStatus(context.Background(), client, query)
			if err != nil || result.TargetCompletionConfirmed != (action == "verify_target") || result.SourceRestorationConfirmed != (action != "verify_target") {
				t.Fatalf("status: %+v %v", result, err)
			}
			_, err = remote.ForgetRuntimeRecovery(context.Background(), client, remote.RuntimeRecoveryForgetInput{Confirmed: true, Expected: query.Expected, AccountID: query.AccountID, RequestID: query.RequestID, ExpectedDigest: query.ExpectedDigest, ExpectedBinding: query.ExpectedBinding})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestRuntimeRecoveryRejectsUnsafeObservations(t *testing.T) {
	for _, mode := range []string{"runtime-original-extra", "runtime-original-null", "runtime-eligible"} {
		t.Run(mode, func(t *testing.T) {
			client, plan := runtimeFixturePlan(t, "verify_target")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.InspectRuntimeRecovery(context.Background(), client, remote.RuntimeRecoveryInspectInput{Expected: plan.Context, AccountID: plan.AccountID, OriginalTaskID: plan.Original.TaskID, Action: plan.Action})
			if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("unsafe plan accepted: %v", err)
			}
		})
	}
}
func TestRuntimeRecoveryMalformedMutationKeepsUnknownOutcome(t *testing.T) {
	for _, mode := range []string{"runtime-v1-null-action", "runtime-wrong-owner", "runtime-wrong-key", "runtime-both-confirmed", "runtime-no-confirmation", "runtime-wrong-digest", "runtime-wrong-opid", "runtime-wrong-version"} {
		t.Run(mode, func(t *testing.T) {
			client, plan := runtimeFixturePlan(t, "verify_target")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.ApplyRuntimeRecovery(context.Background(), client, remote.RuntimeRecoveryApplyInput{Confirmed: true, ExpectedPlan: plan, RequestID: testutil.RuntimeRequestID})
			var op *cliadapter.OperationError
			if !errors.As(err, &op) || op.Code != "runtime_recovery_outcome_unknown" {
				t.Fatalf("mutation did not retain uncertainty: %v", err)
			}
		})
	}
}
func TestRuntimeRecoveryManualQueriesHaveNoJournalAuthority(t *testing.T) {
	client, plan := runtimeFixturePlan(t, "verify_target")
	journal := filepath.Join(t.TempDir(), "journal")
	t.Setenv("DESKTOP_TEST_RUNTIME_JOURNAL", journal)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-manual-repair_source")
	query := remote.RuntimeRecoveryStatusInput{Expected: plan.Context, AccountID: plan.AccountID, RequestID: testutil.RuntimeRequestID}
	first, err := remote.GetRuntimeRecoveryStatus(context.Background(), client, query)
	if err != nil || first.Digest != nil || first.Recovery.Binding.RecoveryAction() != "repair_source" {
		t.Fatalf("manual status: %+v %v", first, err)
	}
	query.ExpectedBinding = &first.Recovery.Binding
	if _, err = remote.GetRuntimeRecoveryStatus(context.Background(), client, query); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-manual-verify_source")
	if _, err = remote.GetRuntimeRecoveryStatus(context.Background(), client, query); !errors.Is(err, fault.CLIProtocol) {
		t.Fatal("changed manual binding accepted")
	}
	if _, err = os.Stat(journal); !os.IsNotExist(err) {
		t.Fatal("manual query created a journal")
	}
	_, err = remote.ForgetRuntimeRecovery(context.Background(), client, remote.RuntimeRecoveryForgetInput{Confirmed: true, Expected: query.Expected, AccountID: query.AccountID, RequestID: query.RequestID, ExpectedBinding: query.ExpectedBinding})
	var op *cliadapter.OperationError
	if !errors.As(err, &op) || op.Code != "invalid_params" {
		t.Fatal("manual query enabled forget")
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "runtime-pending-wrong-home")
	if _, err = remote.ListRuntimeRecoveryPending(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
		t.Fatal("wrong empty journal home accepted")
	}
}

func TestRuntimeRecoveryAuthorityAcceptsOnlyEquivalentCanonicalServer(t *testing.T) {
	client, _ := runtimeFixturePlan(t, "verify_target")
	for _, server := range []string{"https://TEST.invalid:443/", "https://test.invalid///"} {
		value, err := remote.GetRuntimeRecoveryAuthority(context.Background(), client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: server})
		if err != nil || value.Context.ServerURL != "https://test.invalid" {
			t.Fatalf("canonical authority %q: %+v %v", server, value, err)
		}
	}
	for _, server := range []string{"https://other.invalid", "https://test.invalid/api", "http://test.invalid", "https://user@test.invalid", "https://test.invalid?", "https://test.invalid#"} {
		if _, err := remote.GetRuntimeRecoveryAuthority(context.Background(), client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: server}); err == nil {
			t.Fatalf("different/unsafe server %q accepted", server)
		}
	}
}
