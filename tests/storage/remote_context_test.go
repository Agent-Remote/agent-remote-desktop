package storage_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func remoteHistory(t *testing.T) storage.State {
	t.Helper()
	s := testutil.SavedWorkspace(t)
	r := &workspace.RemoteContext{Home: t.TempDir(), ServerURL: "https://example.org", DeviceID: testutil.RemoteAttachDeviceID, SessionID: "11111111-1111-4111-8111-111111111111"}
	s.Sessions[2].Reference, s.Sessions[2].Remote = r.SessionID, r
	return s
}

func TestRemoteHistoryContextIsFrozenAndUsesVersionThree(t *testing.T) {
	home := t.TempDir()
	state := remoteHistory(t)
	state.Sessions[2].RemoteInfo = &workspace.RemoteSessionInfo{AccountID: "account-public", NodeID: "node-public", Backend: "native"}
	frozen := state.Clone()
	state.Sessions[2].Remote.ServerURL = "https://other.invalid"
	state.Sessions[2].RemoteInfo.AccountID = "changed"
	testutil.StoreWorkspace(t, home, frozen)
	got := testutil.LoadWorkspace(t, home)
	if !reflect.DeepEqual(got, frozen) || got.Sessions[2].Remote.ServerURL != "https://example.org" || got.Sessions[2].RemoteInfo.AccountID != "account-public" {
		t.Fatal("background snapshot retained mutable remote context")
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("SELECT version FROM desktop_state").Scan(&version); err != nil || version != 3 {
		t.Fatalf("schema %d: %v", version, err)
	}
	// This is the old application's guarded write; it must not discard context.
	result, err := db.Exec("UPDATE desktop_state SET version=2,value='{}' WHERE id=1 AND version IN (1,2)")
	if err != nil {
		t.Fatal(err)
	}
	count, err := result.RowsAffected()
	if err != nil || count != 0 {
		t.Fatalf("old application overwrote new history: %d %v", count, err)
	}
}

func TestRemoteSessionInfoIsBoundedAndScopedToRemoteHistory(t *testing.T) {
	home := t.TempDir()
	good := remoteHistory(t)
	good.Sessions[2].RemoteInfo = &workspace.RemoteSessionInfo{AccountID: "account-public", NodeID: "node-public", Backend: "native"}
	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, mutate := range []func(*storage.State){
		func(s *storage.State) { s.Sessions[2].RemoteInfo.AccountID = "contains space" },
		func(s *storage.State) { s.Sessions[2].RemoteInfo.NodeID = "" },
		func(s *storage.State) { s.Sessions[1].RemoteInfo = good.Sessions[2].RemoteInfo.Clone() },
	} {
		bad := good.Clone()
		mutate(&bad)
		if err := store.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid remote identity accepted: %v", err)
		}
	}
	if err := store.Save(good); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load()
	if err != nil || got.Sessions[2].RemoteInfo == nil || got.Sessions[2].RemoteInfo.Backend != "native" {
		t.Fatalf("remote identity did not round-trip: %+v %v", got.Sessions[2].RemoteInfo, err)
	}
}

func TestVersionTwoRemoteHistoryDoesNotInventContext(t *testing.T) {
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.Sessions[2].Reference = "11111111-1111-4111-8111-111111111111"
	testutil.StoreWorkspace(t, home, state)
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("UPDATE desktop_state SET version=2"); err != nil {
		t.Fatal(err)
	}
	got := testutil.LoadWorkspace(t, home)
	if !reflect.DeepEqual(got, state) || got.Sessions[2].Remote != nil {
		t.Fatal("legacy history silently acquired current context")
	}
	testutil.StoreWorkspace(t, home, got)
}

func TestInvalidRemoteHistoryCannotOverwriteOrLoad(t *testing.T) {
	home := t.TempDir()
	good := remoteHistory(t)
	testutil.StoreWorkspace(t, home, good)
	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, mutate := range []func(*storage.State){
		func(s *storage.State) { s.Sessions[2].Tool = "codex" },
		func(s *storage.State) { s.Sessions[2].Reference = "another" },
		func(s *storage.State) { s.Sessions[2].Remote.DeviceID = "" },
		func(s *storage.State) { s.Sessions[2].Remote.Home = "relative" },
		func(s *storage.State) { s.Sessions[2].Remote.ServerURL = "https://user:secret@example.org" },
	} {
		bad := good.Clone()
		mutate(&bad)
		if err := store.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid context saved: %v", err)
		}
	}
	if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got, good) {
		t.Fatal("rejected write altered history")
	}
	bad := good.Clone()
	bad.Sessions[2].Remote.SessionID = "prefix"
	raw, err := json.Marshal(bad)
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec("UPDATE desktop_state SET value=?", string(raw)); err != nil {
		t.Fatal(err)
	}
	if _, err = store.Load(); !errors.Is(err, fault.WorkspaceData) {
		t.Fatalf("invalid stored context accepted: %v", err)
	}
}
