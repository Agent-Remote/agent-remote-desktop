package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func waitFixtureFile(t *testing.T, tt *ui.Tester, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	t.Fatal("fixture did not reach the held response")
}

func TestAuthenticationCompletionDiscardsInFlightRemoteQueries(t *testing.T) {
	for _, operation := range []string{"login-success", "login-unknown", "logout-success", "logout-unknown"} {
		t.Run(operation, func(t *testing.T) {
			var tt *ui.Tester
			var log string
			complete := filepath.Join(t.TempDir(), "complete-auth")
			if strings.HasPrefix(operation, "login") {
				mode := "device-wait"
				if operation == "login-unknown" {
					mode = "device-wait-unknown"
				}
				tt, log = loginTester(t, mode)
				t.Setenv("DESKTOP_TEST_DEVICE_CODE_APPROVED", complete)
				for _, label := range []string{locales.LoginWithBrowser(), locales.ReviewLogin(), locales.ConfirmLogin()} {
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
				}
				waitRemoteText(t, tt, "ABCD-1234")
			} else {
				mode := ""
				if operation == "logout-unknown" {
					mode = "logout-malformed"
				}
				tt, log = logoutTester(t, mode, "en")
				t.Setenv("DESKTOP_TEST_LOGOUT_RELEASE", complete)
				if err := tt.Click(locales.ConfirmLogout()); err != nil {
					t.Fatal(err)
				}
			}
			release := filepath.Join(t.TempDir(), "release-query")
			started := filepath.Join(t.TempDir(), "query-started")
			t.Setenv("DESKTOP_TEST_REMOTE_RELEASE", release)
			t.Setenv("DESKTOP_TEST_REMOTE_STARTED", started)
			if err := tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitFixtureFile(t, tt, started)
			if err := os.WriteFile(complete, nil, 0600); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.RemoteContextChanged())
			// Let canceled transport completion reach the UI. It must not replace the
			// invalidation notice with cancellation or an old successful snapshot.
			deadline := time.Now().Add(150 * time.Millisecond)
			for time.Now().Before(deadline) {
				time.Sleep(10 * time.Millisecond)
				tt.Frame()
			}
			if tt.HasText("11111111-1111-4111-8111-111111111111") || tt.HasText(locales.RemoteQueryCanceled()) || !tt.HasText(locales.RemoteContextChanged()) {
				t.Fatal("late query replaced the invalidated authentication context")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "sessions.list\n") != 1 {
				t.Fatalf("automatically refreshed after authentication: %q", calls)
			}
			if err := os.WriteFile(release, nil, 0600); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.RefreshRemote()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			if strings.HasPrefix(operation, "login") {
				if err := tt.Click(locales.LoginOperationStatus()); err != nil {
					t.Fatal(err)
				}
				expected := locales.LoginCompleted()
				if operation == "login-unknown" {
					expected = locales.LoginOutcomeUnknown()
				}
				if !tt.HasText(expected) {
					t.Fatal("context invalidation lost the login receipt")
				}
			} else {
				if err := tt.Click(locales.LogoutOperationStatus()); err != nil {
					t.Fatal(err)
				}
				expected := locales.LogoutLocalComplete()
				if operation == "logout-unknown" {
					expected = locales.LogoutOutcomeUnknown()
				}
				if !tt.HasText(expected) {
					t.Fatal("context invalidation lost the sign-out receipt")
				}
			}
		})
	}
}

func TestAuthenticationInProgressPreventsRemoteAttachment(t *testing.T) {
	t.Setenv("DESKTOP_TEST_REMOTE_DEVICE", testutil.RemoteAttachDeviceID)
	for _, mode := range []string{"device-wait", "slow-logout"} {
		t.Run(mode, func(t *testing.T) {
			testutil.InstallControlHelper(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			i18n.SetLocaleFunc(func() string { return "en" })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			home := t.TempDir()
			state := testutil.SavedWorkspace(t)
			testutil.StoreWorkspace(t, home, state)
			a, err := app.New(home)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			tt := ui.NewTester(a.View, 900, 600)
			if err = tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			// This project/session is connectable before authentication starts.
			if err = tt.Click(locales.ConnectRemoteSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.RemoteAttachConfirmation()) {
				t.Fatal("fixture session was not connectable")
			}
			if err = tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			labels := []string{locales.SignOutRemote(), locales.ConfirmLogout()}
			if mode == "device-wait" {
				labels = []string{locales.SignInRemote(), locales.LoginWithBrowser(), locales.ReviewLogin(), locales.ConfirmLogin()}
			}
			for _, label := range labels {
				if err = tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "device-wait" {
				waitRemoteText(t, tt, "ABCD-1234")
			}
			if err = tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			if err = tt.Click(locales.ConnectRemoteSession()); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(locales.RemoteAttachConfirmation()) || tt.HasText(locales.ConfirmRemoteConnection()) {
				t.Fatal("authentication allowed an attachment using a potentially stale context")
			}
			if err = a.Close(); err != nil {
				t.Fatal(err)
			}
			if after := testutil.LoadWorkspace(t, home); len(after.Sessions) != len(state.Sessions) {
				t.Fatal("authentication-time attachment started a session")
			}
		})
	}
}
