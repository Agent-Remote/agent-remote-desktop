package cliadapter_test

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

// The Go adapter talks to the real Rust CLI and its real SQLite store. HTTP and
// Mutagen are isolated fixtures; the separate native probe verifies actual files.
func TestSyncRecoveryWithBuiltRustCLIRetainsExactSharedState(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" || runtime.GOOS == "windows" {
		t.Skip("requires DESKTOP_CLI_BINARY and the Unix Mutagen fixture")
	}
	for _, scenario := range []string{"retain-workspace", "remove-workspace", "session-absent", "changed-target", "unknown-termination", "daemon-unavailable"} {
		t.Run(scenario, func(t *testing.T) { runSyncRecoveryInterop(t, cli, scenario) })
	}
}

func runSyncRecoveryInterop(t *testing.T, cli, scenario string) {
	t.Helper()
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "project")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	client := cliadapter.Client{Binary: cli, Home: filepath.Join(root, "home")}
	dataDir := filepath.Join(root, "mutagen-data")
	t.Setenv("MUTAGEN_DATA_DIRECTORY", dataDir)
	const workspaceID = "11111111-1111-4111-8111-111111111111"
	const syncID = "22222222-2222-4222-8222-222222222222"
	const endpoint = "agent-remote@fixture.invalid:22:/workspace/fixture"
	var mu sync.Mutex
	var workspace, details map[string]any
	gone, writes := false, 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var result any
		if strings.Contains(r.URL.Path, "workspaces") || strings.Contains(r.URL.Path, "sync-sessions") {
			if r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("wrong recovery credential")
			}
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/auth/login":
			result = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
		case "POST /api/v1/auth/cli/session":
			result = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
		case "POST /api/v1/devices/register":
			result = map[string]any{"device": map[string]any{"id": "device-test", "name": "Recovery fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-18T00:00:00Z"}, "device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": "key-test", "wireguard_peer_id": nil}
		case "POST /api/v1/workspaces":
			writes++
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				t.Error("invalid workspace request")
			}
			workspace = map[string]any{"id": workspaceID, "user_id": "fixture-user", "device_id": "device-test", "project_key": input["project_key"], "local_start_path": input["local_start_path"], "display_name": input["display_name"], "remote_path": "/workspace/fixture", "sync_git": true, "created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T00:00:00Z"}
			result = workspace
		case "POST /api/v1/sync-sessions":
			writes++
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				t.Error("invalid sync request")
			}
			details = map[string]any{"id": syncID, "user_id": "fixture-user", "workspace_id": workspaceID, "node_id": "33333333-3333-4333-8333-333333333333", "local_path": input["local_path"], "remote_path": "/workspace/fixture", "status": "active", "conflict_status": "none", "sync_mode": input["sync_mode"], "sync_git": input["sync_git"], "exclude": input["exclude"], "mutagen_session_id": "agent-remote-recovery-fixture", "remote_endpoint": endpoint, "target_precondition_version": 1, "target_precondition_checked": true, "prepare_task_id": nil, "created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T00:00:00Z"}
			result = details
		case "GET /api/v1/sync-sessions/" + syncID:
			if gone {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			result = details
		case "GET /api/v1/workspaces/" + workspaceID:
			if gone && scenario == "remove-workspace" {
				w.WriteHeader(http.StatusNotFound)
				return
			}
			result = workspace
		default:
			t.Errorf("unexpected recovery endpoint: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": result})
	}))
	defer server.Close()
	key := binary.BigEndian.AppendUint32(nil, 11)
	key = append(key, "ssh-ed25519"...)
	key = binary.BigEndian.AppendUint32(key, 32)
	key = append(key, []byte(strings.Repeat("k", 32))...)
	keyPath := filepath.Join(root, "fixture.pub")
	if err := os.WriteFile(keyPath, []byte("ssh-ed25519 "+base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
		t.Fatal(err)
	}
	name := "Recovery fixture"
	if _, err := remote.PasswordLogin(ctx, client, remote.PasswordLoginInput{Confirmed: true, ServerURL: server.URL, Username: "fixture-user", Password: "fixture-password", DeviceName: &name, SSHKey: &keyPath}); err != nil {
		t.Fatal(err)
	}
	var config cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &config); err != nil {
		t.Fatal(err)
	}
	query := remote.SyncStatusInput{Expected: config, LocalPath: path}
	initial, err := remote.GetSyncStatus(ctx, client, query)
	if err != nil {
		t.Fatal(err)
	}
	created, err := remote.RunSyncAction(ctx, client, "sync.ensure", remote.SyncActionInput{Confirmed: true, Expected: config, ExpectedTarget: initial.Target, DryRun: true})
	if err != nil || created.Outcome != "completed" {
		t.Fatalf("setup: %+v %v", created, err)
	}
	mu.Lock()
	gone = true
	mu.Unlock()
	for _, dir := range []string{filepath.Join(client.Home, "bin"), filepath.Join(dataDir, "daemon")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	script := `#!/bin/sh
test "$MUTAGEN_DISABLE_AUTOSTART" = 1 || exit 8
printf '%s\n' "$@" >> "$AGENT_REMOTE_HOME/mutagen-calls"
test ! -e "$AGENT_REMOTE_HOME/unavailable" || exit 9
if test "$1" = sync && test "$2" = list; then
  test ! -e "$AGENT_REMOTE_HOME/session.json" || cat "$AGENT_REMOTE_HOME/session.json"
elif test "$1" = sync && test "$2" = terminate && test "$3" = -- && test "$4" = sync_RecoveryNativeFixture; then
  test ! -e "$AGENT_REMOTE_HOME/unknown" || exit 10
  rm "$AGENT_REMOTE_HOME/session.json"
else
  exit 11
fi
`
	if err := os.WriteFile(filepath.Join(client.Home, "bin", "mutagen"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	model := map[string]any{"identifier": "sync_RecoveryNativeFixture", "name": "agent-remote-recovery-fixture", "mode": "two-way-safe", "paused": true, "status": "disconnected", "alpha": map[string]any{"protocol": "local", "path": path}, "beta": map[string]any{"protocol": "ssh", "user": "agent-remote", "host": "fixture.invalid", "port": 22, "path": "/workspace/fixture"}}
	writeModel := func() {
		t.Helper()
		encoded, err := json.Marshal(model)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(client.Home, "session.json"), encoded, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if scenario != "session-absent" {
		writeModel()
	}
	if scenario == "daemon-unavailable" {
		if err := os.WriteFile(filepath.Join(client.Home, "unavailable"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	plan, err := remote.PreviewSyncRecovery(ctx, client, remote.SyncRecoveryPreviewInput{Expected: config, LocalPath: path})
	if err != nil {
		t.Fatal(err)
	}
	if scenario == "daemon-unavailable" {
		if plan.Recoverable || plan.Mutagen.State != "unavailable" {
			t.Fatalf("false absence: %+v", plan)
		}
		return
	}
	if !plan.Recoverable || plan.Mapping.Sync == nil || plan.Target.SyncSessionID == nil || *plan.Target.SyncSessionID != syncID {
		t.Fatalf("bad recovery: %+v", plan)
	}
	calls, err := os.ReadFile(filepath.Join(client.Home, "mutagen-calls"))
	if err != nil || strings.Contains(string(calls), "terminate") {
		t.Fatalf("preview mutated: %s %v", calls, err)
	}
	if scenario == "changed-target" {
		model["mode"] = "one-way-replica"
		writeModel()
	}
	if scenario == "unknown-termination" {
		if err := os.WriteFile(filepath.Join(client.Home, "unknown"), nil, 0600); err != nil {
			t.Fatal(err)
		}
	}
	receipt, err := remote.ApplySyncRecovery(ctx, client, remote.SyncRecoveryApplyInput{Confirmed: true, Expected: config, ExpectedPlan: plan})
	if err != nil {
		t.Fatal(err)
	}
	if scenario == "changed-target" || scenario == "unknown-termination" {
		want := "partial"
		if scenario == "unknown-termination" {
			want = "unknown"
		}
		if receipt.Outcome != want || receipt.ResultTarget.SyncSessionID == nil {
			t.Fatalf("lost uncertain state: %+v", receipt)
		}
		_, err := remote.GetSyncStatus(ctx, client, query)
		var op *cliadapter.OperationError
		if !errors.As(err, &op) || op.Code != "sync_not_found" {
			t.Fatalf("old mapping lost: %v", err)
		}
	} else {
		if receipt.Outcome != "completed" || receipt.ResultTarget.SyncSessionID != nil {
			t.Fatalf("cleanup: %+v", receipt)
		}
		status, err := remote.GetSyncStatus(ctx, client, query)
		if err != nil || status.Target.SyncSessionID != nil || (status.Target.WorkspaceID == nil) != (scenario == "remove-workspace") {
			t.Fatalf("shared metadata: %+v %v", status, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if writes != 2 {
		t.Fatalf("recovery replayed remote writes: %d", writes)
	}
}
