package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func TestLoginOperationRemainsReachableAcrossNavigationUntilAcknowledged(t *testing.T) {
	tt, log := loginTester(t, "device-wait")
	approved := filepath.Join(t.TempDir(), "approved")
	t.Setenv("DESKTOP_TEST_DEVICE_CODE_APPROVED", approved)
	for _, label := range []string{locales.LoginWithBrowser(), locales.ReviewLogin(), locales.ConfirmLogin()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	waitRemoteText(t, tt, "ABCD-1234")
	for _, label := range []string{locales.Settings(), locales.LoginOperationStatus()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if !tt.HasText("ABCD-1234") || !tt.HasText(locales.StopLoginWaiting()) {
		t.Fatal("navigation lost live authorization")
	}
	if err := tt.Click(locales.Settings()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(approved, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.LoginOperationStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LoginCompleted())
	// A completed receipt also remains reachable when the sidebar is hidden.
	for _, label := range []string{locales.Settings(), locales.Focus(), locales.SearchWorkbench()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type(locales.LoginOperationStatus())
	if err := tt.Click(locales.LoginOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.LoginCompleted()) || tt.HasText(locales.ConfirmLogin()) {
		t.Fatal("returning reset the completed operation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "login.device_code\n") != 1 {
		t.Fatalf("navigation restarted login: %q", calls)
	}
	if err := tt.Click(locales.Focus()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ReturnRemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if tt.HasText(locales.LoginOperationStatus()) {
		t.Fatal("acknowledged receipt remained pending")
	}
	if err := tt.Click(locales.SignInRemote()); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(locales.LoginCompleted()) {
		t.Fatal("new sign-in retained old receipt")
	}
}

func TestLogoutOperationRetainsResultsAndCancelAcrossNavigation(t *testing.T) {
	for _, mode := range []string{"slow-logout", "logout-partial"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := logoutTester(t, mode, "en")
			if err := tt.Click(locales.ConfirmLogout()); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{locales.Settings(), locales.LogoutOperationStatus()} {
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			want := locales.LogoutOutcomeUnknown()
			if mode == "slow-logout" {
				if !tt.HasText(locales.StopLogoutWaiting()) {
					t.Fatal("lost cancellation after navigating away")
				}
				if err := tt.Click(locales.StopLogoutWaiting()); err != nil {
					t.Fatal(err)
				}
			} else {
				want = locales.LogoutResultItem(locales.LogoutUserRevocation(), locales.LogoutRevocationUnconfirmed())
			}
			waitRemoteText(t, tt, want)
			for _, label := range []string{locales.Settings(), locales.LogoutOperationStatus()} {
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			if !tt.HasText(want) || tt.HasText(locales.ConfirmLogout()) {
				t.Fatal("returning discarded the result or offered another mutation")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "logout\n") > 1 {
				t.Fatalf("navigation restarted sign-out: %q", calls)
			}
			if err := tt.Click(locales.ReturnRemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			if tt.HasText(locales.LogoutOperationStatus()) {
				t.Fatal("acknowledged result remained pending")
			}
		})
	}
}
