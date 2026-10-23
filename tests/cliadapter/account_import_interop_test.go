package cliadapter_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

const importInteropAccount = "22222222-2222-4222-8222-222222222222"
const importInteropPrivateText = "import-fixture-private-content-do-not-return"

// HTTP is an isolated contract fixture; the caller, filesystem snapshot, and
// durable request journal are the real Rust CLI used by the desktop adapter.
type importInteropServer struct {
	mu              sync.Mutex
	server          *httptest.Server
	postCount       int
	validationCount int
	lostResponse    bool
	corruptReceipt  bool
	accountCalls    int
	requests        map[string]map[string]any
	uploaded        map[string][]byte
}

func newImportInteropServer(t *testing.T) *importInteropServer {
	t.Helper()
	f := &importInteropServer{requests: map[string]map[string]any{}, uploaded: map[string][]byte{}}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var result any
		accountPrefix := "/api/v1/tool-accounts/" + importInteropAccount
		if strings.HasPrefix(r.URL.Path, "/api/v1/tool-accounts") {
			f.accountCalls++
			if r.Header.Get("Authorization") != "Bearer fixture-device-token" {
				t.Error("import used the wrong authority")
			}
		}
		switch {
		case r.URL.Path == "/api/v1/auth/login":
			result = map[string]any{"access_token": "fixture-access", "expires_in": 3600}
		case r.URL.Path == "/api/v1/auth/cli/session":
			result = map[string]any{"access_token": "fixture-session", "expires_in": 3600, "refresh_token": "fixture-refresh", "refresh_expires_in": 2592000}
		case r.URL.Path == "/api/v1/devices/register":
			result = map[string]any{"device": map[string]any{"id": "device-test", "name": "Import fixture", "platform": cliadapter.PlatformName(runtime.GOOS), "status": "active", "created_at": "2026-10-19T00:00:00Z"}, "device_token": map[string]any{"access_token": "fixture-device-token", "expires_in": 3600}, "ssh_key_id": "key-test", "wireguard_peer_id": nil}
		case r.Method == "GET" && (r.URL.Path == accountPrefix || r.URL.Path == "/api/v1/tool-accounts"):
			account := map[string]any{"id": importInteropAccount, "user_id": "fixture-user", "tool_type": "claude", "display_name": "Import fixture", "status": "active", "region_code": "US", "timezone": "UTC", "locale": "en_US.UTF-8", "preferred_node_tags": []string{}, "affinity_node_id": nil, "runtime_backend": "native", "created_at": "2026-10-19T00:00:00Z", "updated_at": "2026-10-19T00:00:00Z"}
			result = account
			if r.URL.Path == "/api/v1/tool-accounts" {
				result = map[string]any{"items": []any{account}}
			}
		case r.Method == "POST" && r.URL.Path == accountPrefix+"/config-imports":
			var input map[string]any
			if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			if input["dry_run"] != true {
				t.Error("legacy import must only be used for the explicit policy check")
			}
			if files, ok := input["files"].([]any); !ok || len(files) != 0 {
				t.Error("policy check uploaded content")
			}
			f.validationCount++
			result = map[string]any{"tool_account_id": importInteropAccount, "accepted": input["include"], "rejected": []string{}, "warnings": []string{}, "task_id": nil, "account_remote_path": nil, "imported_file_count": nil, "dry_run": true}
		case r.Method == "POST" && r.URL.Path == accountPrefix+"/config-import-requests":
			var input map[string]any
			decoder := json.NewDecoder(r.Body)
			decoder.UseNumber()
			if err := decoder.Decode(&input); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			requestID, ok := input["request_id"].(string)
			if !ok || input["dry_run"] != false {
				t.Error("invalid reviewed request")
				w.WriteHeader(400)
				return
			}
			f.postCount++
			if _, exists := f.requests[requestID]; exists {
				t.Error("desktop replayed an original import POST")
			}
			files, ok := input["files"].([]any)
			if !ok || len(files) == 0 {
				t.Error("import omitted reviewed content")
				w.WriteHeader(400)
				return
			}
			manifest := make([]map[string]any, 0, len(files))
			for _, value := range files {
				file, ok := value.(map[string]any)
				if !ok {
					t.Error("invalid file")
					w.WriteHeader(400)
					return
				}
				path, pathOK := file["path"].(string)
				encoded, bytesOK := file["content_base64"].(string)
				content, err := base64.StdEncoding.Strict().DecodeString(encoded)
				if !pathOK || !bytesOK || err != nil {
					t.Error("invalid file payload")
					w.WriteHeader(400)
					return
				}
				f.uploaded[path] = content
				manifest = append(manifest, map[string]any{"path": path, "size": len(content), "sha256": fmt.Sprintf("sha256:%x", sha256.Sum256(content)), "mode": file["mode"]})
			}
			sort.Slice(manifest, func(i, j int) bool { return manifest[i]["path"].(string) < manifest[j]["path"].(string) })
			intent := map[string]any{"schema_version": 1, "tool_account_id": importInteropAccount, "tool_type": input["tool_type"], "source": input["source"], "include": input["include"], "exclude": input["exclude"], "include_resume_history": input["include_resume_history"], "dry_run": false, "files": manifest}
			digest := importInteropDigest(t, intent)
			if input["expected_intent_fingerprint"] != digest {
				t.Errorf("Rust content digest did not match the independently encoded manifest")
			}
			receipt := map[string]any{"schema_version": 1, "request_id": requestID, "tool_account_id": importInteropAccount, "tool_type": input["tool_type"], "source": input["source"], "include": input["include"], "exclude": input["exclude"], "include_resume_history": input["include_resume_history"], "dry_run": false, "manifest": manifest, "intent_fingerprint": digest, "task_id": "import_tool_account_config:" + importInteropAccount + ":" + requestID, "accepted": input["include"], "rejected": []string{}, "warnings": []string{}, "account_remote_path": "/accounts/fixture", "file_count": len(files)}
			f.requests[requestID] = receipt
			if f.lostResponse {
				w.WriteHeader(http.StatusServiceUnavailable)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]any{"code": "UNAVAILABLE", "message": importInteropPrivateText}})
				return
			}
			result = receipt
			if f.corruptReceipt {
				corrupted := map[string]any{}
				for key, value := range receipt {
					corrupted[key] = value
				}
				corrupted["intent_fingerprint"] = "sha256:" + strings.Repeat("0", 64)
				result = corrupted
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, accountPrefix+"/config-import-requests/"):
			requestID := strings.TrimPrefix(r.URL.Path, accountPrefix+"/config-import-requests/")
			var ok bool
			result, ok = f.requests[requestID]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":{"code":"COMMON_NOT_FOUND","message":"missing"}}`))
				return
			}
		case r.Method == "GET" && strings.HasPrefix(r.URL.Path, accountPrefix+"/config-imports/"):
			taskID := strings.TrimPrefix(r.URL.Path, accountPrefix+"/config-imports/")
			if taskID == "latest" {
				t.Error("recovery guessed the latest import")
			}
			written := make([]string, 0, len(f.uploaded))
			for path := range f.uploaded {
				written = append(written, path)
			}
			sort.Strings(written)
			result = map[string]any{"task_id": taskID, "status": "succeeded", "files_written": written, "file_count": len(written), "error": nil}
		default:
			t.Errorf("unexpected import endpoint: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": result})
	}))
	t.Cleanup(f.server.Close)
	return f
}

func importInteropDigest(t *testing.T, value any) string {
	t.Helper()
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		t.Fatal(err)
	}
	encoded := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	// The canonical server JSON is UTF-8, including Unicode line separators.
	encoded = bytes.ReplaceAll(encoded, []byte(`\u2028`), []byte("\u2028"))
	encoded = bytes.ReplaceAll(encoded, []byte(`\u2029`), []byte("\u2029"))
	return fmt.Sprintf("sha256:%x", sha256.Sum256(encoded))
}

func loginImportInterop(t *testing.T, ctx context.Context, client cliadapter.Client, serverURL string) cliadapter.ConfigContext {
	t.Helper()
	key := binary.BigEndian.AppendUint32(nil, 11)
	key = append(key, "ssh-ed25519"...)
	key = binary.BigEndian.AppendUint32(key, 32)
	key = append(key, []byte(strings.Repeat("k", 32))...)
	keyPath := filepath.Join(t.TempDir(), "fixture.pub")
	if err := os.WriteFile(keyPath, []byte("ssh-ed25519 "+base64.StdEncoding.EncodeToString(key)), 0600); err != nil {
		t.Fatal(err)
	}
	name := "Import fixture"
	if _, err := remote.PasswordLogin(ctx, client, remote.PasswordLoginInput{Confirmed: true, ServerURL: serverURL, Username: "fixture-user", Password: "fixture-password", DeviceName: &name, SSHKey: &keyPath}); err != nil {
		t.Fatal(err)
	}
	var expected cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &expected); err != nil {
		t.Fatal(err)
	}
	return expected
}

func TestAccountImportWithBuiltRustCLIRecoversOriginalRequestWithoutReupload(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real account import interoperability")
	}
	if root := os.Getenv("DESKTOP_IMPORT_INTEROP_CHILD"); root != "" {
		for _, scenario := range []string{"accepted", "lost-response", "corrupt-receipt", "source-drift", "source-link"} {
			t.Run(scenario, func(t *testing.T) { exerciseAccountImportInterop(t, root, cli, scenario) })
		}
		return
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	home := filepath.Join(root, "os-home")
	if err := os.Mkdir(home, 0700); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, executable, "-test.run=^TestAccountImportWithBuiltRustCLIRecoversOriginalRequestWithoutReupload$", "-test.v")
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if key != "HOME" && key != "USERPROFILE" && key != "DESKTOP_IMPORT_INTEROP_CHILD" && key != "AGENT_REMOTE_SECRET_BACKEND" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "HOME="+home, "USERPROFILE="+home, "DESKTOP_IMPORT_INTEROP_CHILD="+root, "AGENT_REMOTE_SECRET_BACKEND=file")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("isolated import interoperability: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "PASS original key recovered without source reads or duplicate POST") {
		t.Fatalf("child did not complete import recovery: %s", output)
	}
	t.Log(string(output))
}

func exerciseAccountImportInterop(t *testing.T, root, cli, scenario string) {
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	home := filepath.Join(root, "os-home")
	claude := filepath.Join(home, ".claude")
	if err := os.RemoveAll(claude); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"settings.json":                  importInteropPrivateText,
		"rules/说明<&>.md":                 importInteropPrivateText + "-rules",
		"plugins/fixture/skills/keep.md": importInteropPrivateText + "-plugin",
		"skills/excluded.md":             "excluded-account-skill",
		"projects/excluded.jsonl":        "excluded-private-history",
	}
	for path, content := range files {
		full := filepath.Join(claude, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	client := cliadapter.Client{Binary: cli, Home: filepath.Join(root, scenario+"-cli")}
	server := newImportInteropServer(t)
	expected := loginImportInterop(t, ctx, client, server.server.URL)
	selection := remote.AccountImportSelection{Tool: "claude", ExcludeSkills: true}
	discovery, err := remote.DiscoverAccountImport(ctx, client, selection)
	if err != nil {
		t.Fatal(err)
	}
	if discovery.SourceHome != home || len(discovery.Include) != 3 {
		t.Fatalf("discovery: %+v", discovery)
	}
	prepare := remote.AccountImportPrepareInput{Expected: expected, AccountID: importInteropAccount, Selection: selection, ExpectedSourceHome: home}
	if scenario == "source-link" {
		if runtime.GOOS == "windows" {
			t.Skip("Windows link creation requires a separate native privilege fixture")
		}
		target := filepath.Join(root, "private-outside-config")
		if err := os.WriteFile(target, []byte(importInteropPrivateText+"-outside"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Remove(filepath.Join(claude, "settings.json")); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(target, filepath.Join(claude, "settings.json")); err != nil {
			t.Fatal(err)
		}
		if _, err = remote.PrepareAccountImport(ctx, client, prepare); err == nil {
			t.Fatal("followed a source link")
		}
		server.mu.Lock()
		calls := server.accountCalls
		server.mu.Unlock()
		if calls != 0 {
			t.Fatal("unsafe source capture contacted Server")
		}
		return
	}
	plan, err := remote.PrepareAccountImport(ctx, client, prepare)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Manifest) != 3 {
		t.Fatalf("wrong captured scope: %+v", plan.Manifest)
	}
	for _, entry := range plan.Manifest {
		if strings.HasPrefix(entry.Path, "~/.claude/skills/") || strings.Contains(entry.Path, "/projects/") {
			t.Fatal("excluded content entered the plan")
		}
	}
	encoded, _ := json.Marshal(plan)
	if bytes.Contains(encoded, []byte(importInteropPrivateText)) {
		t.Fatal("source contents crossed into Go")
	}
	server.mu.Lock()
	calls := server.accountCalls
	server.mu.Unlock()
	if calls != 0 {
		t.Fatal("local discovery or content preparation contacted Server")
	}
	requestID, err := remote.NewAccountImportRequestID()
	if err != nil {
		t.Fatal(err)
	}
	input := remote.AccountImportApplyInput{ExpectedPlan: plan, RequestID: requestID}
	if _, err := remote.ApplyAccountImport(ctx, client, input); err == nil {
		t.Fatal("unconfirmed import accepted")
	}
	input.Confirmed = true
	if scenario == "source-drift" {
		if err := os.WriteFile(filepath.Join(claude, "settings.json"), []byte("changed after review"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = remote.ApplyAccountImport(ctx, client, input); err == nil {
			t.Fatal("uploaded changed source")
		}
		server.mu.Lock()
		posts := server.postCount
		server.mu.Unlock()
		if posts != 0 {
			t.Fatal("stale content was submitted")
		}
		pending, err := remote.ListAccountImportPending(ctx, client)
		if err != nil || len(pending) != 0 {
			t.Fatalf("stale source reserved a request: %+v %v", pending, err)
		}
		return
	}
	validation, err := remote.ValidateAccountImport(ctx, client, remote.AccountImportValidateInput{Confirmed: true, Expected: expected, AccountID: importInteropAccount, Selection: selection, ExpectedSourceHome: home, ExpectedInclude: discovery.Include})
	if err != nil || !validation.AuditWritten {
		t.Fatalf("explicit Server validation: %+v %v", validation, err)
	}
	server.mu.Lock()
	server.lostResponse = scenario == "lost-response"
	server.corruptReceipt = scenario == "corrupt-receipt"
	server.mu.Unlock()
	receipt, err := remote.ApplyAccountImport(ctx, client, input)
	if scenario == "accepted" {
		if err != nil || !receipt.Accepted || receipt.FileCount != 3 {
			t.Fatalf("accepted import: %+v %v", receipt, err)
		}
	} else if err == nil {
		t.Fatalf("uncertain response became acceptance: %+v", receipt)
	}
	if err != nil && strings.Contains(err.Error(), importInteropPrivateText) {
		t.Fatal("raw remote error escaped")
	}
	server.mu.Lock()
	posts := server.postCount
	for path, content := range server.uploaded {
		suffix := strings.TrimPrefix(path, "~/.claude/")
		if string(content) != files[suffix] {
			t.Errorf("uploaded content differs from confirmed file %q", path)
		}
	}
	server.mu.Unlock()
	if posts != 1 {
		t.Fatalf("import POST count: %d", posts)
	}
	if _, err := remote.ApplyAccountImport(ctx, client, input); err == nil {
		t.Fatal("original request key was accepted for a second submission")
	}
	newKey, err := remote.NewAccountImportRequestID()
	if err != nil {
		t.Fatal(err)
	}
	secondInput := input
	secondInput.RequestID = newKey
	if _, err := remote.ApplyAccountImport(ctx, client, secondInput); err == nil {
		t.Fatal("unresolved original import was bypassed using a new request key")
	}
	if err := os.RemoveAll(claude); err != nil {
		t.Fatal(err)
	}
	// Each adapter call already starts a fresh Rust process. Also replace the Go
	// client to ensure recovery relies on the durable CLI journal, not UI memory.
	recoveredClient := cliadapter.Client{Binary: cli, Home: client.Home}
	pending, err := remote.ListAccountImportPending(ctx, recoveredClient)
	if err != nil || len(pending) != 1 || pending[0].RequestID != requestID || pending[0].Digest != plan.Digest {
		t.Fatalf("durable original key: %+v %v", pending, err)
	}
	reference := pending[0]
	statusInput := remote.AccountImportStatusInput{Expected: reference.Context, AccountID: reference.AccountID, RequestID: reference.RequestID, ExpectedDigest: reference.Digest, ExpectedTaskID: reference.TaskID}
	status, err := remote.GetAccountImportStatus(ctx, recoveredClient, statusInput)
	if err != nil || !status.Accepted || status.Phase != "succeeded" || status.FileCount != 3 {
		t.Fatalf("original request recovery: %+v %v", status, err)
	}
	human := exec.CommandContext(ctx, cli, "account", "import-config", "--account", importInteropAccount, "--request-id", requestID, "--status")
	human.Env = append(os.Environ(), "AGENT_REMOTE_HOME="+client.Home)
	if output, err := human.CombinedOutput(); err != nil {
		t.Fatalf("ordinary CLI could not recover desktop request: %v %s", err, output)
	}
	if _, err := remote.ForgetAccountImport(ctx, recoveredClient, remote.AccountImportForgetInput{Confirmed: true, Expected: reference.Context, AccountID: reference.AccountID, RequestID: reference.RequestID, ExpectedDigest: reference.Digest, ExpectedTaskID: status.TaskID}); err != nil {
		t.Fatal(err)
	}
	pending, err = remote.ListAccountImportPending(ctx, recoveredClient)
	if err != nil || len(pending) != 0 {
		t.Fatalf("settled request was not forgotten: %+v %v", pending, err)
	}
	server.mu.Lock()
	posts = server.postCount
	server.mu.Unlock()
	if posts != 1 {
		t.Fatal("recovery replayed an import POST")
	}
	err = filepath.WalkDir(client.Home, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.Contains(data, []byte(importInteropPrivateText)) || bytes.Contains(data, []byte(base64.StdEncoding.EncodeToString([]byte(importInteropPrivateText)))) {
			return fmt.Errorf("import contents persisted in CLI state: %s", filepath.Base(path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("PASS original key recovered without source reads or duplicate POST")
}
