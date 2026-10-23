package remote_test

import (
	"context"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

const (
	stopSessionID = "11111111-1111-4111-8111-111111111111"
	stopNodeID    = "22222222-2222-4222-8222-222222222222"
)

func TestSessionStopCapabilityRequiresWriteAndReadEffects(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	caps := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform, Operations: []cliadapter.Capability{
		{Method: remote.SessionStopMethod, Effect: "read"},
		{Method: remote.SessionStopRequestMethod, Effect: "write"},
	}}
	if remote.SessionStopSupported(caps) || remote.SessionStopQuerySupported(caps) {
		t.Fatal("wrong stop operation effects were accepted")
	}
	caps.Operations = []cliadapter.Capability{
		{Method: remote.SessionStopMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"},
		{Method: remote.SessionStopRequestMethod, Effect: "read", Credential: "device"},
	}
	if !remote.SessionStopSupported(caps) || !remote.SessionStopQuerySupported(caps) {
		t.Fatal("valid stop operation effects were not accepted")
	}
}

func TestSessionStopCapabilitiesRequireCredentialAndConfirmation(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	caps := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform, Operations: []cliadapter.Capability{
		{Method: remote.SessionStopMethod, Effect: "write", Credential: "user", Confirmation: "exact_request"},
		{Method: remote.SessionStopRequestMethod, Effect: "read", Credential: "device", Confirmation: "prompt"},
	}}
	if remote.SessionStopSupported(caps) || remote.SessionStopQuerySupported(caps) {
		t.Fatal("stop capability accepted wrong credential or confirmation")
	}
}

func TestSessionStopRejectsIncompleteReviewedRuntimeIdentity(t *testing.T) {
	server, device := "https://example.invalid", "device-1"
	_, err := remote.StopSession(context.Background(), cliadapter.Client{Home: "/tmp/home"}, remote.SessionStopInput{
		SchemaVersion: 1,
		Expected:      cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: &server, DeviceID: &device},
		SessionID:     stopSessionID,
		Confirmed:     true,
	})
	if err != fault.SessionStopInputInvalid {
		t.Fatalf("incomplete stop identity error = %v, want %v", err, fault.SessionStopInputInvalid)
	}
}

func TestSessionStopReceiptKeepsProcessAndSaveStatesIndependent(t *testing.T) {
	for _, phase := range []string{"pending", "stopping", "stopped", "saving", "saved", "failed", "unresolved", "deleted"} {
		server, device := "https://example.invalid", "device-1"
		process, task := "stopped", "succeeded"
		receipt := remote.SessionStopReceipt{
			SchemaVersion: 1, RequestID: "11111111-1111-4111-8111-111111111111", Phase: phase,
			Context: cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: &server, DeviceID: &device}, SessionID: stopSessionID, NodeID: stopNodeID, RuntimeBackend: "native",
			StopTaskStatus: &task, SessionStatus: &process, IntentFingerprint: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: "user_requested",
		}
		if !receipt.Valid() {
			t.Fatalf("phase %q rejected", phase)
		}
	}
	server, device := "https://example.invalid", "device-1"
	unknown := remote.SessionStopReceipt{
		SchemaVersion: 1, RequestID: "11111111-1111-4111-8111-111111111111", Phase: "unknown",
		Context: cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: &server, DeviceID: &device}, SessionID: stopSessionID, NodeID: stopNodeID, RuntimeBackend: "native",
		SessionDeleted: false, ErrorCode: ptr("SESSION_STOP_SUBMISSION_UNKNOWN"),
	}
	if !unknown.Valid() {
		t.Fatal("unknown submission receipt rejected")
	}
}

func TestSessionStopDeletedRequiresDeletedProjection(t *testing.T) {
	receipt := remote.SessionStopReceipt{
		SchemaVersion: 1, RequestID: "11111111-1111-4111-8111-111111111111", Phase: "stopped",
		Context: cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: ptr("https://example.invalid"), DeviceID: ptr("device-1")}, SessionID: stopSessionID, NodeID: stopNodeID, RuntimeBackend: "native",
		StopTaskStatus: ptr("succeeded"), SessionStatus: ptr("stopped"), SessionDeleted: true,
		IntentFingerprint: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Reason: "user_requested",
	}
	if receipt.Valid() {
		t.Fatal("inconsistent deleted projection was accepted")
	}
}

func ptr(value string) *string { return &value }
