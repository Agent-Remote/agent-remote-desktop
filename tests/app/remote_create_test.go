package app_test

import (
	"os"
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

func TestRemoteCreateRecoverySurvivesRestartAndQueriesOriginalKey(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-create")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	requestID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	store, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(storage.State{RemoteCreateRecovery: &storage.RemoteCreateRecovery{
		RequestID: requestID, Home: home, ServerURL: "https://test.invalid", DeviceID: "device-1", UpdatedAt: time.Now().UTC(),
	}}); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	log := t.TempDir() + "/calls"
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	binary := testutil.InstallControlHelper(t)
	a, err := app.NewWithOptions(home, app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", os.ErrNotExist
	}})
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(a.View, 1100, 820)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.SessionCreateRecovery())
	if err := tester.Click(locales.SessionCreateRecovery()); err != nil {
		t.Fatal(err)
	}
	if tester.HasText(locales.SessionCreateSubmit()) {
		// The button may remain rendered but must be disabled in query-only mode.
		if _, ok := tester.Find(locales.SessionCreateSubmit()); !ok {
			t.Fatal("query-only create recovery lost its review")
		}
	}
	if err := tester.Click(locales.SessionCreateQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.SessionCreateRequest(requestID))
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(calls), "sessions.create\n") != 0 || strings.Count(string(calls), "sessions.create.request\n") != 1 {
		t.Fatalf("recovery submitted or queried the wrong method: %s", calls)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteCreateReplacementFreezesFullOriginalContext(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-create")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tester := ui.NewTester(a.View, 900, 600)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, "11111111-1111-4111-8111-111111111111")
	if err := tester.Click(locales.CreateRemoteSession()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for !tester.HasText(locales.SessionCreateReplacementContext()) && time.Now().Before(deadline) {
		tester.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tester.HasText(locales.SessionCreateReplacementContext()) {
		t.Fatal("replacement review did not explain exact context binding")
	}
	for _, label := range []string{
		locales.SessionCreateReplacementAccount(),
		locales.SessionCreateReplacementWorkspace(),
		locales.SessionCreateReplacementProject(),
		locales.SessionCreateReplacementNode(),
		locales.SessionCreateReplacementBackend(),
	} {
		if !tester.HasText(label) {
			t.Fatalf("replacement review is missing %q", label)
		}
	}
}

func TestRemoteCreateReviewRejectsIncompleteInputBeforeCLI(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-create")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	testutil.InstallControlHelper(t)
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tester := ui.NewTester(a.View, 1100, 820)
	if err := tester.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, "11111111-1111-4111-8111-111111111111")
	if err := tester.Click(locales.CreateRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, locales.SessionCreateInputInvalid())
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "sessions.create\n") {
		t.Fatalf("incomplete review invoked sessions.create: %s", calls)
	}
}
