package app_test

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestOperationJournalIsShownAfterApplicationRestart(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.Operations = []storage.OperationRecord{{ID: "sync", Status: "attention", Stages: []storage.OperationStage{{Name: "server", State: "completed"}, {Name: "mutagen", State: "unknown"}}, UpdatedAt: time.Now().UTC()}}
	testutil.StoreWorkspace(t, home, state)

	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 900, 600)
	if err := tester.Click(locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText(locales.OperationSync()) || !tester.HasText(locales.OperationAttention()) {
		t.Fatal("persisted operation summary was not restored")
	}
	if !tester.HasText(locales.SyncStage("server", "completed")) || !tester.HasText(locales.SyncStage("mutagen", "unknown")) {
		t.Fatal("persisted sync stages were not restored")
	}
	if err := tester.Click(locales.OperationsClear()); err != nil {
		t.Fatal(err)
	}
	if tester.HasText(locales.OperationSync()) {
		t.Fatal("cleared operation summary remained visible")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	reopened, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if ui.NewTester(reopened.View, 900, 600).HasText(locales.OperationSync()) {
		t.Fatal("cleared operation summary returned after restart")
	}
}

func TestRunningOperationIsRecoveredAsAttention(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.Operations = []storage.OperationRecord{{ID: "remote-stop", Status: "running", UpdatedAt: time.Now().UTC()}}
	testutil.StoreWorkspace(t, home, state)

	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restored, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Operations) != 1 || restored.Operations[0].Status != "attention" {
		t.Fatalf("running operation was not recovered safely: %+v", restored.Operations)
	}
}

func TestDedicatedOperationRestoresAttentionWithoutReceiptAndClearsSafely(t *testing.T) {
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	state.DedicatedOperations = []storage.DedicatedOperationRecord{{
		ID: "0123456789abcdef0123456789abcdef", Command: "agent-remote skill retry",
		Status: "running", UpdatedAt: time.Now().UTC(),
	}}
	testutil.StoreWorkspace(t, home, state)

	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 900, 600)
	if err := tester.Click(locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	if !tester.HasText(locales.DedicatedOperationTitle("agent-remote skill retry")) || !tester.HasText(locales.OperationAttention()) {
		t.Fatalf("dedicated operation was not restored as attention: %v", tester.Texts())
	}
	if !tester.HasText(locales.DedicatedOperationDetailsHelp()) {
		t.Fatal("dedicated operation did not explain CLI journal boundary")
	}
	if tester.HasText(locales.OperationsOpen()) {
		t.Fatal("dedicated operation exposed a misleading owning-page action")
	}
	if err := tester.Click(locales.OperationsClear()); err != nil {
		t.Fatal(err)
	}
	if tester.HasText(locales.DedicatedOperationTitle("agent-remote skill retry")) {
		t.Fatal("cleared dedicated operation remained visible")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}

	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	restored, err := store.Load()
	if err != nil || len(restored.DedicatedOperations) != 0 {
		t.Fatalf("cleared dedicated operation returned after close: %+v %v", restored.DedicatedOperations, err)
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var raw string
	if err := db.QueryRow("SELECT value FROM desktop_state WHERE id=1").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"--token=secret", "terminal output", "session-id", "credentials"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("desktop operation snapshot leaked %q: %s", secret, raw)
		}
	}
}
