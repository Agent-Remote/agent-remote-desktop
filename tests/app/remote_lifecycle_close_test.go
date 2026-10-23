package app_test

import (
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

func waitControlResponse(t *testing.T, path, method string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(path)
		for _, line := range strings.Split(string(data), "\n") {
			if line == method {
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("control helper did not emit a response for %s", method)
}

func lifecycleApp(t *testing.T, home, scenario, marker string) (*app.App, *ui.Tester) {
	t.Helper()
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", scenario)
	t.Setenv("DESKTOP_TEST_CLI_RESPONSE_FILE", marker)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
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
	if err := openPage(tester, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tester, "11111111-1111-4111-8111-111111111111")
	return a, tester
}

func fillRemoteCreateCloseReview(t *testing.T, tester *ui.Tester) {
	t.Helper()
	fields := []struct{ label, value string }{
		{locales.SessionCreateTool(), "claude"},
		{locales.SessionCreateAccount(), "11111111-1111-4111-8111-111111111111"},
		{locales.SessionCreateWorkspace(), "22222222-2222-4222-8222-222222222222"},
		{locales.SessionCreateProject(), "project"},
		{locales.SessionCreateArgvDigest(), "sha256:" + strings.Repeat("a", 64)},
		{locales.SessionCreateSyncTarget(), `{"id":"33333333-3333-4333-8333-333333333333","workspace_id":"22222222-2222-4222-8222-222222222222","node_id":"44444444-4444-4444-8444-444444444444","local_path":"/project","remote_path":"/workspace","mutagen_session_id":"agent-remote-33333333","target_digest":"sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}`},
	}
	for _, field := range fields {
		if err := tester.Click(field.label); err != nil {
			t.Fatal(err)
		}
		tester.Key(ui.Cmd, ui.KeyA)
		tester.Type(field.value)
	}
}

func TestRemoteCreateCloseDrainsBufferedReceipt(t *testing.T) {
	home, marker := t.TempDir(), filepath.Join(t.TempDir(), "responses")
	a, tester := lifecycleApp(t, home, "remote-create", marker)
	if err := tester.Click(locales.CreateRemoteSession()); err != nil {
		t.Fatal(err)
	}
	fillRemoteCreateCloseReview(t, tester)
	if err := tester.Click(locales.SessionCreateSubmit()); err != nil {
		t.Fatal(err)
	}
	waitControlResponse(t, marker, "sessions.create")
	// Let the client goroutine finish decoding and enqueue its result while
	// deliberately avoiding a UI frame that could consume it first. The longer
	// grace period keeps this synchronization stable under the race detector;
	// Close still performs the actual buffered-result drain.
	time.Sleep(time.Second)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.RemoteCreateRecovery == nil || stored.RemoteCreateRecovery.RequestID == "" {
		t.Fatalf("close lost buffered create receipt caller key: %+v", stored.RemoteCreateRecovery)
	}
}

func TestRemoteStopCloseDrainsBufferedReceipt(t *testing.T) {
	home, marker := t.TempDir(), filepath.Join(t.TempDir(), "responses")
	a, tester := lifecycleApp(t, home, "remote-stop-unknown", marker)
	if err := tester.Click(locales.StopRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteStopText(t, tester, locales.RemoteStopQuestion())
	if err := tester.Click(locales.RemoteStopConfirm()); err != nil {
		t.Fatal(err)
	}
	waitControlResponse(t, marker, "sessions.stop")
	time.Sleep(time.Second)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.RemoteStopRecovery == nil || stored.RemoteStopRecovery.RequestID == "" {
		t.Fatalf("close lost buffered stop receipt caller key: %+v", stored.RemoteStopRecovery)
	}
}

func TestRemoteDeleteCloseDrainsBufferedReceipt(t *testing.T) {
	home, marker := t.TempDir(), filepath.Join(t.TempDir(), "responses")
	a, tester := lifecycleApp(t, home, "remote-delete-unknown", marker)
	if err := tester.Click(locales.DeleteRemoteSession()); err != nil {
		t.Fatal(err)
	}
	waitRemoteDeleteText(t, tester, locales.RemoteDeleteQuestion())
	if err := tester.Click(locales.RemoteDeleteConfirm()); err != nil {
		t.Fatal(err)
	}
	waitControlResponse(t, marker, "sessions.delete")
	time.Sleep(time.Second)
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	stored := testutil.LoadWorkspace(t, home)
	if stored.RemoteDeleteRecovery == nil || stored.RemoteDeleteRecovery.RequestID == "" {
		t.Fatalf("close lost buffered delete receipt caller key: %+v", stored.RemoteDeleteRecovery)
	}
}
