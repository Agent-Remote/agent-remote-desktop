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

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestSyncWithBuiltRustCLITracksSharedTargetAndPartialPhases(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real synchronization interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	ctx := context.Background()
	client := cliadapter.Client{Binary: cli, Home: filepath.Join(t.TempDir(), "home")}
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	const workspaceID = "11111111-1111-4111-8111-111111111111"
	const syncID = "22222222-2222-4222-8222-222222222222"
	const nodeID = "33333333-3333-4333-8333-333333333333"
	var mu sync.Mutex
	var workspace map[string]any
	var details map[string]any
	var writes []string
	drift, unknown := false, false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var data any
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/auth/login":
			data = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
		case "POST /api/v1/auth/cli/session":
			data = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
		case "POST /api/v1/devices/register":
			data = map[string]any{"device": map[string]any{"id": "device-test", "name": "Sync fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-18T00:00:00Z"}, "device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": "key-test", "wireguard_peer_id": nil}
		case "POST /api/v1/workspaces":
			writes = append(writes, r.URL.Path)
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				t.Error("workspace request")
			}
			workspace = map[string]any{"id": workspaceID, "user_id": "fixture-user", "device_id": "device-test", "project_key": input["project_key"], "local_start_path": input["local_start_path"], "display_name": input["display_name"], "remote_path": "/workspace/fixture", "sync_git": true, "created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T00:00:00Z"}
			data = workspace
		case "POST /api/v1/sync-sessions":
			writes = append(writes, r.URL.Path)
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil {
				t.Error("sync request")
			}
			details = map[string]any{"id": syncID, "user_id": "fixture-user", "workspace_id": workspaceID, "node_id": nodeID, "local_path": input["local_path"], "remote_path": "/workspace/fixture", "status": "active", "conflict_status": "none", "sync_mode": input["sync_mode"], "sync_git": input["sync_git"], "exclude": input["exclude"], "mutagen_session_id": "sync_22222222222242228222222222222222", "remote_endpoint": "agent@fixture.invalid:/workspace/fixture", "target_precondition_version": 1, "target_precondition_checked": true, "prepare_task_id": nil, "created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T00:00:00Z"}
			data = details
		case "GET /api/v1/sync-sessions/" + syncID:
			if details == nil {
				http.NotFound(w, r)
				return
			}
			clone := map[string]any{}
			for k, v := range details {
				clone[k] = v
			}
			if drift {
				clone["remote_endpoint"] = "agent@changed.invalid:/workspace/fixture"
			}
			data = clone
		default:
			prefix := "/api/v1/sync-sessions/" + syncID + "/"
			if r.Method != "POST" || !strings.HasPrefix(r.URL.Path, prefix) {
				t.Errorf("unexpected endpoint %s %s", r.Method, r.URL.Path)
				http.NotFound(w, r)
				return
			}
			writes = append(writes, r.URL.Path)
			if unknown {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"detail":"private-sync-secret"}`))
				return
			}
			var actionInput map[string]any
			if json.NewDecoder(r.Body).Decode(&actionInput) != nil {
				t.Error("missing action request")
			}
			expectedTarget, ok := actionInput["expected_target"].(map[string]any)
			if !ok || expectedTarget["id"] != syncID || expectedTarget["remote_endpoint"] != details["remote_endpoint"] {
				t.Error("missing atomic target precondition")
			}
			action := strings.TrimPrefix(r.URL.Path, prefix)
			if action == "pause" {
				details["status"] = "paused"
			} else {
				details["status"] = "active"
			}
			data = details
		}
		if strings.Contains(r.URL.Path, "sync-sessions") || strings.Contains(r.URL.Path, "workspaces") {
			if r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("wrong sync credential")
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	key := binary.BigEndian.AppendUint32(nil, 11)
	key = append(key, "ssh-ed25519"...)
	key = binary.BigEndian.AppendUint32(key, 32)
	key = append(key, []byte(strings.Repeat("k", 32))...)
	keyPath := filepath.Join(t.TempDir(), "fixture.pub")
	if err := os.WriteFile(keyPath, []byte("ssh-ed25519 "+base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
		t.Fatal(err)
	}
	name := "Sync fixture"
	if _, err := remote.PasswordLogin(ctx, client, remote.PasswordLoginInput{Confirmed: true, ServerURL: server.URL, Username: "fixture-user", Password: "fixture-password", DeviceName: &name, SSHKey: &keyPath}); err != nil {
		t.Fatal(err)
	}
	var config cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &config); err != nil {
		t.Fatal(err)
	}
	query := remote.SyncStatusInput{Expected: config, LocalPath: path}
	initial, err := remote.GetSyncStatus(ctx, client, query)
	if err != nil || initial.Sync != nil || initial.Target.WorkspaceID != nil {
		t.Fatalf("initial %+v %v", initial, err)
	}
	input := remote.SyncActionInput{Confirmed: true, Expected: config, ExpectedTarget: initial.Target, DryRun: true}
	receipt, err := remote.RunSyncAction(ctx, client, "sync.ensure", input)
	if err != nil || receipt.Outcome != "completed" || receipt.ResultTarget.SyncSessionID == nil {
		t.Fatalf("ensure %+v %v", receipt, err)
	}
	if receipt.Mutagen != nil {
		t.Fatalf("dry-run falsely observed local result %+v", receipt)
	}
	mu.Lock()
	count := len(writes)
	mu.Unlock()
	if count != 2 {
		t.Fatalf("dry-run should create Server resources: %d", count)
	}
	current, err := remote.GetSyncStatus(ctx, client, query)
	if err != nil || current.Target.SyncSessionID == nil || *current.Target.SyncSessionID != syncID {
		t.Fatalf("shared mapping %+v %v", current, err)
	}
	input.ExpectedTarget = current.Target
	for _, action := range []string{"pause", "resume", "resolve", "reset"} {
		receipt, err = remote.RunSyncAction(ctx, client, "sync."+action, input)
		if err != nil || receipt.Outcome != "completed" {
			t.Fatalf("dry-run %s %+v %v", action, receipt, err)
		}
		skipped := false
		for _, stage := range receipt.Stages {
			if stage.Name == "mutagen" && stage.State == "skipped" {
				skipped = true
			}
		}
		if !skipped {
			t.Fatal("local skipped stage lost")
		}
	}
	mu.Lock()
	drift = true
	count = len(writes)
	mu.Unlock()
	_, err = remote.RunSyncAction(ctx, client, "sync.reset", input)
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "sync_target_changed" {
		t.Fatalf("target drift %v", err)
	}
	mu.Lock()
	if len(writes) != count {
		t.Error("target drift mutated")
	}
	drift = false
	mu.Unlock()
	wrong := input
	serverURL := "https://wrong.invalid"
	wrong.Expected.ServerURL = &serverURL
	_, err = remote.RunSyncAction(ctx, client, "sync.reset", wrong)
	if !errors.As(err, &operation) || operation.Code != "configuration_changed" {
		t.Fatalf("context drift %v", err)
	}
	mu.Lock()
	unknown = true
	count = len(writes)
	mu.Unlock()
	receipt, err = remote.RunSyncAction(ctx, client, "sync.reset", input)
	if err != nil || receipt.Outcome != "unknown" || receipt.ErrorCode == nil || *receipt.ErrorCode != "sync_outcome_unknown" {
		t.Fatalf("unknown receipt %+v %v", receipt, err)
	}
	mu.Lock()
	if len(writes) != count+1 {
		t.Error("unknown mutation replayed")
	}
	mu.Unlock()
	encoded, _ := json.Marshal(receipt)
	if strings.Contains(string(encoded), "private-sync-secret") || strings.Contains(string(encoded), "fixture-device-token") {
		t.Fatal("secrets exposed")
	}
}
