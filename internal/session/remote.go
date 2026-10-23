package session

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

// RemoteAttachArguments uses only the public destination reviewed by the user.
// The CLI rechecks current configuration and obtains its own authorization.
func RemoteAttachArguments(target workspace.RemoteContext) ([]string, error) {
	if !target.Valid() {
		return nil, fault.ResumeReference
	}
	return []string{"attach", target.SessionID, "--expected-server", target.ServerURL, "--expected-device", target.DeviceID}, nil
}
