package app

import (
	"context"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

type syncRecoveryFailure struct {
	plan *remote.SyncRecoveryPlan
	err  error
}

type syncRecoveryState struct {
	failures  []syncRecoveryFailure
	plan      *remote.SyncRecoveryPlan
	receipts  []remote.SyncRecoveryReceipt
	err       error
	submitted bool
}

func (a *App) syncRecoveryAvailable(method string) bool {
	p, project := &a.sync, a.project()
	return !p.loading && !p.busy && !a.syncBlocked() && project != nil && project.ID == p.projectID && project.Path == p.localPath && remote.SyncOperationSupported(p.capabilities, method)
}
func (a *App) previewSyncRecovery() {
	p := &a.sync
	if !a.syncRecoveryAvailable("sync.recovery.preview") {
		return
	}
	p.review, p.recovery.plan = false, nil
	p.recovery.err = nil
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.loading = cancel, make(chan struct{}), make(chan syncResult, 1), true
	results, done, home, path, generation, find := p.results, p.done, a.home, p.localPath, p.generation, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		result := syncResult{kind: "recovery.preview", generation: generation}
		binary, err := find("agent-remote")
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			err = client.Call(ctx, "context", struct{}{}, &result.expected)
			if err == nil {
				var plan remote.SyncRecoveryPlan
				plan, err = remote.PreviewSyncRecovery(ctx, client, remote.SyncRecoveryPreviewInput{Expected: result.expected, LocalPath: path})
				if err == nil {
					result.recoveryPlan = &plan
				}
			}
		}
		result.err = err
		results <- result
	}()
}
func (a *App) canApplySyncRecovery() bool {
	p := &a.sync
	return p.review && p.action == "recovery" && p.recovery.plan != nil && p.recovery.plan.Recoverable && !p.recovery.submitted && a.syncRecoveryAvailable("sync.recovery.apply")
}
func (a *App) applySyncRecovery() {
	p := &a.sync
	if !a.canApplySyncRecovery() {
		return
	}
	input := remote.SyncRecoveryApplyInput{Confirmed: true, Expected: p.recovery.plan.Context, ExpectedPlan: *p.recovery.plan}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.done, p.results, p.busy = cancel, make(chan struct{}), make(chan syncResult, 1), true
	p.recovery.submitted, p.submitted, p.recovery.err = true, true, nil
	results, done, home, find := p.results, p.done, a.home, a.findExecutable
	go func() {
		defer close(done)
		defer cancel()
		result := syncResult{kind: "recovery.apply", recoveryPlan: &input.ExpectedPlan}
		binary, err := find("agent-remote")
		if err == nil {
			var receipt remote.SyncRecoveryReceipt
			receipt, err = remote.ApplySyncRecovery(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
			if err == nil {
				result.recoveryReceipt = &receipt
			}
		}
		result.err = err
		results <- result
	}()
}
func (a *App) freshSyncAfterRecovery() {
	p := &a.sync
	if !a.syncRecoveryAvailable("sync.status") || len(p.recovery.receipts) == 0 || p.recovery.receipts[len(p.recovery.receipts)-1].Outcome != "completed" {
		return
	}
	// A fresh query permits a new separately reviewed operation, while keeping
	// all previous receipts. It never dispatches ensure or replays recovery.
	p.submitted, p.dryRun = false, false
	a.refreshSync()
}
