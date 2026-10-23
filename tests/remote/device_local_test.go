package remote_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestDeviceLocalStatusProjectsMacOSInspection(t *testing.T) {
	home := t.TempDir()
	status, err := remote.LoadDeviceLocalStatus(context.Background(), cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home})
	if err != nil {
		t.Fatalf("status error = %v", err)
	}
	if status.Home != home || status.Platform != "macos" || status.App.Installation != "installed" || status.App.CodeSignature != "valid" || !status.App.Running {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestDeviceLocalStatusRequiresExactReadContract(t *testing.T) {
	caps := cliadapter.Capabilities{Version: cliadapter.ProtocolVersion, CLIVersion: "test", Platform: "linux", Operations: []cliadapter.Capability{{Method: "device.local.status", Effect: "read", Credential: "device"}}}
	if remote.DeviceLocalStatusSupported(caps) {
		t.Fatal("device credential must not satisfy local read contract")
	}
	client := cliadapter.Client{Binary: "/does/not/exist", Home: t.TempDir()}
	_, err := remote.LoadDeviceLocalStatus(context.Background(), client)
	if !errors.Is(err, fault.CLIUnavailable) && !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unexpected preflight error: %v", err)
	}
}

func TestDeviceLocalPlanProjectsLifecycleStages(t *testing.T) {
	home := t.TempDir()
	source := filepath.Join(home, "Agent Remote Device.app")
	plan, err := remote.LoadDeviceLocalPlan(context.Background(), cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}, remote.DeviceLocalPlanInput{
		ExpectedHome: home,
		Action:       "install",
		Source:       &source,
	})
	if err != nil {
		t.Fatalf("plan error = %v", err)
	}
	if plan.Action != "install" || plan.TerminalMethod != "agent-remote device install" || !plan.RequiresConfirmation || !plan.Destructive || len(plan.Stages) != 5 {
		t.Fatalf("unexpected lifecycle plan: %+v", plan)
	}
}

func TestDeviceLocalPlanRejectsUnboundSourceAndMutationCapability(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	_, err := remote.LoadDeviceLocalPlan(context.Background(), client, remote.DeviceLocalPlanInput{ExpectedHome: home, Action: "uninstall", Source: planPtr("/tmp/source.app")})
	if !errors.Is(err, fault.CLIContextChanged) {
		t.Fatalf("expected local plan input rejection, got %v", err)
	}
	caps := cliadapter.Capabilities{Version: cliadapter.ProtocolVersion, CLIVersion: "test", Platform: "macos", Operations: []cliadapter.Capability{{Method: "device.local.plan", Effect: "read", Credential: "device"}}}
	if remote.DeviceLocalPlanSupported(caps) {
		t.Fatal("device credential must not satisfy local lifecycle plan contract")
	}
}

func planPtr(value string) *string { return &value }
