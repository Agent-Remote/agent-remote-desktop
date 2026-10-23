package remote

import (
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func AccountOperationSupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method != method || op.Credential != "device" {
			continue
		}
		switch method {
		case "accounts.status":
			return op.Effect == "read" && op.Confirmation == ""
		case "accounts.create", "accounts.bind", "accounts.verify", "accounts.disable":
			return op.Effect == "write" && op.Confirmation == "exact_request"
		}
	}
	return false
}
func accountPreflight(ctx context.Context, client cliadapter.Client, method string) error {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	if !AccountOperationSupported(caps, method) {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return nil
}
func accountContextSnapshot(c cliadapter.ConfigContext) cliadapter.ConfigContext {
	c.ServerURL = copyDefaultText(c.ServerURL)
	c.DeviceID = copyDefaultText(c.DeviceID)
	return c
}
func validAccountContext(c cliadapter.ConfigContext, home string) bool {
	return validDefaultContext(c) && c.Home == home && c.ServerURL != nil && c.DeviceID != nil
}

// ValidateAccountCreate is shared by the form and the process boundary.
func ValidateAccountCreate(input AccountCreateInput) error {
	if !validDefaultTool(input.Tool) || !validAccountText(input.Name, 256) || !validAccountText(input.Region, 64) || !validAccountText(input.Timezone, 128) || !validAccountText(input.Locale, 128) || !validAccountTags(input.Tags) {
		return fault.AccountInputInvalid
	}
	return nil
}
func GetAccountStatus(ctx context.Context, client cliadapter.Client, input AccountStatusInput) (AccountStatus, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if !validAccountContext(input.Expected, client.Home) || !validDefaultTool(input.AccountID) {
		return AccountStatus{}, fault.AccountInputInvalid
	}
	if err := accountPreflight(ctx, client, "accounts.status"); err != nil {
		return AccountStatus{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.status", input, &raw); err != nil {
		return AccountStatus{}, err
	}
	status, err := decodeAccountStatus(raw)
	if err != nil || !sameDefaultContext(status.Context, input.Expected) || status.Account.ID != input.AccountID || status.Binding.AccountID != input.AccountID {
		return AccountStatus{}, fault.CLIProtocol
	}
	return status, nil
}
func CreateAccount(ctx context.Context, client cliadapter.Client, input AccountCreateInput) (AccountReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	input.Tags = slices.Clone(input.Tags)
	if !input.Confirmed {
		return AccountReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if err := ValidateAccountCreate(input); err != nil {
		return AccountReceipt{}, err
	}
	if !validAccountContext(input.Expected, client.Home) {
		return AccountReceipt{}, fault.AccountInputInvalid
	}
	receipt, err := submitAccount(ctx, client, "accounts.create", input, input.Expected)
	if err != nil {
		return AccountReceipt{}, err
	}
	account := receipt.Account
	if account == nil || receipt.Binding != nil || account.Tool != input.Tool || account.Name != input.Name || account.Region != input.Region || account.Timezone != input.Timezone || account.Locale != input.Locale || !slices.Equal(account.Tags, input.Tags) {
		return AccountReceipt{}, fault.AccountOutcomeUnknown
	}
	return receipt, nil
}
func BindAccount(ctx context.Context, client cliadapter.Client, input AccountActionInput) (AccountReceipt, error) {
	return accountAction(ctx, client, "accounts.bind", input)
}
func VerifyAccount(ctx context.Context, client cliadapter.Client, input AccountActionInput) (AccountReceipt, error) {
	return accountAction(ctx, client, "accounts.verify", input)
}
func DisableAccount(ctx context.Context, client cliadapter.Client, input AccountActionInput) (AccountReceipt, error) {
	return accountAction(ctx, client, "accounts.disable", input)
}
func accountAction(ctx context.Context, client cliadapter.Client, method string, input AccountActionInput) (AccountReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if !input.Confirmed {
		return AccountReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if !validAccountContext(input.Expected, client.Home) || !validDefaultTool(input.AccountID) {
		return AccountReceipt{}, fault.AccountInputInvalid
	}
	receipt, err := submitAccount(ctx, client, method, input, input.Expected)
	if err != nil {
		return AccountReceipt{}, err
	}
	if method == "accounts.disable" {
		if receipt.Binding != nil || receipt.Account == nil || receipt.Account.ID != input.AccountID || receipt.Account.Status != "disabled" {
			return AccountReceipt{}, fault.AccountOutcomeUnknown
		}
	} else if receipt.Account != nil || receipt.Binding == nil || receipt.Binding.AccountID != input.AccountID || receipt.Binding.TaskID == nil {
		return AccountReceipt{}, fault.AccountOutcomeUnknown
	}
	return receipt, nil
}
func submitAccount(ctx context.Context, client cliadapter.Client, method string, input any, expected cliadapter.ConfigContext) (AccountReceipt, error) {
	if err := accountPreflight(ctx, client, method); err != nil {
		return AccountReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, method, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return AccountReceipt{}, err
		}
		return AccountReceipt{}, fault.AccountOutcomeUnknown
	}
	receipt, err := decodeAccountReceipt(raw)
	if err != nil || !sameDefaultContext(receipt.Context, expected) {
		return AccountReceipt{}, fault.AccountOutcomeUnknown
	}
	return receipt, nil
}
