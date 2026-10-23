package app

import (
	"context"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/egoist/mygo/ui"
	"time"
)

type syncPage struct {
	projectID, projectName, localPath        string
	expected                                 cliadapter.ConfigContext
	capabilities                             cliadapter.Capabilities
	status                                   *remote.SyncStatus
	receipt                                  *remote.SyncReceipt
	history                                  []syncRecord
	recovery                                 syncRecoveryState
	action                                   string
	review, dryRun, submitted, busy, loading bool
	generation                               uint64
	progressPhase                            string
	err, queryErr                            error
	cancel                                   context.CancelFunc
	done                                     chan struct{}
	results                                  chan syncResult
	progress                                 chan string
}
type syncRecord struct {
	receipt *remote.SyncReceipt
	err     error
}

// syncOperationStages returns only the stable stage projection that is safe to
// keep in desktop storage. The exact receipt remains in the CLI journal and
// is never reconstructed from this summary after a restart.
func (a *App) syncOperationStages() []storage.OperationStage {
	var stages []remote.SyncStage
	if a.sync.receipt != nil {
		stages = a.sync.receipt.Stages
	} else if len(a.sync.history) > 0 {
		for i := len(a.sync.history) - 1; i >= 0; i-- {
			if a.sync.history[i].receipt != nil {
				stages = a.sync.history[i].receipt.Stages
				break
			}
		}
	}
	if len(stages) == 0 && len(a.sync.recovery.receipts) > 0 {
		stages = a.sync.recovery.receipts[len(a.sync.recovery.receipts)-1].Stages
	}
	if len(stages) == 0 {
		return nil
	}
	result := make([]storage.OperationStage, 0, len(stages))
	for _, stage := range stages {
		result = append(result, storage.OperationStage{Name: stage.Name, State: stage.State})
	}
	return result
}

func (p *syncPage) hasOperation() bool {
	return p.submitted || p.receipt != nil || p.err != nil || len(p.history) > 0 || p.recovery.submitted || len(p.recovery.receipts) > 0
}

func (p *syncPage) archiveOperation() {
	if p.receipt != nil || p.err != nil {
		p.history = append(p.history, syncRecord{receipt: p.receipt, err: p.err})
	}
	p.receipt, p.err = nil, nil
}

type syncResult struct {
	kind            string
	recoveryPlan    *remote.SyncRecoveryPlan
	recoveryReceipt *remote.SyncRecoveryReceipt
	generation      uint64
	capabilities    cliadapter.Capabilities
	expected        cliadapter.ConfigContext
	status          remote.SyncStatus
	receipt         remote.SyncReceipt
	err             error
}

func (a *App) syncBlocked() bool {
	return a.mutagenEnvironment.busy || a.login.busy || a.logout.busy || a.setup.busy || a.wireguard.busy || a.account.busy || a.runtimeRecovery.busy || a.accountImport.busy || a.accountDefault.busy || a.closing || a.quitting.Load()
}
func (a *App) openSync() {
	a.page = "sync"
	p := &a.sync
	if p.hasOperation() || p.busy || p.loading {
		return
	}
	project := a.project()
	if project == nil {
		return
	}
	a.sync = syncPage{projectID: project.ID, projectName: project.Name, localPath: project.Path}
	a.refreshSync()
}
func (a *App) refreshSync() {
	p := &a.sync
	if p.loading || p.busy || a.syncBlocked() || p.localPath == "" {
		return
	}
	p.review, p.recovery.plan = false, nil
	p.status, p.queryErr = nil, nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.loading = cancel, make(chan struct{}), make(chan syncResult, 1), true
	results, done, home, path, generation, find := p.results, p.done, a.home, p.localPath, p.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		result := syncResult{generation: generation}
		binary, err := find("agent-remote")
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			err = client.Call(ctx, "capabilities", struct{}{}, &result.capabilities)
			if err == nil && !remote.SyncOperationSupported(result.capabilities, "sync.status") {
				err = &cliadapter.OperationError{Code: "unsupported_method"}
			}
			var expected cliadapter.ConfigContext
			if err == nil {
				err = client.Call(ctx, "context", struct{}{}, &expected)
				result.expected = expected
			}
			if err == nil {
				result.status, err = remote.GetSyncStatus(ctx, client, remote.SyncStatusInput{Expected: expected, LocalPath: path})
			}
		}
		result.err = err
		results <- result
	}()
}
func (a *App) syncActionAvailable(action string) bool {
	p := &a.sync
	project := a.project()
	return !p.submitted && !p.loading && !p.busy && !a.syncBlocked() && p.status != nil && project != nil && project.ID == p.projectID && project.Path == p.localPath && remote.SyncOperationSupported(p.capabilities, "sync."+action) && (action == "ensure" || p.status.Target.SyncSessionID != nil)
}
func (a *App) reviewSync(action string) {
	if !a.syncActionAvailable(action) {
		return
	}
	a.sync.action, a.sync.review = action, true
}
func (a *App) submitSync() {
	p := &a.sync
	if !p.review || !a.syncActionAvailable(p.action) {
		return
	}
	input := remote.SyncActionInput{Confirmed: true, Expected: p.expected, ExpectedTarget: p.status.Target, DryRun: p.dryRun}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.progress, p.busy, p.submitted = cancel, make(chan struct{}), make(chan syncResult, 1), make(chan string, 1), true, true
	p.progressPhase = ""
	p.archiveOperation()
	results, progress, done, home, method, find := p.results, p.progress, p.done, a.home, "sync."+p.action, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var receipt remote.SyncReceipt
		if err == nil {
			receipt, err = remote.RunSyncActionWithProgress(ctx, cliadapter.Client{Binary: binary, Home: home}, method, input, func(event cliadapter.ProgressEvent) error {
				// Keep only the latest phase when rendering lags behind the CLI. The
				// phase is a bounded protocol identifier; event data stays in the
				// adapter and is never copied into desktop state or storage.
				select {
				case progress <- event.Phase:
				default:
					select {
					case <-progress:
					default:
					}
					select {
					case progress <- event.Phase:
					default:
					}
				}
				return nil
			})
		}
		results <- syncResult{receipt: receipt, err: err}
	}()
}
func (a *App) pollSync(c *ui.Context) {
	p := &a.sync
	if !p.loading && !p.busy {
		return
	}
	select {
	case phase := <-p.progress:
		p.progressPhase = phase
		c.After(50 * time.Millisecond)
	case result := <-p.results:
		mutation := p.busy
		p.loading, p.busy = false, false
		p.progressPhase = ""
		if !mutation && result.generation != p.generation {
			return
		}
		if result.kind == "recovery.preview" {
			p.recovery.err, p.recovery.plan = result.err, result.recoveryPlan
			if result.err == nil {
				p.expected = result.expected
				p.action, p.review, p.recovery.submitted = "recovery", true, false
			}
			return
		}
		if result.kind == "recovery.apply" {
			if result.err != nil {
				p.recovery.failures = append(p.recovery.failures, syncRecoveryFailure{plan: result.recoveryPlan, err: result.err})
			}
			p.recovery.err, p.review, p.status, p.recovery.plan, p.queryErr = nil, false, nil, nil, nil
			if result.recoveryReceipt != nil {
				p.recovery.receipts = append(p.recovery.receipts, *result.recoveryReceipt)
			}
			a.invalidateCapabilities()
			return
		}
		if mutation {
			p.err, p.review, p.status = result.err, false, nil
			if result.err == nil {
				p.receipt = &result.receipt
			}
			a.invalidateCapabilities()
		} else {
			p.queryErr = result.err
			p.expected, p.capabilities = result.expected, result.capabilities
			if result.err == nil {
				p.status = &result.status
				p.expected = result.status.Context
				p.capabilities = result.capabilities
			}
		}
	default:
		c.After(50 * time.Millisecond)
	}
}
func (a *App) invalidateSyncContext() {
	p := &a.sync
	p.generation++
	p.status, p.review, p.queryErr, p.recovery.plan = nil, false, fault.CLIContextChanged, nil
	if p.loading && p.cancel != nil {
		p.cancel()
	}
}
func (a *App) finishSync() {
	if a.sync.busy || a.sync.loading {
		return
	}
	a.sync = syncPage{}
	a.page = ""
}
