package remote

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// DeviceCodeLoginInput contains no device authorization secret. The CLI owns
// authorization polling and credential persistence throughout the operation.
type DeviceCodeLoginInput struct {
	ServerURL    string       `json:"server_url"`
	DeviceName   *string      `json:"device_name"`
	SSHKey       *string      `json:"ssh_public_key"`
	WireGuardKey *string      `json:"wireguard_public_key"`
	SkipDevice   bool         `json:"skip_device_registration"`
	Confirmed    bool         `json:"confirmed"`
	Expected     LoginContext `json:"expected_context"`
}

type DeviceCodeApproval struct {
	UserCode        string `json:"user_code"`
	VerificationURL string `json:"verification_url"`
	ExpiresIn       uint64 `json:"expires_in"`
	Interval        uint64 `json:"interval"`
}

func (approval DeviceCodeApproval) Validate() error {
	target, err := url.Parse(approval.VerificationURL)
	if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Hostname() == "" || target.User != nil || target.Fragment != "" || len(approval.VerificationURL) > 2048 || strings.IndexFunc(approval.VerificationURL, unicode.IsControl) >= 0 {
		return fault.CLIProtocol
	}
	if approval.UserCode == "" || len(approval.UserCode) > 64 || strings.IndexFunc(approval.UserCode, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-')
	}) >= 0 || approval.ExpiresIn == 0 || approval.ExpiresIn > 1800 || approval.Interval == 0 || approval.Interval > 60 {
		return fault.CLIProtocol
	}
	query, err := url.ParseQuery(target.RawQuery)
	if err != nil || (len(query) != 0 && (len(query) != 1 || len(query["code"]) != 1 || query.Get("code") != approval.UserCode)) {
		return fault.CLIProtocol
	}
	return nil
}

func DeviceCodeLogin(ctx context.Context, client cliadapter.Client, input DeviceCodeLoginInput, approval func(DeviceCodeApproval)) (LoginReceipt, error) {
	var result LoginReceipt
	err := client.DeviceCodeCall(ctx, input, &result, func(raw json.RawMessage) error {
		var event DeviceCodeApproval
		decoder := json.NewDecoder(strings.NewReader(string(raw)))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&event) != nil {
			return fault.CLIProtocol
		}
		if err := event.Validate(); err != nil {
			return err
		}
		if approval != nil {
			approval(event)
		}
		return nil
	})
	if err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return LoginReceipt{}, err
		}
		return LoginReceipt{}, fault.LoginOutcomeUnknown
	}
	if !validLoginReceipt(result, input.ServerURL, input.SkipDevice) {
		return LoginReceipt{}, fault.LoginOutcomeUnknown
	}
	return result, nil
}
