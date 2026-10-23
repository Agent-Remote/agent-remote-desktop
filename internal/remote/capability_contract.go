package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// supportsDesktopOperation matches the complete capability declaration used by
// the Rust desktop-control bridge. Checking only the effect is insufficient:
// an operation with the right read/write bit but the wrong credential or
// confirmation mode is a different contract and must be treated as unsupported.
func supportsDesktopOperation(c cliadapter.Capabilities, method, effect, credential, confirmation string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, operation := range c.Operations {
		if operation.Method == method {
			return operation.Effect == effect && operation.Credential == credential && operation.Confirmation == confirmation
		}
	}
	return false
}
