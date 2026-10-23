package remote

import (
	"context"
	"errors"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// DeviceRecord is a secret-free projection of the active control-plane device.
type DeviceRecord struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Platform   string  `json:"platform"`
	Status     string  `json:"status"`
	LastSeenAt *string `json:"last_seen_at"`
	CreatedAt  string  `json:"created_at"`
}

type DeviceOverview struct {
	Capabilities cliadapter.Capabilities  `json:"-"`
	Context      cliadapter.ConfigContext `json:"context"`
	Device       DeviceRecord             `json:"device"`
}

type DeviceActionInput struct {
	Confirmed bool                     `json:"confirmed"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	DeviceID  string                   `json:"device_id"`
}

type DeviceReceipt struct {
	OperationID             string                   `json:"operation_id"`
	Context                 cliadapter.ConfigContext `json:"context"`
	DeviceID                string                   `json:"device_id"`
	RemoteStatus            string                   `json:"remote_status"`
	LocalCleanupComplete    *bool                    `json:"local_cleanup_complete"`
	LocalCredentialComplete *bool                    `json:"local_credential_complete"`
	CredentialBackend       string                   `json:"credential_backend"`
	ExpiresIn               uint64                   `json:"expires_in"`
}

func DeviceSupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method != method {
			continue
		}
		switch method {
		case "device.status":
			return op.Effect == "read" && op.Credential == "device" && op.Confirmation == ""
		case "device.revoke", "device.rotate_token":
			return op.Effect == "write" && op.Credential == "user" && op.Confirmation == "exact_request"
		}
	}
	return false
}

func LoadDevice(ctx context.Context, client cliadapter.Client) (DeviceOverview, error) {
	var overview DeviceOverview
	if !validWireGuardPath(client.Home) {
		return overview, fault.CLIProtocol
	}
	if err := client.Call(ctx, "capabilities", struct{}{}, &overview.Capabilities); err != nil {
		return overview, err
	}
	if !DeviceSupported(overview.Capabilities, "device.status") {
		return overview, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var contextResult cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &contextResult); err != nil {
		return overview, err
	}
	if !validDeviceContext(contextResult, client.Home) {
		return overview, fault.CLIProtocol
	}
	var result struct {
		Context cliadapter.ConfigContext `json:"context"`
		Device  DeviceRecord             `json:"device"`
	}
	if err := client.Call(ctx, "device.status", struct {
		Expected cliadapter.ConfigContext `json:"expected_context"`
	}{Expected: contextResult}, &result); err != nil {
		return overview, err
	}
	if !validDeviceContext(result.Context, client.Home) || !validDevice(result.Device) || !sameContext(result.Context, contextResult) {
		return overview, fault.CLIProtocol
	}
	overview.Context, overview.Device = result.Context, result.Device
	return overview, nil
}

func RevokeDevice(ctx context.Context, client cliadapter.Client, input DeviceActionInput) (DeviceReceipt, error) {
	return mutateDevice(ctx, client, input, "device.revoke")
}

func RotateDeviceToken(ctx context.Context, client cliadapter.Client, input DeviceActionInput) (DeviceReceipt, error) {
	return mutateDevice(ctx, client, input, "device.rotate_token")
}

func mutateDevice(ctx context.Context, client cliadapter.Client, input DeviceActionInput, method string) (DeviceReceipt, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !input.Confirmed || !validDeviceContext(input.Expected, client.Home) || !validID(input.DeviceID) || input.Expected.DeviceID == nil || *input.Expected.DeviceID != input.DeviceID {
		return DeviceReceipt{}, fault.CLIProtocol
	}
	caps, err := deviceMutationCapabilities(ctx, client)
	if err != nil {
		return DeviceReceipt{}, err
	}
	if !DeviceSupported(caps, method) {
		return DeviceReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var receipt DeviceReceipt
	if err := client.Call(ctx, method, input, &receipt); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return DeviceReceipt{}, err
		}
		return DeviceReceipt{}, fault.DeviceOutcomeUnknown
	}
	if !validReceipt(receipt, input.Expected, input.DeviceID, method) {
		return DeviceReceipt{}, fault.DeviceOutcomeUnknown
	}
	return receipt, nil
}

func deviceMutationCapabilities(ctx context.Context, client cliadapter.Client) (cliadapter.Capabilities, error) {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return caps, err
	}
	return caps, nil
}

func validReceipt(receipt DeviceReceipt, expected cliadapter.ConfigContext, id, method string) bool {
	if receipt.OperationID == "" || receipt.DeviceID != id || receipt.RemoteStatus != map[string]string{"device.revoke": "revoked", "device.rotate_token": "rotated"}[method] || !sameContext(receipt.Context, expected) {
		return false
	}
	if method == "device.revoke" {
		return receipt.LocalCleanupComplete != nil
	}
	return receipt.LocalCredentialComplete != nil && *receipt.LocalCredentialComplete && receipt.ExpiresIn > 0 && (receipt.CredentialBackend == "system" || receipt.CredentialBackend == "file")
}

func validDeviceContext(value cliadapter.ConfigContext, home string) bool {
	return value.Home == home && value.ServerURL != nil && strings.TrimSpace(*value.ServerURL) != "" && value.DeviceID != nil && validID(*value.DeviceID)
}

func validDevice(value DeviceRecord) bool {
	return validID(value.ID) && value.Name != "" && value.Platform != "" && value.Status != "" && value.CreatedAt != ""
}

func validID(value string) bool {
	return value != "" && len(value) <= 128 && !strings.ContainsAny(value, "\r\n\x00")
}

func cloneText(value *string) *string {
	if value == nil {
		return nil
	}
	v := *value
	return &v
}
