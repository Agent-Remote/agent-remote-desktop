package app

import (
	"context"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/egoist/mygo/ui"
)

type capabilitiesPage struct {
	snapshot     capability.Snapshot
	busy, loaded bool
	tab, filter  string
	cancel       context.CancelFunc
	done         chan struct{}
	results      chan capabilityResult
	generation   uint64
}

type capabilityResult struct {
	generation uint64
	snapshot   capability.Snapshot
}

func (a *App) openCapabilities() { a.page = "capabilities"; a.refreshCapabilities() }

func (a *App) refreshCapabilities() {
	if a.capabilities.busy || a.closing || a.quitting.Load() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.capabilities.cancel = cancel
	a.capabilities.done = make(chan struct{})
	a.capabilities.results = make(chan capabilityResult, 1)
	a.capabilities.busy = true
	results, done, home, generation, find := a.capabilities.results, a.capabilities.done, a.home, a.capabilities.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, _ := find("agent-remote")
		launcher, _ := find("fclaude")
		snapshot := capability.Inspect(ctx, cliadapter.Client{Binary: binary, Home: home}, launcher)
		results <- capabilityResult{generation, snapshot}
	}()
}

func (a *App) pollCapabilities(c *ui.Context) {
	if !a.capabilities.busy {
		return
	}
	select {
	case result := <-a.capabilities.results:
		a.capabilities.busy = false
		if result.generation == a.capabilities.generation {
			a.capabilities.snapshot, a.capabilities.loaded = result.snapshot, true
			if a.page == "skills" && a.skills.exportCheckpoint != nil {
				checkpoint := *a.skills.exportCheckpoint
				a.skills.exportCheckpoint = nil
				a.openSkillStateExport(checkpoint)
			}
			if a.page == "skills" && !a.skills.busy && !a.skills.loaded {
				a.refreshSkills()
			}
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) invalidateCapabilities() {
	a.capabilities.generation++
	a.capabilities.snapshot = capability.Snapshot{ProtocolError: fault.CLIContextChanged}
	a.capabilities.loaded = true
	if a.capabilities.busy {
		a.capabilities.cancel()
	}
}

func (a *App) operationAssessment(method string) capability.Assessment {
	snapshot := a.capabilities.snapshot
	if snapshot.ProtocolError != nil {
		return capability.Assessment{State: "unavailable"}
	}
	if !snapshot.Capabilities.MatchesHostPlatform() {
		return capability.Assessment{State: "platform_unsupported"}
	}
	observed := false
	var outcome error
	if strings.HasPrefix(method, "accounts.runtime_recovery.") && a.runtimeRecovery.authority != nil {
		authority := a.runtimeRecovery.authority
		if snapshot.Config.Home == authority.Context.Home && snapshot.Config.ServerURL != nil && *snapshot.Config.ServerURL == authority.Context.ServerURL {
			observed = true
			if !authority.Admin && method != "accounts.runtime_recovery.authority" {
				outcome = &cliadapter.OperationError{Code: "permission_denied"}
			}
		}
	}
	if a.remote.loaded && !a.remote.busy && a.remote.err == nil && matchingCapabilityContext(snapshot.Config, a.remote.snapshot.Config) {
		switch method {
		case "accounts.list":
			observed, outcome = true, a.remote.snapshot.AccountsError
		case "sessions.list":
			observed, outcome = true, a.remote.snapshot.SessionsError
		}
	}
	return capability.Assess(snapshot.Capabilities, snapshot.Config, snapshot.ContextError, method, observed, outcome)
}

func matchingCapabilityContext(a, b cliadapter.ConfigContext) bool {
	return a.Home == b.Home && a.ServerURL != nil && b.ServerURL != nil && *a.ServerURL == *b.ServerURL && a.DeviceID != nil && b.DeviceID != nil && *a.DeviceID == *b.DeviceID
}
