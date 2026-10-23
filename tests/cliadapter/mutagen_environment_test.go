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

func environmentInputs(t *testing.T, mode string) (cliadapter.Client, remote.MutagenEnvironmentApplyInput) {
	t.Helper()
	client, _ := defaultClient(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	plan, err := remote.PreviewMutagenEnvironment(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	return client, remote.MutagenEnvironmentApplyInput{Confirmed: true, ExpectedHome: client.Home, ExpectedPlan: plan}
}
func TestMutagenEnvironmentRequiresCompleteAndConsistentPlan(t *testing.T) {
	for _, mode := range []string{"environment-bad-home", "environment-bad-field", "environment-missing-field", "environment-bad-fingerprint", "environment-no-tools", "environment-duplicate-tools", "environment-null-namespace", "environment-stopped-running", "environment-duplicate-session", "environment-bad-endpoint"} {
		t.Run(mode, func(t *testing.T) {
			client, _ := defaultClient(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.PreviewMutagenEnvironment(context.Background(), client)
			if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("accepted invalid complete scope: %v", err)
			}
		})
	}
}
func TestMutagenEnvironmentReadAndApplyDoNotRequestCredentials(t *testing.T) {
	for _, mode := range []string{"", "environment-stopped", "environment-new"} {
		t.Run(mode, func(t *testing.T) {
			client, input := environmentInputs(t, mode)
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			receipt, err := remote.ApplyMutagenEnvironment(context.Background(), client, input)
			if err != nil || receipt.Outcome != "completed" || receipt.Environment == nil {
				t.Fatalf("receipt %+v %v", receipt, err)
			}
			if mode == "environment-stopped" || mode == "environment-new" {
				if receipt.Stages[0].State != "skipped" {
					t.Fatal("started stopped service by stopping it first")
				}
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "capabilities\nmutagen.environment.apply\n" {
				t.Fatalf("unexpected credential/query call %s", calls)
			}
		})
	}
}
func TestMutagenEnvironmentLargeScopeUsesDigestOnlyConfirmation(t *testing.T) {
	client, input := environmentInputs(t, "environment-many")
	planBytes, _ := json.Marshal(input.ExpectedPlan)
	if len(planBytes) < 64*1024 {
		t.Fatal("fixture does not exceed request bound")
	}
	capture := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_ENVIRONMENT_REQUEST", capture)
	receipt, err := remote.ApplyMutagenEnvironment(context.Background(), client, input)
	if err != nil || len(receipt.Plan.Sessions) != 160 {
		t.Fatalf("scope lost %+v %v", receipt, err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if json.Unmarshal(data, &wire) != nil || len(wire) != 3 || wire["confirmed"] != true || wire["expected_fingerprint"] != input.ExpectedPlan.Fingerprint || wire["expected_home"] != client.Home {
		t.Fatalf("not exact digest request: %s", data)
	}
	if len(data) >= 64*1024 || strings.Contains(string(data), "sessions") {
		t.Fatal("sent full roster instead of digest")
	}
}
func TestMutagenEnvironmentBlockedOrUnconfirmedDoesNotStartApply(t *testing.T) {
	for _, mode := range []string{"environment-external", "environment-blocked", "unconfirmed"} {
		t.Run(mode, func(t *testing.T) {
			client, input := environmentInputs(t, mode)
			if mode == "unconfirmed" {
				input.Confirmed = false
			}
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			if _, err := remote.ApplyMutagenEnvironment(context.Background(), client, input); err == nil {
				t.Fatal("unsafe apply accepted")
			}
			if calls, _ := os.ReadFile(log); len(calls) > 0 {
				t.Fatalf("started with unreviewed/blocked plan %s", calls)
			}
		})
	}
}
func TestMutagenEnvironmentPartialAndUnknownPreserveReviewedScope(t *testing.T) {
	for _, mode := range []string{"environment-partial", "environment-unknown", "environment-persist-unknown"} {
		t.Run(mode, func(t *testing.T) {
			client, input := environmentInputs(t, "")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			receipt, err := remote.ApplyMutagenEnvironment(context.Background(), client, input)
			if err != nil || receipt.Outcome == "completed" || receipt.Environment != nil || receipt.Plan.Fingerprint != input.ExpectedPlan.Fingerprint {
				t.Fatalf("lost partial outcome %+v %v", receipt, err)
			}
		})
	}
}
func TestMutagenEnvironmentMalformedMutationNeverClaimsCompletion(t *testing.T) {
	for _, mode := range []string{"environment-bad-home", "environment-bad-field", "environment-missing-field", "environment-wrong-review", "environment-wrong-paused", "environment-wrong-config", "environment-missing-session", "environment-wrong-tools", "environment-wrong-namespace", "environment-wrong-binding", "environment-no-post", "environment-bad-stage", "environment-bad-operation", "environment-partial-post"} {
		t.Run(mode, func(t *testing.T) {
			client, input := environmentInputs(t, "")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.ApplyMutagenEnvironment(context.Background(), client, input)
			var operation *cliadapter.OperationError
			if !errors.As(err, &operation) || operation.Code != "sync_outcome_unknown" {
				t.Fatalf("invalid receipt not unknown: %v", err)
			}
		})
	}
}
