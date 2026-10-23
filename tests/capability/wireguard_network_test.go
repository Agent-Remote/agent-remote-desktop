package capability_test

import (
	"errors"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func TestWireGuardNetworkReadinessDoesNotRequireRemoteContext(t *testing.T) {
	caps := cliadapter.Capabilities{Operations: []cliadapter.Capability{
		{Method: "wireguard.check", Effect: "read", Credential: "none"},
		{Method: "wireguard.status", Effect: "read", Credential: "none"},
		{Method: "wireguard.bridge.status", Effect: "read", Credential: "none"},
		{Method: "wireguard.tunnel.plan", Effect: "read", Credential: "none"},
		{Method: "wireguard.tunnel.apply", Effect: "write", Credential: "none", Confirmation: "exact_request"},
	}}
	malformedServer := "://invalid remote configuration"
	for _, tc := range []struct {
		name   string
		config cliadapter.ConfigContext
		err    error
	}{
		{"unconfigured", cliadapter.ConfigContext{}, nil},
		{"malformed-server", cliadapter.ConfigContext{ServerURL: &malformedServer}, errors.New("malformed server context")},
	} {
		for _, method := range []string{"wireguard.check", "wireguard.status", "wireguard.bridge.status", "wireguard.tunnel.plan", "wireguard.tunnel.apply"} {
			t.Run(tc.name+"/"+method, func(t *testing.T) {
				assessment := capability.Assess(caps, tc.config, tc.err, method, false, nil)
				if assessment.State != "ready" || !assessment.CanStart() {
					t.Fatalf("local query blocked on remote context: %+v", assessment)
				}
			})
		}
	}
}
