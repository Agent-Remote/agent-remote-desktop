package remote_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func deviceCapabilities() cliadapter.Capabilities {
	return cliadapter.Capabilities{
		Version:    cliadapter.ProtocolVersion,
		CLIVersion: "test",
		Platform:   runtime.GOOS,
		Operations: []cliadapter.Capability{
			{Method: "device.status", Effect: "read", Credential: "device"},
			{Method: "device.revoke", Effect: "write", Credential: "user", Confirmation: "exact_request"},
			{Method: "device.rotate_token", Effect: "write", Credential: "user", Confirmation: "exact_request"},
		},
	}
}

func TestDeviceCapabilitiesRequireExactCredentialContracts(t *testing.T) {
	caps := deviceCapabilities()
	for _, method := range []string{"device.status", "device.revoke", "device.rotate_token"} {
		if !remote.DeviceSupported(caps, method) {
			t.Fatalf("expected %s to be supported", method)
		}
	}
	caps.Operations[1].Credential = "device"
	if remote.DeviceSupported(caps, "device.revoke") {
		t.Fatal("revoke must require the user credential")
	}
	caps = deviceCapabilities()
	caps.Operations[2].Confirmation = ""
	if remote.DeviceSupported(caps, "device.rotate_token") {
		t.Fatal("token rotation must require exact confirmation")
	}
}

func TestDeviceMutationRejectsUnconfirmedOrStaleRequestsBeforeCLI(t *testing.T) {
	client := cliadapter.Client{Binary: "/does/not/exist", Home: "/tmp/agent-remote-device-test"}
	server := "https://server.example"
	device := "device-test"
	ctx := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}

	for name, input := range map[string]remote.DeviceActionInput{
		"unconfirmed":  {Expected: ctx, DeviceID: device},
		"stale device": {Confirmed: true, Expected: cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: deviceStrptr("other-device")}, DeviceID: device},
		"empty id":     {Confirmed: true, Expected: ctx},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := remote.RevokeDevice(context.Background(), client, input)
			if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("expected protocol rejection before process launch, got %v", err)
			}
		})
	}
}

func deviceStrptr(value string) *string { return &value }
