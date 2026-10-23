package capability_test

import (
	"errors"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func TestCapabilityDistinguishesLocalReadinessFromAuthorization(t *testing.T) {
	server, device := "https://test.invalid", "device"
	config := cliadapter.ConfigContext{ServerURL: &server, DeviceID: &device}
	caps := cliadapter.Capabilities{Operations: []cliadapter.Capability{{Method: "accounts.list", Effect: "read"}, {Method: "logout", Effect: "write"}, {Method: "login.password", Effect: "write"}, {Method: "diagnostics.inspect", Effect: "read"}}}
	caps.Operations = append(caps.Operations, cliadapter.Capability{Method: "wireguard.config.plan", Effect: "read"}, cliadapter.Capability{Method: "wireguard.config.apply", Effect: "write"})
	for _, tc := range []struct {
		method   string
		observed bool
		err      error
		state    string
		start    bool
	}{
		{"accounts.list", false, nil, "unverified", true},
		{"accounts.list", true, nil, "observed_allowed", true},
		{"accounts.list", true, &cliadapter.OperationError{Code: "permission_denied"}, "denied", false},
		{"accounts.list", true, &cliadapter.OperationError{Code: "authentication_required"}, "sign_in_required", false},
		{"accounts.list", true, errors.New("offline"), "unverified", true},
		{"login.device_code", false, nil, "cli_unsupported", false},
		{"logout", false, nil, "ready", true},
		{"wireguard.config.plan", false, nil, "ready", true},
		{"wireguard.config.apply", false, nil, "unverified", true},
	} {
		got := capability.Assess(caps, config, nil, tc.method, tc.observed, tc.err)
		if got.State != tc.state || got.CanStart() != tc.start {
			t.Fatalf("%s: %+v", tc.method, got)
		}
	}
	config.DeviceID = nil
	if got := capability.Assess(caps, config, nil, "wireguard.config.plan", false, nil); got.State != "device_required" {
		t.Fatal("passive review still needs a configured device")
	}
	if got := capability.Assess(caps, config, nil, "accounts.list", true, nil); got.State != "device_required" {
		t.Fatal(got)
	}
	config.ServerURL = nil
	if got := capability.Assess(caps, config, nil, "logout", false, nil); got.CanStart() {
		t.Fatal(got)
	}
	if got := capability.Assess(caps, config, errors.New("corrupt config"), "diagnostics.inspect", false, nil); !got.CanStart() {
		t.Fatal("diagnostics must remain usable for broken configuration")
	}
	if got := capability.Assess(caps, config, errors.New("corrupt config"), "login.password", false, nil); got.CanStart() {
		t.Fatal("login cannot confirm an unreadable context")
	}
}

func TestPlatformDeclarationsAreExplicitAndNeverInherited(t *testing.T) {
	for _, host := range []string{"windows", "linux", "darwin"} {
		if capability.PlatformState(cliadapter.Command{}, host) != "undeclared" {
			t.Fatal("missing declaration became support")
		}
		command := cliadapter.Command{Platforms: []string{"macos"}}
		want := "unsupported"
		if host == "darwin" {
			want = "supported"
		}
		if capability.PlatformState(command, host) != want {
			t.Fatal(host)
		}
	}
}

func TestOperationsExposeLifecycleAndForwardPermissionGates(t *testing.T) {
	wanted := []string{
		"sessions.create.request", "sessions.stop", "sessions.stop.request", "sessions.stop.status",
		"sessions.delete", "sessions.delete.request", "sessions.delete.status",
		"port_forwards.list", "port_forwards.create", "port_forwards.stop", "port_forwards.reconnect",
		"device.status", "device.revoke", "device.rotate_token",
	}
	seen := make(map[string]capability.Operation)
	for _, operation := range capability.Operations() {
		seen[operation.Method] = operation
	}
	for _, method := range wanted {
		operation, ok := seen[method]
		if !ok {
			t.Fatalf("missing operation assessment for %s", method)
		}
		if !operation.Context || !operation.Server || !operation.Device {
			t.Fatalf("%s is missing exact context requirements: %+v", method, operation)
		}
	}
	if !seen["device.revoke"].User || !seen["device.rotate_token"].User {
		t.Fatal("device mutations must require user authorization observation")
	}
}
