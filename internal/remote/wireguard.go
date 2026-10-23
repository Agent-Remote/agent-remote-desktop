package remote

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type WireGuardContext struct {
	ServerURL string `json:"server_url"`
	DeviceID  string `json:"active_device_id"`
}

type WireGuardPlanInput struct {
	ExpectedHome string  `json:"expected_home"`
	Output       *string `json:"output"`
}

type WireGuardOutput struct {
	Path               string   `json:"path"`
	Action             string   `json:"action"`
	AnchorPath         string   `json:"anchor_path"`
	AnchorIdentity     string   `json:"anchor_identity"`
	MissingDirectories []string `json:"missing_directories"`
	TargetIdentity     *string  `json:"target_identity"`
}

type WireGuardConfigPlan struct {
	PlanVersion int              `json:"plan_version"`
	Home        string           `json:"home"`
	Context     WireGuardContext `json:"context"`
	Output      WireGuardOutput  `json:"output"`
}

type WireGuardApplyInput struct {
	Confirmed    bool                `json:"confirmed"`
	ExpectedPlan WireGuardConfigPlan `json:"expected_plan"`
}

type WireGuardConfigReceipt struct {
	OperationID string           `json:"operation_id"`
	Home        string           `json:"home"`
	Context     WireGuardContext `json:"context"`
	OutputPath  string           `json:"output_path"`
	Complete    bool             `json:"complete"`
	KeyState    string           `json:"key_state"`
	Enrollment  string           `json:"enrollment"`
	Publication string           `json:"publication"`
	Code        string           `json:"code"`
	PeerCount   *uint64          `json:"peer_count"`
	PeerID      *string          `json:"peer_id"`
	PeerVersion *uint64          `json:"peer_version"`
}

// PlanWireGuardConfig performs only passive CLI reads. The returned plan is the
// exact scope presented for confirmation; the desktop never opens CLI files.
func PlanWireGuardConfig(ctx context.Context, client cliadapter.Client, input WireGuardPlanInput) (WireGuardConfigPlan, error) {
	if input.Output != nil {
		output := *input.Output
		input.Output = &output
	}
	if !validWireGuardPath(input.ExpectedHome) || input.ExpectedHome != client.Home {
		return WireGuardConfigPlan{}, fault.CLIContextChanged
	}
	if input.Output != nil && !validWireGuardPath(*input.Output) {
		return WireGuardConfigPlan{}, fault.WireGuardOutputPath
	}
	if err := checkWireGuardCapabilities(ctx, client); err != nil {
		return WireGuardConfigPlan{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.config.plan", input, &raw); err != nil {
		return WireGuardConfigPlan{}, wireGuardPreflightError(err)
	}
	plan, err := decodeWireGuardPlan(raw)
	if err != nil || plan.Home != input.ExpectedHome {
		return WireGuardConfigPlan{}, fault.CLIProtocol
	}
	return plan, nil
}

// ApplyWireGuardConfig submits one confirmed plan. Transport loss is uncertain
// because registration or publication may already have happened; never replay it.
func ApplyWireGuardConfig(ctx context.Context, client cliadapter.Client, input WireGuardApplyInput) (WireGuardConfigReceipt, error) {
	// Freeze caller-owned references before awaiting capabilities. Confirmation
	// applies to this snapshot even if the originating form is later edited.
	if input.ExpectedPlan.Output.MissingDirectories != nil {
		input.ExpectedPlan.Output.MissingDirectories = append([]string{}, input.ExpectedPlan.Output.MissingDirectories...)
	}
	if input.ExpectedPlan.Output.TargetIdentity != nil {
		identity := *input.ExpectedPlan.Output.TargetIdentity
		input.ExpectedPlan.Output.TargetIdentity = &identity
	}
	if !input.Confirmed || !validWireGuardPlan(input.ExpectedPlan) {
		return WireGuardConfigReceipt{}, fault.WireGuardReview
	}
	if input.ExpectedPlan.Home != client.Home {
		return WireGuardConfigReceipt{}, fault.CLIContextChanged
	}
	if err := checkWireGuardCapabilities(ctx, client); err != nil {
		return WireGuardConfigReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.config.apply", input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			switch operation.Code {
			case "invalid_params", "confirmation_required", "configuration_changed", "configuration_unavailable", "authentication_required", "output_invalid", "output_unavailable", "output_changed", "operation_busy", "unsupported_method", "unsupported_version":
				return WireGuardConfigReceipt{}, wireGuardPreflightError(err)
			}
		}
		return WireGuardConfigReceipt{}, fault.WireGuardOutcomeUnknown
	}
	receipt, err := decodeWireGuardReceipt(raw)
	if err != nil || receipt.Home != input.ExpectedPlan.Home || receipt.Context != input.ExpectedPlan.Context || receipt.OutputPath != input.ExpectedPlan.Output.Path {
		return WireGuardConfigReceipt{}, fault.WireGuardOutcomeUnknown
	}
	return receipt, nil
}

func checkWireGuardCapabilities(ctx context.Context, client cliadapter.Client) error {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	plan, apply := false, false
	for _, operation := range caps.Operations {
		if operation.Method == "wireguard.config.plan" {
			plan = operation.Effect == "read" && operation.Credential == "none"
		}
		if operation.Method == "wireguard.config.apply" {
			apply = operation.Effect == "write" && operation.Credential == "device" && operation.Confirmation == "exact_request"
		}
	}
	if !plan || !apply {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return nil
}

func wireGuardPreflightError(err error) error {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) && (operation.Code == "invalid_params" || operation.Code == "confirmation_required") {
		return fault.WireGuardReview
	}
	return err
}
