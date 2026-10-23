package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

type diagnosticsPage struct {
	report               remote.DiagnosticReport
	err                  error
	busy, loaded, online bool
	cancel               context.CancelFunc
	done                 chan struct{}
	results              chan diagnosticsResult
	generation           uint64
}

type diagnosticsResult struct {
	generation uint64
	report     remote.DiagnosticReport
	err        error
}

func (a *App) openDiagnostics() {
	a.page = "diagnostics"
	if !a.diagnostics.loaded && !a.diagnostics.busy {
		a.diagnostics.online = true
		// Opening the page is passive. Network/version probes require the
		// clearly labeled controls, so navigation does not run external tools.
		a.refreshDiagnostics(false, false, false)
	}
}

func (a *App) refreshDiagnostics(online, versions, deep bool) {
	if a.diagnostics.busy || a.closing || a.quitting.Load() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.diagnostics.cancel = cancel
	a.diagnostics.done = make(chan struct{})
	a.diagnostics.results = make(chan diagnosticsResult, 1)
	a.diagnostics.busy, a.diagnostics.err = true, nil
	results, done, home, generation := a.diagnostics.results, a.diagnostics.done, a.home, a.diagnostics.generation
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var report remote.DiagnosticReport
		if err == nil {
			report, err = remote.InspectDiagnostics(ctx, cliadapter.Client{Binary: binary, Home: home}, online, versions, deep)
		}
		results <- diagnosticsResult{generation, report, err}
	}()
}

func (a *App) pollDiagnostics(c *ui.Context) {
	if !a.diagnostics.busy {
		return
	}
	select {
	case result := <-a.diagnostics.results:
		a.diagnostics.busy = false
		if result.generation == a.diagnostics.generation {
			a.diagnostics.loaded = true
			a.diagnostics.report, a.diagnostics.err = result.report, result.err
		} else if a.page == "diagnostics" {
			a.refreshDiagnostics(false, false, false)
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) invalidateDiagnostics() {
	a.invalidateDependencies()
	a.invalidateCommandDiagnostics()
	a.diagnostics.generation++
	a.diagnostics.loaded = false
	a.diagnostics.report = remote.DiagnosticReport{}
	a.diagnostics.err = nil
	if a.diagnostics.busy {
		a.diagnostics.cancel()
	} else if a.page == "diagnostics" {
		a.refreshDiagnostics(false, false, false)
	}
}
