package remote

import (
	"context"
	"errors"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type LoginContext struct {
	ServerURL *string `json:"server_url"`
	DeviceID  *string `json:"active_device_id"`
}

// PasswordLoginInput is transient stdin data. It must never be persisted or logged.
type PasswordLoginInput struct {
	ServerURL    string       `json:"server_url"`
	Username     string       `json:"username"`
	Password     string       `json:"password"`
	TOTP         *string      `json:"totp_code"`
	DeviceName   *string      `json:"device_name"`
	SSHKey       *string      `json:"ssh_public_key"`
	WireGuardKey *string      `json:"wireguard_public_key"`
	SkipDevice   bool         `json:"skip_device_registration"`
	Confirmed    bool         `json:"confirmed"`
	Expected     LoginContext `json:"expected_context"`
}

func (PasswordLoginInput) String() string   { return "PasswordLoginInput{redacted}" }
func (PasswordLoginInput) GoString() string { return "PasswordLoginInput{redacted}" }

type LoginReceipt struct {
	OperationID string  `json:"operation_id"`
	ServerURL   string  `json:"server_url"`
	DeviceID    *string `json:"registered_device_id"`
	Backend     string  `json:"credential_backend"`
	ExpiresIn   uint64  `json:"expires_in"`
}

func PasswordLogin(ctx context.Context, client cliadapter.Client, input PasswordLoginInput) (LoginReceipt, error) {
	var result LoginReceipt
	err := client.Call(ctx, "login.password", input, &result)
	if err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return LoginReceipt{}, err
		}
		// Losing local transport cannot prove the server rejected a login/registration.
		return LoginReceipt{}, fault.LoginOutcomeUnknown
	}
	if !validLoginReceipt(result, input.ServerURL, input.SkipDevice) {
		return LoginReceipt{}, fault.LoginOutcomeUnknown
	}
	return result, nil
}

func validLoginReceipt(result LoginReceipt, server string, skipDevice bool) bool {
	return result.OperationID != "" && result.ServerURL == strings.TrimRight(strings.TrimSpace(server), "/") && result.ExpiresIn > 0 && (result.Backend == "system" || result.Backend == "file") && (skipDevice || (result.DeviceID != nil && *result.DeviceID != ""))
}
