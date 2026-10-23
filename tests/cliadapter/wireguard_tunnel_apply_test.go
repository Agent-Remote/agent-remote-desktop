package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardTunnelApplyUsesTheFrozenPlanAndReturnsObservedReceipt(t *testing.T) {
	client, log := wireGuardClient(t)
	plan, err := remote.PlanWireGuardTunnel(context.Background(), client, remote.WireGuardTunnelPlanInput{
		ExpectedHome: client.Home,
		Action:       "up",
	})
	if err != nil {
		t.Fatal(err)
	}
	receipt, err := remote.ApplyWireGuardTunnel(context.Background(), client, plan)
	if len(plan.Issues) != 0 {
		if !errors.Is(err, fault.WireGuardPreconditionFailed) {
			t.Fatalf("precondition was not enforced: %v", err)
		}
		return
	}
	if err != nil || receipt.State != "observed" || receipt.TargetState != "present" {
		t.Fatalf("apply did not return observed receipt: %v %+v", err, receipt)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "capabilities\nwireguard.tunnel.plan\ncapabilities\nwireguard.tunnel.apply\n" {
		t.Fatalf("unexpected operation replay or substitution: %q", calls)
	}
}

func TestWireGuardTunnelApplyRejectsMalformedReviewedPlansBeforeCallingCLI(t *testing.T) {
	client, log := wireGuardClient(t)
	plan, err := remote.PlanWireGuardTunnel(context.Background(), client, remote.WireGuardTunnelPlanInput{
		ExpectedHome: client.Home,
		Action:       "up",
	})
	if err != nil {
		t.Fatal(err)
	}
	plan.Issues = []string{"unknown_issue", "unknown_issue"}
	if _, err := remote.ApplyWireGuardTunnel(context.Background(), client, plan); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("malformed plan was submitted: %v", err)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "capabilities\nwireguard.tunnel.plan\n" {
		t.Fatalf("malformed apply invoked CLI: %q", calls)
	}
}

func TestWireGuardTunnelApplyMapsLostMutationOutcomeToUnknown(t *testing.T) {
	client, log := wireGuardClient(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "wg-tunnel-apply-unknown")
	plan, err := remote.PlanWireGuardTunnel(context.Background(), client, remote.WireGuardTunnelPlanInput{
		ExpectedHome: client.Home,
		Action:       "down",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Issues) != 0 {
		t.Skipf("fixture cannot produce an executable down plan on %s: %v", runtime.GOOS, plan.Issues)
	}
	if _, err := remote.ApplyWireGuardTunnel(context.Background(), client, plan); !errors.Is(err, fault.WireGuardOutcomeUnknown) {
		t.Fatalf("lost tunnel mutation was not fail-closed: %v", err)
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.tunnel.plan\ncapabilities\nwireguard.tunnel.apply\n" {
		t.Fatalf("unexpected operation replay or substitution: %q", calls)
	}
}
