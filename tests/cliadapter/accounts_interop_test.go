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
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestAccountsWithBuiltRustCLIPreservesExactIntentAndSafeReceipts(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for account lifecycle interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	ctx := context.Background()
	client := cliadapter.Client{Binary: cli, Home: filepath.Join(t.TempDir(), "home")}
	const id = "22222222-2222-4222-8222-222222222222"
	var mu sync.Mutex
	var calls []string
	state := "binding_requested"
	reject := false
	account := func() map[string]any {
		return map[string]any{
			"id": id, "user_id": "fixture-user", "tool_type": "claude", "display_name": "桌面账户",
			"status": state, "region_code": "SG", "timezone": "Asia/Singapore", "locale": "en_US.UTF-8",
			"preferred_node_tags": []string{"preferred:asia"}, "affinity_node_id": nil, "runtime_backend": "native",
			"created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T00:00:00Z",
			"private_field": "fixture-secret",
		}
	}
	binding := func() map[string]any {
		return map[string]any{
			"tool_account_id": id, "status": state, "node_id": nil,
			"binding_session_id": "bind-222222222222-0123456789ab", "tmux_session_name": "binding-test",
			"account_remote_path": "/accounts/fixture", "task_id": "binding:fixture:attempt",
			"connect_command": "never execute fixture-secret", "verifier": "fixture-secret", "error": nil,
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var data any
		if strings.HasPrefix(r.URL.Path, "/api/v1/tool-accounts") {
			calls = append(calls, r.Method+" "+r.URL.Path)
			if r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("wrong account authority")
			}
			if reject {
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte(`{"detail":"fixture-secret"}`))
				return
			}
		}
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v1/auth/login":
			data = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
		case "POST /api/v1/auth/cli/session":
			data = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
		case "POST /api/v1/devices/register":
			data = map[string]any{
				"device":       map[string]any{"id": "device-test", "name": "Desktop fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-18T00:00:00Z"},
				"device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": "key-test", "wireguard_peer_id": nil,
			}
		case "POST /api/v1/tool-accounts":
			var body map[string]any
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				t.Error("invalid create body")
			}
			want := map[string]any{"tool_type": "claude", "display_name": "桌面账户", "region_code": "SG", "timezone": "Asia/Singapore", "locale": "en_US.UTF-8", "preferred_node_tags": []any{"preferred:asia"}}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("create intent changed: %#v", body)
			}
			data = account()
		case "GET /api/v1/tool-accounts/" + id:
			data = account()
		case "GET /api/v1/tool-accounts/" + id + "/bind/status":
			data = binding()
		case "POST /api/v1/tool-accounts/" + id + "/bind/start":
			state = "binding_session_starting"
			data = binding()
		case "POST /api/v1/tool-accounts/" + id + "/bind/verify":
			state = "binding_verifying"
			data = binding()
		case "POST /api/v1/tool-accounts/" + id + "/disable":
			state = "disabled"
			data = account()
		default:
			t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
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
	name := "Desktop fixture"
	if _, err := remote.PasswordLogin(ctx, client, remote.PasswordLoginInput{Confirmed: true, ServerURL: server.URL, Username: "fixture-user", Password: "fixture-password", DeviceName: &name, SSHKey: &keyPath}); err != nil {
		t.Fatal(err)
	}
	var expected cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &expected); err != nil {
		t.Fatal(err)
	}
	created, err := remote.CreateAccount(ctx, client, remote.AccountCreateInput{Confirmed: true, Expected: expected, Tool: "claude", Name: "桌面账户", Region: "SG", Timezone: "Asia/Singapore", Locale: "en_US.UTF-8", Tags: []string{"preferred:asia"}})
	if err != nil || created.Account == nil || created.Account.ID != id {
		t.Fatalf("create: %+v %v", created, err)
	}
	input := remote.AccountActionInput{Confirmed: true, Expected: expected, AccountID: id}
	for _, action := range []struct {
		run    func(context.Context, cliadapter.Client, remote.AccountActionInput) (remote.AccountReceipt, error)
		status string
	}{
		{remote.BindAccount, "binding_session_starting"}, {remote.VerifyAccount, "binding_verifying"}, {remote.DisableAccount, "disabled"},
	} {
		receipt, err := action.run(ctx, client, input)
		if err != nil || !receipt.Accepted {
			t.Fatalf("%s: %+v %v", action.status, receipt, err)
		}
		encoded, _ := json.Marshal(receipt)
		if strings.Contains(string(encoded), "fixture-secret") || strings.Contains(string(encoded), "fixture-device-token") {
			t.Fatal("unsafe receipt")
		}
		status, err := remote.GetAccountStatus(ctx, client, remote.AccountStatusInput{Expected: expected, AccountID: id})
		if err != nil || status.Account.Status != action.status || status.Binding.Status != action.status {
			t.Fatalf("status: %+v %v", status, err)
		}
	}
	mu.Lock()
	reject = true
	before := len(calls)
	mu.Unlock()
	_, err = remote.BindAccount(ctx, client, input)
	var op *cliadapter.OperationError
	if !errors.As(err, &op) || op.Code != "account_rejected" || strings.Contains(err.Error(), "fixture-secret") {
		t.Fatalf("rejection: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != before+1 {
		t.Fatal("rejection was retried")
	}
	counts := map[string]int{}
	for _, call := range calls[:before] {
		counts[call]++
	}
	for _, endpoint := range []string{"/api/v1/tool-accounts", "/api/v1/tool-accounts/" + id + "/bind/start", "/api/v1/tool-accounts/" + id + "/bind/verify", "/api/v1/tool-accounts/" + id + "/disable"} {
		if counts["POST "+endpoint] != 1 {
			t.Errorf("mutation replayed or missing: %s", endpoint)
		}
	}
}
