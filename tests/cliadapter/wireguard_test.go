package cliadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func wireGuardClient(t *testing.T) (cliadapter.Client, string) {
	t.Helper()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return client, log
}

func fixtureWireGuardPlan(t *testing.T, client cliadapter.Client) remote.WireGuardConfigPlan {
	t.Helper()
	plan, err := remote.PlanWireGuardConfig(context.Background(), client, remote.WireGuardPlanInput{ExpectedHome: client.Home})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func TestWireGuardInvalidScopeDoesNotInvokeCLI(t *testing.T) {
	client, log := wireGuardClient(t)
	relative := "relative.conf"
	for _, input := range []remote.WireGuardPlanInput{{ExpectedHome: "relative"}, {ExpectedHome: filepath.Join(client.Home, "other")}, {ExpectedHome: client.Home, Output: &relative}} {
		if _, err := remote.PlanWireGuardConfig(context.Background(), client, input); err == nil {
			t.Fatal("accepted invalid plan scope")
		}
	}
	if _, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{}); !errors.Is(err, fault.WireGuardReview) {
		t.Fatal(err)
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("invalid scope invoked CLI: %v", err)
	}
}

func TestWireGuardCapabilitiesBlockReadAndMutation(t *testing.T) {
	client, log := wireGuardClient(t)
	plan := fixtureWireGuardPlan(t, client)
	for _, mode := range []string{"wg-unsupported", "wg-readonly", "other-platform", "duplicate-capability", "invalid-effect"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := remote.PlanWireGuardConfig(context.Background(), client, remote.WireGuardPlanInput{ExpectedHome: client.Home}); err == nil {
				t.Fatal("unsupported plan permitted")
			}
			if _, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan}); err == nil || errors.Is(err, fault.WireGuardOutcomeUnknown) {
				t.Fatalf("incorrect preflight error: %v", err)
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "capabilities\ncapabilities\n" {
				t.Fatalf("capability rejection executed operation: %q", calls)
			}
		})
	}
}

func TestWireGuardPlanConfirmationPreservesExactScope(t *testing.T) {
	client, log := wireGuardClient(t)
	plan := fixtureWireGuardPlan(t, client)
	if plan.Output.Path != filepath.Join(client.Home, "wireguard", "agent-remote.conf") || plan.Output.Action != "create" {
		t.Fatal("incorrect planned target")
	}
	entries, err := os.ReadDir(client.Home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("plan created CLI state: %v %v", entries, err)
	}
	if _, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{ExpectedPlan: plan}); !errors.Is(err, fault.WireGuardReview) {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY", capture)
	receipt, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
	if err != nil || !receipt.Complete || receipt.PeerCount == nil || *receipt.PeerCount != 2 {
		t.Fatalf("valid result: %+v %v", receipt, err)
	}
	var sent remote.WireGuardApplyInput
	data, err := os.ReadFile(capture)
	if err != nil || json.Unmarshal(data, &sent) != nil {
		t.Fatal("missing captured confirmation")
	}
	want, _ := json.Marshal(plan)
	got, _ := json.Marshal(sent.ExpectedPlan)
	if !sent.Confirmed || string(got) != string(want) {
		t.Fatal("confirmation replaced reviewed plan")
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.config.plan\ncapabilities\nwireguard.config.apply\n" {
		t.Fatalf("unexpected invocation: %q", calls)
	}
}

func patchWireGuard(t *testing.T, name string, set map[string]any, remove ...string) {
	t.Helper()
	data, err := json.Marshal(map[string]any{"set": set, "remove": remove})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(name, string(data))
}

func TestWireGuardMalformedPlansCannotBecomeConfirmation(t *testing.T) {
	client, _ := wireGuardClient(t)
	valid := fixtureWireGuardPlan(t, client)
	raw, _ := json.Marshal(valid.Output)
	for _, tc := range []struct {
		name   string
		set    map[string]any
		remove []string
	}{
		{name: "wrong-version", set: map[string]any{"plan_version": 2}},
		{name: "wrong-home", set: map[string]any{"home": filepath.Join(client.Home, "other")}},
		{name: "no-device", set: map[string]any{"context": map[string]any{"server_url": "https://test.invalid"}}},
		{name: "credential-url", set: map[string]any{"context": map[string]any{"server_url": "https://user:password@test.invalid", "active_device_id": "device-1"}}},
		{name: "missing-output", remove: []string{"output"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_PLAN_PATCH", tc.set, tc.remove...)
			if _, err := remote.PlanWireGuardConfig(context.Background(), client, remote.WireGuardPlanInput{ExpectedHome: client.Home}); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("malformed plan accepted: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		field  string
		value  any
		remove bool
	}{
		{"missing-target", "target_identity", nil, true},
		{"missing-components", "missing_directories", nil, true},
		{"null-components", "missing_directories", nil, false},
		{"traversal", "missing_directories", []string{"..", "wireguard"}, false},
		{"wrong-anchor", "anchor_path", filepath.Join(client.Home, "other"), false},
		{"wrong-identity", "anchor_identity", "not-an-identity", false},
		{"create-with-existing-file", "target_identity", strings.Repeat("a", 64), false},
		{"replace-without-identity", "action", "replace", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var output map[string]any
			if json.Unmarshal(raw, &output) != nil {
				t.Fatal("fixture output")
			}
			output[tc.field] = tc.value
			if tc.remove {
				delete(output, tc.field)
			}
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_PLAN_PATCH", map[string]any{"output": output})
			if _, err := remote.PlanWireGuardConfig(context.Background(), client, remote.WireGuardPlanInput{ExpectedHome: client.Home}); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("malformed output accepted: %v", err)
			}
		})
	}
}

func TestWireGuardPartialResultsPreserveStagesWithoutRetry(t *testing.T) {
	client, log := wireGuardClient(t)
	plan := fixtureWireGuardPlan(t, client)
	for _, mode := range []string{"wg-partial", "wg-enrollment-unknown", "wg-key-store-incomplete", "wg-publication-incomplete"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			result, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
			if err != nil || result.Complete || result.Code == "none" {
				t.Fatalf("partial result lost: %+v %v", result, err)
			}
			if mode == "wg-publication-incomplete" && result.Publication != "published" {
				t.Fatal("lost already-published file fact")
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "capabilities\nwireguard.config.apply\n" {
				t.Fatalf("partial result retried: %q", calls)
			}
		})
	}
}

func TestWireGuardMalformedReceiptsRemainUnknown(t *testing.T) {
	client, log := wireGuardClient(t)
	plan := fixtureWireGuardPlan(t, client)
	for _, tc := range []struct {
		name   string
		set    map[string]any
		remove []string
	}{
		{name: "missing-complete", remove: []string{"complete"}},
		{name: "null-complete", set: map[string]any{"complete": nil}},
		{name: "missing-peer-count", remove: []string{"peer_count"}},
		{name: "negative-peer-count", set: map[string]any{"peer_count": -1}},
		{name: "excess-peer-count", set: map[string]any{"peer_count": 1025}},
		{name: "complete-without-count", set: map[string]any{"peer_count": nil}},
		{name: "contradictory-completion", set: map[string]any{"complete": false}},
		{name: "unknown-code", set: map[string]any{"code": "unexpected"}},
		{name: "no-key", set: map[string]any{"key_state": "not_checked"}},
		{name: "unconfirmed-enrollment", set: map[string]any{"enrollment": "unknown"}},
		{name: "unpublished", set: map[string]any{"publication": "not_attempted"}},
		{name: "different-home", set: map[string]any{"home": filepath.Join(client.Home, "other")}},
		{name: "different-output", set: map[string]any{"output_path": filepath.Join(client.Home, "other.conf")}},
		{name: "different-device", set: map[string]any{"context": map[string]any{"server_url": plan.Context.ServerURL, "active_device_id": "other"}}},
		{name: "wrong-operation", set: map[string]any{"operation_id": "unrelated"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_APPLY_PATCH", tc.set, tc.remove...)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan}); !errors.Is(err, fault.WireGuardOutcomeUnknown) {
				t.Fatalf("invalid mutation receipt became known: %v", err)
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "capabilities\nwireguard.config.apply\n" {
				t.Fatalf("invalid receipt replayed: %q", calls)
			}
		})
	}
}

func TestWireGuardCancellationAfterDispatchDoesNotReplay(t *testing.T) {
	client, log := wireGuardClient(t)
	plan := fixtureWireGuardPlan(t, client)
	barrier := t.TempDir()
	started := filepath.Join(barrier, "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_STARTED", started)
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_RELEASE", filepath.Join(barrier, "release"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := remote.ApplyWireGuardConfig(ctx, client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
		done <- err
	}()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("apply never reached barrier")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, fault.WireGuardOutcomeUnknown) {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancel retained CLI")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.config.apply\n") != 1 {
		t.Fatalf("cancellation replayed: %q", calls)
	}
}

func TestWireGuardConfirmationFreezesCallerOwnedReferences(t *testing.T) {
	for _, replacement := range []bool{false, true} {
		t.Run(map[bool]string{false: "missing-directories", true: "existing-file-identity"}[replacement], func(t *testing.T) {
			client, _ := wireGuardClient(t)
			if replacement {
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "wg-replace")
			}
			plan := fixtureWireGuardPlan(t, client)
			original, _ := json.Marshal(plan)
			barrier := t.TempDir()
			started := filepath.Join(barrier, "started")
			release := filepath.Join(barrier, "release")
			t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
			t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
			done := make(chan error, 1)
			go func() {
				_, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
				done <- err
			}()
			waitWireGuardBarrier(t, started)
			capture := filepath.Join(barrier, "confirmation")
			// Set the apply capture only after capability dispatch. The environment lock
			// also orders this handoff after the worker's earlier Environ snapshot.
			t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY", capture)
			if replacement {
				*plan.Output.TargetIdentity = strings.Repeat("c", 64)
			} else {
				plan.Output.MissingDirectories[0] = "changed"
			}
			if err := os.WriteFile(release, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			var sent remote.WireGuardApplyInput
			if json.Unmarshal(data, &sent) != nil {
				t.Fatal("bad capture")
			}
			received, _ := json.Marshal(sent.ExpectedPlan)
			if string(received) != string(original) {
				t.Fatal("caller edits changed the confirmed plan")
			}
		})
	}
}

func TestWireGuardPreviewFreezesOutputSelection(t *testing.T) {
	client, _ := wireGuardClient(t)
	output := filepath.Join(client.Home, "first.conf")
	original := output
	barrier := t.TempDir()
	started := filepath.Join(barrier, "started")
	release := filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
	done := make(chan error, 1)
	go func() {
		plan, err := remote.PlanWireGuardConfig(context.Background(), client, remote.WireGuardPlanInput{ExpectedHome: client.Home, Output: &output})
		if err == nil && plan.Output.Path != original {
			err = errors.New("output selection changed during preview")
		}
		done <- err
	}()
	waitWireGuardBarrier(t, started)
	t.Setenv("DESKTOP_TEST_WIREGUARD_PLAN", filepath.Join(barrier, "request"))
	output = filepath.Join(client.Home, "changed.conf")
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func waitWireGuardBarrier(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("capability dispatch barrier did not open")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestWireGuardPreservesStoredKeyBeforeLateContextOrOutputFailure(t *testing.T) {
	client, _ := wireGuardClient(t)
	plan := fixtureWireGuardPlan(t, client)
	for _, tc := range []struct {
		code, enrollment string
		count            any
	}{{"wireguard_context_changed", "not_attempted", nil}, {"wireguard_context_changed", "confirmed", nil}, {"output_changed", "not_attempted", nil}, {"output_changed", "confirmed", 2}} {
		t.Run(tc.code+"-"+tc.enrollment, func(t *testing.T) {
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_APPLY_PATCH", map[string]any{"complete": false, "key_state": "created", "enrollment": tc.enrollment, "publication": "not_attempted", "code": tc.code, "peer_count": tc.count})
			result, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
			if err != nil || result.KeyState != "created" || result.Enrollment != tc.enrollment || result.Complete {
				t.Fatalf("lost saved key fact: %+v %v", result, err)
			}
		})
	}
}
