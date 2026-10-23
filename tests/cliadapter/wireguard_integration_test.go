package cliadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestWireGuardWithBuiltRustCLIConfirmsScopeAndPreservesPartialState(t *testing.T) {
	binaryPath := os.Getenv("DESKTOP_CLI_BINARY")
	if binaryPath == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real WireGuard plan/apply with isolated HTTP and credentials")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	t.Setenv("AGENT_REMOTE_WIREGUARD_LOCK_ROOT", t.TempDir())
	missingTool := filepath.Join(t.TempDir(), "missing-wireguard-tool")
	for _, variable := range []string{"AGENT_REMOTE_WG", "AGENT_REMOTE_WG_QUICK", "AGENT_REMOTE_WIREGUARD"} {
		t.Setenv(variable, missingTool)
	}
	var mu sync.Mutex
	mode := "valid"
	var calls []string
	var enrolled string
	var badConfigArmed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		data := map[string]any{}
		switch r.URL.Path {
		case "/api/v1/auth/login":
			data = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
		case "/api/v1/auth/cli/session":
			data = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
		case "/api/v1/devices/register":
			data = map[string]any{"device": map[string]any{"id": "device-test", "name": "Desktop fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-18T00:00:00Z"}, "device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": "key-test", "wireguard_peer_id": nil}
		case "/api/v1/network/wireguard/peer":
			if r.Method != "POST" || r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("incorrect registration authority")
			}
			var body map[string]json.RawMessage
			if json.NewDecoder(r.Body).Decode(&body) != nil || len(body) != 4 {
				t.Error("registration included unexpected fields")
			}
			for _, field := range []string{"public_key", "expected_peer_id", "expected_public_key", "expected_peer_version"} {
				if _, ok := body[field]; !ok {
					t.Errorf("registration omitted %s", field)
				}
			}
			var key string
			if err := json.Unmarshal(body["public_key"], &key); err != nil {
				t.Errorf("registration public key was not a string: %v", err)
			}
			if decoded, err := base64.StdEncoding.DecodeString(key); err != nil || len(decoded) != 32 {
				t.Error("registration did not contain a valid public key")
			}
			if enrolled != "" && enrolled != key {
				t.Error("later operation rotated the original private key")
			}
			enrolled = key
			if mode == "bad-config" {
				badConfigArmed = true
			}
			if mode == "lost" {
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "UNAVAILABLE", "message": "fixture"}})
				return
			}
			data = map[string]any{"device_id": "device-test", "peer_id": "peer-test", "interface_address": "10.77.0.2", "peer_version": 1}
		case "/api/v1/network/wireguard/config":
			if r.Method != "GET" || r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("incorrect configuration authority")
			}
			address := "10.77.0.2"
			if mode == "bad-config" && badConfigArmed {
				address += "\nPostUp = forbidden-fixture-command"
			}
			peerKey := enrolled
			if peerKey == "" {
				peerKey = base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{6}, 32))
			}
			data = map[string]any{"device_id": "device-test", "peer_id": "peer-test", "peer_public_key": peerKey, "peer_version": 1, "interface_address": address, "private_key_ref": "local", "dns": []string{}, "peers": []any{}}
		default:
			t.Error("unexpected endpoint: " + r.URL.Path)
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
	defer server.Close()
	client := cliadapter.Client{Binary: binaryPath, Home: t.TempDir()}
	// Let the shared login service create credentials and platform permissions;
	// the desktop test never reconstructs credential filenames or SQLite rows.
	keyBytes := binary.BigEndian.AppendUint32(nil, 11)
	keyBytes = append(keyBytes, []byte("ssh-ed25519")...)
	keyBytes = binary.BigEndian.AppendUint32(keyBytes, 32)
	keyBytes = append(keyBytes, make([]byte, 32)...)
	for i := len(keyBytes) - 32; i < len(keyBytes); i++ {
		keyBytes[i] = 7
	}
	keyPath := filepath.Join(t.TempDir(), "fixture.pub")
	if err := os.WriteFile(keyPath, []byte("ssh-ed25519 "+base64.StdEncoding.EncodeToString(keyBytes)+" fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	name := "Desktop fixture"
	if _, err := remote.PasswordLogin(context.Background(), client, remote.PasswordLoginInput{ServerURL: server.URL, Username: "fixture-user", Password: "fixture-password", DeviceName: &name, SSHKey: &keyPath, Confirmed: true}); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	calls = nil
	mu.Unlock()
	plan := fixtureWireGuardPlan(t, client)
	mu.Lock()
	readCalls := len(calls)
	mu.Unlock()
	if readCalls != 0 {
		t.Fatal("review accessed the network")
	}
	if _, err := os.Stat(plan.Output.Path); !os.IsNotExist(err) {
		t.Fatalf("plan created output: %v", err)
	}
	receipt, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
	if err != nil || !receipt.Complete || receipt.KeyState != "created" || receipt.PeerCount == nil || *receipt.PeerCount != 0 {
		t.Fatalf("configuration failed: %+v %v", receipt, err)
	}
	original, err := os.ReadFile(plan.Output.Path)
	if err != nil || !strings.Contains(string(original), "PrivateKey = ") {
		t.Fatal("configuration was not published")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(plan.Output.Path)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatal("configuration permissions are not private")
		}
	}
	private := ""
	for _, line := range strings.Split(string(original), "\n") {
		if strings.HasPrefix(line, "PrivateKey = ") {
			private = strings.TrimPrefix(line, "PrivateKey = ")
		}
	}
	encoded, _ := json.Marshal(receipt)
	if private == "" || strings.Contains(string(encoded), private) || strings.Contains(string(encoded), "fixture-device-token") {
		t.Fatal("receipt exposed private material")
	}
	mu.Lock()
	beforeChecks := len(calls)
	mu.Unlock()
	for _, config := range []*string{nil, &plan.Output.Path} {
		checked, err := remote.CheckWireGuard(context.Background(), client, remote.WireGuardCheckInput{ExpectedHome: client.Home, Config: config})
		if err != nil {
			t.Fatalf("generated configuration check failed: %v", err)
		}
		wantPath := filepath.Join(client.Home, "wireguard", "agent-remote.conf")
		if config != nil {
			wantPath = *config
		}
		if checked.Home != client.Home || checked.Config.Path != wantPath || checked.Config.State != "ready" ||
			strings.Join(checked.Config.Addresses, ",") != "10.77.0.2" || len(checked.Config.DNS) != 0 ||
			checked.Config.MTU == nil || *checked.Config.MTU != 1000 || checked.Config.PeerCount == nil || *checked.Config.PeerCount != 0 ||
			len(checked.Config.UnsupportedDirectives) != 0 {
			t.Fatalf("generated configuration summary was not preserved: %+v", checked.Config)
		}
		for _, tool := range checked.Tools {
			if tool.State != "missing" || tool.Path != nil {
				t.Fatal("explicit unavailable tool override fell through to host tools")
			}
		}
		encoded, err := json.Marshal(checked)
		if err != nil || strings.Contains(string(encoded), private) || strings.Contains(string(encoded), "fixture-device-token") {
			t.Fatal("configuration check exposed private material")
		}
	}
	mu.Lock()
	wantPublicKey := enrolled
	mu.Unlock()
	digest := sha256.Sum256(original)
	for _, input := range []remote.WireGuardTunnelPlanInput{
		{ExpectedHome: client.Home, Action: "up"},
		{ExpectedHome: client.Home, Action: "down", Config: &plan.Output.Path},
	} {
		tunnel, err := remote.PlanWireGuardTunnel(context.Background(), client, input)
		if err != nil {
			t.Fatalf("generated configuration tunnel plan failed: %v", err)
		}
		wantPath := filepath.Join(client.Home, "wireguard", "agent-remote.conf")
		if input.Config != nil {
			wantPath = *input.Config
		}
		if tunnel.Action != input.Action || tunnel.Config.Path != wantPath || tunnel.Config.State != "ready" ||
			tunnel.Binding == nil || tunnel.Binding.PublicKey != wantPublicKey ||
			tunnel.Binding.ContentSHA256 != hex.EncodeToString(digest[:]) {
			t.Fatalf("tunnel binding did not match the enrolled configuration: %+v", tunnel)
		}
		if !slices.Contains(tunnel.Issues, "execution_bridge_required") {
			t.Fatal("tunnel plan did not retain the protected bridge blocker")
		}
		for _, tool := range tunnel.Tools {
			if tool.State != "missing" || tool.Path != nil || tool.Identity != nil || tool.SHA256 != nil {
				t.Fatal("tunnel plan ignored explicit unavailable tools")
			}
		}
		encoded, err := json.Marshal(tunnel)
		if err != nil || strings.Contains(string(encoded), private) || strings.Contains(string(encoded), "fixture-device-token") {
			t.Fatal("tunnel plan exposed private material")
		}
	}
	mu.Lock()
	afterChecks := len(calls)
	mu.Unlock()
	if afterChecks != beforeChecks {
		t.Fatal("read-only configuration check or tunnel plan accessed the control plane")
	}
	checkedBytes, err := os.ReadFile(plan.Output.Path)
	if err != nil || string(checkedBytes) != string(original) {
		t.Fatal("configuration check or tunnel plan modified the generated file")
	}
	replacement := fixtureWireGuardPlan(t, client)
	if replacement.Output.Action != "replace" {
		t.Fatal("existing output was not reviewed as replacement")
	}
	marker := []byte("changed after review\n")
	if err := os.WriteFile(plan.Output.Path, marker, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: replacement})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "output_changed" {
		t.Fatalf("changed file not blocked: %v", err)
	}
	after, err := os.ReadFile(plan.Output.Path)
	if err != nil || string(after) != string(marker) {
		t.Fatal("unconfirmed edit was overwritten")
	}
	if err := os.WriteFile(plan.Output.Path, original, 0600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct{ mode, code, enrollment string }{{"bad-config", "wireguard_config_invalid", "confirmed"}, {"lost", "wireguard_enrollment_unknown", "unknown"}} {
		mu.Lock()
		mode = scenario.mode
		badConfigArmed = false
		mu.Unlock()
		reviewed := fixtureWireGuardPlan(t, client)
		result, err := remote.ApplyWireGuardConfig(context.Background(), client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: reviewed})
		if err != nil || result.Complete || result.KeyState != "reused" || result.Code != scenario.code || result.Enrollment != scenario.enrollment || result.Publication != "not_attempted" {
			t.Fatalf("partial stage lost (%s): %+v %v", scenario.mode, result, err)
		}
		after, err := os.ReadFile(plan.Output.Path)
		if err != nil || string(after) != string(original) {
			t.Fatal("failed operation replaced existing configuration")
		}
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"GET /api/v1/network/wireguard/config",
		"POST /api/v1/network/wireguard/peer",
		"GET /api/v1/network/wireguard/config",
		"GET /api/v1/network/wireguard/config",
		"POST /api/v1/network/wireguard/peer",
		"GET /api/v1/network/wireguard/config",
		"GET /api/v1/network/wireguard/config",
		"POST /api/v1/network/wireguard/peer",
	}
	if strings.Join(calls, "\n") != strings.Join(want, "\n") {
		t.Fatalf("unexpected retry or pre-confirmation network access: %v", calls)
	}
}
