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

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardTunnelRejectsInvalidInputBeforeCallingCLI(t *testing.T) {
	client, log := wireGuardClient(t)
	relative := "relative.conf"
	for _, input := range []remote.WireGuardTunnelPlanInput{
		{ExpectedHome: client.Home, Action: "restart"},
		{ExpectedHome: "relative", Action: "up"},
		{ExpectedHome: filepath.Join(client.Home, "other"), Action: "down"},
		{ExpectedHome: client.Home, Action: "up", Config: &relative},
	} {
		if _, err := remote.PlanWireGuardTunnel(context.Background(), client, input); err == nil {
			t.Fatal("invalid plan scope accepted")
		}
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatal("invalid input invoked CLI")
	}
}

func TestWireGuardTunnelStrictPlansRejectUnboundOrMisleadingFacts(t *testing.T) {
	client, _ := wireGuardClient(t)
	input := remote.WireGuardTunnelPlanInput{ExpectedHome: client.Home, Action: "down"}
	valid, err := remote.PlanWireGuardTunnel(context.Background(), client, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		edit func(map[string]any)
	}{
		{"version", func(p map[string]any) { p["version"] = 2 }},
		{"home", func(p map[string]any) { p["home"] = filepath.Join(client.Home, "other") }},
		{"action", func(p map[string]any) { p["action"] = "up" }},
		{"path", func(p map[string]any) {
			p["config"].(map[string]any)["path"] = filepath.Join(client.Home, "other.conf")
		}},
		{"missing-binding", func(p map[string]any) { p["binding"] = nil }},
		{"secret-binding", func(p map[string]any) { p["binding"].(map[string]any)["private_key"] = "forbidden" }},
		{"bad-digest", func(p map[string]any) { p["binding"].(map[string]any)["content_sha256"] = "not a digest" }},
		{"nonempty-issues", func(p map[string]any) { p["issues"] = []string{"ready"} }},
		{"unknown-issue", func(p map[string]any) { p["issues"] = []string{"ready", "unknown_issue"} }},
		{"duplicate-issue", func(p map[string]any) {
			p["issues"] = []string{"ready", "ready"}
		}},
		{"target-name", func(p map[string]any) { p["target"].(map[string]any)["logical_name"] = "different" }},
		{"target-missing-reason", func(p map[string]any) { delete(p["target"].(map[string]any), "reason") }},
		{"target-absent-with-identity", func(p map[string]any) { p["target"].(map[string]any)["state"] = "absent" }},
		{"target-missing-identity", func(p map[string]any) { p["target"].(map[string]any)["identity"] = nil }},
		{"target-null-key-without-issue", func(p map[string]any) { p["target"].(map[string]any)["identity"].(map[string]any)["public_key"] = nil }},
		{"target-private-field", func(p map[string]any) {
			p["target"].(map[string]any)["identity"].(map[string]any)["private_key"] = "forbidden"
		}},
		{"unknown-target-kind", func(p map[string]any) {
			p["target"].(map[string]any)["identity"].(map[string]any)["kind"] = "guessed_interface"
		}},
		{"tool-partial-binding", func(p map[string]any) { p["tools"].([]any)[0].(map[string]any)["sha256"] = nil }},
		{"tool-null-path", func(p map[string]any) { p["tools"].([]any)[0].(map[string]any)["path"] = nil }},
		{"tool-unavailable-with-binding", func(p map[string]any) { p["tools"].([]any)[0].(map[string]any)["state"] = "not_executable" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := networkJSONMap(t, valid)
			tc.edit(p)
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_PATCH", p)
			if _, err := remote.PlanWireGuardTunnel(context.Background(), client, input); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("invalid plan accepted: %v", err)
			}
		})
	}
}

func TestWireGuardTunnelPreservesUnknownAndUnobservedTargets(t *testing.T) {
	client, log := wireGuardClient(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "wg-tunnel-unknown")
	plan, err := remote.PlanWireGuardTunnel(context.Background(), client, remote.WireGuardTunnelPlanInput{ExpectedHome: client.Home, Action: "up"})
	if err != nil || plan.Target.State != "unknown" || plan.Target.Identity != nil || strings.Join(plan.Issues, ",") != "target_unknown" {
		t.Fatalf("unknown target lost: %v", err)
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.tunnel.plan\n" {
		t.Fatal("preview retried or executed another operation")
	}
}

func TestWireGuardTunnelFreezesSelectionAndCapturesTheOriginalRequest(t *testing.T) {
	client, _ := wireGuardClient(t)
	dir := t.TempDir()
	started, release, capture := filepath.Join(dir, "started"), filepath.Join(dir, "release"), filepath.Join(dir, "request")
	t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
	path := filepath.Join(dir, "reviewed.conf")
	want := path
	done := make(chan error, 1)
	go func() {
		_, err := remote.PlanWireGuardTunnel(context.Background(), client, remote.WireGuardTunnelPlanInput{ExpectedHome: client.Home, Action: "down", Config: &path})
		done <- err
	}()
	waitWireGuardBarrier(t, started)
	// The shared environment lock orders mutation after the worker's Environ
	// snapshot; an external file barrier alone does not order Go memory accesses.
	t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN", capture)
	path = filepath.Join(dir, "changed.conf")
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
	var input remote.WireGuardTunnelPlanInput
	if json.Unmarshal(data, &input) != nil || input.Config == nil || *input.Config != want || input.Action != "down" {
		t.Fatal("mutable input changed plan request")
	}
}

func TestWireGuardTunnelCancellationDoesNotReplay(t *testing.T) {
	client, log := wireGuardClient(t)
	dir := t.TempDir()
	started := filepath.Join(dir, "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_STARTED", started)
	t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_RELEASE", filepath.Join(dir, "release"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := remote.PlanWireGuardTunnel(ctx, client, remote.WireGuardTunnelPlanInput{ExpectedHome: client.Home, Action: "up"})
		done <- err
	}()
	waitWireGuardBarrier(t, started)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation lost: %v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("preview retained worker")
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.tunnel.plan\n" {
		t.Fatal("canceled preview was replayed")
	}
}
