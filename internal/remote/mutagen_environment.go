package remote

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func MutagenEnvironmentSupported(caps cliadapter.Capabilities, method string) bool {
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return false
	}
	for _, operation := range caps.Operations {
		if operation.Method != method || operation.Credential != "none" {
			continue
		}
		switch method {
		case "mutagen.environment.preview":
			return operation.Effect == "read" && operation.Confirmation == ""
		case "mutagen.environment.apply":
			return operation.Effect == "write" && operation.Confirmation == "exact_request"
		}
	}
	return false
}
func mutagenEnvironmentPreflight(ctx context.Context, client cliadapter.Client, method string) error {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	if !MutagenEnvironmentSupported(caps, method) {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return nil
}
func PreviewMutagenEnvironment(ctx context.Context, client cliadapter.Client) (MutagenEnvironmentPlan, error) {
	if !validWireGuardPath(client.Home) {
		return MutagenEnvironmentPlan{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	if err := mutagenEnvironmentPreflight(ctx, client, "mutagen.environment.preview"); err != nil {
		return MutagenEnvironmentPlan{}, err
	}
	input := struct {
		Home string `json:"expected_home"`
	}{client.Home}
	var raw json.RawMessage
	if err := client.Call(ctx, "mutagen.environment.preview", input, &raw); err != nil {
		return MutagenEnvironmentPlan{}, err
	}
	plan, err := decodeMutagenEnvironmentPlan(raw)
	if err != nil || plan.Home != client.Home {
		return MutagenEnvironmentPlan{}, fault.CLIProtocol
	}
	return plan, nil
}
func ApplyMutagenEnvironment(ctx context.Context, client cliadapter.Client, input MutagenEnvironmentApplyInput) (MutagenEnvironmentReceipt, error) {
	if !input.Confirmed {
		return MutagenEnvironmentReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	encoded, err := json.Marshal(input.ExpectedPlan)
	if err != nil {
		return MutagenEnvironmentReceipt{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	plan, err := decodeMutagenEnvironmentPlan(encoded)
	if err != nil || !plan.Repairable || input.ExpectedHome != client.Home || plan.Home != input.ExpectedHome {
		return MutagenEnvironmentReceipt{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	if err := mutagenEnvironmentPreflight(ctx, client, "mutagen.environment.apply"); err != nil {
		return MutagenEnvironmentReceipt{}, err
	}
	wire := struct {
		Confirmed   bool   `json:"confirmed"`
		Home        string `json:"expected_home"`
		Fingerprint string `json:"expected_fingerprint"`
	}{true, input.ExpectedHome, plan.Fingerprint}
	var raw json.RawMessage
	if err := client.Call(ctx, "mutagen.environment.apply", wire, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return MutagenEnvironmentReceipt{}, err
		}
		return MutagenEnvironmentReceipt{}, &cliadapter.OperationError{Code: "sync_outcome_unknown"}
	}
	receipt, err := decodeMutagenEnvironmentReceipt(raw)
	if err != nil || receipt.Home != input.ExpectedHome || !reflect.DeepEqual(receipt.Plan, plan) {
		return MutagenEnvironmentReceipt{}, &cliadapter.OperationError{Code: "sync_outcome_unknown"}
	}
	return receipt, nil
}
