package remote

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func ValidateAccountImport(ctx context.Context, client cliadapter.Client, input AccountImportValidateInput) (AccountImportValidation, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	input.ExpectedInclude = slices.Clone(input.ExpectedInclude)
	if !input.Confirmed {
		return AccountImportValidation{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if !validAccountContext(input.Expected, client.Home) || !validDefaultTool(input.AccountID) || !validAccountImportSelection(input.Selection) || !validWireGuardPath(input.ExpectedSourceHome) || !validAccountImportPaths(input.ExpectedInclude) {
		return AccountImportValidation{}, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.validate"); err != nil {
		return AccountImportValidation{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.validate", input, &raw); err != nil {
		return AccountImportValidation{}, accountImportValidationError(err)
	}
	value, err := decodeAccountImportValidation(raw)
	if err != nil || !sameDefaultContext(value.Context, input.Expected) || value.AccountID != input.AccountID || value.Selection != input.Selection || value.SourceHome != input.ExpectedSourceHome || !slices.Equal(value.Include, input.ExpectedInclude) {
		return AccountImportValidation{}, fault.CLIProtocol
	}
	return value, nil
}
func ApplyAccountImport(ctx context.Context, client cliadapter.Client, input AccountImportApplyInput) (AccountImportReceipt, error) {
	if !input.Confirmed {
		return AccountImportReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	planBytes, err := json.Marshal(input.ExpectedPlan)
	if err != nil {
		return AccountImportReceipt{}, fault.AccountInputInvalid
	}
	plan, err := decodeAccountImportPlan(planBytes)
	if err != nil || !validAccountContext(plan.Context, client.Home) || len(plan.Manifest) == 0 || !validAccountImportRequestID(input.RequestID) {
		return AccountImportReceipt{}, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.apply"); err != nil {
		return AccountImportReceipt{}, err
	}
	params := struct {
		Confirmed  bool                     `json:"confirmed"`
		Expected   cliadapter.ConfigContext `json:"expected_context"`
		AccountID  string                   `json:"account_id"`
		Selection  AccountImportSelection   `json:"selection"`
		SourceHome string                   `json:"expected_source_home"`
		Digest     string                   `json:"expected_digest"`
		RequestID  string                   `json:"request_id"`
	}{true, plan.Context, plan.AccountID, plan.Selection, plan.SourceHome, plan.Digest, input.RequestID}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.apply", params, &raw); err != nil {
		return AccountImportReceipt{}, accountImportMutationError(err)
	}
	value, err := decodeAccountImportReceipt(raw, true)
	if err != nil || !sameDefaultContext(value.Context, plan.Context) || value.AccountID != plan.AccountID || value.RequestID != input.RequestID || value.Digest != plan.Digest || value.IntentFingerprint != plan.IntentFingerprint || value.FileCount != uint64(len(plan.Manifest)) {
		return AccountImportReceipt{}, fault.AccountOutcomeUnknown
	}
	return value, nil
}
func accountImportMutationError(err error) error {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		return err
	}
	return fault.AccountOutcomeUnknown
}
func ListAccountImportPending(ctx context.Context, client cliadapter.Client) ([]AccountImportReference, error) {
	if !validWireGuardPath(client.Home) {
		return nil, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.pending"); err != nil {
		return nil, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.pending", struct {
		Home string `json:"expected_home"`
	}{client.Home}, &raw); err != nil {
		return nil, err
	}
	return decodeAccountImportPending(raw, client.Home)
}
func validAccountImportStatusInput(input AccountImportStatusInput, home string) bool {
	return validAccountContext(input.Expected, home) && validDefaultTool(input.AccountID) && validAccountImportRequestID(input.RequestID) && validSyncFingerprint(input.ExpectedDigest) && validAccountImportTask(input.ExpectedTaskID, input.AccountID)
}
func GetAccountImportStatus(ctx context.Context, client cliadapter.Client, input AccountImportStatusInput) (AccountImportReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	input.ExpectedTaskID = copyDefaultText(input.ExpectedTaskID)
	if !validAccountImportStatusInput(input, client.Home) {
		return AccountImportReceipt{}, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.status"); err != nil {
		return AccountImportReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.status", input, &raw); err != nil {
		return AccountImportReceipt{}, err
	}
	value, err := decodeAccountImportReceipt(raw, false)
	if err != nil || !sameDefaultContext(value.Context, input.Expected) || value.AccountID != input.AccountID || value.RequestID != input.RequestID || value.Digest != input.ExpectedDigest || input.ExpectedTaskID != nil && !sameText(value.TaskID, input.ExpectedTaskID) {
		return AccountImportReceipt{}, fault.CLIProtocol
	}
	return value, nil
}
func ForgetAccountImport(ctx context.Context, client cliadapter.Client, input AccountImportForgetInput) (AccountImportForgetReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	input.ExpectedTaskID = copyDefaultText(input.ExpectedTaskID)
	if !input.Confirmed {
		return AccountImportForgetReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if !validAccountImportStatusInput(AccountImportStatusInput{Expected: input.Expected, AccountID: input.AccountID, RequestID: input.RequestID, ExpectedDigest: input.ExpectedDigest, ExpectedTaskID: input.ExpectedTaskID}, client.Home) {
		return AccountImportForgetReceipt{}, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.forget"); err != nil {
		return AccountImportForgetReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.forget", input, &raw); err != nil {
		return AccountImportForgetReceipt{}, accountImportMutationError(err)
	}
	value, err := decodeAccountImportForget(raw)
	if err != nil || !sameDefaultContext(value.Context, input.Expected) || value.AccountID != input.AccountID || value.RequestID != input.RequestID || value.Digest != input.ExpectedDigest || input.ExpectedTaskID != nil && !sameText(value.TaskID, input.ExpectedTaskID) {
		return AccountImportForgetReceipt{}, fault.AccountOutcomeUnknown
	}
	return value, nil
}

func accountImportValidationError(err error) error {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) && operation.Code != "account_import_outcome_unknown" && operation.Code != "canceled" && operation.Code != "timeout" {
		return err
	}
	return &cliadapter.OperationError{Code: "account_import_validation_unknown"}
}
