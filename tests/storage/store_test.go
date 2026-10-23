package storage_test

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

func TestDesktopStateRoundtripDoesNotModifyCLIState(t *testing.T) {
	home := t.TempDir()
	cli := filepath.Join(home, "state.sqlite3")
	config := filepath.Join(home, "config.toml")
	for _, path := range []string{cli, config} {
		if err := os.WriteFile(path, []byte("CLI-owned"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	state.Preferences.Theme = "daybreak"
	state.Projects = []workspace.Project{{ID: "p", Name: "项目", Path: "/tmp/project"}}
	if err = s.Save(state); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || len(got.Projects) != 1 || got.Preferences.Theme != "daybreak" {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	for _, path := range []string{cli, config} {
		value, _ := os.ReadFile(path)
		if string(value) != "CLI-owned" {
			t.Fatal("modified shared CLI state")
		}
	}
}

func TestDedicatedDeviceCommandJournalAllowlist(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	commands := []string{
		"agent-remote device install",
		"agent-remote device uninstall",
		"agent-remote device status",
		"agent-remote device launch",
		"agent-remote device diagnose",
	}
	state := storage.State{DedicatedOperations: make([]storage.DedicatedOperationRecord, 0, len(commands))}
	for i, command := range commands {
		state.DedicatedOperations = append(state.DedicatedOperations, storage.DedicatedOperationRecord{
			ID: fmt.Sprintf("%032x", i+1), Command: command, Status: "attention", UpdatedAt: when,
		})
	}
	if err := s.Save(state); err != nil {
		t.Fatalf("device dedicated commands rejected by journal allowlist: %v", err)
	}
}

func TestDedicatedFClaudeCommandJournalAllowlist(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	commands := []string{"fclaude run", "fclaude new", "fclaude list", "fclaude attach"}
	state := storage.State{DedicatedOperations: make([]storage.DedicatedOperationRecord, 0, len(commands))}
	for i, command := range commands {
		state.DedicatedOperations = append(state.DedicatedOperations, storage.DedicatedOperationRecord{
			ID: fmt.Sprintf("%032x", i+1), Command: command, Status: "attention", UpdatedAt: when,
		})
	}
	if err := s.Save(state); err != nil {
		t.Fatalf("fclaude dedicated commands rejected by journal allowlist: %v", err)
	}
}

func TestSchemaCommandJournalAllowsSafeGenericPaths(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	state := storage.State{DedicatedOperations: []storage.DedicatedOperationRecord{
		{ID: "0123456789abcdef0123456789abcdef", Command: "agent-remote status", Status: "completed", UpdatedAt: when},
		{ID: "1123456789abcdef0123456789abcdef", Command: "agent-remote deps status", Status: "failed", UpdatedAt: when},
		{ID: "2123456789abcdef0123456789abcdef", Command: "fclaude", Status: "completed", UpdatedAt: when},
	}}
	if err := s.Save(state); err != nil {
		t.Fatalf("safe schema command paths rejected: %v", err)
	}
	for _, command := range []string{
		"agent-remote status --json",
		"shell status",
		"agent-remote status value=bad",
	} {
		bad := storage.State{DedicatedOperations: []storage.DedicatedOperationRecord{{
			ID: "3123456789abcdef0123456789abcdef", Command: command, Status: "failed", UpdatedAt: when,
		}}}
		if err := s.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("unsafe command path accepted (%q): %v", command, err)
		}
	}
}

func TestFutureSchemaIsNotOverwritten(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("INSERT INTO desktop_state VALUES(1,99,'{}')"); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Load(); err == nil {
		t.Fatal("future schema silently accepted")
	}
	if err = s.Save(storage.State{}); !errors.Is(err, fault.StorageVersion) {
		t.Fatalf("incompatible save should be rejected: %v", err)
	}
	var version int
	if err = db.QueryRow("SELECT version FROM desktop_state").Scan(&version); err != nil || version != 99 {
		t.Fatal("future schema overwritten")
	}
}

func TestOperationJournalRoundtripAndValidation(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	state := storage.State{Operations: []storage.OperationRecord{{ID: "sync", Status: "attention", Stages: []storage.OperationStage{{Name: "server", State: "completed"}, {Name: "mutagen", State: "unknown"}}, UpdatedAt: when}}}
	if err := s.Save(state); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || len(got.Operations) != 1 || !reflect.DeepEqual(got.Operations[0], state.Operations[0]) {
		t.Fatalf("operation journal roundtrip: %+v %v", got.Operations, err)
	}
	for _, invalid := range []storage.OperationRecord{
		{ID: "../secret", Status: "failed", UpdatedAt: when},
		{ID: "login", Status: "unknown", UpdatedAt: when},
		{ID: "login", Status: "failed"},
		{ID: "sync", Status: "attention", Stages: []storage.OperationStage{{Name: "secrets", State: "completed"}}, UpdatedAt: when},
		{ID: "sync", Status: "attention", Stages: []storage.OperationStage{{Name: "server", State: "running"}}, UpdatedAt: when},
	} {
		bad := storage.State{Operations: []storage.OperationRecord{invalid}}
		if err := s.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid operation journal accepted (%+v): %v", invalid, err)
		}
	}
}

func TestRemoteStopRecoveryRoundtripKeepsOnlyExactQueryScope(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	recovery := &storage.RemoteStopRecovery{
		RequestID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Home: home,
		ServerURL: "https://test.invalid", DeviceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		SessionID: "cccccccc-cccc-4ccc-8ccc-cccccccccccc", NodeID: "dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		RuntimeBackend: "native", StopTaskID: "stop_tool_session:cccccccc-cccc-4ccc-8ccc-cccccccccccc", UpdatedAt: when,
	}
	state := storage.State{RemoteStopRecovery: recovery}
	if err := s.Save(state); err != nil {
		t.Fatalf("recovery rejected: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || got.RemoteStopRecovery == nil || *got.RemoteStopRecovery != *recovery {
		t.Fatalf("recovery roundtrip: %+v %v", got.RemoteStopRecovery, err)
	}

	for _, bad := range []*storage.RemoteStopRecovery{
		{RequestID: recovery.RequestID, Home: home, ServerURL: "https://test.invalid/?secret=x", DeviceID: recovery.DeviceID, SessionID: recovery.SessionID, NodeID: recovery.NodeID, RuntimeBackend: recovery.RuntimeBackend, UpdatedAt: when},
		{RequestID: recovery.RequestID, Home: home, ServerURL: "https://test.invalid", DeviceID: recovery.DeviceID, SessionID: recovery.SessionID, NodeID: recovery.NodeID, RuntimeBackend: "native\noutput", UpdatedAt: when},
	} {
		if err := s.Save(storage.State{RemoteStopRecovery: bad}); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid recovery accepted: %v", err)
		}
	}
}

func TestDedicatedOperationJournalRoundtripRejectsDynamicCommandData(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	want := storage.State{DedicatedOperations: []storage.DedicatedOperationRecord{{
		ID: "0123456789abcdef0123456789abcdef", Command: "agent-remote skill retry",
		Status: "attention", UpdatedAt: when,
	}}}
	if err := s.Save(want); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.QueryRow("SELECT value FROM desktop_state WHERE id=1").Scan(&raw); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	for _, secret := range []string{"--token=secret", "terminal output", "session-id", "credentials", "argv"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("dedicated operation snapshot leaked %q: %s", secret, raw)
		}
	}
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || len(got.DedicatedOperations) != 1 || got.DedicatedOperations[0] != want.DedicatedOperations[0] {
		t.Fatalf("dedicated operation roundtrip: %+v %v", got.DedicatedOperations, err)
	}
	for _, invalid := range []storage.DedicatedOperationRecord{
		{ID: "0123456789abcdef0123456789abcdef", Command: "agent-remote skill retry --token=secret", Status: "failed", UpdatedAt: when},
		{ID: "0123456789abcdef0123456789abcdef", Command: "agent-remote skill retry", Status: "unknown", UpdatedAt: when},
		{ID: "0123456789abcdef0123456789abcdef", Command: "agent-remote skill retry", Status: "failed"},
	} {
		bad := storage.State{DedicatedOperations: []storage.DedicatedOperationRecord{invalid}}
		if err := s.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid dedicated operation accepted (%+v): %v", invalid, err)
		}
	}
}

func TestRemoteCreateRecoveryRoundtripKeepsOnlyExactQueryScope(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	recovery := &storage.RemoteCreateRecovery{
		RequestID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Home: home,
		ServerURL: "https://test.invalid", DeviceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb", UpdatedAt: when,
	}
	if err := s.Save(storage.State{RemoteCreateRecovery: recovery}); err != nil {
		t.Fatalf("create recovery rejected: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || got.RemoteCreateRecovery == nil || *got.RemoteCreateRecovery != *recovery {
		t.Fatalf("create recovery roundtrip: %+v %v", got.RemoteCreateRecovery, err)
	}
	for _, bad := range []*storage.RemoteCreateRecovery{
		{RequestID: recovery.RequestID, Home: home, ServerURL: "https://test.invalid/?secret=x", DeviceID: recovery.DeviceID, UpdatedAt: when},
		{RequestID: recovery.RequestID, Home: home, ServerURL: "https://test.invalid", DeviceID: "device\noutput", UpdatedAt: when},
		{RequestID: recovery.RequestID, Home: home, ServerURL: "https://test.invalid", DeviceID: recovery.DeviceID},
	} {
		if err := s.Save(storage.State{RemoteCreateRecovery: bad}); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid create recovery accepted: %v", err)
		}
	}
}

func TestRemoteDeleteRecoveryRoundtripKeepsExactCollectionQueryScope(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 10, 19, 1, 2, 3, 0, time.UTC)
	recovery := &storage.RemoteDeleteRecovery{
		RequestID: "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", Home: home,
		ServerURL: "https://test.invalid", DeviceID: "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		ToolType: "claude",
		SessionIDs: []string{
			"11111111-1111-4111-8111-111111111111",
			"22222222-2222-4222-8222-222222222222",
		},
		SelectionDigest:   "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		IntentFingerprint: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		UpdatedAt:         when,
	}
	if err := s.Save(storage.State{RemoteDeleteRecovery: recovery}); err != nil {
		t.Fatalf("delete recovery rejected: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil || got.RemoteDeleteRecovery == nil || !reflect.DeepEqual(*got.RemoteDeleteRecovery, *recovery) {
		t.Fatalf("delete recovery roundtrip: %+v %v", got.RemoteDeleteRecovery, err)
	}
	for _, bad := range []*storage.RemoteDeleteRecovery{
		{RequestID: recovery.RequestID, Home: home, ServerURL: recovery.ServerURL, DeviceID: recovery.DeviceID, ToolType: "codex", SessionIDs: recovery.SessionIDs, SelectionDigest: recovery.SelectionDigest, IntentFingerprint: recovery.IntentFingerprint, UpdatedAt: when},
		{RequestID: recovery.RequestID, Home: home, ServerURL: recovery.ServerURL, DeviceID: recovery.DeviceID, ToolType: recovery.ToolType, SessionIDs: []string{recovery.SessionIDs[1], recovery.SessionIDs[0]}, SelectionDigest: recovery.SelectionDigest, IntentFingerprint: recovery.IntentFingerprint, UpdatedAt: when},
		{RequestID: recovery.RequestID, Home: home, ServerURL: recovery.ServerURL + "/?secret=x", DeviceID: recovery.DeviceID, ToolType: recovery.ToolType, SessionIDs: recovery.SessionIDs, SelectionDigest: recovery.SelectionDigest, IntentFingerprint: recovery.IntentFingerprint, UpdatedAt: when},
		{RequestID: recovery.RequestID, Home: home, ServerURL: recovery.ServerURL, DeviceID: recovery.DeviceID, ToolType: recovery.ToolType, SessionIDs: recovery.SessionIDs, SelectionDigest: "sha256:bad", IntentFingerprint: recovery.IntentFingerprint, UpdatedAt: when},
	} {
		if err := s.Save(storage.State{RemoteDeleteRecovery: bad}); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid delete recovery accepted: %+v: %v", bad, err)
		}
	}
}
