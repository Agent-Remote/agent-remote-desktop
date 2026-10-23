package remote

import (
	"context"
	"encoding/json"
	"runtime"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// WireGuardBridgeStatus is a passive observation of the platform permission
// bridge. It never means that a tunnel action is authorized. In particular,
// installation and approval are separate facts from the protocol and caller
// authentication channel.
type WireGuardBridgeStatus struct {
	Version        int    `json:"version"`
	Home           string `json:"home"`
	Platform       string `json:"platform"`
	State          string `json:"state"`
	Installation   string `json:"installation"`
	Approval       string `json:"approval"`
	Protocol       string `json:"protocol"`
	Authentication string `json:"authentication"`
	Execution      string `json:"execution"`
}

// WireGuardBridgeLifecycle is the desktop's fail-closed interpretation of a
// bridge status observation. The wire protocol intentionally keeps the
// individual facts independent so a platform broker can report partial
// installation/approval progress. UI actions should use Lifecycle (or
// CanExecute) rather than inferring readiness from one field such as
// installation or process elevation.
type WireGuardBridgeLifecycle string

const (
	WireGuardBridgeUnavailable            WireGuardBridgeLifecycle = "unavailable"
	WireGuardBridgeUnsupported            WireGuardBridgeLifecycle = "unsupported"
	WireGuardBridgeInstallationRequired   WireGuardBridgeLifecycle = "installation_required"
	WireGuardBridgeApprovalPending        WireGuardBridgeLifecycle = "approval_pending"
	WireGuardBridgeApprovalRequired       WireGuardBridgeLifecycle = "approval_required"
	WireGuardBridgeProtocolUnavailable    WireGuardBridgeLifecycle = "protocol_unavailable"
	WireGuardBridgeAuthenticationRequired WireGuardBridgeLifecycle = "authentication_required"
	WireGuardBridgeAuthorizationRequired  WireGuardBridgeLifecycle = "authorization_required"
	WireGuardBridgeNotReady               WireGuardBridgeLifecycle = "not_ready"
	WireGuardBridgeReady                  WireGuardBridgeLifecycle = "ready"
)

// Lifecycle returns the first unmet prerequisite in the order required by a
// protected bridge. This is a local projection only; it never authorizes a
// request or starts a broker. Unknown/partial observations remain blocked.
func (status WireGuardBridgeStatus) Lifecycle() WireGuardBridgeLifecycle {
	switch {
	case status.State == "unavailable":
		return WireGuardBridgeUnavailable
	case status.State == "unsupported":
		return WireGuardBridgeUnsupported
	case status.State != "observed":
		// Keep direct callers fail-closed even when they have not gone through
		// the protocol decoder's enum validation.
		return WireGuardBridgeNotReady
	case status.Installation != "installed":
		return WireGuardBridgeInstallationRequired
	case status.Approval == "pending":
		return WireGuardBridgeApprovalPending
	case status.Approval != "approved":
		return WireGuardBridgeApprovalRequired
	case status.Protocol != "compatible":
		return WireGuardBridgeProtocolUnavailable
	case status.Authentication != "authenticated":
		return WireGuardBridgeAuthenticationRequired
	case status.Execution == "not_authorized":
		return WireGuardBridgeAuthorizationRequired
	case status.Execution == "ready":
		return WireGuardBridgeReady
	default:
		return WireGuardBridgeNotReady
	}
}

// CanExecute is deliberately stricter than the protocol's execution field:
// every independent lifecycle fact must be observed in its approved state.
func (status WireGuardBridgeStatus) CanExecute() bool {
	return status.Lifecycle() == WireGuardBridgeReady
}

// CanApply gates both tunnel directions through the same observed bridge
// lifecycle. Callers must still submit a reviewed tunnel plan; this helper
// only prevents UI actions from treating an installed component or elevated
// process as permission. Unknown actions are rejected so a future command
// cannot accidentally inherit the up/down authorization boundary.
func (status WireGuardBridgeStatus) CanApply(action string) bool {
	return (action == "up" || action == "down") && status.CanExecute()
}

// InspectWireGuardBridge performs one explicit, read-only bridge query. The
// current CLI reports unsupported until a protected native broker is present;
// desktop must preserve that state and never infer readiness from elevation or
// tool presence.
func InspectWireGuardBridge(ctx context.Context, client cliadapter.Client) (WireGuardBridgeStatus, error) {
	if !validWireGuardPath(client.Home) {
		return WireGuardBridgeStatus{}, fault.CLIContextChanged
	}
	if err := checkWireGuardReadCapability(ctx, client, "wireguard.bridge.status"); err != nil {
		return WireGuardBridgeStatus{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.bridge.status", struct {
		ExpectedHome string `json:"expected_home"`
	}{client.Home}, &raw); err != nil {
		return WireGuardBridgeStatus{}, err
	}
	status, err := decodeWireGuardBridgeStatus(raw)
	if err != nil || status.Home != client.Home {
		return WireGuardBridgeStatus{}, fault.CLIProtocol
	}
	return status, nil
}

func decodeWireGuardBridgeStatus(raw json.RawMessage) (WireGuardBridgeStatus, error) {
	var status WireGuardBridgeStatus
	if _, ok := wireGuardObject(raw, "version", "home", "platform", "state", "installation", "approval", "protocol", "authentication", "execution"); !ok || decodeWireGuardJSON(raw, &status) != nil || !validWireGuardBridgeStatus(status) {
		return WireGuardBridgeStatus{}, fault.CLIProtocol
	}
	return status, nil
}

func validWireGuardBridgeStatus(status WireGuardBridgeStatus) bool {
	return status.Version == 1 && validWireGuardPath(status.Home) && status.Platform == cliadapter.PlatformName(runtime.GOOS) &&
		validWireGuardBridgeState(status.State) && validWireGuardBridgeInstallation(status.Installation) &&
		validWireGuardBridgeApproval(status.Approval) && validWireGuardBridgeProtocol(status.Protocol) &&
		validWireGuardBridgeAuthentication(status.Authentication) && validWireGuardBridgeExecution(status.Execution) &&
		(status.State == "observed" || status.Execution == "unavailable")
}

func validWireGuardBridgeState(value string) bool {
	return value == "unavailable" || value == "unsupported" || value == "observed"
}

func validWireGuardBridgeInstallation(value string) bool {
	return value == "not_installed" || value == "installed" || value == "unknown"
}

func validWireGuardBridgeApproval(value string) bool {
	return value == "not_requested" || value == "pending" || value == "approved" || value == "unknown"
}

func validWireGuardBridgeProtocol(value string) bool {
	return value == "unsupported" || value == "incompatible" || value == "compatible" || value == "unknown"
}

func validWireGuardBridgeAuthentication(value string) bool {
	return value == "unavailable" || value == "unauthenticated" || value == "authenticated" || value == "unknown"
}

func validWireGuardBridgeExecution(value string) bool {
	return value == "unavailable" || value == "not_ready" || value == "not_authorized" || value == "ready"
}
