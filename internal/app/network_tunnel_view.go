package app

import (
	"strconv"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) networkTunnelView(c *ui.Context) {
	ui.Text(c, locales.WireGuardTunnelPreviewOnly()).FontSize(12).TextColor(c.Theme().TextMuted)
	query := &a.network.plan
	if query.report == nil {
		ui.Text(c, locales.WireGuardTunnelPreviewHelp()).FontSize(12)
		ui.Text(c, locales.WireGuardTunnelActionLabel()).Bold()
		ui.Row(c).Gap(10).Wrap().Children(func() {
			for _, action := range []string{"up", "down"} {
				ui.Radio(c, &a.network.action, action, locales.WireGuardTunnelAction(action)).Disabled(query.busy || a.network.tunnelApply.busy).OnChange(a.invalidateTunnelPlan)
			}
		})
		a.networkConfigForm(c)
		if query.stale {
			ui.Text(c, locales.WireGuardTunnelPlanStale()).TextColor(c.Theme().TextMuted)
		} else if wireguardUnsupported(query.err) {
			ui.Text(c, locales.WireGuardTunnelPlanUnsupported()).TextColor(c.Theme().Danger)
		} else {
			networkQueryNotice(c, query.busy, false, query.err)
		}
		return
	}
	plan := query.report
	ui.Text(c, locales.WireGuardTunnelPlanSummary()).Bold()
	ui.Text(c, locales.WireGuardTunnelAction(plan.Action))
	if query.err != nil {
		networkQueryNotice(c, query.busy, query.stale, query.err)
	}
	if a.network.tunnelApply.busy {
		ui.Text(c, locales.WireGuardTunnelApplying()).TextColor(c.Theme().TextMuted)
	} else if a.network.tunnelApply.err != nil {
		networkQueryNotice(c, a.network.tunnelApply.busy, false, a.network.tunnelApply.err)
	} else if receipt := a.network.tunnelApply.report; receipt != nil {
		ui.Text(c, locales.WireGuardTunnelApplied(receipt.Action)).TextColor(c.Theme().Success)
	}
	if plan.Platform == "windows" {
		ui.Text(c, locales.WireGuardTunnelWindowsSemantics(plan.Action)).FontSize(12)
	}
	ui.Text(c, locales.WireGuardTunnelPrerequisites()).Bold()
	for _, issue := range plan.Issues {
		ui.Text(c, locales.WireGuardTunnelPrerequisite(issue)).FontSize(13)
	}
	ui.Text(c, locales.WireGuardTunnelTargetState(plan.Target.State)).Bold()
	tunnelField(c, "logical_name", plan.Target.LogicalName)
	if plan.Target.Reason != nil {
		ui.Text(c, locales.WireGuardTunnelTargetReason(*plan.Target.Reason)).FontSize(12)
	}
	if identity := plan.Target.Identity; identity != nil {
		switch {
		case identity.Linux != nil:
			tunnelField(c, "target_name", identity.Linux.Name)
		case identity.MacOS != nil:
			tunnelField(c, "target_name", identity.MacOS.Name)
		case identity.Windows != nil:
			tunnelField(c, "service_name", identity.Windows.ServiceName)
		}
	}
	ui.Text(c, locales.WireGuardConfigState(plan.Config.State))
	tunnelField(c, "config_path", plan.Config.Path)
	ui.Button(c, locales.Details()).OnClick(func() { a.network.planDetails = !a.network.planDetails })
	if a.network.planDetails {
		networkTunnelDetails(c, *plan)
	}
}

func networkTunnelDetails(c *ui.Context, plan remote.WireGuardTunnelPlan) {
	ui.Text(c, locales.WireGuardTunnelSnapshotHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	networkField(c, locales.WireGuardHome(), plan.Home)
	if binding := plan.Binding; binding != nil {
		tunnelField(c, "file_identity", binding.FileIdentity)
		tunnelField(c, "content_sha256", binding.ContentSHA256)
		tunnelField(c, "public_key", binding.PublicKey)
	}
	ui.Text(c, locales.WireGuardTools()).Bold()
	for _, tool := range plan.Tools {
		networkTool(c, remote.WireGuardTool{Name: tool.Name, State: tool.State, Path: tool.Path})
		tunnelField(c, "tool_identity", tunnelOptional(tool.Identity))
		tunnelField(c, "tool_sha256", tunnelOptional(tool.SHA256))
	}
	tunnelField(c, "status_state", locales.WireGuardTunnelStatusState(plan.Target.StatusState))
	if identity := plan.Target.Identity; identity != nil {
		networkTunnelIdentity(c, *identity)
	}
}

func networkTunnelIdentity(c *ui.Context, identity remote.WireGuardTargetIdentity) {
	switch {
	case identity.Linux != nil:
		target := identity.Linux
		tunnelField(c, "index", networkNumber(target.Index))
		tunnelField(c, "namespace", target.Namespace)
		tunnelField(c, "public_key", tunnelOptional(target.PublicKey))
	case identity.MacOS != nil:
		target := identity.MacOS
		tunnelField(c, "index", networkNumber(target.Index))
		tunnelField(c, "mapping_identity", target.MappingIdentity)
		tunnelField(c, "socket_identity", target.SocketIdentity)
		tunnelField(c, "public_key", tunnelOptional(target.PublicKey))
	case identity.Windows != nil:
		target := identity.Windows
		tunnelField(c, "binary_path", target.BinaryPath)
		tunnelField(c, "config_path", target.ConfigPath)
		tunnelField(c, "config_identity", tunnelOptional(target.ConfigIdentity))
		tunnelField(c, "service_state", locales.WireGuardTunnelServiceState(strconv.FormatUint(uint64(target.ServiceState), 10)))
		tunnelField(c, "start_type", locales.WireGuardTunnelServiceStartType(strconv.FormatUint(uint64(target.StartType), 10)))
		tunnelField(c, "account", target.Account)
		tunnelField(c, "public_key", tunnelOptional(target.PublicKey))
	}
}

func tunnelField(c *ui.Context, field, value string) {
	networkField(c, locales.WireGuardTunnelPlanField(field), value)
}
func tunnelOptional(value *string) string {
	if value != nil {
		return *value
	}
	return locales.WireGuardUnavailableValue()
}
