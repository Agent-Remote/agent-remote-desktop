package app

import (
	"context"
	"errors"
	"sort"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/egoist/mygo/ui"
)

func (a *App) saveWorker() {
	defer close(a.saveDone)
	for state := range a.saveQueue {
		if err := a.store.Save(state); err != nil {
			a.saveError.Store(&saveFailure{err: err})
		} else {
			a.saveError.Store(nil)
		}
		a.savePending.Add(-1)
	}
}

func (a *App) persist() {
	a.captureExits()
	snapshot := a.state.Clone()
	snapshot.Operations = a.operationRecords()
	snapshot.DedicatedOperations = a.dedicatedOperationRecords()
	snapshot.RemoteStopRecovery = a.remoteStopRecoveryRecord()
	snapshot.RemoteCreateRecovery = a.remoteCreateRecoveryRecord()
	snapshot.RemoteDeleteRecovery = a.remoteDeleteRecoveryRecord()
	a.layoutSaveAt = time.Time{}
	a.savePending.Add(1)
	select {
	case a.saveQueue <- snapshot:
	default:
		select {
		case <-a.saveQueue:
			a.savePending.Add(-1)
		default:
		}
		a.saveQueue <- snapshot
	}
}

// remoteCreateRecoveryRecord keeps only the exact caller key and reviewed
// configuration scope. The CLI owns the durable create receipt and journal.
func (a *App) remoteCreateRecoveryRecord() *storage.RemoteCreateRecovery {
	if a.remote.createQueryKey == "" || !a.remote.createRecovery || a.remote.createContext.Home != a.home || a.remote.createContext.ServerURL == nil || a.remote.createContext.DeviceID == nil {
		// Opening a fresh create review must not discard an older exact key.
		// Only the CLI journal owns that request; it remains queryable until a
		// future explicit forget action is added.
		if a.state.RemoteCreateRecovery != nil {
			recovery := *a.state.RemoteCreateRecovery
			return &recovery
		}
		return nil
	}
	return &storage.RemoteCreateRecovery{
		RequestID: a.remote.createQueryKey,
		Home:      a.remote.createContext.Home,
		ServerURL: *a.remote.createContext.ServerURL,
		DeviceID:  *a.remote.createContext.DeviceID,
		UpdatedAt: time.Now().UTC(),
	}
}

// remoteStopRecoveryRecord keeps only the exact caller key and reviewed public
// scope. The CLI owns the durable receipt; this pointer lets a restarted
// desktop ask for that same receipt without submitting a second stop.
func (a *App) remoteStopRecoveryRecord() *storage.RemoteStopRecovery {
	if a.remote.stopRequestKey == "" || a.remote.stopContext.Home != a.home || a.remote.stopContext.ServerURL == nil || a.remote.stopContext.DeviceID == nil || a.remote.stopSession.ID == "" || a.remote.stopSession.NodeID == "" || a.remote.stopSession.Backend == "" {
		// Starting a new stop review must not discard an older exact caller
		// key before the new request has produced its own receipt. The CLI
		// journal remains the durable owner; keep the previous pointer so a
		// restart can still query that exact request instead of silently losing
		// the only recovery affordance.
		if a.state.RemoteStopRecovery != nil {
			recovery := *a.state.RemoteStopRecovery
			return &recovery
		}
		return nil
	}
	record := &storage.RemoteStopRecovery{
		RequestID:      a.remote.stopRequestKey,
		Home:           a.remote.stopContext.Home,
		ServerURL:      *a.remote.stopContext.ServerURL,
		DeviceID:       *a.remote.stopContext.DeviceID,
		SessionID:      a.remote.stopSession.ID,
		NodeID:         a.remote.stopSession.NodeID,
		RuntimeBackend: a.remote.stopSession.Backend,
		UpdatedAt:      time.Now().UTC(),
	}
	if a.remote.stopSession.StopTaskID != nil {
		record.StopTaskID = *a.remote.stopSession.StopTaskID
	}
	return record
}

// remoteDeleteRecoveryRecord keeps only the original caller key, exact
// authentication context and reviewed collection digest. The CLI owns the
// durable receipt; this pointer can only drive a query for the same request.
func (a *App) remoteDeleteRecoveryRecord() *storage.RemoteDeleteRecovery {
	if a.remote.deleteRequestKey == "" || a.remote.deleteContext.Home != a.home ||
		a.remote.deleteContext.ServerURL == nil || a.remote.deleteContext.DeviceID == nil ||
		a.remote.deleteSelection.ToolType != "claude" || len(a.remote.deleteSessions) == 0 ||
		a.remote.deleteDigest == "" || a.remote.deleteFingerprint == "" {
		// Starting a new review must not discard an older exact key before the
		// new request has produced its own caller key.
		if a.state.RemoteDeleteRecovery != nil {
			recovery := *a.state.RemoteDeleteRecovery
			recovery.SessionIDs = append([]string(nil), recovery.SessionIDs...)
			return &recovery
		}
		return nil
	}
	ids := make([]string, 0, len(a.remote.deleteSessions))
	for _, session := range a.remote.deleteSessions {
		ids = append(ids, session.ID)
	}
	sort.Strings(ids)
	return &storage.RemoteDeleteRecovery{
		RequestID: a.remote.deleteRequestKey, Home: a.remote.deleteContext.Home,
		ServerURL: *a.remote.deleteContext.ServerURL, DeviceID: *a.remote.deleteContext.DeviceID,
		ToolType: a.remote.deleteSelection.ToolType, SessionIDs: ids,
		SelectionDigest: a.remote.deleteDigest, IntentFingerprint: a.remote.deleteFingerprint,
		UpdatedAt: time.Now().UTC(),
	}
}

func (a *App) dedicatedOperationRecords() []storage.DedicatedOperationRecord {
	if len(a.dedicatedOperations) == 0 {
		return nil
	}
	records := make([]storage.DedicatedOperationRecord, 0, len(a.dedicatedOperations))
	for _, operation := range a.dedicatedOperations {
		updated := operation.updatedAt
		if updated.IsZero() {
			updated = time.Now().UTC()
		}
		records = append(records, storage.DedicatedOperationRecord{
			ID: operation.id, Command: operation.command,
			Status: operation.status, UpdatedAt: updated,
		})
	}
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return records
}

func (a *App) operationRecords() []storage.OperationRecord {
	if len(a.operationJournal) == 0 {
		return nil
	}
	ids := make([]string, 0, len(a.operationJournal))
	for id := range a.operationJournal {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	records := make([]storage.OperationRecord, 0, len(ids))
	for _, id := range ids {
		if operationPersistable(id) {
			record := a.operationJournal[id]
			// Mutagen environment receipts contain ordered lifecycle stages.
			// Keep only that bounded public projection in desktop state so the
			// operation center can explain an interrupted restart without
			// copying the plan, paths, inventory, or CLI receipt.
			if id == "mutagen" {
				record.Stages = mutagenOperationStages(a.mutagenEnvironment.records)
			}
			records = append(records, record)
		}
	}
	return records
}

func mutagenOperationStages(records []mutagenEnvironmentRecord) []storage.OperationStage {
	if len(records) == 0 || records[len(records)-1].receipt == nil {
		// A newer failed/unknown attempt must not inherit the stage list from
		// an older receipt and appear to have reached those stages.
		return nil
	}
	stages := records[len(records)-1].receipt.Stages
	if len(stages) == 0 {
		return nil
	}
	if len(stages) > 8 {
		stages = stages[:8]
	}
	result := make([]storage.OperationStage, 0, len(stages))
	for _, stage := range stages {
		result = append(result, storage.OperationStage{Name: stage.Name, State: stage.State})
	}
	return result
}

func (a *App) Close() error {
	if a.closing {
		return nil
	}
	a.closing = true
	// Capture an in-flight sync mutation before cancellation. The worker may be
	// waiting on the CLI when the user explicitly quits, and there is no safe
	// way to infer whether the remote mutation reached the server after the
	// process is terminated. Persisting its bounded lifecycle projection lets a
	// restarted desktop restore it as "attention" (and require fresh
	// observation) instead of silently losing the operation when no UI frame
	// happened between submit and quit.
	a.syncOperationJournal()
	// Prevent a forward startup that wins a cancellation race from publishing
	// a new runner after the existing runner snapshot is collected.
	a.network.forwardClosing.Store(true)
	if a.runtimeRecovery.cancel != nil {
		a.runtimeRecovery.cancel()
		<-a.runtimeRecovery.done
	}
	if a.mutagenEnvironment.cancel != nil {
		a.mutagenEnvironment.cancel()
		<-a.mutagenEnvironment.done
	}
	if a.sync.cancel != nil {
		a.sync.cancel()
		<-a.sync.done
	}
	if a.accountImport.cancel != nil {
		a.accountImport.cancel()
		<-a.accountImport.done
	}
	if a.account.cancel != nil {
		a.account.cancel()
		<-a.account.done
	}
	if a.accountDefault.cancel != nil {
		a.accountDefault.cancel()
		<-a.accountDefault.done
	}
	a.network.check.close()
	a.network.status.close()
	a.network.bridge.close()
	a.network.plan.close()
	a.network.tunnelApply.close()
	a.network.forwards.close()
	// Drain the launch/action worker first. It may still be between the Rust
	// startup event and insertion into forwardRunners; collecting the map before
	// this barrier can leak a child process after the app has closed.
	if a.network.forwardActionCancel != nil {
		a.network.forwardActionCancel()
		if a.network.forwardActionDone != nil {
			<-a.network.forwardActionDone
		}
	}
	_ = a.stopLocalPortForwards()
	a.waitLocalPortForwardStops()
	if a.capabilities.cancel != nil {
		a.capabilities.cancel()
		<-a.capabilities.done
	}
	if a.credentials.cancel != nil {
		a.credentials.cancel()
		<-a.credentials.done
	}
	if a.diagnostics.cancel != nil {
		a.diagnostics.cancel()
		<-a.diagnostics.done
	}
	if a.dependencies.cancel != nil {
		a.dependencies.cancel()
		<-a.dependencies.done
	}
	if a.commandDiagnostics.cancel != nil {
		a.commandDiagnostics.cancel()
		if a.commandDiagnostics.done != nil {
			<-a.commandDiagnostics.done
		}
	}
	if a.wireguard.cancel != nil {
		a.wireguard.cancel()
		<-a.wireguard.done
	}
	if a.setup.cancel != nil {
		a.setup.cancel()
		<-a.setup.done
	}
	if a.logout.cancel != nil {
		a.logout.cancel()
		<-a.logout.done
	}
	a.login.password, a.login.totp = "", ""
	if a.login.cancel != nil {
		a.login.cancel()
		<-a.login.done
	}
	if a.remote.cancel != nil {
		a.remote.cancel()
		<-a.remote.done
	}
	if a.remote.createCancel != nil && a.remote.createDone != nil {
		waitRemoteWorker(a.remote.createCancel, a.remote.createDone)
		// The worker sends its result after closing the process but before
		// returning. Drain that buffered result before the final snapshot so an
		// already accepted request cannot lose its caller key merely because
		// the UI had not rendered another frame yet.
		a.pollRemoteCreate(nil)
	}
	if a.remote.stopCancel != nil && a.remote.stopDone != nil {
		waitRemoteWorker(a.remote.stopCancel, a.remote.stopDone)
		a.finishRemoteStop(nil)
	}
	if a.remote.deleteCancel != nil && a.remote.deleteDone != nil {
		waitRemoteWorker(a.remote.deleteCancel, a.remote.deleteDone)
		a.finishRemoteDelete(nil)
	}
	if a.nodes.cancel != nil {
		a.nodes.cancel()
	}
	// Node discovery runs in a separate worker. Wait for its cancellation path
	// before closing the store so a late result cannot race application teardown
	// or be retained as a live query after an explicit quit.
	if a.nodes.done != nil {
		<-a.nodes.done
	}
	// A launch worker may have already created a managed terminal while its
	// frame result is still buffered. Wait for that worker before stopping all
	// sessions, then discard the UI-only result under the closing guard.
	a.launches.Wait()
	a.pollLaunch()
	// Stop managed Agent terminals before the final state snapshot. A local
	// process may only publish its final exit code after the PTY shutdown
	// completes; persisting first would leave a restarted workbench with an
	// incomplete history entry even though explicit quit successfully stopped
	// the process.
	sessionRecords, sessionErr := a.sessions.CloseWithRecords()
	a.captureExitsFrom(sessionRecords)
	a.persist()
	close(a.saveQueue)
	<-a.saveDone
	if a.localeOff != nil {
		a.localeOff()
	}
	if a.tray != nil {
		a.tray.Destroy()
		a.tray = nil
	}
	err := sessionErr
	if failure := a.saveError.Load(); failure != nil {
		err = errors.Join(err, failure.err)
	}
	return errors.Join(err, a.store.Close())
}

// waitRemoteWorker gives an already-completing lifecycle mutation a short
// grace period before cancellation. This preserves a caller key when the CLI
// has emitted its response but the UI has not rendered the next frame yet,
// while still bounding application shutdown for a genuinely stalled request.
func waitRemoteWorker(cancel context.CancelFunc, done <-chan struct{}) {
	if cancel == nil || done == nil {
		return
	}
	select {
	case <-done:
		return
	case <-time.After(time.Second):
		cancel()
		<-done
	}
}

// Debounce pointer-driven layout changes; explicit hide/quit flushes immediately.
func (a *App) layoutChanged() { a.layoutSaveAt = time.Now().Add(180 * time.Millisecond) }

func (a *App) saveLayoutWhenIdle(c *ui.Context) {
	if a.layoutSaveAt.IsZero() {
		return
	}
	if remaining := time.Until(a.layoutSaveAt); remaining > 0 {
		c.After(remaining)
	} else {
		a.persist()
	}
}

func (a *App) captureExits() {
	a.captureExitsFrom(a.sessions.Records())
}

// captureExitsFrom also accepts a pre-close snapshot. Manager.Close releases
// its records after every process is stopped, so App.Close must retain the
// process handles long enough to observe their final exit codes.
func (a *App) captureExitsFrom(records []session.Record) {
	for _, record := range records {
		if record.Exited {
			for i := range a.state.Sessions {
				if a.state.Sessions[i].ID == record.ID {
					code := record.ExitCode
					a.state.Sessions[i].ExitCode = &code
					break
				}
			}
			continue
		}
		select {
		case <-record.Process.Done():
			for i := range a.state.Sessions {
				if a.state.Sessions[i].ID == record.ID {
					code := record.Process.ExitCode()
					a.state.Sessions[i].ExitCode = &code
					break
				}
			}
		default:
		}
	}
}
