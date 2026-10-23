package app

import (
	"errors"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// operationEntry is the safe projection used by the operation center. It
// deliberately contains no request keys, paths, account IDs, argv, or receipt
// payloads. Those remain on the owning page and in the CLI's durable journal.
type operationEntry struct {
	id          string
	title       string
	details     string
	status      string
	open        func()
	journalOnly bool
}

func (a *App) openOperations() { a.page = "operations" }

func operationState(busy bool, err error, complete bool) string {
	if busy {
		return "running"
	}
	if err != nil {
		return "failed"
	}
	if complete {
		return "completed"
	}
	return "attention"
}

func operationEntryFor(id, title string, busy bool, err error, complete, present bool, open func()) (operationEntry, bool) {
	if !present {
		return operationEntry{}, false
	}
	return operationEntry{id: id, title: title, status: operationState(busy, err, complete), open: open}, true
}

func operationReceiptState(phase string) string {
	switch phase {
	case "unknown", "unresolved", "pending", "accepted", "running", "stopping", "saving", "interrupted":
		return "attention"
	case "failed":
		return "failed"
	default:
		return "completed"
	}
}

// liveOperationEntries builds the projection from the current page workers.
// It intentionally does not include persisted records, which are merged only
// by operationEntries after this process has restarted.
func (a *App) liveOperationEntries() []operationEntry {
	entries := make([]operationEntry, 0, 16)
	add := func(id, title string, busy bool, err error, complete, present bool, open func()) {
		if item, ok := operationEntryFor(id, title, busy, err, complete, present, open); ok {
			entries = append(entries, item)
		}
	}

	add("login", locales.OperationLogin(), a.login.busy, a.login.err, a.login.receipt != nil, a.login.busy || a.login.err != nil || a.login.receipt != nil, a.openLogin)
	add("logout", locales.OperationLogout(), a.logout.busy, a.logout.err, a.logout.receipt != nil, a.logout.busy || a.logout.err != nil || a.logout.receipt != nil, a.openLogout)
	add("account", locales.OperationAccount(), a.account.busy, a.account.err, a.account.receipt != nil, a.account.submitted || a.account.busy || a.account.err != nil || a.account.receipt != nil, a.openAccountOperation)
	add("default-account", locales.OperationDefaultAccount(), a.accountDefault.busy, a.accountDefault.err, a.accountDefault.receipt != nil, a.accountDefault.submitted || a.accountDefault.busy || a.accountDefault.err != nil || a.accountDefault.receipt != nil, func() { a.page = "account-default" })
	add("credentials", locales.OperationCredentials(), a.credentials.mutationBusy, a.credentials.err, a.credentials.receipt != nil, a.credentials.mutationBusy || a.credentials.err != nil || a.credentials.receipt != nil, a.openCredentials)

	importErr, importComplete := a.accountImport.err, len(a.accountImport.records) > 0
	for _, record := range a.accountImport.records {
		if record.err != nil {
			importErr = record.err
		}
		if record.receipt == nil {
			importComplete = false
		}
	}
	add("account-import", locales.OperationAccountImport(), a.accountImport.busy, importErr, importComplete, a.accountImport.busy || a.accountImport.submitted || importErr != nil || len(a.accountImport.records) > 0, a.openAccountImportHistory)

	recoveryErr, recoveryComplete := a.runtimeRecovery.err, len(a.runtimeRecovery.records) > 0
	for _, record := range a.runtimeRecovery.records {
		if record.err != nil {
			recoveryErr = record.err
		}
		if record.receipt == nil {
			recoveryComplete = false
		}
	}
	add("runtime-recovery", locales.OperationRuntimeRecovery(), a.runtimeRecovery.busy, recoveryErr, recoveryComplete, a.runtimeRecovery.busy || a.runtimeRecovery.submitted || recoveryErr != nil || len(a.runtimeRecovery.records) > 0, a.openRuntimeRecovery)

	syncComplete := a.sync.receipt != nil || len(a.sync.history) > 0 || len(a.sync.recovery.receipts) > 0
	syncErr := errors.Join(a.sync.err, a.sync.recovery.err)
	for _, record := range a.sync.history {
		syncErr = errors.Join(syncErr, record.err)
	}
	for _, failure := range a.sync.recovery.failures {
		syncErr = errors.Join(syncErr, failure.err)
	}
	add("sync", locales.OperationSync(), a.sync.busy, syncErr, syncComplete, a.sync.busy || a.sync.submitted || syncErr != nil || syncComplete, a.openSync)
	if a.sync.busy && a.sync.progressPhase != "" {
		for i := range entries {
			if entries[i].id == "sync" {
				entries[i].details = locales.OperationProgress(a.sync.progressPhase)
				break
			}
		}
	}

	mutagenErr, mutagenComplete := a.mutagenEnvironment.err, len(a.mutagenEnvironment.records) > 0
	for _, record := range a.mutagenEnvironment.records {
		if record.err != nil {
			mutagenErr = record.err
		}
		if record.receipt == nil {
			mutagenComplete = false
		}
	}
	add("mutagen", locales.OperationMutagen(), a.mutagenEnvironment.busy, mutagenErr, mutagenComplete, a.mutagenEnvironment.busy || a.mutagenEnvironment.submitted || mutagenErr != nil || len(a.mutagenEnvironment.records) > 0, a.openMutagenEnvironment)
	add("setup", locales.OperationSetup(), a.setup.busy, a.setup.err, a.setup.receipt != nil, a.setup.submitted || a.setup.busy || a.setup.err != nil || a.setup.receipt != nil, a.openLocalSetup)
	initComplete := a.initWizard.stage == initComplete
	add("init", locales.InitWizardOperation(), a.initWizard.busy, a.initWizard.err, initComplete, a.initWizard.busy || a.initWizard.err != nil || initComplete, a.openInitWizard)
	add("wireguard", locales.OperationWireGuard(), a.wireguard.busy, a.wireguard.err, a.wireguard.receipt != nil, a.wireguard.submitted || a.wireguard.busy || a.wireguard.err != nil || a.wireguard.receipt != nil, a.openWireGuard)
	add("device", locales.OperationDevice(), a.device.busy, a.device.err, a.device.receipt != nil, a.device.busy || a.device.err != nil || a.device.receipt != nil, a.openDevice)
	for _, item := range a.dedicatedOperations {
		op := item
		complete := op.status == "completed"
		operationErr := op.err
		if op.status == "failed" && operationErr == nil {
			operationErr = errors.New("managed terminal exited with a non-zero status")
		}
		entry, ok := operationEntryFor("dedicated-cli-"+op.id, locales.DedicatedOperationTitle(op.command), op.status == "pending" || op.status == "running", operationErr, complete, true, func() { a.page = "" })
		if ok {
			entry.details = locales.DedicatedOperationDetailsHelp()
			// Once a dedicated terminal is no longer active, the desktop has no
			// receipt or owning form to reopen. Keep the summary clearable while
			// avoiding an action that could imply replay or receipt recovery.
			entry.open = nil
			entry.journalOnly = op.recovered || !strings.EqualFold(op.status, "pending") && !strings.EqualFold(op.status, "running")
			entries = append(entries, entry)
		}
	}

	tunnelErr := a.network.tunnelApply.err
	tunnelComplete := a.network.tunnelApply.report != nil
	add("tunnel", locales.OperationTunnel(), a.network.tunnelApply.busy, tunnelErr, tunnelComplete, a.network.tunnelApply.busy || tunnelErr != nil || tunnelComplete, a.openNetwork)
	forwardErr := a.network.forwardActionErr
	forwardComplete := a.network.forwardActionReceipt != nil
	add("port-forward", locales.OperationPortForward(), a.network.forwardActionBusy, forwardErr, forwardComplete, a.network.forwardActionBusy || forwardErr != nil || forwardComplete, a.openNetwork)

	createComplete := a.remote.createReceipt != nil
	if a.remote.createReceipt != nil {
		createComplete = operationReceiptState(a.remote.createReceipt.Phase) == "completed"
	}
	add("remote-create", locales.OperationRemoteCreate(), a.remote.createBusy, a.remote.createErr, createComplete, a.remote.createBusy || a.remote.createErr != nil || a.remote.createReceipt != nil, a.openRemote)
	stopComplete := a.remote.stopReceipt != nil && operationReceiptState(a.remote.stopReceipt.Phase) == "completed"
	add("remote-stop", locales.OperationRemoteStop(), a.remote.stopBusy, a.remote.stopErr, stopComplete, a.remote.stopBusy || a.remote.stopErr != nil || a.remote.stopReceipt != nil || a.remote.stopSubmitted, a.openRemote)
	deleteComplete := a.remote.deleteReceipt != nil && operationReceiptState(a.remote.deleteReceipt.Phase) == "completed"
	add("remote-delete", locales.OperationRemoteDelete(), a.remote.deleteBusy, a.remote.deleteErr, deleteComplete, a.remote.deleteBusy || a.remote.deleteErr != nil || a.remote.deleteReceipt != nil || a.remote.deleteSubmitted, a.openRemote)

	// Read-only queries are included only while active. This gives the user one
	// place to see a hidden-window refresh without turning every cached snapshot
	// into a pseudo operation.
	add("remote-query", locales.OperationQuery(), a.remote.busy, a.remote.err, false, a.remote.busy || a.remote.err != nil, a.openRemote)
	add("diagnostics-query", locales.OperationQuery(), a.diagnostics.busy, a.diagnostics.err, false, a.diagnostics.busy || a.diagnostics.err != nil, a.openDiagnostics)
	add("capabilities-query", locales.OperationQuery(), a.capabilities.busy, nil, false, a.capabilities.busy, a.openCapabilities)
	add("skills-query", locales.OperationQuery(), a.skills.busy, a.skills.err, false, a.skills.busy || a.skills.err != nil, a.openSkills)

	return entries
}

func operationJournalTitle(id string) string {
	switch id {
	case "login":
		return locales.OperationLogin()
	case "logout":
		return locales.OperationLogout()
	case "account":
		return locales.OperationAccount()
	case "default-account":
		return locales.OperationDefaultAccount()
	case "credentials":
		return locales.OperationCredentials()
	case "account-import":
		return locales.OperationAccountImport()
	case "runtime-recovery":
		return locales.OperationRuntimeRecovery()
	case "sync":
		return locales.OperationSync()
	case "mutagen":
		return locales.OperationMutagen()
	case "setup":
		return locales.OperationSetup()
	case "init":
		return locales.InitWizardOperation()
	case "wireguard":
		return locales.OperationWireGuard()
	case "device":
		return locales.OperationDevice()
	case "tunnel":
		return locales.OperationTunnel()
	case "port-forward":
		return locales.OperationPortForward()
	case "remote-create":
		return locales.OperationRemoteCreate()
	case "remote-stop":
		return locales.OperationRemoteStop()
	case "remote-delete":
		return locales.OperationRemoteDelete()
	default:
		return locales.OperationsTitle()
	}
}

func (a *App) openOperation(id string) func() {
	return func() {
		switch id {
		case "login":
			a.openLogin()
		case "logout":
			a.openLogout()
		case "account":
			a.openAccountOperation()
		case "default-account":
			a.page = "account-default"
		case "credentials":
			a.openCredentials()
		case "account-import":
			a.openAccountImportHistory()
		case "runtime-recovery":
			a.openRuntimeRecovery()
		case "sync":
			a.openSync()
		case "mutagen":
			a.openMutagenEnvironment()
		case "setup":
			a.openLocalSetup()
		case "init":
			a.openInitWizard()
		case "wireguard":
			a.openWireGuard()
		case "device":
			a.openDevice()
		case "tunnel", "port-forward":
			a.openNetwork()
		case "remote-create", "remote-stop", "remote-delete":
			a.openRemote()
		default:
			a.page = "operations"
		}
	}
}

func operationPersistable(id string) bool {
	if strings.HasPrefix(id, "dedicated-cli-") {
		return false
	}
	switch id {
	case "remote-query", "diagnostics-query", "capabilities-query", "skills-query":
		return false
	default:
		return true
	}
}

func (a *App) operationEntries() []operationEntry {
	live := a.liveOperationEntries()
	entries := append([]operationEntry(nil), live...)
	seen := make(map[string]bool, len(live))
	for _, entry := range live {
		seen[entry.id] = true
	}
	ids := make([]string, 0, len(a.operationJournal))
	for id := range a.operationJournal {
		if !seen[id] && operationPersistable(id) {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	for _, id := range ids {
		record := a.operationJournal[id]
		entries = append(entries, operationEntry{id: record.ID, title: operationJournalTitle(record.ID), status: record.Status, details: operationRecordDetails(record), open: a.openOperation(record.ID), journalOnly: true})
	}
	return entries
}

func operationRecordDetails(record storage.OperationRecord) string {
	if len(record.Stages) == 0 {
		return ""
	}
	parts := make([]string, 0, len(record.Stages))
	for _, stage := range record.Stages {
		if record.ID == "mutagen" {
			parts = append(parts, locales.MutagenEnvironmentStage(stage.Name, stage.State))
		} else {
			parts = append(parts, locales.SyncStage(stage.Name, stage.State))
		}
	}
	return strings.Join(parts, " · ")
}

// syncOperationJournal snapshots only stable, non-sensitive lifecycle state.
// A running operation from a previous process remains "attention" until its
// owning page performs the exact CLI query; it is never replayed here.
func (a *App) syncOperationJournal() {
	changed := false
	now := time.Now().UTC()
	for _, entry := range a.liveOperationEntries() {
		if !operationPersistable(entry.id) {
			continue
		}
		status := entry.status
		if status == "" {
			continue
		}
		record := storage.OperationRecord{ID: entry.id, Status: status, UpdatedAt: now}
		if entry.id == "sync" {
			record.Stages = a.syncOperationStages()
		}
		previous, ok := a.operationJournal[entry.id]
		if !ok || previous.Status != record.Status || !slices.Equal(previous.Stages, record.Stages) {
			a.operationJournal[entry.id] = record
			changed = true
		}
	}
	if changed {
		a.persist()
	}
}

func (a *App) clearOperation(id string) {
	if strings.HasPrefix(id, "dedicated-cli-") {
		operationID := strings.TrimPrefix(id, "dedicated-cli-")
		for i, operation := range a.dedicatedOperations {
			if operation.id != operationID {
				continue
			}
			a.dedicatedOperations = append(a.dedicatedOperations[:i], a.dedicatedOperations[i+1:]...)
			a.persist()
			return
		}
	}
	if _, ok := a.operationJournal[id]; !ok {
		return
	}
	delete(a.operationJournal, id)
	a.persist()
}

func operationStatusText(status string) string {
	switch status {
	case "running":
		return locales.OperationRunning()
	case "completed":
		return locales.OperationCompleted()
	case "failed":
		return locales.OperationFailed()
	default:
		return locales.OperationAttention()
	}
}

func operationStatusColor(c *ui.Context, status string) ui.Color {
	switch status {
	case "running":
		return c.Theme().Accent
	case "failed":
		return c.Theme().Danger
	case "attention":
		return c.Theme().AccentHover
	default:
		return c.Theme().TextMuted
	}
}

func (a *App) operationsView(c *ui.Context) {
	entries := a.operationEntries()
	active := 0
	for _, entry := range entries {
		if entry.status == "running" {
			active++
		}
	}
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		ui.Text(c, locales.OperationsHelp()).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.OperationsSummary(active, len(entries))).FontSize(12).TextColor(c.Theme().TextMuted)
		if len(entries) == 0 {
			ui.Column(c).Grow(1).Center().Children(func() {
				ui.Text(c, locales.OperationsEmpty()).TextColor(c.Theme().TextMuted)
			})
			return
		}
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			for _, entry := range entries {
				item := entry
				a.card(c, ui.Column(c.Key("operation-"+item.id)).FillWidth().Padding(12).Gap(6)).Children(func() {
					ui.Row(c).Gap(10).Wrap().Children(func() {
						ui.Text(c, item.title).Bold().Grow(1)
						ui.Text(c, operationStatusText(item.status)).TextColor(operationStatusColor(c, item.status))
					})
					details := item.details
					if details == "" {
						details = locales.OperationDetailsHelp()
					}
					ui.Text(c, details).FontSize(12).TextColor(c.Theme().TextMuted)
					ui.Row(c).Gap(8).Children(func() {
						if item.open != nil {
							ui.Button(c, locales.OperationsOpen()).OnClick(item.open)
						}
						if item.journalOnly {
							ui.Button(c, locales.OperationsClear()).OnClick(func() { a.clearOperation(item.id) })
						}
					})
				})
			}
		})
	})
}
