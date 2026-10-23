package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func tunnelTester(t *testing.T, mode, language string) (*ui.Tester, *app.App, string, string) {
	t.Helper()
	tt, a, home, log := networkTester(t, mode, language)
	if err := tt.Click(locales.WireGuardTunnelPreviewTitle()); err != nil {
		t.Fatal(err)
	}
	return tt, a, home, log
}

func previewTunnel(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.WireGuardPreviewTunnelPlan()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardEditTunnelPlan())
}

func TestTunnelPreviewIsExplicitAndPreservesSelectedScope(t *testing.T) {
	for _, action := range []string{"up", "down"} {
		for _, custom := range []bool{false, true} {
			t.Run(action+map[bool]string{false: "/default", true: "/custom"}[custom], func(t *testing.T) {
				tt, _, home, log := tunnelTester(t, "unconfigured", "en")
				if err := tt.Click(locales.WireGuardTunnelAction(action)); err != nil {
					t.Fatal(err)
				}
				config := filepath.Join(t.TempDir(), "chosen.conf")
				if custom {
					for _, label := range []string{locales.WireGuardCustomConfig(), locales.WireGuardConfigPath()} {
						if err := tt.Click(label); err != nil {
							t.Fatal(err)
						}
					}
					tt.Type(config)
				}
				if calls, _ := os.ReadFile(log); len(calls) != 0 {
					t.Fatalf("form dispatched query: %s", calls)
				}
				capture := filepath.Join(t.TempDir(), "request")
				t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN", capture)
				previewTunnel(t, tt)
				data, err := os.ReadFile(capture)
				if err != nil {
					t.Fatal(err)
				}
				var input remote.WireGuardTunnelPlanInput
				if json.Unmarshal(data, &input) != nil || input.ExpectedHome != home || input.Action != action || (custom && (input.Config == nil || *input.Config != config)) || (!custom && input.Config != nil) {
					t.Fatalf("lost selected scope: %s", data)
				}
				if !tt.HasText(locales.WireGuardTunnelPreviewOnly()) {
					t.Fatal("preview concealed execution prerequisite")
				}
				if err := openPage(tt, locales.Settings()); err != nil {
					t.Fatal(err)
				}
				openNetworkSearch(t, tt)
				if !tt.HasText(locales.WireGuardTunnelPlanSummary()) {
					t.Fatal("navigation lost plan")
				}
				calls, _ := os.ReadFile(log)
				if string(calls) != "capabilities\nwireguard.tunnel.plan\n" {
					t.Fatalf("preview executed, replayed, or required server context: %s", calls)
				}
			})
		}
	}
}

func TestTunnelPreviewShowsExactSnapshotAndPlatformTargetDetails(t *testing.T) {
	tt, _, _, _ := tunnelTester(t, "", "en")
	if err := tt.Click(locales.WireGuardTunnelAction("down")); err != nil {
		t.Fatal(err)
	}
	previewTunnel(t, tt)
	scrollNetworkTo(t, tt, locales.Details())
	if err := tt.Click(locales.Details()); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{strings.Repeat("a", 64), strings.Repeat("b", 64), strings.Repeat("c", 64), strings.Repeat("d", 64), locales.WireGuardTunnelSnapshotHelp()} {
		if !tt.HasText(value) {
			t.Errorf("snapshot detail missing %q", value)
		}
	}
	switch runtime.GOOS {
	case "darwin":
		for _, value := range []string{"utun7", strings.Repeat("e", 64), strings.Repeat("f", 64)} {
			if !tt.HasText(value) {
				t.Errorf("missing actual mapped target %q", value)
			}
		}
	case "linux":
		if !tt.HasText(strings.Repeat("e", 64)) {
			t.Fatal("network namespace omitted")
		}
	case "windows":
		if !tt.HasText("WireGuardTunnel$agent-remote") || !tt.HasText("LocalSystem") || !tt.HasText(locales.WireGuardTunnelWindowsSemantics("down")) {
			t.Fatal("service identity or delete semantics omitted")
		}
	}
}

func TestTunnelPreviewEditingRequiresFreshObservation(t *testing.T) {
	tt, _, _, log := tunnelTester(t, "", "en")
	previewTunnel(t, tt)
	if err := tt.Click(locales.WireGuardEditTunnelPlan()); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(locales.WireGuardTunnelPlanSummary()) || !tt.HasText(locales.WireGuardTunnelPlanStale()) {
		t.Fatal("editing retained old exact plan")
	}
	if err := tt.Click(locales.WireGuardTunnelAction("down")); err != nil {
		t.Fatal(err)
	}
	previewTunnel(t, tt)
	if !tt.HasText(locales.WireGuardTunnelAction("down")) || !tt.HasText(locales.WireGuardTunnelTargetState("present")) {
		t.Fatal("new action reused old target observation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.tunnel.plan\n") != 2 || strings.Contains(string(calls), "apply") {
		t.Fatalf("edit failed to make two previews only: %s", calls)
	}
}

func TestTunnelPreviewUnsupportedAndUnknownRemainNonExecutable(t *testing.T) {
	for _, mode := range []string{"wg-tunnel-unsupported", "wg-tunnel-error", "wg-tunnel-unknown"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := tunnelTester(t, mode, "en")
			if err := tt.Click(locales.WireGuardPreviewTunnelPlan()); err != nil {
				t.Fatal(err)
			}
			want := locales.WireGuardTunnelPrerequisite("target_unknown")
			if mode == "wg-tunnel-unsupported" {
				want = locales.WireGuardTunnelPlanUnsupported()
			}
			if mode == "wg-tunnel-error" {
				want = locales.RemoteConfigurationUnavailable()
			}
			waitRemoteText(t, tt, want)
			if !tt.HasText(locales.WireGuardTunnelPreviewOnly()) || tt.HasText(locales.WireGuardConfigComplete()) {
				t.Fatal("non-executable preview claimed completion")
			}
			calls, _ := os.ReadFile(log)
			if strings.Contains(string(calls), "apply") || strings.Contains(string(calls), "wireguard.up") || strings.Contains(string(calls), "wireguard.down") {
				t.Fatalf("preview invoked execution: %s", calls)
			}
		})
	}
}

func TestTunnelApplyCancellationIsUnknownAndInvalidatesReviewedPlan(t *testing.T) {
	tt, _, _, log := tunnelTester(t, "wg-tunnel-apply-slow", "en")
	if err := tt.Click(locales.WireGuardTunnelAction("down")); err != nil {
		t.Fatal(err)
	}
	previewTunnel(t, tt)
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_WIREGUARD_TUNNEL_APPLY_STARTED", started)
	if err := tt.Click(locales.WireGuardTunnelApply()); err != nil {
		t.Fatal(err)
	}
	waitFixtureFile(t, tt, started)
	if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardTunnelPlanStale())
	if tt.HasText(locales.WireGuardTunnelApply()) {
		t.Fatal("cancelled tunnel mutation left the reviewed apply action available")
	}
	if err := openPage(tt, locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.OperationTunnel())
	if !tt.HasText(locales.OperationFailed()) {
		t.Fatal("cancelled tunnel mutation was not retained as an attention/failure operation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.tunnel.apply\n") != 1 {
		t.Fatalf("cancellation replayed tunnel mutation: %q", calls)
	}
}

func TestTunnelApplyStaleReceiptInvalidatesReviewedPlan(t *testing.T) {
	tt, _, _, log := tunnelTester(t, "wg-tunnel-apply-stale", "en")
	if err := tt.Click(locales.WireGuardTunnelAction("down")); err != nil {
		t.Fatal(err)
	}
	previewTunnel(t, tt)
	if err := tt.Click(locales.WireGuardTunnelApply()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardTunnelPlanStale())
	if tt.HasText(locales.WireGuardTunnelApply()) {
		t.Fatal("stale tunnel response left the old reviewed action available")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.tunnel.apply\n") != 1 {
		t.Fatalf("stale tunnel response replayed mutation: %q", calls)
	}
}
