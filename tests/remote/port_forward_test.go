package remote_test

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestResolveLocalPortSupportsAutoAndBlank(t *testing.T) {
	for _, value := range []string{"", "auto", " AUTO "} {
		port, err := remote.ResolveLocalPort(value)
		if err != nil || port == 0 {
			t.Fatalf("ResolveLocalPort(%q) = %d, %v; want an allocated port", value, port, err)
		}
		listener, err := net.Listen("tcp4", fmt.Sprintf("127.0.0.1:%d", port))
		if err != nil {
			t.Fatalf("allocated port %d was not bindable: %v", port, err)
		}
		_ = listener.Close()
	}
}

func TestResolveLocalPortPreservesExplicitPortAndRejectsZero(t *testing.T) {
	port, err := remote.ResolveLocalPort("43123")
	if err != nil || port != 43123 {
		t.Fatalf("explicit port = %d, %v; want 43123", port, err)
	}
	for _, value := range []string{"0", "-1", "65536", "invalid"} {
		if port, err := remote.ResolveLocalPort(value); err == nil || port != 0 {
			t.Fatalf("ResolveLocalPort(%q) = %d, %v; want rejection", value, port, err)
		}
	}
}

func TestPortForwardCapabilitiesRequireExactEffects(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	caps := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform}
	if remote.PortForwardListSupported(caps) || remote.PortForwardCreateSupported(caps) || remote.PortForwardStopSupported(caps) || remote.PortForwardReconnectSupported(caps) {
		t.Fatal("missing port-forward methods were reported as supported")
	}
	caps.Operations = []cliadapter.Capability{
		{Method: remote.PortForwardListMethod, Effect: "write", Credential: "device"},
		{Method: remote.PortForwardCreateMethod, Effect: "read", Credential: "device"},
		{Method: remote.PortForwardStopMethod, Effect: "read", Credential: "none"},
		{Method: remote.PortForwardReconnectMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"},
	}
	if remote.PortForwardListSupported(caps) || remote.PortForwardCreateSupported(caps) || remote.PortForwardStopSupported(caps) || !remote.PortForwardReconnectSupported(caps) {
		t.Fatalf("port-forward support ignored method effects: list=%v create=%v stop=%v reconnect=%v", remote.PortForwardListSupported(caps), remote.PortForwardCreateSupported(caps), remote.PortForwardStopSupported(caps), remote.PortForwardReconnectSupported(caps))
	}
}

func TestPortForwardCapabilitiesRequireCredentialAndConfirmation(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	base := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform}
	base.Operations = []cliadapter.Capability{
		{Method: remote.PortForwardCreateMethod, Effect: "write", Credential: "device"},
		{Method: remote.PortForwardStopMethod, Effect: "write", Credential: "user", Confirmation: "exact_request"},
		{Method: remote.PortForwardReconnectMethod, Effect: "write", Credential: "device", Confirmation: "prompt"},
		{Method: remote.PortForwardListMethod, Effect: "read", Credential: "device", Confirmation: "exact_request"},
	}
	if remote.PortForwardCreateSupported(base) || remote.PortForwardStopSupported(base) || remote.PortForwardReconnectSupported(base) || remote.PortForwardListSupported(base) {
		t.Fatal("port-forward capability accepted wrong credential or confirmation")
	}
	base.Operations = []cliadapter.Capability{
		{Method: remote.PortForwardListMethod, Effect: "read", Credential: "device"},
		{Method: remote.PortForwardCreateMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"},
		{Method: remote.PortForwardStopMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"},
		{Method: remote.PortForwardReconnectMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"},
	}
	if !remote.PortForwardListSupported(base) || !remote.PortForwardCreateSupported(base) || !remote.PortForwardStopSupported(base) || !remote.PortForwardReconnectSupported(base) {
		t.Fatal("valid port-forward capability was rejected")
	}
}

func TestPortForwardMethodsUseStableNames(t *testing.T) {
	want := []string{"port_forwards.list", "port_forwards.create", "port_forwards.stop", "port_forwards.reconnect"}
	got := []string{remote.PortForwardListMethod, remote.PortForwardCreateMethod, remote.PortForwardStopMethod, remote.PortForwardReconnectMethod}
	for i, method := range got {
		if method != want[i] {
			t.Fatalf("method %d = %q, want %q", i, method, want[i])
		}
	}
}

func TestPortForwardActionsRejectNonUUIDBindingBeforeCLI(t *testing.T) {
	server, device := "https://example.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: &server, DeviceID: &device}
	create := remote.PortForwardCreateInput{
		SchemaVersion: 1, Expected: expected, SessionID: "11111111-1111-4111-8111-111111111111",
		ExpectedNodeID: "node-1", RemotePort: 4173, LocalPort: 4173, ClientInstanceID: "client-1", Confirmed: true,
	}
	if _, err := remote.CreatePortForward(context.Background(), cliadapter.Client{Home: expected.Home}, create); err != fault.PortForwardInputInvalid {
		t.Fatalf("non-UUID create node was accepted: %v", err)
	}
	stop := remote.PortForwardActionInput{
		SchemaVersion: 1, Expected: expected, ForwardID: "11111111-1111-4111-8111-111111111111",
		ExpectedSessionID: "session-1", ExpectedNodeID: "33333333-3333-4333-8333-333333333333", Confirmed: true,
	}
	if _, err := remote.StopPortForward(context.Background(), cliadapter.Client{Home: expected.Home}, stop); err != fault.PortForwardInputInvalid {
		t.Fatalf("non-UUID stop session was accepted: %v", err)
	}
}
