package capability_test

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"testing"
)

func TestRuntimeRecoveryRequiresUserAuthorityWithoutDevice(t *testing.T) {
	server := "https://test.invalid"
	config := cliadapter.ConfigContext{ServerURL: &server}
	method := "accounts.runtime_recovery.apply"
	caps := cliadapter.Capabilities{Operations: []cliadapter.Capability{{Method: method, Effect: "write", Credential: "user"}}}
	if got := capability.Assess(caps, config, nil, method, false, nil); got.State != "unverified" {
		t.Fatalf("device-less user operation: %+v", got)
	}
	if got := capability.Assess(caps, config, nil, method, true, &cliadapter.OperationError{Code: "permission_denied"}); got.State != "denied" || got.CanStart() {
		t.Fatalf("non-admin allowed: %+v", got)
	}
	if got := capability.Assess(caps, config, nil, method, true, &cliadapter.OperationError{Code: "authentication_required"}); got.State != "sign_in_required" {
		t.Fatal(got)
	}
}
