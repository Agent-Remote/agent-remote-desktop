package app

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func (a *App) networkView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(10).Children(func() {
		ui.Text(c, locales.WireGuardNetworkHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Radio(c, &a.network.section, "", locales.WireGuardCheckSummary())
			ui.Radio(c, &a.network.section, "status", locales.WireGuardStatusSummary())
			ui.Radio(c, &a.network.section, "bridge", locales.WireGuardBridgeSummary())
			ui.Radio(c, &a.network.section, "plan", locales.WireGuardTunnelPreviewTitle())
			ui.Radio(c, &a.network.section, "forwards", locales.PortForwardTitle()).OnChange(func() {
				if a.network.section == "forwards" {
					a.readPortForwards()
				}
			})
		})
		ui.Scroll(c.Key(a.network.section)).Grow(1).FillWidth().Gap(12).Children(func() {
			if a.network.section == "forwards" {
				a.portForwardView(c)
				return
			}
			if a.network.section == "plan" {
				a.networkTunnelView(c)
				return
			}
			if a.network.section == "status" {
				query := &a.network.status
				if networkQueryNotice(c, query.busy, query.stale, query.err) {
					return
				}
				if query.report != nil {
					a.networkStatusView(c, *query.report)
				}
				return
			}
			if a.network.section == "bridge" {
				query := &a.network.bridge
				if networkQueryNotice(c, query.busy, query.stale, query.err) {
					return
				}
				if query.report != nil {
					a.networkBridgeView(c, *query.report)
				}
				return
			}
			a.networkConfigForm(c)
			query := &a.network.check
			if networkQueryNotice(c, query.busy, query.stale, query.err) {
				return
			}
			if query.report != nil {
				networkCheckView(c, *query.report)
			}
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			ui.Button(c, locales.ReturnDiagnostics()).OnClick(a.openDiagnostics)
			if a.network.section == "plan" {
				if a.network.tunnelApply.busy {
					ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.cancelTunnelApply)
				} else if plan := a.network.plan.report; plan != nil && len(plan.Issues) == 0 && a.network.tunnelApply.report == nil {
					ui.PrimaryButton(c, locales.WireGuardTunnelApply()).OnClick(func() { a.applyTunnelPlan(*plan) })
				}
				if a.network.plan.busy {
					ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.network.plan.stop)
				}
				if a.network.plan.report != nil {
					ui.Button(c, locales.WireGuardEditTunnelPlan()).OnClick(a.invalidateTunnelPlan)
				}
				invalid := a.network.configMode == "custom" && !filepath.IsAbs(a.network.config)
				ui.PrimaryButton(c, locales.WireGuardPreviewTunnelPlan()).Disabled(invalid || a.network.plan.busy || a.networkBlocked()).OnClick(a.previewTunnelPlan)
				return
			}
			if a.network.section == "forwards" {
				if a.network.forwards.busy || a.network.forwardActionBusy {
					ui.Button(c, locales.CancelRemoteQuery()).Disabled(true)
				}
				ui.Button(c, locales.PortForwardRefresh()).Disabled(a.network.forwards.busy || a.networkBlocked()).OnClick(a.readPortForwards)
				return
			}
			if a.network.section == "bridge" {
				if a.network.bridge.busy {
					ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.network.bridge.stop)
				}
				ui.Button(c, locales.WireGuardBridgeReadStatus()).Disabled(a.network.bridge.busy || a.networkBlocked()).OnClick(a.readWireGuardBridgeStatus)
				return
			}
			queryBusy, cancel := a.network.check.busy, a.network.check.stop
			if a.network.section == "status" {
				queryBusy, cancel = a.network.status.busy, a.network.status.stop
			}
			if queryBusy {
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(cancel)
			}
			invalid := a.network.configMode == "custom" && !filepath.IsAbs(a.network.config)
			ui.Button(c, locales.WireGuardCheckConfig()).Disabled(invalid || a.network.check.busy || a.networkBlocked()).OnClick(a.checkNetworkConfig)
			ui.Button(c, locales.WireGuardReadStatus()).Disabled(a.network.status.busy || a.networkBlocked()).OnClick(a.readNetworkStatus)
		})
	})
}

func (a *App) networkConfigForm(c *ui.Context) {
	busy := a.network.check.busy || a.network.plan.busy || a.network.tunnelApply.busy
	networkField(c, locales.WireGuardHome(), a.home)
	ui.Row(c).Gap(10).Wrap().Children(func() {
		ui.Radio(c, &a.network.configMode, "", locales.WireGuardDefaultConfig()).Disabled(busy).OnChange(a.invalidateNetworkConfig)
		ui.Radio(c, &a.network.configMode, "custom", locales.WireGuardCustomConfig()).Disabled(busy).OnChange(a.invalidateNetworkConfig)
	})
	if a.network.configMode == "custom" {
		ui.Row(c).Gap(10).Children(func() {
			ui.TextInput(c, &a.network.config).Label(locales.WireGuardConfigPath()).Placeholder(locales.WireGuardConfigPath()).Grow(1).Disabled(busy).OnChange(a.invalidateNetworkConfig)
			ui.Button(c, locales.Browse()).Disabled(busy).OnClick(a.selectNetworkConfig)
		})
		ui.Text(c, locales.WireGuardAbsolutePathHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	}
}

func networkQueryNotice(c *ui.Context, busy, stale bool, err error) bool {
	if err != nil {
		text := presentation.Error(err)
		if wireguardUnsupported(err) {
			text = locales.WireGuardNetworkUnsupported()
		}
		if errors.Is(err, context.Canceled) {
			text = locales.WireGuardQueryCanceled()
		}
		ui.Text(c, text).TextColor(c.Theme().Danger)
		return true
	}
	if stale {
		ui.Text(c, locales.WireGuardObservationStale()).TextColor(c.Theme().TextMuted)
		return true
	}
	if busy {
		ui.Text(c, locales.WireGuardQueryBusy()).TextColor(c.Theme().TextMuted)
		return true
	}
	return false
}

func networkCheckView(c *ui.Context, report remote.WireGuardCheckReport) {
	ui.Text(c, locales.WireGuardConfigState(report.Config.State)).Bold()
	networkField(c, locales.WireGuardConfigPath(), report.Config.Path)
	if report.Config.PeerCount != nil {
		ui.Text(c, locales.WireGuardPeerCount(int(*report.Config.PeerCount)))
	}
	if len(report.Config.Addresses) > 0 {
		networkField(c, locales.WireGuardNetworkField("addresses"), strings.Join(report.Config.Addresses, ", "))
	}
	if len(report.Config.DNS) > 0 {
		networkField(c, locales.WireGuardNetworkField("dns"), strings.Join(report.Config.DNS, ", "))
	}
	if report.Config.MTU != nil {
		networkField(c, locales.WireGuardNetworkField("mtu"), networkNumber(*report.Config.MTU))
	}
	if len(report.Config.UnsupportedDirectives) > 0 {
		networkField(c, locales.WireGuardNetworkField("unsupported_directives"), strings.Join(report.Config.UnsupportedDirectives, ", "))
	}
	networkPrivileges(c, report.Platform, report.Elevation)
	ui.Text(c, locales.WireGuardTools()).Bold()
	for _, tool := range report.Tools {
		networkTool(c, tool)
	}
}

func networkPrivileges(c *ui.Context, platform, elevation string) {
	networkField(c, locales.WireGuardNetworkField("platform"), locales.WireGuardBridgePlatform(platform))
	ui.Text(c, locales.WireGuardElevation(elevation))
	ui.Text(c, locales.WireGuardProcessPrivilegeHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
}

func networkTool(c *ui.Context, tool remote.WireGuardTool) {
	ui.Column(c.Key(tool.Name)).Gap(4).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, tool.Name)
			ui.Text(c, locales.WireGuardToolState(tool.State)).TextColor(c.Theme().TextMuted)
		})
		if tool.Path != nil {
			ui.Text(c, *tool.Path).FontSize(12)
		}
	})
}

func networkField(c *ui.Context, label, value string) {
	ui.Column(c).Gap(3).FillWidth().Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(c.Theme().TextMuted)
		ui.Text(c, value).FontSize(13)
	})
}

func networkNumber(value any) string {
	return i18n.FormatNumber(i18n.Locale(), value, i18n.NumberOptions{Integer: true})
}
