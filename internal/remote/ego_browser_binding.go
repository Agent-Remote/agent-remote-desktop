package remote

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	EgoBrowserBindingPlanMethod    = "ego_browser.binding.plan"
	EgoBrowserBindingControlMethod = "ego_browser.binding.control"
)

type EgoBrowserBindingPlanInput struct {
	Expected          cliadapter.ConfigContext
	BindingID         string
	BindingGeneration uint64
	Generation        uint64
	Action            string
}

type EgoBrowserBindingControlState struct {
	ID                string  `json:"id"`
	DeviceID          string  `json:"device_id"`
	ToolSessionID     string  `json:"tool_session_id"`
	NodeID            string  `json:"node_id"`
	Status            string  `json:"status"`
	LeaseHealth       string  `json:"lease_health"`
	Generation        uint64  `json:"generation"`
	BindingGeneration uint64  `json:"binding_generation"`
	LeaseUntil        *string `json:"lease_until"`
	StopReason        *string `json:"stop_reason"`
}

type EgoBrowserBindingPlan struct {
	SchemaVersion        int                           `json:"schema_version"`
	Operation            string                        `json:"operation"`
	Context              cliadapter.ConfigContext      `json:"context"`
	Binding              EgoBrowserBindingControlState `json:"binding"`
	Action               string                        `json:"action"`
	Allowed              bool                          `json:"allowed"`
	RequiresConfirmation bool                          `json:"requires_confirmation"`
	Generation           uint64                        `json:"generation"`
	BindingGeneration    uint64                        `json:"binding_generation"`
}

type EgoBrowserBindingControlInput struct {
	Confirmed         bool
	Expected          cliadapter.ConfigContext
	BindingID         string
	BindingGeneration uint64
	Generation        uint64
	Action            string
	Reason            string
}

type EgoBrowserBindingControlReceipt struct {
	OperationID       string                        `json:"operation_id"`
	SchemaVersion     int                           `json:"schema_version"`
	Operation         string                        `json:"operation"`
	Context           cliadapter.ConfigContext      `json:"context"`
	Binding           EgoBrowserBindingControlState `json:"binding"`
	Action            string                        `json:"action"`
	Status            string                        `json:"status"`
	Generation        uint64                        `json:"generation"`
	BindingGeneration uint64                        `json:"binding_generation"`
}

func EgoBrowserBindingPlanSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserBindingPlanMethod, "read", "user", "")
}

func EgoBrowserBindingControlSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserBindingControlMethod, "write", "user", "exact_request")
}

func LoadEgoBrowserBindingPlan(ctx context.Context, client cliadapter.Client, input EgoBrowserBindingPlanInput) (EgoBrowserBindingPlan, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !validEgoBindingInput(input.Expected, client.Home, input.BindingID, input.BindingGeneration, input.Generation, input.Action) {
		return EgoBrowserBindingPlan{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserBindingPlan{}, err
	}
	if !EgoBrowserBindingPlanSupported(caps) {
		return EgoBrowserBindingPlan{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	params := struct {
		Expected          cliadapter.ConfigContext `json:"expected_context"`
		BindingID         string                   `json:"binding_id"`
		BindingGeneration uint64                   `json:"binding_generation"`
		Generation        uint64                   `json:"generation"`
		Action            string                   `json:"action"`
	}{input.Expected, input.BindingID, input.BindingGeneration, input.Generation, input.Action}
	if err := client.Call(ctx, EgoBrowserBindingPlanMethod, params, &raw); err != nil {
		return EgoBrowserBindingPlan{}, err
	}
	var result EgoBrowserBindingPlan
	if decodeStrict(raw, &result) != nil || result.SchemaVersion != 1 || result.Operation != EgoBrowserBindingPlanMethod || !sameEgoContext(result.Context, input.Expected) || result.Binding.ID != input.BindingID || result.BindingGeneration != input.BindingGeneration || result.Generation != input.Generation || result.Binding.BindingGeneration != result.BindingGeneration || result.Binding.Generation != result.Generation || result.Action != input.Action || !validEgoBinding(result.Binding) {
		return EgoBrowserBindingPlan{}, fault.CLIProtocol
	}
	return result, nil
}

func ControlEgoBrowserBinding(ctx context.Context, client cliadapter.Client, input EgoBrowserBindingControlInput) (EgoBrowserBindingControlReceipt, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !input.Confirmed || !validEgoBindingInput(input.Expected, client.Home, input.BindingID, input.BindingGeneration, input.Generation, input.Action) {
		return EgoBrowserBindingControlReceipt{}, fault.CLIProtocol
	}
	if input.Reason == "" {
		input.Reason = "desktop_user_request"
	}
	if len(input.Reason) > 128 || strings.ContainsAny(input.Reason, "\r\n") {
		return EgoBrowserBindingControlReceipt{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserBindingControlReceipt{}, err
	}
	if !EgoBrowserBindingControlSupported(caps) {
		return EgoBrowserBindingControlReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := struct {
		Confirmed         bool                     `json:"confirmed"`
		Expected          cliadapter.ConfigContext `json:"expected_context"`
		BindingID         string                   `json:"binding_id"`
		BindingGeneration uint64                   `json:"binding_generation"`
		Generation        uint64                   `json:"generation"`
		Action            string                   `json:"action"`
		Reason            string                   `json:"reason"`
	}{true, input.Expected, input.BindingID, input.BindingGeneration, input.Generation, input.Action, input.Reason}
	var receipt EgoBrowserBindingControlReceipt
	if err := client.Call(ctx, EgoBrowserBindingControlMethod, params, &receipt); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return EgoBrowserBindingControlReceipt{}, err
		}
		return EgoBrowserBindingControlReceipt{}, fault.EgoBrowserBindingOutcomeUnknown
	}
	if receipt.OperationID == "" || receipt.SchemaVersion != 1 || receipt.Operation != EgoBrowserBindingControlMethod || !sameEgoContext(receipt.Context, input.Expected) || receipt.Binding.ID != input.BindingID || receipt.Action != input.Action || receipt.Generation == 0 || receipt.BindingGeneration == 0 || receipt.Binding.BindingGeneration != receipt.BindingGeneration || receipt.Binding.Generation != receipt.Generation || !validEgoBinding(receipt.Binding) {
		return EgoBrowserBindingControlReceipt{}, fault.EgoBrowserBindingOutcomeUnknown
	}
	return receipt, nil
}

func validEgoBindingInput(expected cliadapter.ConfigContext, home, bindingID string, bindingGeneration, generation uint64, action string) bool {
	return validEgoContext(expected, home) && validIdentifier(bindingID) && bindingGeneration > 0 && generation > 0 && (action == "pause" || action == "resume" || action == "stop" || action == "revoke")
}

func validEgoBinding(value EgoBrowserBindingControlState) bool {
	return validIdentifier(value.ID) && validIdentifier(value.DeviceID) && validIdentifier(value.ToolSessionID) && validIdentifier(value.NodeID) && validText(value.Status, 32) && validText(value.LeaseHealth, 32) && value.Generation > 0 && value.BindingGeneration > 0 && optionalText(value.LeaseUntil, 128) && optionalText(value.StopReason, 256)
}
