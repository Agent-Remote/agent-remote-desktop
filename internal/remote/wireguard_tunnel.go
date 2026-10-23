package remote

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type WireGuardTunnelPlanInput struct {
	ExpectedHome string  `json:"expected_home"`
	Action       string  `json:"action"`
	Config       *string `json:"config"`
}

type WireGuardTunnelBinding struct {
	FileIdentity  string `json:"file_identity"`
	ContentSHA256 string `json:"content_sha256"`
	PublicKey     string `json:"public_key"`
}

type WireGuardTunnelTool struct {
	Name     string  `json:"name"`
	State    string  `json:"state"`
	Path     *string `json:"path"`
	Identity *string `json:"identity"`
	SHA256   *string `json:"sha256"`
}

type WireGuardTunnelPlan struct {
	Version  int                        `json:"version"`
	Home     string                     `json:"home"`
	Platform string                     `json:"platform"`
	Action   string                     `json:"action"`
	Config   WireGuardConfigInspection  `json:"config"`
	Binding  *WireGuardTunnelBinding    `json:"binding"`
	Tools    []WireGuardTunnelTool      `json:"tools"`
	Target   WireGuardTargetObservation `json:"target"`
	Issues   []string                   `json:"issues"`
}

// WireGuardTunnelReceipt is returned only after the native helper has run and
// a second read-only observation confirms the requested target state.
type WireGuardTunnelReceipt struct {
	OperationID string `json:"operation_id"`
	Home        string `json:"home"`
	Action      string `json:"action"`
	Config      string `json:"config"`
	State       string `json:"state"`
	TargetState string `json:"target_state"`
}

// ApplyWireGuardTunnel submits a previously reviewed plan to the machine
// protocol. The Rust CLI rechecks the complete frozen plan, sends the reviewed
// bytes through the installed platform-native permission broker, and only
// returns after a broker receipt plus a second observation confirms the
// requested target state. There is no sudo, runas, or ordinary helper fallback.
func ApplyWireGuardTunnel(ctx context.Context, client cliadapter.Client, plan WireGuardTunnelPlan) (WireGuardTunnelReceipt, error) {
	if !validWireGuardPath(plan.Home) || plan.Home != client.Home {
		return WireGuardTunnelReceipt{}, fault.CLIContextChanged
	}
	encoded, err := json.Marshal(plan)
	if err != nil {
		return WireGuardTunnelReceipt{}, fault.CLIProtocol
	}
	if _, err := decodeWireGuardTunnelPlan(encoded); err != nil {
		return WireGuardTunnelReceipt{}, err
	}
	if len(plan.Issues) != 0 {
		return WireGuardTunnelReceipt{}, fault.WireGuardPreconditionFailed
	}
	if err := checkWireGuardTunnelCapability(ctx, client); err != nil {
		return WireGuardTunnelReceipt{}, err
	}
	var config *string
	defaultPath := filepath.Join(plan.Home, "wireguard", "agent-remote.conf")
	if plan.Config.Path != defaultPath {
		path := plan.Config.Path
		config = &path
	}
	params := struct {
		Confirmed    bool                `json:"confirmed"`
		ExpectedHome string              `json:"expected_home"`
		Action       string              `json:"action"`
		Config       *string             `json:"config"`
		ExpectedPlan WireGuardTunnelPlan `json:"expected_plan"`
	}{
		Confirmed:    true,
		ExpectedHome: plan.Home,
		Action:       plan.Action,
		Config:       config,
		ExpectedPlan: plan,
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.tunnel.apply", params, &raw); err != nil {
		return WireGuardTunnelReceipt{}, wireGuardTunnelApplyError(err)
	}
	var receipt WireGuardTunnelReceipt
	wantTarget := map[string]string{"up": "present", "down": "absent"}[plan.Action]
	if decodeWireGuardJSON(raw, &receipt) != nil || !validWireGuardIdentity(receipt.OperationID) || receipt.Home != plan.Home || receipt.Action != plan.Action || receipt.Config != plan.Config.Path || receipt.State != "observed" || receipt.TargetState != wantTarget {
		return WireGuardTunnelReceipt{}, fault.WireGuardOutcomeUnknown
	}
	return receipt, nil
}

// wireGuardTunnelApplyError keeps deterministic validation failures visible
// while converting every transport/helper failure after submission into an
// outcome-unknown result. A lost response cannot prove that the native target
// stayed unchanged, so callers must never treat context cancellation or an
// unfamiliar write error as permission to replay the reviewed plan.
func wireGuardTunnelApplyError(err error) error {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		switch operation.Code {
		case "invalid_params", "confirmation_required":
			return fault.WireGuardReview
		case "wireguard_plan_stale":
			return fault.WireGuardPlanStale
		case "wireguard_precondition_failed":
			return fault.WireGuardPreconditionFailed
		case "execution_bridge_required":
			return fault.WireGuardBridgeRequired
		case "unsupported_method", "unsupported_version", "configuration_unavailable", "authentication_required", "credential_unavailable":
			return err
		default:
			return fault.WireGuardOutcomeUnknown
		}
	}
	return fault.WireGuardOutcomeUnknown
}

func checkWireGuardTunnelCapability(ctx context.Context, client cliadapter.Client) error {
	var capabilities cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &capabilities); err != nil {
		return err
	}
	if capabilities.Validate() != nil || !capabilities.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	for _, operation := range capabilities.Operations {
		if operation.Method == "wireguard.tunnel.apply" && operation.Effect == "write" && operation.Credential == "none" && operation.Confirmation == "exact_request" {
			return nil
		}
	}
	return &cliadapter.OperationError{Code: "unsupported_method"}
}

// PlanWireGuardTunnel observes one requested action. It neither authorizes nor
// executes that action and never substitutes another configuration or target.
func PlanWireGuardTunnel(ctx context.Context, client cliadapter.Client, input WireGuardTunnelPlanInput) (WireGuardTunnelPlan, error) {
	if input.Config != nil {
		config := *input.Config
		input.Config = &config
	}
	if !validWireGuardPath(input.ExpectedHome) || input.ExpectedHome != client.Home {
		return WireGuardTunnelPlan{}, fault.CLIContextChanged
	}
	if input.Action != "up" && input.Action != "down" {
		return WireGuardTunnelPlan{}, fault.CLIProtocol
	}
	path := filepath.Join(input.ExpectedHome, "wireguard", "agent-remote.conf")
	if input.Config != nil {
		if !validWireGuardPath(*input.Config) {
			return WireGuardTunnelPlan{}, fault.WireGuardOutputPath
		}
		path = *input.Config
	}
	if err := checkWireGuardReadCapability(ctx, client, "wireguard.tunnel.plan"); err != nil {
		return WireGuardTunnelPlan{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.tunnel.plan", input, &raw); err != nil {
		return WireGuardTunnelPlan{}, err
	}
	plan, err := decodeWireGuardTunnelPlan(raw)
	if err != nil || plan.Home != input.ExpectedHome || plan.Action != input.Action || plan.Config.Path != path {
		return WireGuardTunnelPlan{}, fault.CLIProtocol
	}
	return plan, nil
}
