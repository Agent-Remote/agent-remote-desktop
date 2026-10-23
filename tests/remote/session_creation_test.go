package remote_test

import (
	"encoding/json"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestSessionCreateInputRejectsPrivateFieldsAndPreservesCanonicalShape(t *testing.T) {
	raw := []byte(`{"schema_version":1,"expected_context":{"home":"/tmp/home","server_url":"https://example.invalid","active_device_id":"device-1"},"tool_type":"claude","tool_account_id":"account-1","workspace_id":"workspace-1","project_key":"project-1","argv_digest":"sha256:` + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + `","sync_target":{},"confirmed":true}`)
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	if _, ok := wire["request_id"]; ok {
		t.Fatal("desktop must not supply a caller request ID")
	}
	if _, ok := wire["user_id"]; ok {
		t.Fatal("desktop must not supply an authenticated user ID")
	}
	if _, err := remote.DecodeSessionCreateInput(raw); err != nil {
		t.Fatalf("canonical request rejected: %v", err)
	}
	wire["user_id"] = "private"
	private, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := remote.DecodeSessionCreateInput(private); err == nil {
		t.Fatal("unknown private field was accepted")
	}
}

func TestSessionCreateReceiptAcceptsAllStablePhases(t *testing.T) {
	for _, phase := range []string{"prepared", "admission_pending", "accepted", "running", "failed", "unknown", "deleted"} {
		receipt := remote.SessionCreateReceipt{SchemaVersion: 1, RequestID: "11111111-1111-4111-8111-111111111111", Phase: phase}
		if !receipt.Valid() {
			t.Fatalf("phase %q rejected", phase)
		}
	}
}

func TestSessionCreateReceiptRejectsMismatchedSessionProjection(t *testing.T) {
	sessionID := "session-1"
	receipt := remote.SessionCreateReceipt{
		SchemaVersion: 1,
		RequestID:     "11111111-1111-4111-8111-111111111111",
		Phase:         "accepted",
		SessionID:     &sessionID,
		Session:       &cliadapter.RemoteSession{ID: "session-2"},
	}
	if receipt.Valid() {
		t.Fatal("receipt with a mismatched session projection was accepted")
	}
}

func TestSessionCreateCapabilityChecksRequireExactOperationEffect(t *testing.T) {
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	base := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform}
	if remote.SessionCreateSupported(base) || remote.SessionCreateQuerySupported(base) {
		t.Fatal("missing operations were reported as supported")
	}
	base.Operations = []cliadapter.Capability{
		{Method: remote.SessionCreateMethod, Effect: "read", Credential: "device"},
		{Method: remote.SessionCreateRequestMethod, Effect: "write", Credential: "device"},
	}
	if remote.SessionCreateSupported(base) || remote.SessionCreateQuerySupported(base) {
		t.Fatal("wrong operation effects were reported as supported")
	}
	base.Operations = []cliadapter.Capability{
		{Method: remote.SessionCreateMethod, Effect: "write", Credential: "device"},
		{Method: remote.SessionCreateRequestMethod, Effect: "read", Credential: "device"},
	}
	if remote.SessionCreateSupported(base) || !remote.SessionCreateQuerySupported(base) {
		t.Fatal("create capability accepted a mutation without exact confirmation")
	}
	base.Operations[0].Confirmation = "exact_request"
	if !remote.SessionCreateSupported(base) || !remote.SessionCreateQuerySupported(base) {
		t.Fatal("valid create capability was rejected")
	}
}

func TestSessionCreateSyncTargetRejectsAmbiguousOrIncompleteEvidence(t *testing.T) {
	valid := []byte(`{"id":"33333333-3333-4333-8333-333333333333","workspace_id":"22222222-2222-4222-8222-222222222222","node_id":"44444444-4444-4444-8444-444444444444","local_path":"/tmp/project","remote_path":"/workspace/project","mutagen_session_id":"agent-remote-33333333","target_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`)
	if !remote.SessionCreateSyncTargetValid(valid) {
		t.Fatal("complete sync target was rejected")
	}
	cases := map[string]string{
		"missing stable digest":  `{"id":"33333333-3333-4333-8333-333333333333","workspace_id":"22222222-2222-4222-8222-222222222222","local_path":"/tmp/project","remote_path":"/workspace/project","target_digest":"sha256:bad"}`,
		"wrong scalar type":      `{"id":"33333333-3333-4333-8333-333333333333","workspace_id":42,"local_path":"/tmp/project","remote_path":"/workspace/project"}`,
		"control character path": `{"id":"33333333-3333-4333-8333-333333333333","workspace_id":"22222222-2222-4222-8222-222222222222","local_path":"/tmp/project\nprivate","remote_path":"/workspace/project"}`,
		"unknown field":          `{"id":"33333333-3333-4333-8333-333333333333","workspace_id":"22222222-2222-4222-8222-222222222222","local_path":"/tmp/project","remote_path":"/workspace/project","argv":"secret"}`,
		"duplicate identity":     `{"id":"33333333-3333-4333-8333-333333333333","id":"55555555-5555-4555-8555-555555555555","workspace_id":"22222222-2222-4222-8222-222222222222","local_path":"/tmp/project","remote_path":"/workspace/project"}`,
	}
	for name, raw := range cases {
		t.Run(name, func(t *testing.T) {
			if remote.SessionCreateSyncTargetValid([]byte(raw)) {
				t.Fatalf("malformed sync target accepted: %s", raw)
			}
		})
	}
}
