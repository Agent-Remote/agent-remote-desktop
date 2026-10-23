package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) networkBridgeView(c *ui.Context, status remote.WireGuardBridgeStatus) {
	ui.Text(c, locales.WireGuardBridgeState(status.State)).Bold()
	ui.Text(c, locales.WireGuardBridgeHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	bridgeField(c, locales.WireGuardHome(), status.Home)
	bridgeField(c, locales.WireGuardNetworkField("platform"), locales.WireGuardBridgePlatform(status.Platform))
	bridgeField(c, locales.WireGuardBridgeLifecycleLabel(), locales.WireGuardBridgeLifecycle(string(status.Lifecycle())))
	bridgeField(c, locales.WireGuardBridgeInstallationLabel(), locales.WireGuardBridgeInstallation(status.Installation))
	bridgeField(c, locales.WireGuardBridgeApprovalLabel(), locales.WireGuardBridgeApproval(status.Approval))
	bridgeField(c, locales.WireGuardBridgeProtocolLabel(), locales.WireGuardBridgeProtocol(status.Protocol))
	bridgeField(c, locales.WireGuardBridgeAuthenticationLabel(), locales.WireGuardBridgeAuthentication(status.Authentication))
	bridgeField(c, locales.WireGuardBridgeExecutionLabel(), locales.WireGuardBridgeExecution(status.Execution))
}

func bridgeField(c *ui.Context, label, value string) {
	networkField(c, label, value)
}
