package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func waitRemoteDeleteText(t *testing.T, tester *ui.Tester, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !tester.HasText(text) && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tester.HasText(text) {
		t.Fatalf("remote delete UI did not show %q", text)
	}
}

func TestRemoteDeleteRecoverySurvivesRestartAndQueriesOriginalKey(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-delete-unknown")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	requestID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	sessionID := "11111111-1111-4111-8111-111111111111"
	selection := remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"failed", "interrupted", "stopped"}}
	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(storage.State{RemoteDeleteRecovery: &storage.RemoteDeleteRecovery{
		RequestID: requestID, Home: home, ServerURL: "https://test.invalid", DeviceID: "device-1",
		ToolType: "claude", SessionIDs: []string{sessionID},
		SelectionDigest:   remote.SessionDeleteSelectionDigest("claude", []string{sessionID}, selection),
		IntentFingerprint: remote.SessionDeleteIntentFingerprint("claude", []string{sessionID}, selection),
		UpdatedAt:         time.Now().UTC(),
	}}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	logPath := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", logPath)
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
	tester := ui.NewTester(a.View, 1100, 820)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeleteRecovery())
	if err := tester.Click(locales.RemoteDeleteRecovery()); err != nil {
		t.Fatal(err)
	}
	if err := tester.Click(locales.RemoteDeleteQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeleteRequest(requestID))
	calls, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "sessions.delete\n") != 0 {
		t.Fatalf("recovery submitted a new delete request: %s", calls)
	}
	if strings.Count(string(calls), "sessions.delete.request\n") != 1 && strings.Count(string(calls), "sessions.delete.status\n") != 1 {
		t.Fatalf("recovery did not query original delete key: %s", calls)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteDeleteUnknownQueriesOriginalCallerKey(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-delete-unknown")
	t.Setenv("DESKTOP_TEST_CLI_LOG", filepath.Join(t.TempDir(), "calls"))
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
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
	waitRemoteDeleteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.DeleteRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tt, locales.RemoteDeleteQuestion())
	if err := tt.Click(locales.RemoteDeleteConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tt, locales.RemoteDeletePhase(locales.RemoteDeleteUnknown()))
	if !tt.HasText(locales.RemoteDeleteQuery()) {
		t.Fatal("unknown delete did not expose original-key query")
	}
	if err := tt.Click(locales.RemoteDeleteQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tt, locales.RemoteDeletePhase(locales.RemoteStatusStopped()))
	// The helper log path is checked through the environment below; this test
	// primarily proves the UI retains the original request key and does not
	// expose a second submit action after an unknown response.
	logPath := os.Getenv("DESKTOP_TEST_CLI_LOG")
	if logPath == "" {
		t.Fatal("missing helper log path")
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(data), "sessions.delete\n") != 1 {
		t.Fatalf("delete was submitted more than once: %s", data)
	}
	if strings.Count(string(data), "sessions.delete.request\n") != 1 && strings.Count(string(data), "sessions.delete.status\n") != 1 {
		t.Fatalf("original delete key was not queried: %s", data)
	}
}

func TestRemoteDeleteRecoveryPointerSurvivesOpeningNewReview(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-delete-unknown")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	logPath := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", logPath)
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
	tester := ui.NewTester(a.View, 1100, 820)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, "11111111-1111-4111-8111-111111111111")
	if err := tester.Click(locales.DeleteRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeleteQuestion())
	if err := tester.Click(locales.RemoteDeleteConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeletePhase(locales.RemoteDeleteUnknown()))
	if err := tester.Click(locales.RemoteDeleteQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeletePhase(locales.RemoteStatusStopped()))
	if err := tester.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	// The first review targeted the running session. Once it is closed, the
	// fixture's interrupted session offers a new delete review. Persisting this
	// intermediate review must retain the completed request's exact key.
	waitRemoteDeleteText(t, tester, "22222222-2222-4222-8222-222222222222")
	if err := tester.Click(locales.DeleteRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeleteQuestion())
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.RemoteDeleteRecovery == nil || stored.RemoteDeleteRecovery.RequestID != "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa" {
		t.Fatalf("opening a new delete review discarded the previous exact recovery pointer: %+v", stored.RemoteDeleteRecovery)
	}
}
