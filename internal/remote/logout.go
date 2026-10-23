package remote

import (
	"context"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type LogoutInput struct {
	Confirmed bool         `json:"confirmed"`
	Revoke    bool         `json:"revoke_remote"`
	Expected  LoginContext `json:"expected_context"`
}

type CredentialLogout struct {
	LocalRemoved *bool  `json:"local_removed"`
	Remote       string `json:"remote"`
}

type LogoutReceipt struct {
	OperationID    string           `json:"operation_id"`
	ServerURL      *string          `json:"server_url"`
	DeviceID       *string          `json:"active_device_id"`
	User           CredentialLogout `json:"user"`
	Device         CredentialLogout `json:"device"`
	BrokerRemoved  *bool            `json:"broker_removed"`
	RefreshRemoved *bool            `json:"refresh_metadata_removed"`
	ConfigCleared  *bool            `json:"config_cleared"`
	LocalComplete  *bool            `json:"local_complete"`
}

func Logout(ctx context.Context, client cliadapter.Client, input LogoutInput) (LogoutReceipt, error) {
	var result LogoutReceipt
	if err := client.Call(ctx, "logout", input, &result); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return LogoutReceipt{}, err
		}
		return LogoutReceipt{}, fault.LogoutOutcomeUnknown
	}
	if result.OperationID == "" || !sameText(result.ServerURL, input.Expected.ServerURL) || !sameText(result.DeviceID, input.Expected.DeviceID) {
		return LogoutReceipt{}, fault.LogoutOutcomeUnknown
	}
	complete := true
	for _, field := range []*bool{result.User.LocalRemoved, result.Device.LocalRemoved, result.BrokerRemoved, result.RefreshRemoved, result.ConfigCleared} {
		if field == nil {
			return LogoutReceipt{}, fault.LogoutOutcomeUnknown
		}
		complete = complete && *field
	}
	if result.LocalComplete == nil || *result.LocalComplete != complete {
		return LogoutReceipt{}, fault.LogoutOutcomeUnknown
	}
	for _, state := range []string{result.User.Remote, result.Device.Remote} {
		if (!input.Revoke && state != "not_requested") || (input.Revoke && state != "confirmed" && state != "not_present" && state != "unconfirmed") {
			return LogoutReceipt{}, fault.LogoutOutcomeUnknown
		}
	}
	return result, nil
}

func sameText(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}
