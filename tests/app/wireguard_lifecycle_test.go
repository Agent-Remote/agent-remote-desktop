package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func openWireGuardSearch(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := openSearch(tt); err != nil {
		t.Fatal(err)
	}
	tt.Type(locales.WireGuardConfigTitle())
	if err := tt.Click(locales.WireGuardConfigTitle()); err != nil {
		t.Fatal(err)
	}
}

func TestWireGuardPlanCancellationReturnsToDiagnosticsWithoutMutation(t *testing.T) {
	tt, _, _, log := wireguardTester(t, "wg-plan-slow", "en")
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_PLAN_STARTED", started)
	if err := tt.Click(locales.ReviewWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if tt.HasText(locales.WireGuardOperationStatus()) {
		t.Fatal("read-only plan cancellation became a mutation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "wireguard.config.apply") {
		t.Fatal("plan cancellation submitted write")
	}
}

func TestWireGuardApplyCancellationAndExitDoNotClaimRollback(t *testing.T) {
	for _, exit := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "exit"}[exit], func(t *testing.T) {
			tt, a, _, log := wireguardTester(t, "wg-apply-slow", "en")
			reviewWireGuard(t, tt)
			started := filepath.Join(t.TempDir(), "started")
			t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_STARTED", started)
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			waitFixtureFile(t, tt, started)
			if !tt.HasText(locales.WireGuardCancelHelp()) {
				t.Fatal("missing cancellation warning")
			}
			if exit {
				before := time.Now()
				if err := a.Close(); err != nil {
					t.Fatal(err)
				}
				if time.Since(before) > 5*time.Second {
					t.Fatal("exit left apply worker running")
				}
			} else {
				if err := tt.Click(locales.StopWireGuardWaiting()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, locales.WireGuardOutcomeUnknown())
				if tt.HasText(locales.ConfirmWireGuardConfig()) || tt.HasText(locales.WireGuardConfigComplete()) {
					t.Fatal("canceled mutation offered retry or success")
				}
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.config.apply\n") != 1 {
				t.Fatalf("canceled mutation replayed: %q", calls)
			}
		})
	}
}

func TestWireGuardOperationSurvivesNavigationAndExplicitAcknowledgment(t *testing.T) {
	tt, _, _, log := wireguardTester(t, "", "en")
	reviewWireGuard(t, tt)
	release, started := filepath.Join(t.TempDir(), "release"), filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_RELEASE", release)
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_STARTED", started)
	if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	for _, label := range []string{locales.Settings(), locales.WireGuardOperationStatus()} {
		if err := navigate(tt, label); err != nil {
			t.Fatal(err)
		}
	}
	if !tt.HasText(locales.WireGuardConfigBusy()) {
		t.Fatal("navigation lost active operation")
	}
	if err := openPage(tt, locales.Settings()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{locales.Focus(), locales.SearchWorkbench()} {
		if err := navigate(tt, label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type(locales.WireGuardOperationStatus())
	if err := tt.Click(locales.WireGuardOperationStatus()); err != nil { // the palette's item
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardConfigComplete())
	if err := toggleFocusMode(tt); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ReturnDiagnostics()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if tt.HasText(locales.WireGuardOperationStatus()) {
		t.Fatal("acknowledged receipt still pending")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.config.apply\n") != 1 || strings.Count(string(calls), "diagnostics.inspect\n") != 2 {
		t.Fatalf("incorrect mutation/refresh count: %q", calls)
	}
}

func TestWireGuardApplyBlocksLoginLogoutAndSetup(t *testing.T) {
	tt, _, _, log := wireguardTester(t, "wg-apply-slow", "en")
	reviewWireGuard(t, tt)
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_STARTED", started)
	if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	for _, label := range []string{locales.SignInRemote(), locales.SignOutRemote()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if tt.HasText(locales.ReviewLogin()) || tt.HasText(locales.ConfirmLogout()) {
		t.Fatal("WireGuard apply allowed concurrent credential changes")
	}
	for _, label := range []string{locales.DiagnosticsTitle(), locales.LocalSetupTitle()} {
		if err := navigate(tt, label); err != nil {
			t.Fatal(err)
		}
	}
	waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
	if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	for _, method := range []string{"login.password\n", "logout\n", "setup.local\n"} {
		if strings.Contains(string(calls), method) {
			t.Fatalf("concurrent mutation %s: %q", method, calls)
		}
	}
	if err := openPage(tt, locales.WireGuardOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.StopWireGuardWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardOutcomeUnknown())
}

func TestWireGuardApplyWaitsForOtherLocalMutations(t *testing.T) {
	for _, kind := range []string{"login", "logout", "setup"} {
		t.Run(kind, func(t *testing.T) {
			tt, _, _, log := wireguardTester(t, "", "en")
			reviewWireGuard(t, tt)
			switch kind {
			case "setup":
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "slow-setup")
				for _, label := range []string{locales.DiagnosticsTitle(), locales.LocalSetupTitle()} {
					if err := navigate(tt, label); err != nil {
						t.Fatal(err)
					}
				}
				waitRemoteText(t, tt, locales.LocalSetupReviewHelp())
				if err := tt.Click(locales.ConfirmLocalSetup()); err != nil {
					t.Fatal(err)
				}
			default:
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "slow-"+kind)
				if err := openPage(tt, locales.RemoteManagement()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
				if kind == "logout" {
					for _, label := range []string{locales.SignOutRemote(), locales.ConfirmLogout()} {
						if err := tt.Click(label); err != nil {
							t.Fatal(err)
						}
					}
				} else {
					if err := tt.Click(locales.SignInRemote()); err != nil {
						t.Fatal(err)
					}
					for _, input := range []struct{ label, value string }{{locales.LoginUsername(), "desktop-user"}, {locales.LoginPassword(), "private-login-password"}} {
						if err := tt.Click(input.label); err != nil {
							t.Fatal(err)
						}
						tt.Type(input.value)
					}
					for _, label := range []string{locales.ReviewLogin(), locales.ConfirmLogin()} {
						if err := tt.Click(label); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			openWireGuardSearch(t, tt)
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "wireguard.config.apply") {
				t.Fatalf("WireGuard started during %s: %q", kind, calls)
			}
		})
	}
}

func TestAuthenticationInvalidatesWireGuardConfirmationIncludingPendingPlans(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "pending"}[pending], func(t *testing.T) {
			tt, _, _, log := wireguardTester(t, "", "en")
			if pending {
				release, started := filepath.Join(t.TempDir(), "release"), filepath.Join(t.TempDir(), "started")
				t.Setenv("DESKTOP_TEST_WIREGUARD_PLAN_RELEASE", release)
				t.Setenv("DESKTOP_TEST_WIREGUARD_PLAN_STARTED", started)
				if err := tt.Click(locales.ReviewWireGuardConfig()); err != nil {
					t.Fatal(err)
				}
				waitFixtureFile(t, tt, started)
			} else {
				reviewWireGuard(t, tt)
			}
			if err := openPage(tt, locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			for _, label := range []string{locales.SignOutRemote(), locales.ConfirmLogout()} {
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			waitRemoteText(t, tt, locales.LogoutLocalComplete())
			openWireGuardSearch(t, tt)
			waitRemoteText(t, tt, locales.RemoteContextChanged())
			if tt.HasText(locales.ConfirmWireGuardConfig()) {
				t.Fatal("authentication left stale plan confirmable")
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "wireguard.config.apply") {
				t.Fatal("authentication replayed stale plan")
			}
		})
	}
}
