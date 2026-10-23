package remote

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func RuntimeRecoverySupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	credential, effect, confirmation := "user", "read", ""
	switch method {
	case "accounts.runtime_recovery.authority", "accounts.runtime_recovery.inspect", "accounts.runtime_recovery.status":
	case "accounts.runtime_recovery.apply":
		effect, confirmation = "write", "exact_request"
	case "accounts.runtime_recovery.pending":
		credential = "none"
	case "accounts.runtime_recovery.forget":
		credential, effect, confirmation = "none", "write", "exact_request"
	default:
		return false
	}
	for _, op := range c.Operations {
		if op.Method == method {
			return op.Credential == credential && op.Effect == effect && op.Confirmation == confirmation
		}
	}
	return false
}
func runtimePreflight(ctx context.Context, client cliadapter.Client, method string) error {
	var c cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &c); err != nil {
		return err
	}
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	if !RuntimeRecoverySupported(c, method) {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return nil
}
func runtimeInputError() error { return &cliadapter.OperationError{Code: "invalid_params"} }
func runtimeMutationError(err error) error {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		return err
	}
	return &cliadapter.OperationError{Code: "runtime_recovery_outcome_unknown"}
}
func NewRuntimeRecoveryRequestID() (string, error) { return NewAccountImportRequestID() }

func GetRuntimeRecoveryAuthority(ctx context.Context, client cliadapter.Client, input RuntimeRecoveryAuthorityInput) (RuntimeRecoveryAuthority, error) {
	var value RuntimeRecoveryAuthority
	if input.ExpectedHome != client.Home || !validWireGuardPath(client.Home) || !validRuntimeServer(input.ExpectedServerURL) {
		return value, runtimeInputError()
	}
	if err := runtimePreflight(ctx, client, "accounts.runtime_recovery.authority"); err != nil {
		return value, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.runtime_recovery.authority", input, &raw); err != nil {
		return value, err
	}
	fields, err := decodeAccountObject(raw, &value, []string{"context", "admin"}, nil)
	if err != nil {
		return value, err
	}
	if value.Context, err = decodeRuntimeContext(fields["context"]); err != nil {
		return value, err
	}
	if value.Context.Home != input.ExpectedHome || !sameRuntimeServer(value.Context.ServerURL, input.ExpectedServerURL) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func InspectRuntimeRecovery(ctx context.Context, client cliadapter.Client, input RuntimeRecoveryInspectInput) (RuntimeRecoveryPlan, error) {
	if !validRuntimeContext(input.Expected, client.Home) || !validAccountImportRequestID(input.AccountID) || !validRuntimeTask(input.OriginalTaskID, "migrate_tool_account_runtime", input.AccountID) || !validRuntimeAction(input.Action) {
		return RuntimeRecoveryPlan{}, runtimeInputError()
	}
	if err := runtimePreflight(ctx, client, "accounts.runtime_recovery.inspect"); err != nil {
		return RuntimeRecoveryPlan{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.runtime_recovery.inspect", input, &raw); err != nil {
		return RuntimeRecoveryPlan{}, err
	}
	value, err := decodeRuntimePlan(raw)
	if err != nil || value.Context != input.Expected || value.AccountID != input.AccountID || value.Original.TaskID != input.OriginalTaskID || value.Action != input.Action {
		return RuntimeRecoveryPlan{}, fault.CLIProtocol
	}
	return value, nil
}
func ApplyRuntimeRecovery(ctx context.Context, client cliadapter.Client, input RuntimeRecoveryApplyInput) (RuntimeRecoveryReceipt, error) {
	if !input.Confirmed {
		return RuntimeRecoveryReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	planBytes, err := json.Marshal(input.ExpectedPlan)
	if err != nil {
		return RuntimeRecoveryReceipt{}, runtimeInputError()
	}
	plan, err := decodeRuntimePlan(planBytes)
	if err != nil || !validRuntimeContext(plan.Context, client.Home) || !validAccountImportRequestID(input.RequestID) {
		return RuntimeRecoveryReceipt{}, runtimeInputError()
	}
	if err := runtimePreflight(ctx, client, "accounts.runtime_recovery.apply"); err != nil {
		return RuntimeRecoveryReceipt{}, err
	}
	params := struct {
		Confirmed      bool                    `json:"confirmed"`
		Expected       RuntimeRecoveryContext  `json:"expected_context"`
		AccountID      string                  `json:"account_id"`
		Original       RuntimeRecoveryOriginal `json:"original"`
		Action         string                  `json:"action"`
		ExpectedDigest string                  `json:"expected_digest"`
		RequestID      string                  `json:"request_id"`
	}{true, plan.Context, plan.AccountID, plan.Original, plan.Action, plan.Digest, input.RequestID}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.runtime_recovery.apply", params, &raw); err != nil {
		return RuntimeRecoveryReceipt{}, runtimeMutationError(err)
	}
	value, err := decodeRuntimeReceipt(raw, true)
	if err != nil || value.Context != plan.Context || value.AccountID != plan.AccountID || value.RequestID != input.RequestID || value.Digest == nil || *value.Digest != plan.Digest || value.Recovery.Binding.Original() != plan.Original || value.Recovery.Binding.RecoveryAction() != plan.Action {
		return RuntimeRecoveryReceipt{}, runtimeMutationError(fault.CLIProtocol)
	}
	return value, nil
}
func validRuntimeStatusInput(input RuntimeRecoveryStatusInput, home string) bool {
	if !validRuntimeContext(input.Expected, home) || !validAccountImportRequestID(input.AccountID) || !validAccountImportRequestID(input.RequestID) || input.ExpectedDigest != nil && !validSyncFingerprint(*input.ExpectedDigest) {
		return false
	}
	if input.ExpectedBinding != nil {
		raw, err := json.Marshal(input.ExpectedBinding)
		if err != nil {
			return false
		}
		binding, err := decodeRuntimeBinding(raw)
		if err != nil || binding.ToolAccountID != input.AccountID || binding.TaskID != "recover_tool_account_runtime:"+input.AccountID+":"+input.RequestID {
			return false
		}
	}
	return true
}
func GetRuntimeRecoveryStatus(ctx context.Context, client cliadapter.Client, input RuntimeRecoveryStatusInput) (RuntimeRecoveryReceipt, error) {
	if !validRuntimeStatusInput(input, client.Home) {
		return RuntimeRecoveryReceipt{}, runtimeInputError()
	}
	if err := runtimePreflight(ctx, client, "accounts.runtime_recovery.status"); err != nil {
		return RuntimeRecoveryReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.runtime_recovery.status", input, &raw); err != nil {
		return RuntimeRecoveryReceipt{}, err
	}
	value, err := decodeRuntimeReceipt(raw, false)
	if err != nil || value.Context != input.Expected || value.AccountID != input.AccountID || value.RequestID != input.RequestID || !sameText(value.Digest, input.ExpectedDigest) || input.ExpectedBinding != nil && !reflect.DeepEqual(*input.ExpectedBinding, value.Recovery.Binding) {
		return RuntimeRecoveryReceipt{}, fault.CLIProtocol
	}
	return value, nil
}
func ListRuntimeRecoveryPending(ctx context.Context, client cliadapter.Client) ([]RuntimeRecoveryRecord, error) {
	if !validWireGuardPath(client.Home) {
		return nil, runtimeInputError()
	}
	if err := runtimePreflight(ctx, client, "accounts.runtime_recovery.pending"); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.runtime_recovery.pending", struct {
		Home string `json:"expected_home"`
	}{client.Home}, &raw); err != nil {
		return nil, err
	}
	return decodeRuntimePending(raw, client.Home)
}
func ForgetRuntimeRecovery(ctx context.Context, client cliadapter.Client, input RuntimeRecoveryForgetInput) (RuntimeRecoveryForgetReceipt, error) {
	if !input.Confirmed {
		return RuntimeRecoveryForgetReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if input.ExpectedDigest == nil || !validRuntimeStatusInput(RuntimeRecoveryStatusInput{input.Expected, input.AccountID, input.RequestID, input.ExpectedDigest, input.ExpectedBinding}, client.Home) {
		return RuntimeRecoveryForgetReceipt{}, runtimeInputError()
	}
	if err := runtimePreflight(ctx, client, "accounts.runtime_recovery.forget"); err != nil {
		return RuntimeRecoveryForgetReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.runtime_recovery.forget", input, &raw); err != nil {
		return RuntimeRecoveryForgetReceipt{}, runtimeMutationError(err)
	}
	value, err := decodeRuntimeForget(raw)
	if err != nil || value.Context != input.Expected || value.AccountID != input.AccountID || value.RequestID != input.RequestID || value.Digest != *input.ExpectedDigest {
		return RuntimeRecoveryForgetReceipt{}, runtimeMutationError(fault.CLIProtocol)
	}
	return value, nil
}
