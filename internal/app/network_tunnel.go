package app

import (
	"context"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func (a *App) previewTunnelPlan() {
	if a.network.plan.busy || a.networkBlocked() {
		return
	}
	input := remote.WireGuardTunnelPlanInput{ExpectedHome: a.home, Action: a.network.action}
	if a.network.configMode == "custom" {
		if !filepath.IsAbs(a.network.config) {
			return
		}
		config := a.network.config
		input.Config = &config
	}
	a.network.planDetails = false
	home, find := a.home, a.findExecutable
	a.network.plan.start(func(ctx context.Context) (remote.WireGuardTunnelPlan, error) {
		binary, err := find("agent-remote")
		if err != nil {
			return remote.WireGuardTunnelPlan{}, err
		}
		return remote.PlanWireGuardTunnel(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
	})
}
