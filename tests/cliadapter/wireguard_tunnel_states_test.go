package cliadapter_test

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardTunnelValidatesReadOnlyCapabilityBeforePreview(t *testing.T) {
	client, log := wireGuardClient(t)
	for _, mode := range []string{"wg-tunnel-unsupported", "wg-tunnel-write", "wg-tunnel-credential", "other-platform", "duplicate-capability"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := os.WriteFile(log, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := remote.PlanWireGuardTunnel(context.Background(), client, remote.WireGuardTunnelPlanInput{ExpectedHome: client.Home, Action: "up"}); err == nil {
				t.Fatal("unsupported preview ran")
			}
			calls, err := os.ReadFile(log)
			if err != nil || string(calls) != "capabilities\n" {
				t.Fatalf("preview ran after rejected capability: %q, %v", calls, err)
			}
		})
	}
}

func TestWireGuardTunnelTargetStatesRemainDistinct(t *testing.T) {
	client, _ := wireGuardClient(t)
	input := remote.WireGuardTunnelPlanInput{ExpectedHome: client.Home, Action: "down"}
	valid, err := remote.PlanWireGuardTunnel(context.Background(), client, input)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		edit     func(map[string]any, map[string]any)
		issues   []string
		rejected bool
	}{
		{"unconfigured-key", func(target, identity map[string]any) { identity["public_key"] = nil }, []string{"target_key_unobserved"}, false},
		{"status-failed", func(target, identity map[string]any) {
			identity["public_key"] = nil
			target["status_state"] = "query_failed"
			target["reason"] = "status_unavailable"
		}, []string{"target_key_unobserved"}, false},
		{"interface-not-in-status", func(target, identity map[string]any) {
			identity["public_key"] = nil
			target["reason"] = "target_not_in_status"
		}, []string{"target_key_unobserved"}, false},
		{"key-mismatch", func(target, identity map[string]any) {
			key := make([]byte, 32)
			key[0] = 2
			identity["public_key"] = base64.StdEncoding.EncodeToString(key)
		}, []string{"config_target_mismatch"}, false},
		{"target-changed", func(target, identity map[string]any) {
			target["identity"] = nil
			target["state"] = "unknown"
			target["reason"] = "target_changed"
		}, []string{"target_unknown"}, false},
		{"native-failure-cannot-claim-status", func(target, identity map[string]any) {
			target["identity"] = nil
			target["state"] = "unknown"
			target["reason"] = "native_query_failed"
		}, []string{"target_unknown"}, true},
		{"present-never-queried", func(target, identity map[string]any) {
			identity["public_key"] = nil
			target["status_state"] = "not_queried"
			target["reason"] = "status_unavailable"
		}, []string{"target_key_unobserved"}, true},
		{"absent-with-status", func(target, identity map[string]any) { target["identity"] = nil; target["state"] = "absent" }, []string{"target_absent"}, true},
		{"conflict-with-wrong-reason", func(target, identity map[string]any) {
			target["identity"] = nil
			target["state"] = "conflict"
			target["reason"] = "native_query_failed"
			target["status_state"] = "not_queried"
		}, []string{"target_conflict"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := networkJSONMap(t, valid)
			target := p["target"].(map[string]any)
			tc.edit(target, target["identity"].(map[string]any))
			p["issues"] = tc.issues
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_PATCH", p)
			plan, err := remote.PlanWireGuardTunnel(context.Background(), client, input)
			if tc.rejected {
				if !errors.Is(err, fault.CLIProtocol) {
					t.Fatalf("inconsistent target accepted: %v", err)
				}
			} else if err != nil || len(plan.Issues) != len(tc.issues) {
				t.Fatalf("valid uncertainty discarded: %+v, %v", plan.Target, err)
			}
		})
	}
	if runtime.GOOS == "darwin" {
		for _, name := range []string{"utun01", "utun4294967296", "utun+1"} {
			t.Run(name, func(t *testing.T) {
				p := networkJSONMap(t, valid)
				p["target"].(map[string]any)["identity"].(map[string]any)["name"] = name
				patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_PATCH", p)
				if _, err := remote.PlanWireGuardTunnel(context.Background(), client, input); !errors.Is(err, fault.CLIProtocol) {
					t.Fatalf("noncanonical interface accepted: %v", err)
				}
			})
		}
	}
}
