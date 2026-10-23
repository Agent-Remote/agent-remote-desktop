package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/egoist/mygo/ui"
)

// dependenciesPage contains only the read-only projection returned by the
// dependencies.inspect control method. It deliberately has no install or
// repair action: those mutations stay in the CLI's explicitly reviewed
// workflows.
type dependenciesPage struct {
	report         remote.DiagnosticReport
	err            error
	busy, loaded   bool
	verifyVersions bool
	cancel         context.CancelFunc
	done           chan struct{}
	results        chan dependenciesResult
	generation     uint64
}

type dependenciesResult struct {
	generation uint64
	report     remote.DiagnosticReport
	err        error
}

func (a *App) openDependencies() {
	a.page = "dependencies"
	if !a.dependencies.loaded && !a.dependencies.busy {
		a.refreshDependencies(false)
	}
}

func (a *App) refreshDependencies(verifyVersions bool) {
	if a.dependencies.busy || a.closing || a.quitting.Load() {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.dependencies.cancel = cancel
	a.dependencies.done = make(chan struct{})
	a.dependencies.results = make(chan dependenciesResult, 1)
	a.dependencies.busy, a.dependencies.err = true, nil
	results, done, home, generation, find := a.dependencies.results, a.dependencies.done, a.home, a.dependencies.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var report remote.DiagnosticReport
		if err == nil {
			report, err = remote.InspectDependencies(ctx, cliadapter.Client{Binary: binary, Home: home}, verifyVersions)
		}
		results <- dependenciesResult{generation: generation, report: report, err: err}
	}()
}

func (a *App) pollDependencies(c *ui.Context) {
	if !a.dependencies.busy {
		return
	}
	select {
	case result := <-a.dependencies.results:
		a.dependencies.busy = false
		if result.generation == a.dependencies.generation {
			a.dependencies.loaded = true
			a.dependencies.report, a.dependencies.err = result.report, result.err
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) invalidateDependencies() {
	a.dependencies.generation++
	a.dependencies.loaded = false
	a.dependencies.report = remote.DiagnosticReport{}
	a.dependencies.err = nil
	if a.dependencies.busy {
		a.dependencies.cancel()
	} else if a.page == "dependencies" {
		a.refreshDependencies(false)
	}
}
