//go:build !windows

package cliadapter_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

const attachInteropSession = "11111111-1111-4111-8111-111111111111"
const attachInteropDevice = "22222222-2222-4222-8222-222222222222"
const attachInteropNode = "33333333-3333-4333-8333-333333333333"
const attachInteropRecord = "44444444-4444-4444-8444-444444444444"

// Exercise the production Go launch contract and real Rust credential/authorization
// service with isolated HTTP and an argv-recording SSH fixture, not deployed Nodes.
func TestSessionAttachWithBuiltRustCLIBindsReviewedContextAndPollsOnce(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real session attach interoperability")
	}
	fclaude := filepath.Join(filepath.Dir(cli), "fclaude")
	if _, err := os.Stat(fclaude); err != nil {
		t.Fatal("build the matching fclaude binary beside agent-remote")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	t.Setenv("AGENT_REMOTE_CLIPBOARD", "0")
	t.Setenv("AGENT_REMOTE_LOGIN_CLIPBOARD", "0")
	t.Setenv("AGENT_REMOTE_ATTACHMENTS", "1") // Exact mode must not initialize the legacy receiver.
	for _, scenario := range []string{"accepted", "lost_response", "changed_record", "missing_status", "changed_device", "changed_server"} {
		t.Run(scenario, func(t *testing.T) {
			var mu sync.Mutex
			var wires []string
			posts, gets := 0, 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				wires = append(wires, r.Method+" "+r.URL.Path)
				var data any
				switch r.Method + " " + r.URL.Path {
				case "POST /api/v1/auth/login":
					data = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
				case "POST /api/v1/auth/cli/session":
					data = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
				case "POST /api/v1/devices/register":
					data = map[string]any{"device": map[string]any{"id": attachInteropDevice, "name": "Desktop fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-19T00:00:00Z"}, "device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": attachInteropRecord, "wireguard_peer_id": nil}
				case "GET /api/v1/tool-accounts":
					data = map[string]any{"items": []any{}}
				case "GET /api/v1/sessions":
					data = map[string]any{"items": []any{map[string]any{"id": attachInteropSession, "tool_type": "claude", "user_id": attachInteropDevice, "tool_account_id": attachInteropDevice, "workspace_id": attachInteropRecord, "node_id": attachInteropNode, "project_key": "fixture-project", "status": "running", "runtime_backend": "native", "created_at": "2026-10-19T00:00:00Z", "updated_at": "2026-10-19T00:00:00Z"}}}
				case "POST /api/v1/sessions/" + attachInteropSession + "/attach/exact", "GET /api/v1/sessions/" + attachInteropSession + "/attach/exact/ssh-key-sync:device:digest":
					if r.Header.Get("Authorization") != "Bearer fixture-device-token" {
						t.Error("exact authorization used wrong credential")
					}
					status := "pending"
					if r.Method == "POST" {
						posts++
						var body map[string]any
						if json.NewDecoder(r.Body).Decode(&body) != nil || !reflect.DeepEqual(body, map[string]any{"expected_device_id": attachInteropDevice}) {
							t.Error("POST changed reviewed device")
						}
						if scenario == "lost_response" {
							w.WriteHeader(http.StatusServiceUnavailable)
							_, _ = w.Write([]byte(`{"error":{"message":"synthetic-private-response"}}`))
							return
						}
					} else {
						gets++
						status = "succeeded"
						if r.URL.Query().Get("expected_device_id") != attachInteropDevice || r.URL.Query().Get("expected_task_record_id") != attachInteropRecord {
							t.Error("GET did not bind original device and task row")
						}
					}
					result := map[string]any{"version": 1, "tool_type": "claude", "session_id": attachInteropSession, "device_id": attachInteropDevice, "node_id": attachInteropNode, "ssh_host": "127.0.0.1", "ssh_port": 2222, "ssh_user": "agent-remote", "tmux_session_name": "fixture-tmux", "forward_ssh_agent": false, "authorization_task_id": "ssh-key-sync:device:digest", "authorization_task_record_id": attachInteropRecord, "authorization_task_status": status, "expires_in": 300}
					if scenario == "changed_record" && r.Method == "GET" {
						result["authorization_task_record_id"] = attachInteropNode
					}
					if scenario == "missing_status" {
						delete(result, "authorization_task_status")
					}
					data = result
				default:
					t.Errorf("unexpected endpoint: %s %s", r.Method, r.URL.Path)
					http.NotFound(w, r)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"data": data, "request_id": "fixture-trace"})
			}))
			defer server.Close()
			client := cliadapter.Client{Binary: cli, Home: filepath.Join(t.TempDir(), "home")}
			loginCtx, cancelLogin := context.WithTimeout(context.Background(), 30*time.Second)
			loginImportInterop(t, loginCtx, client, server.URL)
			cancelLogin()
			loadCtx, cancelLoad := context.WithTimeout(context.Background(), 30*time.Second)
			overview, err := remote.Load(loadCtx, client)
			cancelLoad()
			if err != nil || overview.SessionsError != nil || len(overview.Sessions) != 1 || !overview.Capabilities.SupportsTerminal("sessions.attach") {
				t.Fatalf("real list/capability: %+v %v", overview, err)
			}
			target := workspace.RemoteContext{Home: overview.Config.Home, ServerURL: *overview.Config.ServerURL, DeviceID: *overview.Config.DeviceID, SessionID: overview.Sessions[0].ID}
			arguments, err := session.RemoteAttachArguments(target)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "changed_device" || scenario == "changed_server" {
				path := filepath.Join(client.Home, "config.toml")
				config, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				changed := strings.ReplaceAll(string(config), attachInteropDevice, attachInteropNode)
				if scenario == "changed_server" {
					changed = strings.ReplaceAll(string(config), server.URL, "https://changed.invalid")
				}
				if changed == string(config) {
					t.Fatal("fixture did not change stored context")
				}
				if err := os.WriteFile(path, []byte(changed), 0600); err != nil {
					t.Fatal(err)
				}
			}
			mu.Lock()
			before := len(wires)
			mu.Unlock()
			bin := t.TempDir()
			capture := filepath.Join(bin, "ssh-args")
			script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DESKTOP_ATTACH_ARGV\"\nprintf 'fixture terminal\\n'\n"
			if err := os.WriteFile(filepath.Join(bin, "ssh"), []byte(script), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DESKTOP_ATTACH_ARGV", capture)
			prepared, err := session.PrepareWithResolver(session.Launch{Tool: session.RemoteClaude, Directory: bin, Home: target.Home, Arguments: arguments}, func(name string) (string, error) { return fclaude, nil })
			if err != nil {
				t.Fatal(err)
			}
			runCtx, cancelRun := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancelRun()
			command := exec.CommandContext(runCtx, prepared.Argv[0], prepared.Argv[1:]...)
			command.Dir = prepared.Directory
			command.Env = attachInteropEnvironment(os.Environ(), prepared.Env)
			output, runErr := command.CombinedOutput()
			if (runErr == nil) != (scenario == "accepted") {
				t.Fatalf("scenario=%s err=%v output=%s", scenario, runErr, output)
			}
			for _, secret := range []string{"fixture-device-token", "fixture-refresh", "synthetic-private-response"} {
				if strings.Contains(string(output), secret) {
					t.Error("secret leaked into terminal output")
				}
			}
			captured, captureErr := os.ReadFile(capture)
			if scenario == "accepted" {
				if captureErr != nil {
					t.Fatal(captureErr)
				}
				args := strings.Split(strings.TrimSpace(string(captured)), "\n")
				expected := []string{"agent-remote-attach", "--session", target.SessionID, "--expected-device", target.DeviceID}
				if len(args) < len(expected) || !reflect.DeepEqual(args[len(args)-len(expected):], expected) {
					t.Fatalf("SSH command drift: %q", args)
				}
				if !strings.Contains(string(output), "fixture terminal") {
					t.Fatal("native terminal output missing")
				}
			} else if !os.IsNotExist(captureErr) {
				t.Fatalf("rejected target started SSH: %s %v", captured, captureErr)
			}
			mu.Lock()
			defer mu.Unlock()
			switch scenario {
			case "changed_device", "changed_server":
				if posts != 0 || gets != 0 || len(wires) != before {
					t.Fatalf("changed context touched network: %v", wires[before:])
				}
			case "accepted", "changed_record":
				if posts != 1 || gets != 1 || len(wires) != before+2 {
					t.Fatalf("authorization was replayed or fell back: %v", wires[before:])
				}
			default:
				if posts != 1 || gets != 0 || len(wires) != before+1 {
					t.Fatalf("unknown result replayed: %v", wires[before:])
				}
			}
		})
	}
}

func attachInteropEnvironment(base, updates []string) []string {
	replaced := map[string]bool{}
	for _, value := range updates {
		key, _, _ := strings.Cut(value, "=")
		replaced[key] = true
	}
	result := make([]string, 0, len(base)+len(updates))
	for _, value := range base {
		key, _, _ := strings.Cut(value, "=")
		if !replaced[key] {
			result = append(result, value)
		}
	}
	return append(result, updates...)
}
