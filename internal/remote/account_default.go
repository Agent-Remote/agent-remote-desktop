package remote

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// AccountDefault is the CLI-home-wide selection for a tool, shared across servers.
type AccountDefault struct {
	Context   cliadapter.ConfigContext `json:"context"`
	Tool      string                   `json:"tool"`
	AccountID *string                  `json:"account_id"`
}

type AccountDefaultOverview struct {
	Capabilities  cliadapter.Capabilities
	Default       AccountDefault
	Accounts      []cliadapter.Account
	AccountsError error
}

type AccountDefaultInput struct {
	Confirmed bool                     `json:"confirmed"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	Previous  *string                  `json:"expected_account_id"`
	Tool      string                   `json:"tool"`
	AccountID string                   `json:"account_id,omitempty"`
}

type AccountDefaultReceipt struct {
	OperationID string                   `json:"operation_id"`
	Context     cliadapter.ConfigContext `json:"context"`
	Tool        string                   `json:"tool"`
	Previous    *string                  `json:"previous_account_id"`
	AccountID   *string                  `json:"account_id"`
	Changed     bool                     `json:"changed"`
}

func AccountDefaultSupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method != method {
			continue
		}
		switch method {
		case "accounts.default.get":
			return op.Effect == "read" && op.Credential == "none" && op.Confirmation == ""
		case "accounts.default.set":
			return op.Effect == "write" && op.Credential == "device" && op.Confirmation == "exact_request"
		case "accounts.default.clear":
			return op.Effect == "write" && op.Credential == "none" && op.Confirmation == "exact_request"
		}
	}
	return false
}

// LoadAccountDefault gets local state first; an unavailable remote list never
// prevents reviewing or clearing a local default.
func LoadAccountDefault(ctx context.Context, client cliadapter.Client, tool string) (AccountDefaultOverview, error) {
	var result AccountDefaultOverview
	if !validDefaultTool(tool) || !validWireGuardPath(client.Home) {
		return result, fault.CLIProtocol
	}
	if err := client.Call(ctx, "capabilities", struct{}{}, &result.Capabilities); err != nil {
		return result, err
	}
	if result.Capabilities.Validate() != nil || !result.Capabilities.MatchesHostPlatform() {
		return result, fault.CLIProtocol
	}
	if !AccountDefaultSupported(result.Capabilities, "accounts.default.get") {
		return result, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	value, err := GetAccountDefault(ctx, client, tool)
	if err != nil {
		return result, err
	}
	result.Default = value
	if value.Context.ServerURL == nil || value.Context.DeviceID == nil || !defaultAccountsSupported(result.Capabilities) || !AccountDefaultSupported(result.Capabilities, "accounts.default.set") {
		return result, nil
	}
	var list struct {
		Context LoginContext         `json:"context"`
		Items   []cliadapter.Account `json:"items"`
	}
	result.AccountsError = client.Call(ctx, "accounts.list", struct{}{}, &list)
	if result.AccountsError == nil {
		if !sameText(list.Context.ServerURL, value.Context.ServerURL) || !sameText(list.Context.DeviceID, value.Context.DeviceID) {
			result.AccountsError = fault.CLIContextChanged
		} else if list.Items == nil {
			result.AccountsError = fault.CLIProtocol
		} else {
			seen := map[string]bool{}
			for _, account := range list.Items {
				if !validDefaultTool(account.ID) || !validDefaultTool(account.Tool) || seen[account.ID] {
					result.AccountsError = fault.CLIProtocol
					break
				}
				seen[account.ID] = true
			}
			if result.AccountsError == nil {
				result.Accounts = list.Items
			}
		}
	}
	return result, nil
}

func GetAccountDefault(ctx context.Context, client cliadapter.Client, tool string) (AccountDefault, error) {
	var result AccountDefault
	if !validDefaultTool(tool) || !validWireGuardPath(client.Home) {
		return result, fault.CLIProtocol
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.default.get", struct {
		Home string `json:"expected_home"`
		Tool string `json:"tool"`
	}{client.Home, tool}, &raw); err != nil {
		return result, err
	}
	if strictDefaultJSON(raw, &result, "context", "tool", "account_id") != nil || !validDefaultContext(result.Context) || !validDefaultID(result.AccountID) || result.Context.Home != client.Home || result.Tool != tool {
		return AccountDefault{}, fault.CLIProtocol
	}
	return result, nil
}

func SetAccountDefault(ctx context.Context, client cliadapter.Client, input AccountDefaultInput) (AccountDefaultReceipt, error) {
	return changeAccountDefault(ctx, client, input, false)
}
func ClearAccountDefault(ctx context.Context, client cliadapter.Client, input AccountDefaultInput) (AccountDefaultReceipt, error) {
	return changeAccountDefault(ctx, client, input, true)
}

func changeAccountDefault(ctx context.Context, client cliadapter.Client, input AccountDefaultInput, clearDefault bool) (AccountDefaultReceipt, error) {
	// Copy pointer fields before starting the external process; a review is a snapshot.
	input.Expected.ServerURL = copyDefaultText(input.Expected.ServerURL)
	input.Expected.DeviceID = copyDefaultText(input.Expected.DeviceID)
	input.Previous = copyDefaultText(input.Previous)
	if !input.Confirmed {
		return AccountDefaultReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if !validDefaultContext(input.Expected) || input.Expected.Home != client.Home || !validDefaultTool(input.Tool) || !validDefaultID(input.Previous) || (!clearDefault && !validDefaultTool(input.AccountID)) {
		return AccountDefaultReceipt{}, fault.CLIProtocol
	}
	method := "accounts.default.set"
	if clearDefault {
		method = "accounts.default.clear"
		input.AccountID = ""
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return AccountDefaultReceipt{}, err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return AccountDefaultReceipt{}, fault.CLIProtocol
	}
	if !AccountDefaultSupported(caps, method) {
		return AccountDefaultReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, method, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return AccountDefaultReceipt{}, err
		}
		return AccountDefaultReceipt{}, fault.AccountDefaultUnknown
	}
	var receipt AccountDefaultReceipt
	if strictDefaultJSON(raw, &receipt, "operation_id", "context", "tool", "previous_account_id", "account_id", "changed") != nil || receipt.OperationID == "" || !validDefaultContext(receipt.Context) || !sameDefaultContext(receipt.Context, input.Expected) || receipt.Tool != input.Tool || !sameText(receipt.Previous, input.Previous) || !validDefaultID(receipt.AccountID) || receipt.Changed == sameText(receipt.Previous, receipt.AccountID) {
		return AccountDefaultReceipt{}, fault.AccountDefaultUnknown
	}
	if (clearDefault && receipt.AccountID != nil) || (!clearDefault && (receipt.AccountID == nil || *receipt.AccountID != input.AccountID)) {
		return AccountDefaultReceipt{}, fault.AccountDefaultUnknown
	}
	return receipt, nil
}

func copyDefaultText(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}
func sameDefaultContext(a, b cliadapter.ConfigContext) bool {
	return a.Home == b.Home && sameText(a.ServerURL, b.ServerURL) && sameText(a.DeviceID, b.DeviceID)
}

func defaultAccountsSupported(c cliadapter.Capabilities) bool {
	for _, op := range c.Operations {
		if op.Method == "accounts.list" {
			return op.Effect == "read" && op.Credential == "device" && op.Confirmation == ""
		}
	}
	return false
}
