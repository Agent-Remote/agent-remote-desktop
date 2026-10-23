package remote_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func egoBrowserTestContext(home string) cliadapter.ConfigContext {
	server := "https://test.invalid"
	device := "device-1"
	return cliadapter.ConfigContext{Home: home, ServerURL: &server, DeviceID: &device}
}

func TestEgoBrowserStatusProjectsLocalAndRemoteState(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	status, err := remote.LoadEgoBrowserStatus(context.Background(), client)
	if err != nil {
		t.Fatalf("status error = %v", err)
	}
	if status.Context.Home != home || status.Local.Installation != "installed" || !status.Policy.Enabled || len(status.Devices) != 1 || len(status.Bindings) != 1 {
		t.Fatalf("unexpected status: %+v", status)
	}
	if status.Devices[0].ID == "" || status.Bindings[0].ID == "" {
		t.Fatal("status omitted stable identifiers")
	}
}

func TestEgoBrowserStatusRequiresExactUserReadContract(t *testing.T) {
	caps := cliadapter.Capabilities{Version: cliadapter.ProtocolVersion, CLIVersion: "test", Platform: "linux", Operations: []cliadapter.Capability{{Method: "ego_browser.status", Effect: "read", Credential: "device"}}}
	if remote.EgoBrowserStatusSupported(caps) {
		t.Fatal("device credential must not satisfy user status contract")
	}
	client := cliadapter.Client{Binary: "/does/not/exist", Home: t.TempDir()}
	_, err := remote.LoadEgoBrowserStatus(context.Background(), client)
	if !errors.Is(err, fault.CLIUnavailable) && !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unexpected preflight error: %v", err)
	}
}

func TestEgoBrowserStatusRejectsUnknownFixtureFields(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "ego-browser-unknown-field")
	_, err := remote.LoadEgoBrowserStatus(context.Background(), client)
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unknown fields must fail closed, got %v", err)
	}
}

func TestEgoBrowserRequestsProjectsBoundedMetadata(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	requests, err := remote.LoadEgoBrowserRequests(context.Background(), client, remote.EgoBrowserRequestListInput{Expected: egoBrowserTestContext(home), BindingID: "binding-1"})
	if err != nil {
		t.Fatalf("requests error = %v", err)
	}
	if requests.BindingID != "binding-1" || requests.BindingGeneration != 2 || len(requests.Items) != 1 || requests.Items[0].PayloadBytes != 42 {
		t.Fatalf("unexpected request projection: %+v", requests)
	}
}

func TestEgoBrowserRequestsRejectUnknownFields(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "ego-browser-request-unknown-field")
	_, err := remote.LoadEgoBrowserRequests(context.Background(), client, remote.EgoBrowserRequestListInput{Expected: egoBrowserTestContext(home), BindingID: "binding-1"})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unknown fields must fail closed, got %v", err)
	}
}

func TestEgoBrowserRequestsRejectIdentifiersOutsideRustRequestContract(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	if _, err := remote.LoadEgoBrowserRequests(context.Background(), client, remote.EgoBrowserRequestListInput{Expected: egoBrowserTestContext(home), BindingID: "binding.id"}); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("binding identifier with dot should fail closed before CLI, got %v", err)
	}
	if _, err := remote.CancelEgoBrowserRequest(context.Background(), client, remote.EgoBrowserRequestCancelInput{Confirmed: true, Expected: egoBrowserTestContext(home), BindingID: "binding-1", RequestID: "request.id", BindingGeneration: 2, Generation: 2, Sequence: 5}); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("request identifier with dot should fail closed before CLI, got %v", err)
	}
	if _, err := remote.LoadEgoBrowserRequests(context.Background(), client, remote.EgoBrowserRequestListInput{Expected: egoBrowserTestContext(home), BindingID: strings.Repeat("b", 129)}); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("overlong binding identifier should fail closed before CLI, got %v", err)
	}
}

func TestEgoBrowserRequestCancelFreezesGeneration(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	receipt, err := remote.CancelEgoBrowserRequest(context.Background(), client, remote.EgoBrowserRequestCancelInput{
		Confirmed: true, Expected: egoBrowserTestContext(home), BindingID: "binding-1", RequestID: "request-ledger-1",
		BindingGeneration: 2, Generation: 2, Sequence: 5,
	})
	if err != nil {
		t.Fatalf("cancel error = %v", err)
	}
	if receipt.Status != "cancel_requested" || receipt.Item.ID != "request-ledger-1" || receipt.Item.Sequence != 5 {
		t.Fatalf("unexpected cancel receipt: %+v", receipt)
	}
}

func TestEgoBrowserBindingPlanAndControlFreezeGeneration(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	input := remote.EgoBrowserBindingPlanInput{Expected: egoBrowserTestContext(home), BindingID: "binding-1", BindingGeneration: 2, Generation: 2, Action: "pause"}
	plan, err := remote.LoadEgoBrowserBindingPlan(context.Background(), client, input)
	if err != nil {
		t.Fatalf("binding plan error = %v", err)
	}
	if !plan.Allowed || plan.Binding.ID != input.BindingID || plan.BindingGeneration != 2 || plan.Generation != 2 {
		t.Fatalf("unexpected binding plan: %+v", plan)
	}
	receipt, err := remote.ControlEgoBrowserBinding(context.Background(), client, remote.EgoBrowserBindingControlInput{
		Confirmed: true, Expected: input.Expected, BindingID: input.BindingID, BindingGeneration: input.BindingGeneration, Generation: input.Generation, Action: input.Action,
	})
	if err != nil {
		t.Fatalf("binding control error = %v", err)
	}
	if receipt.OperationID == "" || receipt.Binding.Status != "paused" || receipt.Action != "pause" || receipt.Generation != 3 || receipt.BindingGeneration != 3 {
		t.Fatalf("unexpected binding receipt: %+v", receipt)
	}
}

func TestEgoBrowserBindingPlanRejectsInvalidActionBeforeCLI(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	_, err := remote.LoadEgoBrowserBindingPlan(context.Background(), client, remote.EgoBrowserBindingPlanInput{Expected: egoBrowserTestContext(home), BindingID: "binding-1", BindingGeneration: 2, Generation: 2, Action: "delete"})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("invalid action should fail closed, got %v", err)
	}
}

func TestEgoBrowserDeletePlanAndControlFreezeTargetGenerations(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	devicePlan, err := remote.LoadEgoBrowserDeletePlan(context.Background(), client, remote.EgoBrowserDeletePlanInput{
		Expected: egoBrowserTestContext(home), Target: "device", ID: "browser-device-1", Generation: 3,
	})
	if err != nil {
		t.Fatalf("device plan error = %v", err)
	}
	if !devicePlan.Allowed || devicePlan.Target != "device" || devicePlan.BindingGeneration != 0 {
		t.Fatalf("unexpected device plan: %+v", devicePlan)
	}
	bindingPlan, err := remote.LoadEgoBrowserDeletePlan(context.Background(), client, remote.EgoBrowserDeletePlanInput{
		Expected: egoBrowserTestContext(home), Target: "binding", ID: "binding-1", Generation: 2, BindingGeneration: 2,
	})
	if err != nil {
		t.Fatalf("binding plan error = %v", err)
	}
	if !bindingPlan.Allowed || bindingPlan.Target != "binding" || bindingPlan.BindingGeneration != 2 {
		t.Fatalf("unexpected binding plan: %+v", bindingPlan)
	}
	receipt, err := remote.ControlEgoBrowserDelete(context.Background(), client, remote.EgoBrowserDeleteControlInput{
		Confirmed: true, Expected: egoBrowserTestContext(home), Target: "binding", ID: "binding-1", Generation: 2, BindingGeneration: 2,
	})
	if err != nil {
		t.Fatalf("delete control error = %v", err)
	}
	if receipt.OperationID == "" || receipt.Status != "deleted" || receipt.Target != "binding" || receipt.ID != "binding-1" || receipt.LocalCleanupComplete == nil || !*receipt.LocalCleanupComplete {
		t.Fatalf("unexpected delete receipt: %+v", receipt)
	}
}

func TestEgoBrowserDeleteRejectsUnknownReceiptFields(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "ego-browser-delete-unknown-field")
	_, err := remote.LoadEgoBrowserDeletePlan(context.Background(), client, remote.EgoBrowserDeletePlanInput{
		Expected: egoBrowserTestContext(home), Target: "device", ID: "browser-device-1", Generation: 3,
	})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unknown fields must fail closed, got %v", err)
	}
}

func TestEgoBrowserDeleteRejectsMalformedReceiptAsUnknownOutcome(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "ego-browser-delete-malformed-receipt")
	_, err := remote.ControlEgoBrowserDelete(context.Background(), client, remote.EgoBrowserDeleteControlInput{
		Confirmed: true, Expected: egoBrowserTestContext(home), Target: "device", ID: "browser-device-1", Generation: 3,
	})
	if !errors.Is(err, fault.EgoBrowserDeleteOutcomeUnknown) {
		t.Fatalf("malformed receipt must be unknown outcome, got %v", err)
	}
}

func TestEgoBrowserDeleteRejectsTargetGenerationShapeBeforeCLI(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	_, err := remote.LoadEgoBrowserDeletePlan(context.Background(), client, remote.EgoBrowserDeletePlanInput{
		Expected: egoBrowserTestContext(home), Target: "device", ID: "browser-device-1", Generation: 3, BindingGeneration: 2,
	})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("device binding generation must fail closed, got %v", err)
	}
}
