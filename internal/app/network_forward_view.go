package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) portForwardView(c *ui.Context) {
	ui.Text(c, locales.PortForwardHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	if a.remote.snapshot.Config.ServerURL == nil || a.remote.snapshot.Config.DeviceID == nil {
		ui.Text(c, locales.PortForwardSetupRequired()).TextColor(c.Theme().TextMuted)
		return
	}
	listSupported := remote.PortForwardListSupported(a.remote.snapshot.Capabilities)
	createSupported := remote.PortForwardCreateSupported(a.remote.snapshot.Capabilities)
	if !listSupported {
		ui.Text(c, locales.PortForwardUnsupported()).TextColor(c.Theme().TextMuted)
		return
	}
	if !createSupported {
		ui.Text(c, locales.PortForwardCreateUnsupported()).TextColor(c.Theme().TextMuted)
	}
	if !remote.PortForwardStopSupported(a.remote.snapshot.Capabilities) {
		ui.Text(c, locales.PortForwardStopUnsupported()).TextColor(c.Theme().TextMuted)
	}
	if !remote.PortForwardReconnectSupported(a.remote.snapshot.Capabilities) {
		ui.Text(c, locales.PortForwardReconnectUnsupported()).TextColor(c.Theme().TextMuted)
	}
	a.card(c, ui.Column(c).FillWidth().Gap(10).Padding(12)).Children(func() {
		ui.Text(c, locales.PortForwardCreateTitle()).Bold()
		ui.TextInput(c, &a.network.forwardSession).Label(locales.PortForwardSession()).Placeholder(locales.PortForwardSessionPlaceholder()).FillWidth()
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.TextInput(c, &a.network.forwardRemotePort).Label(locales.PortForwardRemotePort()).Placeholder(locales.PortForwardRemotePortPlaceholder())
			ui.TextInput(c, &a.network.forwardLocalPort).Label(locales.PortForwardLocalPort()).Placeholder(locales.PortForwardLocalPortPlaceholder())
			ui.TextInput(c, &a.network.forwardTTL).Label(locales.PortForwardTTL()).Placeholder(locales.PortForwardTTLPlaceholder())
			ui.PrimaryButton(c, locales.PortForwardCreate()).Disabled(!createSupported || a.network.forwardActionBusy || a.network.forwardActionUncertain || a.networkBlocked()).OnClick(a.submitPortForwardCreate)
		})
	})
	if a.network.forwardActionBusy {
		ui.Text(c, locales.PortForwardActionPending()).TextColor(c.Theme().TextMuted)
	}
	if a.network.forwardActionErr != nil {
		a.errorText(c, a.network.forwardActionErr, presentation.Error(a.network.forwardActionErr))
	}
	if receipt := a.network.forwardActionReceipt; receipt != nil {
		ui.Text(c, locales.PortForwardActionComplete(receipt.Forward.ID, receipt.Forward.Status)).TextColor(c.Theme().TextMuted)
	}
	query := &a.network.forwards
	if query.err != nil {
		a.errorText(c, query.err, presentation.Error(query.err))
		if query.err.Error() == "incompatible_cli_protocol" {
			ui.Text(c, locales.PortForwardUnsupported()).TextColor(c.Theme().TextMuted)
		}
		return
	}
	if query.busy {
		ui.Text(c, locales.PortForwardLoading()).TextColor(c.Theme().TextMuted)
		return
	}
	if query.stale {
		ui.Text(c, locales.PortForwardStale()).TextColor(c.Theme().TextMuted)
	}
	if query.report == nil {
		ui.Text(c, locales.PortForwardEmpty()).TextColor(c.Theme().TextMuted)
		return
	}
	if len(query.report.Items) == 0 {
		ui.Text(c, locales.PortForwardEmpty()).TextColor(c.Theme().TextMuted)
		return
	}
	for _, item := range query.report.Items {
		portForwardRow(c, item, a)
	}
}

func portForwardRow(c *ui.Context, item remote.PortForward, a *App) {
	a.card(c, ui.Column(c.Key("port-forward-"+item.ID)).FillWidth().Gap(5).Padding(12)).Children(func() {
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Text(c, locales.PortForwardPorts(int(item.RequestedLocalPort), int(item.RemotePort))).Bold()
			ui.Text(c, locales.PortForwardStatus(item.Status)).TextColor(c.Theme().TextMuted)
		})
		ui.Text(c, locales.PortForwardSessionValue(item.SessionID)).FontSize(12)
		if listener, ok := a.localPortForward(item.ID); ok {
			ui.Text(c, locales.PortForwardListenerBound(int(listener.LocalPort))).FontSize(12).TextColor(c.Theme().TextMuted)
		} else if a.hasLocalPortForward(item.ID) {
			ui.Text(c, locales.PortForwardListenerExited()).FontSize(12).TextColor(c.Theme().Danger)
		}
		ui.Text(c, locales.PortForwardConnections(int(item.ConnectionCount), int(item.BytesUp), int(item.BytesDown))).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.PortForwardExpires(item.ExpiresAt)).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			terminal := item.Status == "active" || item.Status == "pending" || item.Status == "disconnected"
			ui.Button(c, locales.PortForwardStop()).Disabled(!remote.PortForwardStopSupported(a.remote.snapshot.Capabilities) || !terminal || a.network.forwardActionBusy || a.network.forwardActionUncertain || a.networkBlocked()).OnClick(func() {
				a.network.forwardID = item.ID
				a.network.forwardSession = item.SessionID
				a.network.forwardExpectedNode = item.NodeID
				a.submitPortForwardAction(false)
			})
			reconnectable := item.Status == "active" || item.Status == "pending" || item.Status == "disconnected"
			_, localListener := a.localPortForward(item.ID)
			ui.Button(c, locales.PortForwardReconnect()).Disabled(!remote.PortForwardReconnectSupported(a.remote.snapshot.Capabilities) || !reconnectable || localListener || a.network.forwardActionBusy || a.network.forwardActionUncertain || a.networkBlocked()).OnClick(func() {
				a.network.forwardID = item.ID
				a.network.forwardSession = item.SessionID
				a.network.forwardExpectedNode = item.NodeID
				a.submitPortForwardAction(true)
			})
		})
	})
}
