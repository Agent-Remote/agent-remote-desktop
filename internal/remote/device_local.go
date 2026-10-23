package remote

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const deviceLocalStatusMethod = "device.local.status"

// DeviceLocalStatus is a bounded, secret-free projection of the local Device
// app. It describes the macOS community-signed bundle and its XPC helpers.
type DeviceLocalStatus struct {
	SchemaVersion int            `json:"schema_version"`
	Operation     string         `json:"operation"`
	Home          string         `json:"home"`
	Platform      string         `json:"platform"`
	App           DeviceLocalApp `json:"app"`
}

type DeviceLocalApp struct {
	Installation     string  `json:"installation"`
	Path             string  `json:"path"`
	Version          *string `json:"version"`
	BundleIdentifier string  `json:"bundle_identifier"`
	CodeSignature    string  `json:"code_signature"`
	SigningIdentity  string  `json:"signing_identity"`
	Gatekeeper       string  `json:"gatekeeper"`
	NetworkBrokerXPC string  `json:"network_broker_xpc"`
	GUIExecutorXPC   string  `json:"gui_executor_xpc"`
	Running          bool    `json:"running"`
}

func DeviceLocalStatusSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, deviceLocalStatusMethod, "read", "none", "")
}

func LoadDeviceLocalStatus(ctx context.Context, client cliadapter.Client) (DeviceLocalStatus, error) {
	var result DeviceLocalStatus
	if client.Binary == "" || !validWireGuardPath(client.Home) {
		return result, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return result, err
	}
	if !DeviceLocalStatusSupported(caps) {
		return result, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var contextResult cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &contextResult); err != nil {
		return result, err
	}
	if contextResult.Home != client.Home {
		return result, fault.CLIProtocol
	}
	var raw json.RawMessage
	if err := client.Call(ctx, deviceLocalStatusMethod, struct {
		ExpectedHome string `json:"expected_home"`
	}{ExpectedHome: client.Home}, &raw); err != nil {
		return result, err
	}
	if decodeStrict(raw, &result) != nil || !validDeviceLocalStatus(result) {
		return DeviceLocalStatus{}, fault.CLIProtocol
	}
	return result, nil
}

func validDeviceLocalStatus(status DeviceLocalStatus) bool {
	if status.SchemaVersion != 1 || status.Operation != deviceLocalStatusMethod || status.Platform != "macos" || !validWireGuardPath(status.Home) || !validDeviceLocalApp(status.App) {
		return false
	}
	return true
}

func validDeviceLocalApp(app DeviceLocalApp) bool {
	if app.Installation != "installed" && app.Installation != "missing" && app.Installation != "invalid" ||
		!validWireGuardPath(app.Path) ||
		(app.Version != nil && (strings.ContainsAny(*app.Version, "\r\n\x00") || len(*app.Version) > 128)) {
		return false
	}
	if app.BundleIdentifier != "valid" && app.BundleIdentifier != "invalid" && app.BundleIdentifier != "unknown" ||
		app.CodeSignature != "valid" && app.CodeSignature != "invalid" && app.CodeSignature != "unknown" ||
		app.SigningIdentity != "valid" && app.SigningIdentity != "invalid" && app.SigningIdentity != "unknown" ||
		app.Gatekeeper != "accepted" && app.Gatekeeper != "not_required" && app.Gatekeeper != "rejected" && app.Gatekeeper != "unknown" ||
		app.NetworkBrokerXPC != "present" && app.NetworkBrokerXPC != "missing" ||
		app.GUIExecutorXPC != "present" && app.GUIExecutorXPC != "missing" {
		return false
	}
	return true
}
