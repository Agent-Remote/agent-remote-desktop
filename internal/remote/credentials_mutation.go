package remote

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	CredentialsCreateMethod = "credentials.create"
	CredentialsBindMethod   = "credentials.bind"
	CredentialsUnbindMethod = "credentials.unbind"
)

type CredentialCreateInput struct {
	Confirmed    bool                     `json:"confirmed"`
	Expected     cliadapter.ConfigContext `json:"expected_context"`
	DisplayName  string                   `json:"display_name"`
	GitUserName  *string                  `json:"git_user_name,omitempty"`
	GitUserEmail *string                  `json:"git_user_email,omitempty"`
	GHMode       string                   `json:"gh_mode"`
	SSHMode      string                   `json:"ssh_mode"`
}

type CredentialBindingInput struct {
	Confirmed bool                     `json:"confirmed"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	AccountID string                   `json:"account_id"`
	ProfileID string                   `json:"profile_id"`
}

type CredentialUnbindInput struct {
	Confirmed bool                     `json:"confirmed"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	AccountID string                   `json:"account_id"`
}

type CredentialMutationReceipt struct {
	OperationID string                        `json:"operation_id"`
	Context     cliadapter.ConfigContext      `json:"context"`
	Accepted    bool                          `json:"accepted"`
	Action      string                        `json:"action"`
	AccountID   *string                       `json:"account_id"`
	ProfileID   *string                       `json:"profile_id"`
	Profile     *cliadapter.CredentialProfile `json:"profile"`
}

func CredentialMutationSupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == method && op.Effect == "write" && op.Credential == "device" && op.Confirmation == "exact_request" {
			return method == CredentialsCreateMethod || method == CredentialsBindMethod || method == CredentialsUnbindMethod
		}
	}
	return false
}

func CreateCredentialProfile(ctx context.Context, client cliadapter.Client, input CredentialCreateInput) (CredentialMutationReceipt, error) {
	if !input.Confirmed {
		return CredentialMutationReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if err := validateCredentialContext(client, input.Expected); err != nil {
		return CredentialMutationReceipt{}, err
	}
	if err := validateCredentialCreate(input); err != nil {
		return CredentialMutationReceipt{}, err
	}
	receipt, err := submitCredentialMutation(ctx, client, CredentialsCreateMethod, input, input.Expected)
	if err != nil {
		return CredentialMutationReceipt{}, err
	}
	if receipt.Profile == nil || receipt.Profile.DisplayName != input.DisplayName || receipt.Profile.GitHubCLIMode != input.GHMode || receipt.Profile.SSHMode != input.SSHMode {
		return CredentialMutationReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

// ValidateCredentialCreateInput validates only profile metadata. It never
// accepts token, password, private-key, or other secret fields.
func ValidateCredentialCreateInput(input CredentialCreateInput) error {
	return validateCredentialCreate(input)
}

func BindCredentialProfile(ctx context.Context, client cliadapter.Client, input CredentialBindingInput) (CredentialMutationReceipt, error) {
	if !input.Confirmed {
		return CredentialMutationReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if err := validateCredentialContext(client, input.Expected); err != nil {
		return CredentialMutationReceipt{}, err
	}
	if !validCredentialID(input.AccountID) || !validCredentialID(input.ProfileID) {
		return CredentialMutationReceipt{}, fault.AccountInputInvalid
	}
	receipt, err := submitCredentialMutation(ctx, client, CredentialsBindMethod, input, input.Expected)
	if err != nil {
		return CredentialMutationReceipt{}, err
	}
	if receipt.Profile == nil || receipt.Profile.ID != input.ProfileID || receipt.AccountID == nil || *receipt.AccountID != input.AccountID {
		return CredentialMutationReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

func UnbindCredentialProfile(ctx context.Context, client cliadapter.Client, input CredentialUnbindInput) (CredentialMutationReceipt, error) {
	if !input.Confirmed {
		return CredentialMutationReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	if err := validateCredentialContext(client, input.Expected); err != nil {
		return CredentialMutationReceipt{}, err
	}
	if !validCredentialID(input.AccountID) {
		return CredentialMutationReceipt{}, fault.AccountInputInvalid
	}
	receipt, err := submitCredentialMutation(ctx, client, CredentialsUnbindMethod, input, input.Expected)
	if err != nil {
		return CredentialMutationReceipt{}, err
	}
	if receipt.Profile != nil || receipt.AccountID == nil || *receipt.AccountID != input.AccountID {
		return CredentialMutationReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

func submitCredentialMutation(ctx context.Context, client cliadapter.Client, method string, input any, expected cliadapter.ConfigContext) (CredentialMutationReceipt, error) {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return CredentialMutationReceipt{}, err
	}
	if !CredentialMutationSupported(caps, method) {
		return CredentialMutationReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, method, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return CredentialMutationReceipt{}, err
		}
		return CredentialMutationReceipt{}, fault.CLIProtocol
	}
	var receipt CredentialMutationReceipt
	if err := decodeCredentialReceipt(raw, &receipt); err != nil {
		return CredentialMutationReceipt{}, fault.CLIProtocol
	}
	if filepath.Clean(receipt.Context.Home) != filepath.Clean(expected.Home) || !sameContext(receipt.Context, expected) || receipt.OperationID == "" || !receipt.Accepted {
		return CredentialMutationReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

func validateCredentialContext(client cliadapter.Client, expected cliadapter.ConfigContext) error {
	if filepath.Clean(expected.Home) != filepath.Clean(client.Home) || expected.ServerURL == nil || expected.DeviceID == nil || strings.TrimSpace(*expected.ServerURL) == "" || strings.TrimSpace(*expected.DeviceID) == "" {
		// context mismatch is intentionally fail-closed
		return fault.CLIContextChanged
	}
	return nil
}

func validateCredentialCreate(input CredentialCreateInput) error {
	if !validCredentialText(input.DisplayName, 256) || !validCredentialOptionalText(input.GitUserName, 256) || !validCredentialOptionalText(input.GitUserEmail, 320) || !validCredentialGHMode(input.GHMode) || !validCredentialSSHMode(input.SSHMode) {
		return fault.AccountInputInvalid
	}
	return nil
}

func validCredentialID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, c := range value {
		if !(c == '-' || c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}

func validCredentialText(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "\r\n\x00")
}

func validCredentialOptionalText(value *string, limit int) bool {
	return value == nil || validCredentialText(*value, limit)
}
func validCredentialGHMode(value string) bool {
	return value == "remote_login" || value == "import_token" || value == "disabled"
}
func validCredentialSSHMode(value string) bool {
	return value == "agent_forwarding" || value == "deploy_key" || value == "disabled"
}

func decodeCredentialReceipt(raw json.RawMessage, receipt *CredentialMutationReceipt) error {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(receipt); err != nil || receipt.OperationID == "" || receipt.Action == "" || !receipt.Accepted || receipt.Context.Home == "" {
		return fault.CLIProtocol
	}
	if receipt.Action != "create" && receipt.Action != "bind" && receipt.Action != "unbind" {
		return fault.CLIProtocol
	}
	return nil
}
