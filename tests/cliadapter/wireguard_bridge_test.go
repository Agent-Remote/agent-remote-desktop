package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardBridgeStatusIsExplicitReadOnlyAndUnsupported(t *testing.T) {
	client, log := wireGuardClient(t)
	status, err := remote.InspectWireGuardBridge(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if status.Version != 1 || status.Home != client.Home || status.State != "unsupported" || status.Installation != "not_installed" || status.Approval != "not_requested" || status.Protocol != "unsupported" || status.Authentication != "unavailable" || status.Execution != "unavailable" {
		t.Fatalf("unexpected bridge status: %+v", status)
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.bridge.status\n" {
		t.Fatalf("bridge query retried or substituted another operation: %q", calls)
	}
}

func TestWireGuardBridgeStatusRejectsMisleadingOrUnknownFacts(t *testing.T) {
	client, _ := wireGuardClient(t)
	_, err := remote.InspectWireGuardBridge(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		set  map[string]any
	}{
		{"state", map[string]any{"state": "ready"}},
		{"platform", map[string]any{"platform": "windows"}},
		{"execution", map[string]any{"execution": "ready"}},
		{"unknown-field", map[string]any{"secret": "no"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			patchWireGuard(t, "DESKTOP_TEST_WIREGUARD_BRIDGE_STATUS_PATCH", tc.set)
			if _, err := remote.InspectWireGuardBridge(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("misleading bridge observation accepted: %v", err)
			}
		})
	}
}

func TestWireGuardBridgeLifecycleStopsAtFirstUnmetPlatformGate(t *testing.T) {
	base := remote.WireGuardBridgeStatus{
		Version: 1, Home: "/tmp/agent-remote", Platform: "linux", State: "observed",
		Installation: "installed", Approval: "approved", Protocol: "compatible",
		Authentication: "authenticated", Execution: "ready",
	}
	tests := []struct {
		name       string
		mutate     func(*remote.WireGuardBridgeStatus)
		want       remote.WireGuardBridgeLifecycle
		canExecute bool
	}{
		{"unavailable", func(status *remote.WireGuardBridgeStatus) { status.State = "unavailable" }, remote.WireGuardBridgeUnavailable, false},
		{"unsupported", func(status *remote.WireGuardBridgeStatus) { status.State = "unsupported" }, remote.WireGuardBridgeUnsupported, false},
		{"unknown-state", func(status *remote.WireGuardBridgeStatus) { status.State = "future_state" }, remote.WireGuardBridgeNotReady, false},
		{"installation", func(status *remote.WireGuardBridgeStatus) { status.Installation = "not_installed" }, remote.WireGuardBridgeInstallationRequired, false},
		{"approval-pending", func(status *remote.WireGuardBridgeStatus) { status.Approval = "pending" }, remote.WireGuardBridgeApprovalPending, false},
		{"approval", func(status *remote.WireGuardBridgeStatus) { status.Approval = "not_requested" }, remote.WireGuardBridgeApprovalRequired, false},
		{"protocol", func(status *remote.WireGuardBridgeStatus) { status.Protocol = "incompatible" }, remote.WireGuardBridgeProtocolUnavailable, false},
		{"authentication", func(status *remote.WireGuardBridgeStatus) { status.Authentication = "unauthenticated" }, remote.WireGuardBridgeAuthenticationRequired, false},
		{"authorization", func(status *remote.WireGuardBridgeStatus) { status.Execution = "not_authorized" }, remote.WireGuardBridgeAuthorizationRequired, false},
		{"not-ready", func(status *remote.WireGuardBridgeStatus) { status.Execution = "not_ready" }, remote.WireGuardBridgeNotReady, false},
		{"ready", func(status *remote.WireGuardBridgeStatus) {}, remote.WireGuardBridgeReady, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status := base
			test.mutate(&status)
			if got := status.Lifecycle(); got != test.want {
				t.Fatalf("lifecycle = %q, want %q", got, test.want)
			}
			if got := status.CanExecute(); got != test.canExecute {
				t.Fatalf("can execute = %v, want %v", got, test.canExecute)
			}
		})
	}
}

func TestWireGuardBridgeCanApplyOnlyReviewedTunnelDirectionsWhenReady(t *testing.T) {
	status := remote.WireGuardBridgeStatus{
		Version: 1, Home: "/tmp/agent-remote", Platform: "linux", State: "observed",
		Installation: "installed", Approval: "approved", Protocol: "compatible",
		Authentication: "authenticated", Execution: "ready",
	}
	for _, action := range []string{"up", "down"} {
		if !status.CanApply(action) {
			t.Fatalf("ready bridge rejected %q", action)
		}
	}
	for _, action := range []string{"restart", "install", "", "future"} {
		if status.CanApply(action) {
			t.Fatalf("unknown bridge action was authorized: %q", action)
		}
	}
	status.Execution = "not_authorized"
	if status.CanApply("up") {
		t.Fatal("unauthorized bridge allowed tunnel apply")
	}
}
