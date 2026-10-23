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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestAccountDefaultWithBuiltRustCLISharesHumanSelectionAndRejectsStaleConfirmation(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real account-default interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	client := cliadapter.Client{Binary: cli, Home: filepath.Join(t.TempDir(), "new-home")}
	ctx := context.Background()
	initial, err := remote.GetAccountDefault(ctx, client, "claude")
	if err != nil || initial.AccountID != nil || initial.Context.Home != client.Home || initial.Context.ServerURL != nil || initial.Context.DeviceID != nil {
		t.Fatalf("unconfigured default: %+v %v", initial, err)
	}
	if _, err := os.Stat(client.Home); !os.IsNotExist(err) {
		t.Fatalf("reading an absent default created local state: %v", err)
	}

	const accountID = "22222222-2222-4222-8222-222222222222"
	var mu sync.Mutex
	var calls []string
	wrongTool := false
	account := func() map[string]any {
		tool := "claude"
		if wrongTool {
			tool = "codex"
		}
		return map[string]any{
			"id": accountID, "user_id": "fixture-user", "tool_type": tool,
			"display_name": "Fixture account", "status": "active", "region_code": "US",
			"timezone": "UTC", "locale": "en_US.UTF-8", "preferred_node_tags": []string{},
			"affinity_node_id": nil, "runtime_backend": "tmux",
			"created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T00:00:00Z",
		}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		var data any
		switch r.URL.Path {
		case "/api/v1/auth/login":
			data = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
		case "/api/v1/auth/cli/session":
			data = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
		case "/api/v1/devices/register":
			data = map[string]any{
				"device":       map[string]any{"id": "device-test", "name": "Desktop fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-18T00:00:00Z"},
				"device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": "key-test", "wireguard_peer_id": nil,
			}
		case "/api/v1/tool-accounts", "/api/v1/tool-accounts/" + accountID:
			if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("account validation used the wrong method or authority")
			}
			if r.URL.Path == "/api/v1/tool-accounts" {
				data = map[string]any{"items": []any{account()}}
			} else {
				data = account()
			}
		default:
			t.Errorf("unexpected endpoint: %s", r.URL.Path)
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
	current, err := remote.GetAccountDefault(ctx, client, "claude")
	if err != nil || current.AccountID != nil || current.Context.ServerURL == nil || *current.Context.ServerURL != server.URL {
		t.Fatalf("default after login: %+v %v", current, err)
	}
	input := remote.AccountDefaultInput{Expected: current.Context, Tool: "claude", AccountID: accountID}
	mu.Lock()
	before := len(calls)
	mu.Unlock()
	if _, err := remote.SetAccountDefault(ctx, client, input); err == nil {
		t.Fatal("unconfirmed default selection was accepted")
	}
	mu.Lock()
	if len(calls) != before {
		t.Error("unconfirmed selection accessed the server")
	}
	mu.Unlock()
	input.Confirmed = true
	receipt, err := remote.SetAccountDefault(ctx, client, input)
	if err != nil || !receipt.Changed || receipt.Previous != nil || receipt.AccountID == nil || *receipt.AccountID != accountID {
		t.Fatalf("set default: %+v %v", receipt, err)
	}
	encoded, err := json.Marshal(receipt)
	if err != nil || strings.Contains(string(encoded), "fixture-device-token") || strings.Contains(string(encoded), "fixture-refresh") {
		t.Fatal("receipt exposed authentication material")
	}
	if output := runAccountDefaultHumanCLI(t, client, "get", "--tool", "claude"); !strings.Contains(output, accountID) {
		t.Fatalf("ordinary CLI did not see GUI selection: %s", output)
	}
	// The stale review said no default. It must not clear the newer choice.
	_, err = remote.ClearAccountDefault(ctx, client, input)
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "default_changed" {
		t.Fatalf("stale confirmation did not report its conflict: %v", err)
	}
	runAccountDefaultHumanCLI(t, client, "clear", "--tool", "claude")
	current, err = remote.GetAccountDefault(ctx, client, "claude")
	if err != nil || current.AccountID != nil {
		t.Fatalf("GUI did not see ordinary CLI clear: %+v %v", current, err)
	}
	// Human CLI still accepts unique prefixes; desktop receives the exact stored ID.
	runAccountDefaultHumanCLI(t, client, "set", "--tool", "claude", "--account-id", "22222222")
	current, err = remote.GetAccountDefault(ctx, client, "claude")
	if err != nil || current.AccountID == nil || *current.AccountID != accountID {
		t.Fatalf("GUI did not see ordinary CLI set: %+v %v", current, err)
	}
	input.Previous = current.AccountID
	cleared, err := remote.ClearAccountDefault(ctx, client, input)
	if err != nil || !cleared.Changed || cleared.AccountID != nil || cleared.Previous == nil || *cleared.Previous != accountID {
		t.Fatalf("clear receipt lost original selection: %+v %v", cleared, err)
	}
	mu.Lock()
	wrongTool = true
	mu.Unlock()
	input.Previous = nil
	_, err = remote.SetAccountDefault(ctx, client, input)
	if !errors.As(err, &operation) || operation.Code != "account_mismatch" {
		t.Fatalf("mismatched account tool was accepted: %v", err)
	}
	current, err = remote.GetAccountDefault(ctx, client, "claude")
	if err != nil || current.AccountID != nil {
		t.Fatal("rejected account replaced the local default")
	}
	mu.Lock()
	defer mu.Unlock()
	for _, call := range calls[before:] {
		if !strings.HasPrefix(call, "GET /api/v1/tool-accounts") {
			t.Fatalf("default operation made an unexpected server mutation: %s", call)
		}
	}
}

func runAccountDefaultHumanCLI(t *testing.T, client cliadapter.Client, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, client.Binary, append([]string{"--color", "never", "account", "default"}, args...)...)
	for _, variable := range os.Environ() {
		key, _, _ := strings.Cut(variable, "=")
		if !strings.EqualFold(key, "AGENT_REMOTE_HOME") {
			command.Env = append(command.Env, variable)
		}
	}
	command.Env = append(command.Env, "AGENT_REMOTE_HOME="+client.Home)
	command.WaitDelay = time.Second
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("ordinary CLI account default failed: %v %s", err, output)
	}
	return string(output)
}
