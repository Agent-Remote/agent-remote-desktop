package app_test

import (
	"os"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func TestRemoteRejectsIncompatiblePlatformBeforeAuthenticatedRequests(t *testing.T) {
	for _, mode := range []string{"other-platform", "missing-platform", "duplicate-capability", "invalid-effect"} {
		t.Run(mode, func(t *testing.T) {
			_, tt, log := capabilityTester(t, "en")
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, presentation.Error(fault.CLIProtocol))
			if tt.HasText(locales.SignInRemote()) || tt.HasText(locales.SignOutRemote()) {
				t.Fatal("incompatible CLI exposed authentication actions")
			}
			calls, err := os.ReadFile(log)
			if err != nil || string(calls) != "capabilities\n" {
				t.Fatalf("incompatible CLI performed further operations: %q (%v)", calls, err)
			}
			if mode == "other-platform" {
				if err := tt.Click(locales.CapabilitiesTitle()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, locales.CapabilitiesState("platform_unsupported"))
			}
		})
	}
}

func TestRemoteOldCLIReadOperationsDoNotEnableMissingAuthenticationMethods(t *testing.T) {
	_, tt, log := capabilityTester(t, "en")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "read-only-capabilities")
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if tt.HasText(locales.SignInRemote()) || tt.HasText(locales.SignOutRemote()) {
		t.Fatal("legacy CLI exposed unsupported authentication actions")
	}
	calls, err := os.ReadFile(log)
	if err != nil || string(calls) != "capabilities\ncontext\naccounts.list\nsessions.list\n" {
		t.Fatalf("legacy CLI lost read support or attempted a mutation: %q (%v)", calls, err)
	}
}

func TestCapabilityFailurePreservesExistingAuthenticationResultNavigation(t *testing.T) {
	for _, operation := range []string{"login", "logout"} {
		t.Run(operation, func(t *testing.T) {
			var tt *ui.Tester
			var log, result, status, mutation string
			if operation == "login" {
				tt, log = loginTester(t, "login-success")
				result, status, mutation = locales.LoginCompleted(), locales.LoginOperationStatus(), "login.password\n"
				for _, label := range []string{locales.ReviewLogin(), locales.ConfirmLogin()} {
					if err := tt.Click(label); err != nil {
						t.Fatal(err)
					}
				}
			} else {
				tt, log = logoutTester(t, "", "en")
				result, status, mutation = locales.LogoutLocalComplete(), locales.LogoutOperationStatus(), "logout\n"
				if err := tt.Click(locales.ConfirmLogout()); err != nil {
					t.Fatal(err)
				}
			}
			waitRemoteText(t, tt, result)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "other-platform")
			if err := tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, presentation.Error(fault.CLIProtocol))
			if err := tt.Click(status); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(result) {
				t.Fatal("capability rejection blocked the existing operation result")
			}
			calls, err := os.ReadFile(log)
			if err != nil || strings.Count(string(calls), mutation) != 1 {
				t.Fatalf("result navigation replayed mutation: %q (%v)", calls, err)
			}
		})
	}
}
