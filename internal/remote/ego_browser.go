package remote

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const egoBrowserStatusMethod = "ego_browser.status"

// EgoBrowserStatus is the bounded, secret-free projection used by the
// desktop status page. Local installation state and control-plane state are
// intentionally kept separate so a registered device cannot imply a local
// Bridge binary is installed.
type EgoBrowserStatus struct {
	SchemaVersion int                      `json:"schema_version"`
	Operation     string                   `json:"operation"`
	Capabilities  cliadapter.Capabilities  `json:"-"`
	Context       cliadapter.ConfigContext `json:"context"`
	Local         EgoBrowserLocal          `json:"local"`
	Policy        EgoBrowserPolicy         `json:"policy"`
	Devices       []EgoBrowserDevice       `json:"devices"`
	Bindings      []EgoBrowserBinding      `json:"bindings"`
}

type EgoBrowserLocal struct {
	Installation string `json:"installation"`
	Store        string `json:"store"`
	Admission    string `json:"admission"`
}

type EgoBrowserPolicy struct {
	Enabled    bool    `json:"enabled"`
	Enrollment string  `json:"enrollment"`
	Execution  string  `json:"execution"`
	Protocol   *string `json:"protocol"`
}

type EgoBrowserDevice struct {
	ID               string  `json:"id"`
	Generation       uint64  `json:"generation"`
	DeviceGeneration *uint64 `json:"device_generation"`
	Status           string  `json:"status"`
	ReleaseProfile   string  `json:"release_profile"`
	BridgeVersion    *string `json:"bridge_version"`
	RuntimeVersion   *string `json:"runtime_version"`
	SkillVersion     *string `json:"skill_version"`
}

type EgoBrowserBinding struct {
	ID                    string  `json:"id"`
	DeviceID              string  `json:"device_id"`
	ToolSessionID         string  `json:"tool_session_id"`
	NodeID                string  `json:"node_id"`
	Status                string  `json:"status"`
	RelayBindingKind      string  `json:"relay_binding_kind"`
	AuthorizationMode     string  `json:"authorization_mode"`
	ReleaseProfile        string  `json:"release_profile"`
	BridgeProtocolVersion string  `json:"bridge_protocol_version"`
	LeaseHealth           string  `json:"lease_health"`
	Generation            uint64  `json:"generation"`
	BindingGeneration     *uint64 `json:"binding_generation"`
	ConnectedAt           *string `json:"connected_at"`
	LeaseUntil            *string `json:"lease_until"`
	StopReason            *string `json:"stop_reason"`
}

// EgoBrowserStatusSupported requires the full control-plane contract. A
// terminal capability or a similarly named operation is not a substitute.
func EgoBrowserStatusSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, egoBrowserStatusMethod, "read", "user", "")
}

func LoadEgoBrowserStatus(ctx context.Context, client cliadapter.Client) (EgoBrowserStatus, error) {
	var result EgoBrowserStatus
	if client.Binary == "" || client.Home == "" {
		return result, fault.CLIProtocol
	}
	if err := client.Call(ctx, "capabilities", struct{}{}, &result.Capabilities); err != nil {
		return result, err
	}
	if !EgoBrowserStatusSupported(result.Capabilities) {
		return result, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var expected cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &expected); err != nil {
		return result, err
	}
	if !validEgoContext(expected, client.Home) || expected.ServerURL == nil {
		return result, fault.CLIProtocol
	}
	var raw json.RawMessage
	if err := client.Call(ctx, egoBrowserStatusMethod, struct {
		Expected cliadapter.ConfigContext `json:"expected_context"`
	}{Expected: expected}, &raw); err != nil {
		return result, err
	}
	if err := decodeStrict(raw, &result); err != nil || result.SchemaVersion != 1 || result.Operation != egoBrowserStatusMethod || result.Context.Home == "" || !sameEgoContext(result.Context, expected) || !validEgoStatus(result) {
		return EgoBrowserStatus{}, fault.CLIProtocol
	}
	return result, nil
}

func validEgoContext(value cliadapter.ConfigContext, home string) bool {
	return value.Home == home && value.ServerURL != nil && strings.TrimSpace(*value.ServerURL) != "" && (value.DeviceID == nil || validID(*value.DeviceID))
}

func sameEgoContext(a, b cliadapter.ConfigContext) bool {
	if !validEgoContext(a, b.Home) || !validEgoContext(b, b.Home) {
		return false
	}
	if *a.ServerURL != *b.ServerURL {
		return false
	}
	if a.DeviceID == nil || b.DeviceID == nil {
		return a.DeviceID == nil && b.DeviceID == nil
	}
	return *a.DeviceID == *b.DeviceID
}

func validEgoStatus(status EgoBrowserStatus) bool {
	if status.Local.Installation != "installed" && status.Local.Installation != "missing" ||
		(status.Local.Store != "present" && status.Local.Store != "legacy" && status.Local.Store != "missing") ||
		status.Local.Admission != "unknown" ||
		(status.Policy.Enrollment != "allowed" && status.Policy.Enrollment != "denied") ||
		(status.Policy.Execution != "allowed" && status.Policy.Execution != "denied") ||
		(status.Policy.Protocol != nil && !validText(*status.Policy.Protocol, 64)) ||
		len(status.Devices) > 128 || len(status.Bindings) > 128 {
		return false
	}
	for _, device := range status.Devices {
		if !validIdentifier(device.ID) || !validText(device.Status, 32) || !validText(device.ReleaseProfile, 128) || !optionalText(device.BridgeVersion, 128) || !optionalText(device.RuntimeVersion, 128) || !optionalText(device.SkillVersion, 128) {
			return false
		}
	}
	for _, binding := range status.Bindings {
		if !validIdentifier(binding.ID) || !validIdentifier(binding.DeviceID) || !validIdentifier(binding.ToolSessionID) || !validIdentifier(binding.NodeID) || !validText(binding.Status, 32) || !validText(binding.RelayBindingKind, 64) || !validText(binding.AuthorizationMode, 64) || !validText(binding.ReleaseProfile, 128) || !validText(binding.BridgeProtocolVersion, 64) || !validText(binding.LeaseHealth, 32) || !optionalText(binding.ConnectedAt, 128) || !optionalText(binding.LeaseUntil, 128) || !optionalText(binding.StopReason, 256) {
			return false
		}
	}
	return true
}

func validText(value string, max int) bool {
	return value != "" && len(value) <= max && !strings.ContainsFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 256 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return true
}

func optionalText(value *string, max int) bool {
	return value == nil || validText(*value, max)
}
