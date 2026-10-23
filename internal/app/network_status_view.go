package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

const networkPeersPerPage = 5

func (a *App) networkStatusView(c *ui.Context, report remote.WireGuardStatusReport) {
	ui.Text(c, locales.WireGuardStatusState(report.State)).Bold()
	if report.State == "observed" && len(report.Interfaces) == 0 {
		ui.Text(c, locales.WireGuardNoInterfaces())
	}
	if len(report.Interfaces) > 0 {
		a.network.interfaceIndex = min(max(0, a.network.interfaceIndex), len(report.Interfaces)-1)
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.WireGuardInterfacePage(a.network.interfaceIndex+1, len(report.Interfaces)))
			ui.Button(c, locales.WireGuardPreviousInterface()).Disabled(a.network.interfaceIndex == 0).OnClick(func() { a.network.interfaceIndex--; a.network.peerPage = 0 })
			ui.Button(c, locales.WireGuardNextInterface()).Disabled(a.network.interfaceIndex+1 == len(report.Interfaces)).OnClick(func() { a.network.interfaceIndex++; a.network.peerPage = 0 })
		})
		a.networkInterface(c, report.Interfaces[a.network.interfaceIndex])
	}
	networkPrivileges(c, report.Platform, report.Elevation)
	networkField(c, locales.WireGuardHome(), report.Home)
	networkTool(c, report.Tool)
}

func (a *App) networkInterface(c *ui.Context, iface remote.WireGuardInterface) {
	ui.Text(c, iface.Name).FontSize(18).Bold()
	publicKey := locales.WireGuardUnavailableValue()
	if iface.PublicKey != nil {
		publicKey = *iface.PublicKey
	}
	networkField(c, locales.WireGuardNetworkField("public_key"), publicKey)
	ui.Row(c).Gap(20).Wrap().Children(func() {
		networkField(c, locales.WireGuardNetworkField("listen_port"), networkNumber(iface.ListenPort))
		networkField(c, locales.WireGuardNetworkField("fwmark"), fmt.Sprintf("0x%x", iface.FWMark))
	})
	ui.Text(c, locales.WireGuardPeerCount(len(iface.Peers))).Bold()
	if len(iface.Peers) == 0 {
		ui.Text(c, locales.WireGuardNoPeers())
		return
	}
	pages := (len(iface.Peers) + networkPeersPerPage - 1) / networkPeersPerPage
	a.network.peerPage = min(max(0, a.network.peerPage), pages-1)
	ui.Row(c.Key("peer-pagination")).Gap(10).Wrap().Children(func() {
		ui.Text(c, locales.WireGuardPeerPage(a.network.peerPage+1, pages))
		ui.Button(c, locales.WireGuardPreviousPage()).Disabled(a.network.peerPage == 0).OnClick(func() { a.network.peerPage-- })
		ui.Button(c, locales.WireGuardNextPage()).Disabled(a.network.peerPage+1 == pages).OnClick(func() { a.network.peerPage++ })
	})
	start := a.network.peerPage * networkPeersPerPage
	for _, peer := range iface.Peers[start:min(start+networkPeersPerPage, len(iface.Peers))] {
		a.card(c, ui.Column(c.Key(peer.PublicKey)).FillWidth().Gap(8).Padding(12)).Children(func() { networkPeer(c, peer) })
	}
}

func networkPeer(c *ui.Context, peer remote.WireGuardPeer) {
	networkField(c, locales.WireGuardNetworkField("public_key"), peer.PublicKey)
	endpoint := locales.WireGuardUnavailableValue()
	if peer.Endpoint != nil {
		endpoint = *peer.Endpoint
	}
	networkField(c, locales.WireGuardNetworkField("endpoint"), endpoint)
	if len(peer.AllowedIPs) > 0 {
		networkField(c, locales.WireGuardNetworkField("allowed_ips"), strings.Join(peer.AllowedIPs, ", "))
	}
	handshake := locales.WireGuardNeverHandshake()
	if peer.LatestHandshake != 0 {
		handshake = i18n.FormatDate(i18n.Locale(), time.Unix(int64(peer.LatestHandshake), 0).In(time.Local), i18n.DateOptions{Date: i18n.MediumStyle, Time: i18n.LongStyle})
	}
	networkField(c, locales.WireGuardNetworkField("latest_handshake"), handshake)
	for _, field := range []struct {
		label string
		value any
	}{{"received_bytes", peer.ReceivedBytes}, {"sent_bytes", peer.SentBytes}, {"persistent_keepalive", peer.PersistentKeepalive}} {
		networkField(c, locales.WireGuardNetworkField(field.label), networkNumber(field.value))
	}
}
