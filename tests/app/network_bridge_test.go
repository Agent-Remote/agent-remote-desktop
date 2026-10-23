package app_test

import (
	"os"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func TestWireGuardBridgeStatusIsExplicitAndDoesNotAuthorizeExecution(t *testing.T) {
	tt, _, _, log := networkTester(t, "", "en")
	if err := tt.Click(locales.WireGuardBridgeSummary()); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(locales.WireGuardBridgeState("unsupported")) {
		t.Fatal("bridge query ran when the page was opened")
	}
	if err := tt.Click(locales.WireGuardBridgeReadStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardBridgeState("unsupported"))
	for _, label := range []string{
		locales.WireGuardBridgeLifecycleLabel(),
		locales.WireGuardBridgeInstallationLabel(),
		locales.WireGuardBridgeApprovalLabel(),
		locales.WireGuardBridgeProtocolLabel(),
		locales.WireGuardBridgeAuthenticationLabel(),
		locales.WireGuardBridgeExecutionLabel(),
	} {
		if !tt.HasText(label) {
			t.Errorf("bridge status omitted %q", label)
		}
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\nwireguard.bridge.status\n" {
		t.Fatalf("bridge page executed an unexpected operation: %q", calls)
	}
}
