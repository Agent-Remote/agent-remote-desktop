// Package capability separates local GUI prerequisites from server authorization.
package capability

import (
	"errors"
	"slices"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

type Operation struct {
	Method, Effect          string
	Context, Server, Device bool
	User                    bool
	Passive                 bool
}

// Operations lists implemented desktop workflows, not all discoverable CLI syntax.
func Operations() []Operation {
	return []Operation{
		{Method: "accounts.runtime_recovery.authority", Effect: "read", Context: true, Server: true, User: true},
		{Method: "accounts.runtime_recovery.inspect", Effect: "read", Context: true, Server: true, User: true},
		{Method: "accounts.runtime_recovery.apply", Effect: "write", Context: true, Server: true, User: true},
		{Method: "accounts.runtime_recovery.status", Effect: "read", Context: true, Server: true, User: true},
		{Method: "accounts.runtime_recovery.pending", Effect: "read"},
		{Method: "accounts.runtime_recovery.forget", Effect: "write"},
		{Method: "setup.local", Effect: "write"},
		{Method: "mutagen.environment.preview", Effect: "read"},
		{Method: "mutagen.environment.apply", Effect: "write"},
		{Method: "wireguard.check", Effect: "read"},
		{Method: "wireguard.status", Effect: "read"},
		{Method: "wireguard.bridge.status", Effect: "read"},
		{Method: "wireguard.tunnel.plan", Effect: "read"},
		{Method: "wireguard.tunnel.apply", Effect: "write"},
		{Method: "wireguard.config.plan", Effect: "read", Context: true, Server: true, Device: true, Passive: true},
		{Method: "wireguard.config.apply", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "login.password", Effect: "write", Context: true},
		{Method: "login.device_code", Effect: "write", Context: true},
		{Method: "logout", Effect: "write", Context: true, Server: true},
		{Method: "accounts.default.get", Effect: "read"},
		{Method: "accounts.default.clear", Effect: "write"},
		{Method: "accounts.default.set", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sync.status", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sync.recovery.preview", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sync.recovery.apply", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sync.ensure", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sync.pause", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sync.resume", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sync.resolve", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sync.reset", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.create", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.import.discover", Effect: "read"},
		{Method: "accounts.import.prepare", Effect: "read"},
		{Method: "accounts.import.pending", Effect: "read"},
		{Method: "accounts.import.validate", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.import.apply", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.import.status", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "accounts.import.forget", Effect: "write"},
		{Method: "accounts.status", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "accounts.bind", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.verify", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.disable", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "accounts.list", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "credentials.list", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "credentials.create", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "credentials.bind", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "credentials.unbind", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "skills.list", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.info", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.state.list", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.state.conflicts", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.state.info", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.state.diff", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.change", Effect: "write", Context: true, Server: true, User: true},
		{Method: "skills.retry.plan", Effect: "read", Context: true, Server: true, User: true},
		{Method: "skills.retry.control", Effect: "write", Context: true, Server: true, User: true},
		{Method: "sessions.list", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sessions.create", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sessions.create.request", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sessions.stop", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sessions.stop.request", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sessions.stop.status", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sessions.delete", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "sessions.delete.request", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "sessions.delete.status", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "port_forwards.list", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "port_forwards.create", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "port_forwards.stop", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "port_forwards.reconnect", Effect: "write", Context: true, Server: true, Device: true},
		{Method: "device.status", Effect: "read", Context: true, Server: true, Device: true},
		{Method: "device.local.status", Effect: "read", Context: true, Passive: true},
		{Method: "device.local.plan", Effect: "read", Context: true, Passive: true},
		{Method: "device.revoke", Effect: "write", Context: true, Server: true, Device: true, User: true},
		{Method: "device.rotate_token", Effect: "write", Context: true, Server: true, Device: true, User: true},
		{Method: "nodes.list", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.status", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.candidates", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.claim.plan", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.claim.control", Effect: "write", Context: true, Server: true, User: true},
		{Method: "ego_browser.requests", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.request.cancel", Effect: "write", Context: true, Server: true, User: true},
		{Method: "ego_browser.binding.plan", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.binding.control", Effect: "write", Context: true, Server: true, User: true},
		{Method: "ego_browser.delete.plan", Effect: "read", Context: true, Server: true, User: true},
		{Method: "ego_browser.delete.control", Effect: "write", Context: true, Server: true, User: true},
		{Method: "diagnostics.inspect", Effect: "read"},
		{Method: "dependencies.inspect", Effect: "read"},
		{Method: "status.inspect", Effect: "read"},
		{Method: "doctor.inspect", Effect: "read"},
		{Method: "doctor.repair", Effect: "write"},
		{Method: "deps.inspect", Effect: "read"},
		{Method: "deps.repair", Effect: "write"},
		{Method: "ssh.inspect", Effect: "read"},
	}
}

type Assessment struct{ State string }

// CanStart permits an explicit attempt when authorization has not been checked.
// It never treats a saved device ID or method declaration as authorization proof.
func (a Assessment) CanStart() bool {
	return a.State == "ready" || a.State == "unverified" || a.State == "observed_allowed"
}

func Assess(caps cliadapter.Capabilities, config cliadapter.ConfigContext, contextErr error, method string, observed bool, outcome error) Assessment {
	var requirement *Operation
	for _, operation := range Operations() {
		if operation.Method == method {
			copy := operation
			requirement = &copy
			break
		}
	}
	if requirement == nil || !caps.SupportsEffect(method, requirement.Effect) {
		return Assessment{"cli_unsupported"}
	}
	if requirement.Context && contextErr != nil {
		return Assessment{"context_unavailable"}
	}
	if requirement.Server && (config.ServerURL == nil || *config.ServerURL == "") {
		return Assessment{"setup_required"}
	}
	if requirement.Device && (config.DeviceID == nil || *config.DeviceID == "") {
		return Assessment{"device_required"}
	}
	if (!requirement.Device && !requirement.User) || requirement.Passive {
		return Assessment{"ready"}
	}
	if !observed {
		return Assessment{"unverified"}
	}
	if outcome == nil {
		return Assessment{"observed_allowed"}
	}
	var operation *cliadapter.OperationError
	if errors.As(outcome, &operation) {
		switch operation.Code {
		case "permission_denied":
			return Assessment{"denied"}
		case "authentication_required", "credential_unavailable":
			return Assessment{"sign_in_required"}
		case "unsupported_method":
			return Assessment{"cli_unsupported"}
		}
	}
	return Assessment{"unverified"}
}

// HostPlatform normalizes Go's Darwin name to the Rust schema's macos name.
func HostPlatform(host string) string {
	return cliadapter.PlatformName(host)
}

func PlatformState(command cliadapter.Command, host string) string {
	if len(command.Platforms) == 0 {
		return "undeclared"
	}
	if slices.Contains(command.Platforms, HostPlatform(host)) {
		return "supported"
	}
	return "unsupported"
}
