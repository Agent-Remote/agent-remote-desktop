package remote

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const deviceLocalPlanMethod = "device.local.plan"

// DeviceLocalPlanInput identifies one reviewed local Device lifecycle action.
// Install is the only action with a source path; the CLI remains responsible
// for reading the bundle and performing the mutation in its managed PTY.
type DeviceLocalPlanInput struct {
	ExpectedHome string  `json:"expected_home"`
	Action       string  `json:"action"`
	Source       *string `json:"source,omitempty"`
}

type DeviceLocalPlan struct {
	SchemaVersion        int                    `json:"schema_version"`
	Operation            string                 `json:"operation"`
	Home                 string                 `json:"home"`
	Platform             string                 `json:"platform"`
	Action               string                 `json:"action"`
	Source               *string                `json:"source,omitempty"`
	TerminalMethod       string                 `json:"terminal_method"`
	RequiresConfirmation bool                   `json:"requires_confirmation"`
	Destructive          bool                   `json:"destructive"`
	ResumePolicy         string                 `json:"resume_policy"`
	Stages               []DeviceLocalPlanStage `json:"stages"`
}

type DeviceLocalPlanStage struct {
	ID     string `json:"id"`
	Effect string `json:"effect"`
	State  string `json:"state"`
}

// DeviceLocalPlanSupported accepts only the read-only, credential-free plan
// contract. A terminal capability or local status response cannot substitute
// for this exact method declaration.
func DeviceLocalPlanSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, deviceLocalPlanMethod, "read", "none", "")
}

func LoadDeviceLocalPlan(ctx context.Context, client cliadapter.Client, input DeviceLocalPlanInput) (DeviceLocalPlan, error) {
	if !validWireGuardPath(client.Home) || input.ExpectedHome != client.Home || !validDeviceLocalPlanAction(input) {
		return DeviceLocalPlan{}, fault.CLIContextChanged
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return DeviceLocalPlan{}, err
	}
	if !DeviceLocalPlanSupported(caps) {
		return DeviceLocalPlan{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, deviceLocalPlanMethod, input, &raw); err != nil {
		return DeviceLocalPlan{}, err
	}
	var plan DeviceLocalPlan
	if decodeStrict(raw, &plan) != nil || !validDeviceLocalPlan(plan, input) {
		return DeviceLocalPlan{}, fault.CLIProtocol
	}
	return plan, nil
}

func validDeviceLocalPlanAction(input DeviceLocalPlanInput) bool {
	if input.Action != "install" && input.Action != "uninstall" && input.Action != "launch" && input.Action != "diagnose" {
		return false
	}
	if input.Source == nil {
		return input.Action != "install"
	}
	return input.Action == "install" && validWireGuardPath(*input.Source) && len(*input.Source) <= 4096
}

func validDeviceLocalPlan(plan DeviceLocalPlan, input DeviceLocalPlanInput) bool {
	if plan.SchemaVersion != 1 || plan.Operation != deviceLocalPlanMethod || plan.Home != input.ExpectedHome || plan.Platform != "macos" || plan.Action != input.Action || plan.TerminalMethod != "agent-remote device "+input.Action || plan.ResumePolicy != "inspect_before_retry" || len(plan.Stages) == 0 || len(plan.Stages) > 16 {
		return false
	}
	if !sameOptionalText(plan.Source, input.Source) || (plan.Action == "diagnose" && plan.RequiresConfirmation) || (plan.Action == "uninstall" && !plan.Destructive) || (plan.Action == "install" && (!plan.RequiresConfirmation || !plan.Destructive)) || plan.Action == "launch" && !plan.RequiresConfirmation || strings.ContainsAny(plan.TerminalMethod, "\r\n\x00") {
		return false
	}
	seen := make(map[string]struct{}, len(plan.Stages))
	for _, stage := range plan.Stages {
		if !validDeviceLocalPlanText(stage.ID, 64) || (stage.Effect != "read" && stage.Effect != "write") || stage.State != "planned" {
			return false
		}
		if _, ok := seen[stage.ID]; ok {
			return false
		}
		seen[stage.ID] = struct{}{}
	}
	return true
}

func validDeviceLocalPlanText(value string, max int) bool {
	return value != "" && len(value) <= max && !strings.ContainsAny(value, "\r\n\x00")
}

func sameOptionalText(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
