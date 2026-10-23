package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func TestNetworkStatusCanCompleteWhileConfigQueryWaits(t *testing.T) {
	tt, _, _, _ := networkTester(t, "", "en")
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK_STARTED", started)
	t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK_RELEASE", filepath.Join(t.TempDir(), "never"))
	if err := tt.Click(locales.WireGuardCheckConfig()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "wg-desktop")
	if err := tt.Click(locales.WireGuardCheckSummary()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.WireGuardQueryBusy()) {
		t.Fatal("status query lost pending config worker")
	}
	if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardQueryCanceled())
	if err := tt.Click(locales.WireGuardStatusSummary()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("wg-desktop") {
		t.Fatal("canceling config query discarded independent status")
	}
}

func TestNetworkQueriesCancelAndCloseTheirWorkers(t *testing.T) {
	for _, operation := range []string{"CHECK", "STATUS"} {
		for _, closeApp := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "/cancel", true: "/close"}[closeApp], func(t *testing.T) {
				tt, a, _, log := networkTester(t, "", "en")
				started := filepath.Join(t.TempDir(), "started")
				t.Setenv("DESKTOP_TEST_WIREGUARD_"+operation+"_STARTED", started)
				t.Setenv("DESKTOP_TEST_WIREGUARD_"+operation+"_RELEASE", filepath.Join(t.TempDir(), "never"))
				label := locales.WireGuardCheckConfig()
				if operation == "STATUS" {
					label = locales.WireGuardReadStatus()
				}
				if err := tt.Click(label); err != nil {
					t.Fatal(err)
				}
				waitFixtureFile(t, tt, started)
				if closeApp {
					before := time.Now()
					if err := a.Close(); err != nil {
						t.Fatal(err)
					}
					if time.Since(before) > 5*time.Second {
						t.Fatal("Close failed to collect network worker")
					}
				} else {
					if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
						t.Fatal(err)
					}
					waitRemoteText(t, tt, locales.WireGuardQueryCanceled())
					if err := tt.Click(locales.Settings()); err != nil {
						t.Fatal(err)
					}
					openNetworkSearch(t, tt)
					if !tt.HasText(locales.WireGuardQueryCanceled()) || tt.HasText("wg-desktop") || tt.HasText(locales.WireGuardNoInterfaces()) {
						t.Fatal("canceled worker published a success")
					}
				}
				calls, _ := os.ReadFile(log)
				if strings.Count(string(calls), "wireguard."+strings.ToLower(operation)+"\n") != 1 {
					t.Fatalf("cancellation retried query: %s", calls)
				}
			})
		}
	}
}

func TestNetworkConfigurationPublicationInvalidatesReadyAndPendingObservations(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "pending"}[pending], func(t *testing.T) {
			tt, _, _, log := networkTester(t, "", "en")
			if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "wg-desktop")
			started := filepath.Join(t.TempDir(), "started")
			if pending {
				t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK_STARTED", started)
				t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK_RELEASE", filepath.Join(t.TempDir(), "never"))
			}
			if err := tt.Click(locales.WireGuardCheckConfig()); err != nil {
				t.Fatal(err)
			}
			if pending {
				waitFixtureFile(t, tt, started)
			} else {
				waitRemoteText(t, tt, locales.WireGuardConfigState("ready"))
			}
			openWireGuardSearch(t, tt)
			reviewWireGuard(t, tt)
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.WireGuardConfigComplete())
			openNetworkSearch(t, tt)
			waitRemoteText(t, tt, locales.WireGuardObservationStale())
			if tt.HasText(locales.WireGuardConfigState("ready")) {
				t.Fatal("old config result survived publication")
			}
			if err := tt.Click(locales.WireGuardStatusSummary()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.WireGuardObservationStale()) || tt.HasText("wg-desktop") {
				t.Fatal("old status survived publication")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.check\n") != 1 || strings.Count(string(calls), "wireguard.status\n") != 1 {
				t.Fatalf("invalidation automatically queried: %s", calls)
			}
		})
	}
}

func TestNetworkDoesNotQueryDuringConfigurationPublication(t *testing.T) {
	tt, _, _, log := networkTester(t, "wg-apply-slow", "en")
	openWireGuardSearch(t, tt)
	reviewWireGuard(t, tt)
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_STARTED", started)
	if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	openNetworkSearch(t, tt)
	for _, label := range []string{locales.WireGuardCheckConfig(), locales.WireGuardReadStatus()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "wireguard.check\n") || strings.Contains(string(calls), "wireguard.status\n") {
		t.Fatalf("query started during publication: %s", calls)
	}
}
