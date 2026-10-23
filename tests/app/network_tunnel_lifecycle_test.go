package app_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func TestTunnelPreviewCancellationAndCloseCollectWorkers(t *testing.T) {
	for _, closeApp := range []bool{false, true} {
		t.Run(map[bool]string{false: "cancel", true: "close"}[closeApp], func(t *testing.T) {
			tt, a, _, log := tunnelTester(t, "", "en")
			started := filepath.Join(t.TempDir(), "started")
			t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_STARTED", started)
			t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_RELEASE", filepath.Join(t.TempDir(), "never"))
			if err := tt.Click(locales.WireGuardPreviewTunnelPlan()); err != nil {
				t.Fatal(err)
			}
			waitFixtureFile(t, tt, started)
			if closeApp {
				before := time.Now()
				if err := a.Close(); err != nil {
					t.Fatal(err)
				}
				if time.Since(before) > 5*time.Second {
					t.Fatal("Close retained preview worker")
				}
			} else {
				if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
					t.Fatal(err)
				}
				waitRemoteText(t, tt, locales.WireGuardQueryCanceled())
				if err := openPage(tt, locales.Settings()); err != nil {
					t.Fatal(err)
				}
				openNetworkSearch(t, tt)
				if !tt.HasText(locales.WireGuardQueryCanceled()) || tt.HasText(locales.WireGuardTunnelPlanSummary()) {
					t.Fatal("canceled preview became a result")
				}
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.tunnel.plan\n") != 1 {
				t.Fatalf("preview retried: %s", calls)
			}
		})
	}
}

func TestTunnelPreviewPublicationInvalidatesReadyAndPendingPlans(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready", true: "pending"}[pending], func(t *testing.T) {
			tt, _, _, log := tunnelTester(t, "", "en")
			started := filepath.Join(t.TempDir(), "started")
			if pending {
				t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_STARTED", started)
				t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN_RELEASE", filepath.Join(t.TempDir(), "never"))
				if err := tt.Click(locales.WireGuardPreviewTunnelPlan()); err != nil {
					t.Fatal(err)
				}
				waitFixtureFile(t, tt, started)
			} else {
				previewTunnel(t, tt)
			}
			openWireGuardSearch(t, tt)
			reviewWireGuard(t, tt)
			if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.WireGuardConfigComplete())
			openNetworkSearch(t, tt)
			waitRemoteText(t, tt, locales.WireGuardTunnelPlanStale())
			if tt.HasText(locales.WireGuardTunnelPlanSummary()) {
				t.Fatal("old plan survived config publication")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.tunnel.plan\n") != 1 {
				t.Fatalf("publication silently replaced reviewed plan: %s", calls)
			}
		})
	}
}

func TestTunnelPreviewMinimumLocalizedLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _, _, _ := tunnelTester(t, "", language)
			assertWireGuardControls(t, tt, locales.WireGuardNetworkTitle(), locales.WireGuardPreviewTunnelPlan(), locales.ReturnDiagnostics())
			renderLocalSetup(t, tt, "tunnel-form-"+language)
			if err := tt.Click(locales.WireGuardTunnelAction("down")); err != nil {
				t.Fatal(err)
			}
			previewTunnel(t, tt)
			assertWireGuardControls(t, tt, locales.WireGuardPreviewTunnelPlan(), locales.WireGuardEditTunnelPlan(), locales.ReturnDiagnostics(), locales.WireGuardTunnelPlanSummary())
			renderLocalSetup(t, tt, "tunnel-preview-"+language)
			scrollNetworkTo(t, tt, locales.Details())
			if err := tt.Click(locales.Details()); err != nil {
				t.Fatal(err)
			}
			scrollNetworkTo(t, tt, locales.WireGuardTunnelSnapshotHelp())
			renderLocalSetup(t, tt, "tunnel-details-"+language)
			tt.Scroll(650, 350, 0, 350)
			renderLocalSetup(t, tt, "tunnel-identities-"+language)
		})
	}
}

func TestTunnelPreviewSharedConfigEditingInvalidatesPlanOnly(t *testing.T) {
	tt, _, _, log := tunnelTester(t, "", "en")
	previewTunnel(t, tt)
	if err := tt.Click(locales.WireGuardStatusSummary()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "wg-desktop")
	if err := tt.Click(locales.WireGuardCheckSummary()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.WireGuardCustomConfig()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.WireGuardTunnelPreviewTitle()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.WireGuardTunnelPlanStale()) || tt.HasText(locales.WireGuardEditTunnelPlan()) {
		t.Fatal("shared configuration edit retained old preview")
	}
	if err := tt.Click(locales.WireGuardStatusSummary()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("wg-desktop") {
		t.Fatal("configuration selection discarded unrelated interface observation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.tunnel.plan\n") != 1 {
		t.Fatalf("editing implicitly previewed plan: %s", calls)
	}
}

func TestTunnelPreviewIsDisabledDuringConfigurationPublication(t *testing.T) {
	tt, _, _, log := tunnelTester(t, "wg-apply-slow", "en")
	openWireGuardSearch(t, tt)
	reviewWireGuard(t, tt)
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_APPLY_STARTED", started)
	if err := tt.Click(locales.ConfirmWireGuardConfig()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	openNetworkSearch(t, tt)
	if err := tt.Click(locales.WireGuardPreviewTunnelPlan()); err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "wireguard.tunnel.plan\n") {
		t.Fatalf("preview raced configuration publication: %s", calls)
	}
}
