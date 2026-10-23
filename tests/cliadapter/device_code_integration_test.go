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

func TestDeviceCodeLoginWithBuiltRustCLI(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY to verify real device authorization with an isolated HTTP fixture")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	var mu sync.Mutex
	var calls []string
	progressReceived := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/auth/cli/start":
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"device_code": "private-device-code", "user_code": "ABCD-EFGH", "verification_url": "https://web.example.test/cli?code=ABCD-EFGH", "expires_in": 600, "interval": 1}})
		case "/api/v1/auth/cli/complete":
			var request map[string]any
			if json.NewDecoder(r.Body).Decode(&request) != nil || request["device_code"] != "private-device-code" {
				t.Error("CLI lost its private device code")
			}
			if !progressReceived {
				t.Error("approval event did not arrive before polling")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"access_token": "private-access", "expires_in": 3600}})
		case "/api/v1/auth/cli/session":
			if r.Header.Get("Authorization") != "Bearer private-access" {
				t.Error("CLI did not reuse approved login")
			}
			json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"access_token": "private-session", "expires_in": 3600, "refresh_token": "private-refresh", "refresh_expires_in": 2592000}})
		default:
			t.Error("unexpected endpoint")
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	client := cliadapter.Client{Binary: binary, Home: t.TempDir()}
	result, err := remote.DeviceCodeLogin(context.Background(), client, remote.DeviceCodeLoginInput{ServerURL: server.URL, Confirmed: true, SkipDevice: true}, func(event remote.DeviceCodeApproval) {
		mu.Lock()
		defer mu.Unlock()
		progressReceived = event.UserCode == "ABCD-EFGH" && event.VerificationURL == "https://web.example.test/cli?code=ABCD-EFGH"
	})
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
		t.Fatal("did not update shared CLI context")
	}
	mu.Lock()
	defer mu.Unlock()
	if !progressReceived || len(calls) != 3 || calls[0] != "POST /api/v1/auth/cli/start" || calls[1] != "POST /api/v1/auth/cli/complete" || calls[2] != "POST /api/v1/auth/cli/session" {
		t.Fatalf("unexpected side effects: %v", calls)
	}
}
