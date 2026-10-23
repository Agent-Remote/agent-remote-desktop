package remote

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func AccountImportSupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	credential, effect, confirmation := "device", "read", ""
	switch method {
	case "accounts.import.discover", "accounts.import.pending", "accounts.import.prepare":
		credential = "none"
	case "accounts.import.status":
	case "accounts.import.validate", "accounts.import.apply":
		effect, confirmation = "write", "exact_request"
	case "accounts.import.forget":
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
func accountImportPreflight(ctx context.Context, client cliadapter.Client, method string) error {
	var capabilities cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &capabilities); err != nil {
		return err
	}
	if capabilities.Validate() != nil || !capabilities.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	if !AccountImportSupported(capabilities, method) {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return nil
}

// NewAccountImportRequestID reserves one caller-generated key before dispatch.
func NewAccountImportRequestID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", err
	}
	value[6] = (value[6] & 15) | 64
	value[8] = (value[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", value[:4], value[4:6], value[6:8], value[8:10], value[10:]), nil
}
func validAccountImportRequestID(value string) bool {
	return validCanonicalUUID(value)
}
func DiscoverAccountImport(ctx context.Context, client cliadapter.Client, selection AccountImportSelection) (AccountImportDiscovery, error) {
	if !validAccountImportSelection(selection) {
		return AccountImportDiscovery{}, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.discover"); err != nil {
		return AccountImportDiscovery{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.discover", struct {
		Selection AccountImportSelection `json:"selection"`
	}{selection}, &raw); err != nil {
		return AccountImportDiscovery{}, err
	}
	value, err := decodeAccountImportDiscovery(raw)
	if err != nil || value.Selection != selection {
		return AccountImportDiscovery{}, fault.CLIProtocol
	}
	return value, nil
}
func PrepareAccountImport(ctx context.Context, client cliadapter.Client, input AccountImportPrepareInput) (AccountImportPlan, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if !validAccountContext(input.Expected, client.Home) || !validDefaultTool(input.AccountID) || !validAccountImportSelection(input.Selection) || !validWireGuardPath(input.ExpectedSourceHome) {
		return AccountImportPlan{}, fault.AccountInputInvalid
	}
	if err := accountImportPreflight(ctx, client, "accounts.import.prepare"); err != nil {
		return AccountImportPlan{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "accounts.import.prepare", input, &raw); err != nil {
		return AccountImportPlan{}, err
	}
	plan, err := decodeAccountImportPlan(raw)
	if err != nil || !sameDefaultContext(plan.Context, input.Expected) || plan.AccountID != input.AccountID || plan.Selection != input.Selection || plan.SourceHome != input.ExpectedSourceHome {
		return AccountImportPlan{}, fault.CLIProtocol
	}
	return plan, nil
}
