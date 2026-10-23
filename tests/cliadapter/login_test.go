package cliadapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestPasswordLoginWithBuiltRustCLI(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY to verify real login against an isolated HTTP fixture")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	var mu sync.Mutex
	var calls []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/login":
			var request map[string]any
			if json.NewDecoder(r.Body).Decode(&request) != nil || request["password"] != "fixture-password" || request["username"] != "fixture-user" {
				t.Error("incorrect login request")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"access_token": "fixture-access", "expires_in": 3600}})
		case "/api/v1/auth/cli/session":
			if r.Header.Get("Authorization") != "Bearer fixture-access" {
				t.Error("CLI did not use its own auth receipt")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}})
		case "/api/v1/auth/logout":
			if r.Header.Get("Authorization") != "Bearer fixture-session" {
				t.Error("logout did not reuse the stored CLI session")
			}
			json.NewEncoder(w).Encode(map[string]any{})
		default:
			t.Error("unexpected endpoint")
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := cliadapter.Client{Binary: binary, Home: t.TempDir()}
	result, err := remote.PasswordLogin(context.Background(), client, remote.PasswordLoginInput{ServerURL: server.URL, Username: "fixture-user", Password: "fixture-password", Confirmed: true, SkipDevice: true})
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerURL != server.URL || result.DeviceID != nil || result.OperationID == "" {
		t.Fatalf("invalid receipt: %+v", result)
	}
	snapshot, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Config.ServerURL == nil || *snapshot.Config.ServerURL != server.URL || snapshot.Config.DeviceID != nil {
		t.Fatal("login did not update shared CLI context")
	}
	logout, err := remote.Logout(context.Background(), client, remote.LogoutInput{Confirmed: true, Revoke: true, Expected: remote.LoginContext{ServerURL: snapshot.Config.ServerURL, DeviceID: snapshot.Config.DeviceID}})
	if err != nil {
		t.Fatal(err)
	}
	if !*logout.LocalComplete || logout.User.Remote != "confirmed" || logout.Device.Remote != "not_present" {
		t.Fatalf("incorrect logout receipt: %+v", logout)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 3 || calls[0] != "POST /api/v1/auth/login" || calls[1] != "POST /api/v1/auth/cli/session" || calls[2] != "POST /api/v1/auth/logout" {
		t.Fatalf("unexpected side effects: %v", calls)
	}
}
