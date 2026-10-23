package cliadapter

import (
	"runtime"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// Validate checks the common capability contract without treating a declaration
// as permission to contact a server or proof that it matches the local platform.
func (c Capabilities) Validate() error {
	if c.Version != ProtocolVersion || c.CLIVersion == "" || c.Platform == "" || len(c.Operations) == 0 || len(c.Operations) > 256 {
		return fault.CLIProtocol
	}
	seen := map[string]bool{}
	for _, operation := range c.Operations {
		if operation.Method == "" || seen[operation.Method] || (operation.Effect != "read" && operation.Effect != "write") {
			return fault.CLIProtocol
		}
		seen[operation.Method] = true
	}
	if len(c.TerminalOperations) > 256 {
		return fault.CLIProtocol
	}
	for _, operation := range c.TerminalOperations {
		if operation.Method == "" || seen[operation.Method] || operation.Version < 1 ||
			(operation.Effect != "" && operation.Effect != "read" && operation.Effect != "write") ||
			(operation.Credential != "" && !validTerminalCredential(operation.Credential)) ||
			(operation.Confirmation != "" && operation.Confirmation != "exact_request") ||
			(operation.Workflow != "" && !validTerminalWorkflow(operation.Workflow)) {
			return fault.CLIProtocol
		}
		seen[operation.Method] = true
	}
	return nil
}

func validTerminalCredential(value string) bool {
	switch value {
	case "none", "device", "user", "stored":
		return true
	default:
		return false
	}
}

func validTerminalWorkflow(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-') {
			return false
		}
	}
	return true
}

// SupportsTerminal accepts only the implemented native terminal protocol and
// current platform. A control operation cannot substitute for this capability.
func (c Capabilities) SupportsTerminal(method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, operation := range c.TerminalOperations {
		if operation.Method == method && operation.Version == 1 {
			return true
		}
	}
	return false
}

// SupportsExactAttachment requires both halves of the desktop-side bridge:
// the exact session terminal and the versioned payload transfer declaration.
// A missing field remains unsupported, which keeps older CLI builds
// fail-closed instead of treating a terminal-only attach as file capable.
func (c Capabilities) SupportsExactAttachment() bool {
	return c.SupportsTerminal("sessions.attach") && c.AttachmentTransfer.Valid()
}

// SupportsTerminalContract verifies the optional security metadata attached to
// a native terminal capability. Older CLI builds may omit effect, credential,
// or confirmation, so an omitted field remains compatible; an explicit value
// must match the desktop workflow's reviewed contract. This prevents a CLI
// that advertises a known method with weaker semantics from being treated as a
// usable workflow.
func (c Capabilities) SupportsTerminalContract(method, effect, credential, confirmation string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, operation := range c.TerminalOperations {
		if operation.Method != method || operation.Version != 1 {
			continue
		}
		if operation.Effect != "" && operation.Effect != effect {
			return false
		}
		if operation.Credential != "" && operation.Credential != credential {
			return false
		}
		if operation.Confirmation != "" && operation.Confirmation != confirmation {
			return false
		}
		return true
	}
	return false
}

// SupportsTerminalWorkflow verifies an optional workflow label supplied by the
// CLI. Older compatible CLIs may omit the label; the exact command capability
// remains sufficient in that case. A present label must match, preventing a
// generic terminal entry from being mistaken for a conflict-specific review.
func (c Capabilities) SupportsTerminalWorkflow(method, workflow string) bool {
	if workflow == "" || c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, operation := range c.TerminalOperations {
		if operation.Method == method && operation.Version == 1 && (operation.Workflow == "" || operation.Workflow == workflow) {
			return true
		}
	}
	return false
}

func (c Capabilities) MatchesHostPlatform() bool {
	return PlatformName(c.Platform) == PlatformName(runtime.GOOS)
}

// PlatformName normalizes Go's Darwin name to the Rust schema's macos name.
func PlatformName(platform string) string {
	if platform == "darwin" {
		return "macos"
	}
	return platform
}
