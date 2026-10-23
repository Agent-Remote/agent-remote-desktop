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
	deleteSessionOne = "11111111-1111-4111-8111-111111111111"
	deleteSessionTwo = "22222222-2222-4222-8222-222222222222"
	deleteRequestID  = "33333333-3333-4333-8333-333333333333"
)

func TestSessionDeleteCapabilityRequiresExactWriteAndReadMethods(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	caps := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform, Operations: []cliadapter.Capability{
		{Method: remote.SessionDeleteMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"},
		{Method: remote.SessionDeleteRequestMethod, Effect: "read", Credential: "device"},
	}}
	if !remote.SessionDeleteSupported(caps) || !remote.SessionDeleteQuerySupported(caps) {
		t.Fatal("valid delete capabilities were rejected")
	}
	caps.Operations[0].Effect = "read"
	if remote.SessionDeleteSupported(caps) {
		t.Fatal("read-only delete was accepted as a mutation")
	}
}

func TestSessionDeleteDigestsMatchCanonicalCollectionContract(t *testing.T) {
	selection := remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"failed", "interrupted", "stopped"}}
	ids := []string{deleteSessionOne, deleteSessionTwo}
	gotSelection := remote.SessionDeleteSelectionDigest("claude", ids, selection)
	gotIntent := remote.SessionDeleteIntentFingerprint("claude", ids, selection)
	if gotSelection != "sha256:00653ec4fc0f7180e59a760ffdf9110bf329945e444fd8ca11f64056eb00f9f7" {
		t.Fatalf("selection digest %q", gotSelection)
	}
	if gotIntent != "sha256:59fd2c96e5f1bae0db03eb3eb48581abe856f7fabdd1ee0d135f6973909c1437" {
		t.Fatalf("intent fingerprint %q", gotIntent)
	}
}

func TestSessionDeleteReceiptKeepsCollectionAndRetentionSeparate(t *testing.T) {
	server, device := "https://example.invalid", "device-1"
	task := "delete-task"
	contexts := []remote.SessionDeleteSessionContext{
		{SessionID: deleteSessionOne, ToolAccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", WorkspaceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", ProjectKey: "project-1", NodeID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", RuntimeBackend: "native"},
		{SessionID: deleteSessionTwo, ToolAccountID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", WorkspaceID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd", ProjectKey: "project-2", NodeID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", RuntimeBackend: "native"},
	}
	receipt := remote.SessionDeleteReceipt{
		SchemaVersion: 1, RequestID: deleteRequestID, Context: cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: &server, DeviceID: &device},
		ToolType: "claude", SessionIDs: []string{deleteSessionOne, deleteSessionTwo},
		Selection:         remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"failed", "interrupted", "stopped"}},
		SelectionDigest:   remote.SessionDeleteSelectionDigest("claude", []string{deleteSessionOne, deleteSessionTwo}, remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"failed", "interrupted", "stopped"}}),
		IntentFingerprint: remote.SessionDeleteIntentFingerprint("claude", []string{deleteSessionOne, deleteSessionTwo}, remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"failed", "interrupted", "stopped"}}),
		Reason:            "user_requested",
		Phase:             "blocked", DeleteTaskID: &task,
		DeletedSessionIDs: []string{}, SessionStatuses: map[string]string{deleteSessionOne: "stopped", deleteSessionTwo: "saving"},
		SessionContexts:    contexts,
		CollectionMismatch: false, RetentionBlockers: []string{"skill_snapshot_pending"},
	}
	if !receipt.Valid() {
		t.Fatal("blocked receipt with retention evidence was rejected")
	}
	bad := receipt
	bad.DeletedSessionIDs = []string{deleteSessionOne, "44444444-4444-4444-8444-444444444444"}
	if bad.Valid() {
		t.Fatal("receipt expanded the frozen collection")
	}
}

func TestSessionDeleteUnknownReceiptMayOmitRuntimeContexts(t *testing.T) {
	selection := remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"failed", "interrupted", "stopped"}}
	receipt := remote.SessionDeleteReceipt{
		SchemaVersion: 1, RequestID: deleteRequestID, ToolType: "claude", SessionIDs: []string{deleteSessionOne}, Selection: selection,
		SelectionDigest: remote.SessionDeleteSelectionDigest("claude", []string{deleteSessionOne}, selection), IntentFingerprint: remote.SessionDeleteIntentFingerprint("claude", []string{deleteSessionOne}, selection),
		Reason: "user_requested",
		Phase:  "unknown", DeletedSessionIDs: []string{}, SessionStatuses: map[string]string{},
	}
	if !receipt.Valid() {
		t.Fatal("unknown receipt without a pre-acceptance runtime projection was rejected")
	}
}

func TestSessionDeleteRejectsNonClaudeToolBeforeCLIInvocation(t *testing.T) {
	server, device := "https://example.invalid", "device-1"
	_, err := remote.DeleteSessions(context.Background(), cliadapter.Client{Home: "/tmp/home"}, remote.SessionDeleteInput{
		SchemaVersion: 1,
		Expected:      cliadapter.ConfigContext{Home: "/tmp/home", ServerURL: &server, DeviceID: &device},
		ToolType:      "codex",
		SessionIDs:    []string{deleteSessionOne},
		Selection:     remote.SessionDeleteSelection{ToolType: "codex", Statuses: []string{"stopped"}},
		Confirmed:     true,
	})
	if err != fault.SessionDeleteInputInvalid {
		t.Fatalf("non-Claude delete error = %v, want %v", err, fault.SessionDeleteInputInvalid)
	}
}
