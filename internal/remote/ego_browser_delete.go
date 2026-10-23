package remote

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	EgoBrowserDeletePlanMethod    = "ego_browser.delete.plan"
	EgoBrowserDeleteControlMethod = "ego_browser.delete.control"
)

// EgoBrowserDeletePlanInput is the exact target snapshot selected from the
// status page. Device deletion uses binding_generation=0; binding deletion
// requires both generations.
type EgoBrowserDeletePlanInput struct {
	Expected          cliadapter.ConfigContext `json:"expected_context"`
	Target            string                   `json:"target"`
	ID                string                   `json:"id"`
	Generation        uint64                   `json:"generation"`
	BindingGeneration uint64                   `json:"binding_generation"`
}

type EgoBrowserDeletePlan struct {
	SchemaVersion        int                      `json:"schema_version"`
	Operation            string                   `json:"operation"`
	Context              cliadapter.ConfigContext `json:"context"`
	Target               string                   `json:"target"`
	ID                   string                   `json:"id"`
	Status               string                   `json:"status"`
	Allowed              bool                     `json:"allowed"`
	RequiresConfirmation bool                     `json:"requires_confirmation"`
	Generation           uint64                   `json:"generation"`
	BindingGeneration    uint64                   `json:"binding_generation"`
}

type EgoBrowserDeleteControlInput struct {
	Confirmed         bool                     `json:"confirmed"`
	Expected          cliadapter.ConfigContext `json:"expected_context"`
	Target            string                   `json:"target"`
	ID                string                   `json:"id"`
	Generation        uint64                   `json:"generation"`
	BindingGeneration uint64                   `json:"binding_generation"`
	Reason            string                   `json:"reason,omitempty"`
}

type EgoBrowserDeleteReceipt struct {
	OperationID          string                   `json:"operation_id"`
	SchemaVersion        int                      `json:"schema_version"`
	Operation            string                   `json:"operation"`
	Context              cliadapter.ConfigContext `json:"context"`
	Target               string                   `json:"target"`
	ID                   string                   `json:"id"`
	Status               string                   `json:"status"`
	Generation           uint64                   `json:"generation"`
	BindingGeneration    uint64                   `json:"binding_generation"`
	LocalCleanupComplete *bool                    `json:"local_cleanup_complete"`
}

func EgoBrowserDeletePlanSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserDeletePlanMethod, "read", "user", "")
}

func EgoBrowserDeleteControlSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserDeleteControlMethod, "write", "user", "exact_request")
}

func LoadEgoBrowserDeletePlan(ctx context.Context, client cliadapter.Client, input EgoBrowserDeletePlanInput) (EgoBrowserDeletePlan, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !validEgoDeleteInput(input.Expected, client.Home, input.Target, input.ID, input.Generation, input.BindingGeneration) {
		return EgoBrowserDeletePlan{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserDeletePlan{}, err
	}
	if !EgoBrowserDeletePlanSupported(caps) {
		return EgoBrowserDeletePlan{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, EgoBrowserDeletePlanMethod, input, &raw); err != nil {
		return EgoBrowserDeletePlan{}, err
	}
	var result EgoBrowserDeletePlan
	if decodeStrict(raw, &result) != nil || result.SchemaVersion != 1 || result.Operation != EgoBrowserDeletePlanMethod || !sameEgoContext(result.Context, input.Expected) || result.Target != input.Target || result.ID != input.ID || result.Generation != input.Generation || result.BindingGeneration != input.BindingGeneration || result.Status == "" || !result.RequiresConfirmation {
		return EgoBrowserDeletePlan{}, fault.CLIProtocol
	}
	return result, nil
}

func ControlEgoBrowserDelete(ctx context.Context, client cliadapter.Client, input EgoBrowserDeleteControlInput) (EgoBrowserDeleteReceipt, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !input.Confirmed || !validEgoDeleteInput(input.Expected, client.Home, input.Target, input.ID, input.Generation, input.BindingGeneration) {
		return EgoBrowserDeleteReceipt{}, fault.CLIProtocol
	}
	if input.Reason == "" {
		input.Reason = "desktop_user_request"
	}
	if len(input.Reason) > 128 || containsControl(input.Reason) {
		return EgoBrowserDeleteReceipt{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserDeleteReceipt{}, err
	}
	if !EgoBrowserDeleteControlSupported(caps) {
		return EgoBrowserDeleteReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var receipt EgoBrowserDeleteReceipt
	if err := client.Call(ctx, EgoBrowserDeleteControlMethod, input, &receipt); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return EgoBrowserDeleteReceipt{}, err
		}
		return EgoBrowserDeleteReceipt{}, fault.EgoBrowserDeleteOutcomeUnknown
	}
	if receipt.OperationID == "" || receipt.SchemaVersion != 1 || receipt.Operation != EgoBrowserDeleteControlMethod || !sameEgoContext(receipt.Context, input.Expected) || receipt.Target != input.Target || receipt.ID != input.ID || receipt.Status != "deleted" || receipt.Generation != input.Generation || receipt.BindingGeneration != input.BindingGeneration || receipt.LocalCleanupComplete == nil {
		return EgoBrowserDeleteReceipt{}, fault.EgoBrowserDeleteOutcomeUnknown
	}
	return receipt, nil
}

func validEgoDeleteInput(expected cliadapter.ConfigContext, home, target, id string, generation, bindingGeneration uint64) bool {
	if !validEgoContext(expected, home) || !validIdentifier(id) || generation == 0 {
		return false
	}
	return (target == "device" && bindingGeneration == 0) || (target == "binding" && bindingGeneration > 0)
}

func containsControl(value string) bool {
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}
