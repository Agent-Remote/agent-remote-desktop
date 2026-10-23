package cliadapter_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

const recoveryInteropAccount = "11111111-1111-4111-8111-111111111111"
const recoveryInteropActor = "22222222-2222-4222-8222-222222222222"
const recoveryInteropOwner = "33333333-3333-4333-8333-333333333333"
const recoveryInteropKey = "66666666-6666-4666-8666-666666666666"
const recoveryInteropOriginalTask = "migrate_tool_account_runtime:" + recoveryInteropAccount + ":77777777-7777-4777-8777-777777777777"

// This HTTP fixture exercises the real Rust credentials, shared service and
// caller-key registry through the production Go adapter. It is not a deployed
// Server/Node execution test.
type recoveryInteropServer struct {
	mu              sync.Mutex
	server          *httptest.Server
	admin           bool
	lostResponse    bool
	corruptResponse bool
	reviewChanged   bool
	posts           int
	authCalls       int
	records         map[string]map[string]any
}

func recoveryInteropOriginal() map[string]any {
	return map[string]any{
		"task_id": recoveryInteropOriginalTask, "task_record_id": "44444444-4444-4444-8444-444444444444",
		"user_id": recoveryInteropOwner, "tool_account_id": recoveryInteropAccount,
		"node_id": "55555555-5555-4555-8555-555555555555", "tool_type": "claude",
		"source_runtime_backend": "docker_sandbox", "target_runtime_backend": "native",
	}
}

func recoveryInteropReceipt(key, action string) map[string]any {
	version := 1
	if action == "verify_source" {
		version = 2
	} else if action == "repair_source" {
		version = 3
	}
	binding := map[string]any{
		"version": version, "task_id": "recover_tool_account_runtime:" + recoveryInteropAccount + ":" + key,
		"task_record_id": "88888888-8888-4888-8888-888888888888", "original_task_id": recoveryInteropOriginalTask,
		"original_task_record_id": "44444444-4444-4444-8444-444444444444", "user_id": recoveryInteropOwner,
		"tool_account_id": recoveryInteropAccount, "node_id": "55555555-5555-4555-8555-555555555555",
		"tool_type": "claude", "source_runtime_backend": "docker_sandbox", "target_runtime_backend": "native",
	}
	if version != 1 {
		binding["action"] = action
	}
	return map[string]any{"schema_version": 1, "request_id": key, "action": action, "binding": binding, "status": "succeeded"}
}

func newRecoveryInteropServer(t *testing.T) *recoveryInteropServer {
	t.Helper()
	f := &recoveryInteropServer{admin: true, records: map[string]map[string]any{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var result any
		if strings.HasPrefix(r.URL.Path, "/api/v1/auth/") {
			f.authCalls++
		} else if r.Header.Get("Authorization") != "Bearer recovery-fixture-user-session" {
			t.Error("administrator operation did not use the isolated user credential")
			w.WriteHeader(http.StatusForbidden)
			return
		}
		prefix := "/api/v1/tool-accounts/" + recoveryInteropAccount + "/runtime-migration/"
		if strings.HasPrefix(r.URL.Path, prefix) && !f.admin {
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "COMMON_FORBIDDEN", "message": "Administrator role required"}})
			return
		}
		switch {
		case r.Method == "POST" && r.URL.Path == "/api/v1/auth/login":
			result = map[string]any{"access_token": "recovery-fixture-login", "expires_in": 3600}
		case r.Method == "POST" && r.URL.Path == "/api/v1/auth/cli/session":
			if r.Header.Get("Authorization") != "Bearer recovery-fixture-login" {
				t.Error("session creation did not use its original login")
			}
			result = map[string]any{"access_token": "recovery-fixture-user-session", "expires_in": 3600, "refresh_token": "recovery-fixture-refresh", "refresh_expires_in": 2592000}
		case r.Method == "GET" && r.URL.Path == "/api/v1/runtime-recovery/authority":
			result = map[string]any{"schema_version": 1, "actor_user_id": recoveryInteropActor, "admin": f.admin}
		case r.Method == "GET" && r.URL.Path == prefix+"recovery-inspection":
			if r.URL.Query().Get("original_task_id") != recoveryInteropOriginalTask {
				t.Error("inspection did not select the exact original task")
			}
			result = map[string]any{"schema_version": 1, "original": recoveryInteropOriginal(), "original_status": "failed", "account_status": "failed"}
		case r.Method == "POST" && r.URL.Path == prefix+"recovery-requests":
			var input map[string]any
			if json.NewDecoder(r.Body).Decode(&input) != nil || len(input) != 4 || input["schema_version"] != float64(1) || !reflect.DeepEqual(input["expected_original"], recoveryInteropOriginal()) {
				t.Error("submission did not bind the complete reviewed original projection")
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			key, keyOK := input["request_id"].(string)
			action, actionOK := input["action"].(string)
			if !keyOK || !actionOK || (action != "verify_target" && action != "verify_source" && action != "repair_source") {
				t.Error("missing explicit recovery key/action")
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			f.posts++
			if f.reviewChanged {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "RUNTIME_MIGRATION_RECOVERY_REVIEW_CHANGED", "message": "review changed"}})
				return
			}
			if _, exists := f.records[key]; exists {
				t.Error("CLI replayed a retained recovery POST")
			}
			result = recoveryInteropReceipt(key, action)
			f.records[key] = result.(map[string]any)
			if f.lostResponse {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "UNAVAILABLE", "message": "private-node-error-must-not-reach-desktop"}})
				return
			}
			if f.corruptResponse {
				result = recoveryInteropReceipt(key, action)
				result.(map[string]any)["binding"].(map[string]any)["original_task_record_id"] = "99999999-9999-4999-8999-999999999999"
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, prefix+"recovery-requests/"):
			key := strings.TrimPrefix(r.URL.Path, prefix+"recovery-requests/")
			var ok bool
			result, ok = f.records[key]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "COMMON_NOT_FOUND", "message": "missing"}})
				return
			}
		default:
			t.Errorf("unexpected recovery endpoint: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": result, "request_id": "fixture-http-trace"})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func loginRecoveryInterop(t *testing.T, ctx context.Context, cli string, f *recoveryInteropServer) cliadapter.Client {
	t.Helper()
	client := cliadapter.Client{Binary: cli, Home: t.TempDir()}
	receipt, err := remote.PasswordLogin(ctx, client, remote.PasswordLoginInput{Confirmed: true, ServerURL: f.server.URL, Username: "fixture-admin", Password: "fixture-password", SkipDevice: true})
	if err != nil || receipt.DeviceID != nil {
		t.Fatalf("isolated user-only login failed: %v", err)
	}
	return client
}

func recoveryInteropFiles(t *testing.T, home string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(home, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(home, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[relative] = "directory"
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unexpected fixture entry %q", relative)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[relative] = fmt.Sprintf("sha256:%x", sha256.Sum256(content))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestRuntimeRecoveryWithBuiltRustCLIUsesUserAuthorityAndOriginalRequest(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real administrator runtime recovery interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	for _, action := range []string{"verify_target", "verify_source", "repair_source"} {
		for _, lost := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/lost=%t", action, lost), func(t *testing.T) {
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				f := newRecoveryInteropServer(t)
				client := loginRecoveryInterop(t, ctx, cli, f)
				before := recoveryInteropFiles(t, client.Home)
				authority, err := remote.GetRuntimeRecoveryAuthority(ctx, client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: f.server.URL})
				if err != nil || !authority.Admin || authority.Context.ActorUserID != recoveryInteropActor {
					t.Fatalf("user authority: %+v %v", authority, err)
				}
				plan, err := remote.InspectRuntimeRecovery(ctx, client, remote.RuntimeRecoveryInspectInput{Expected: authority.Context, AccountID: recoveryInteropAccount, OriginalTaskID: recoveryInteropOriginalTask, Action: action})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before, recoveryInteropFiles(t, client.Home)) {
					t.Fatal("authority/inspection changed local credentials or metadata")
				}
				input := remote.RuntimeRecoveryApplyInput{ExpectedPlan: plan, RequestID: recoveryInteropKey}
				if _, err := remote.ApplyRuntimeRecovery(ctx, client, input); err == nil {
					t.Fatal("unconfirmed recovery accepted")
				}
				f.mu.Lock()
				f.lostResponse = lost
				f.mu.Unlock()
				input.Confirmed = true
				receipt, err := remote.ApplyRuntimeRecovery(ctx, client, input)
				if lost && err == nil || !lost && err != nil {
					t.Fatalf("recovery submission lost=%t: %+v %v", lost, receipt, err)
				}
				if err != nil && strings.Contains(err.Error(), "private-node-error") {
					t.Fatal("raw remote error escaped")
				}
				// Replace the Go client; every call starts a fresh Rust process.
				client = cliadapter.Client{Binary: cli, Home: client.Home}
				pending, err := remote.ListRuntimeRecoveryPending(ctx, client)
				if err != nil || len(pending) != 1 || pending[0].RequestID != recoveryInteropKey {
					t.Fatalf("original key was not retained: %+v %v", pending, err)
				}
				ref := pending[0]
				status, err := remote.GetRuntimeRecoveryStatus(ctx, client, remote.RuntimeRecoveryStatusInput{Expected: ref.Context, AccountID: ref.AccountID, RequestID: ref.RequestID, ExpectedDigest: &ref.Digest, ExpectedBinding: ref.Binding})
				if err != nil || status.Recovery.Status != "succeeded" || status.TargetCompletionConfirmed != (action == "verify_target") || status.SourceRestorationConfirmed != (action != "verify_target") {
					t.Fatalf("original request recovery: %+v %v", status, err)
				}
				if _, err := remote.ForgetRuntimeRecovery(ctx, client, remote.RuntimeRecoveryForgetInput{Confirmed: true, Expected: ref.Context, AccountID: ref.AccountID, RequestID: ref.RequestID, ExpectedDigest: &ref.Digest, ExpectedBinding: &status.Recovery.Binding}); err != nil {
					t.Fatal(err)
				}
				pending, err = remote.ListRuntimeRecoveryPending(ctx, client)
				if err != nil || len(pending) != 0 {
					t.Fatalf("settled local receipt remained: %+v %v", pending, err)
				}
				f.mu.Lock()
				defer f.mu.Unlock()
				if f.posts != 1 || f.authCalls != 2 {
					t.Fatalf("replayed recovery or refreshed user credentials: posts=%d auth=%d", f.posts, f.authCalls)
				}
			})
		}
	}
}
