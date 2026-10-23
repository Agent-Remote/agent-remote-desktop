package app_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func waitRemoteStopText(t *testing.T, tester *ui.Tester, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !tester.HasText(text) && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tester.HasText(text) {
		t.Fatalf("remote stop UI did not show %q", text)
	}
}

func TestRemoteStopUnknownQueriesOriginalCallerKey(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-stop-unknown")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	binary := testutil.InstallControlHelper(t)
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", fault.ExecutableMissing
	}})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.StopRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tt, locales.RemoteStopQuestion())
	if err := tt.Click(locales.RemoteStopConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tt, locales.RemoteStopUnknown())
	if !tt.HasText(locales.RemoteStopQuery()) {
		t.Fatal("unknown stop did not preserve the original request key")
	}
	if err := tt.Click(locales.RemoteStopQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tt, locales.RemoteStopSaved())
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "sessions.stop\n") != 1 {
		t.Fatalf("stop was submitted more than once: %s", calls)
	}
	if strings.Count(string(calls), "sessions.stop.request\n") != 1 && strings.Count(string(calls), "sessions.stop.status\n") != 1 {
		t.Fatalf("original stop key was not queried exactly once: %s", calls)
	}
}

func TestRemoteStopRecoverySurvivesRestartAndQueriesOriginalKey(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-stop-unknown")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	binary := testutil.InstallControlHelper(t)
	options := app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", fault.ExecutableMissing
	}}
	a, err := app.NewWithOptions(home, options)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 1100, 820)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, "11111111-1111-4111-8111-111111111111")
	if err := tester.Click(locales.StopRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopQuestion())
	if err := tester.Click(locales.RemoteStopConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopUnknown())
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.RemoteStopRecovery == nil || stored.RemoteStopRecovery.RequestID == "" {
		t.Fatal("exact stop recovery pointer was not persisted")
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	var storedRaw string
	if err := db.QueryRow("SELECT value FROM desktop_state WHERE id=1").Scan(&storedRaw); err != nil {
		db.Close()
		t.Fatal(err)
	}
	db.Close()
	for _, secret := range []string{"process_stopped", "save_status", "terminal output", "argv"} {
		if strings.Contains(storedRaw, secret) {
			t.Fatalf("stop recovery persisted receipt content %q", secret)
		}
	}

	reopened, err := app.NewWithOptions(home, options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	ret := ui.NewTester(reopened.View, 1100, 820)
	if err := ret.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, ret, locales.RemoteStopRecovery())
	if err := ret.Click(locales.RemoteStopRecovery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, ret, locales.RemoteStopQuery())
	if err := ret.Click(locales.RemoteStopQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, ret, locales.RemoteStopSaved())
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "sessions.stop\n") != 1 {
		t.Fatalf("restart recovery submitted a second stop: %s", calls)
	}
	if strings.Count(string(calls), "sessions.stop.request\n") != 1 && strings.Count(string(calls), "sessions.stop.status\n") != 1 {
		t.Fatalf("restart recovery did not query the original key: %s", calls)
	}
}

func TestRemoteStopRecoveryPointerSurvivesOpeningNewReview(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-stop-unknown")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	binary := testutil.InstallControlHelper(t)
	options := app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", fault.ExecutableMissing
	}}
	a, err := app.NewWithOptions(home, options)
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 1100, 820)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, "11111111-1111-4111-8111-111111111111")
	if err := tester.Click(locales.StopRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopQuestion())
	if err := tester.Click(locales.RemoteStopConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopUnknown())
	if err := tester.Click(locales.RemoteStopQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopSaved())
	if err := tester.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	// Opening another stop review clears the in-memory key while it waits for
	// a new receipt. Closing the app persists this intermediate state.
	waitRemoteStopText(t, tester, locales.StopRemoteSession())
	if err := tester.Click(locales.StopRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopQuestion())
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.RemoteStopRecovery == nil || stored.RemoteStopRecovery.RequestID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatal("opening a new stop review discarded the previous exact recovery pointer")
	}
}
