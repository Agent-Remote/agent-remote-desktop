package app

import (
	"context"
	"errors"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) remoteDeleteAvailable() bool {
	return a.remote.loaded && !a.remote.busy && a.remote.err == nil && a.remote.snapshot.SessionsError == nil &&
		remote.SessionDeleteSupported(a.remote.snapshot.Capabilities) && !a.remote.deleteBusy && !a.remote.createBusy && !a.remote.stopBusy && !a.login.busy && !a.logout.busy && !a.quitting.Load()
}

func (a *App) beginRemoteDelete(selected cliadapter.RemoteSession) {
	if !a.remoteDeleteAvailable() || !remoteSessionCanDelete(selected) {
		return
	}
	// A collection currently starts with the explicitly chosen row. Additional
	// rows can be selected before confirmation; the snapshot never expands on
	// refresh or during a pending request.
	current, ok := a.currentRemoteSessionByID(selected.ID)
	if !ok || !reflect.DeepEqual(current, selected) {
		a.remote.deleteErr = fault.CLIContextChanged
		a.remote.deleteOpen = true
		return
	}
	a.remote.deleteSessions = []cliadapter.RemoteSession{selected}
	a.remote.deleteSelected = map[string]bool{selected.ID: true}
	a.remote.deleteContext = copyConfigContext(a.remote.snapshot.Config)
	a.rebuildRemoteDeleteIntent()
	a.remote.deleteOpen = true
	a.remote.deleteBusy = false
	a.remote.deleteErr = nil
	a.remote.deleteReceipt = nil
	a.remote.deleteRequestKey = ""
	a.remote.deleteSubmitted = false
	a.remote.deleteRecovery = false
}

func (a *App) beginRemoteDeleteCollection() {
	if !a.remoteDeleteAvailable() || len(a.remote.deleteSelected) == 0 {
		return
	}
	var selected []cliadapter.RemoteSession
	for _, session := range a.remote.snapshot.Sessions {
		if a.remote.deleteSelected[session.ID] {
			selected = append(selected, session)
		}
	}
	if len(selected) == 0 {
		return
	}
	tool := selected[0].Tool
	for _, session := range selected[1:] {
		if session.Tool != tool || !remoteSessionCanDelete(session) {
			a.remote.deleteErr = fault.SessionDeleteInputInvalid
			a.remote.deleteOpen = true
			return
		}
	}
	if !remoteSessionCanDelete(selected[0]) {
		a.remote.deleteErr = fault.SessionDeleteInputInvalid
		a.remote.deleteOpen = true
		return
	}
	a.remote.deleteSessions = selected
	a.remote.deleteContext = copyConfigContext(a.remote.snapshot.Config)
	a.rebuildRemoteDeleteIntent()
	a.remote.deleteBusy = false
	a.remote.deleteErr = nil
	a.remote.deleteReceipt = nil
	a.remote.deleteRequestKey = ""
	a.remote.deleteSubmitted = false
	a.remote.deleteRecovery = false
	a.remote.deleteOpen = true
}

// remoteDeleteRecoveryPresent reports whether the persisted pointer can be
// offered for an explicit query. It never enables a second delete submission.
func (a *App) remoteDeleteRecoveryPresent() bool {
	return a.remote.deleteRecovery && strings.TrimSpace(a.remote.deleteRequestKey) != "" &&
		a.remote.deleteContext.Home == a.home && a.remote.deleteContext.ServerURL != nil &&
		a.remote.deleteContext.DeviceID != nil && len(a.remote.deleteSessions) > 0 &&
		a.remote.deleteSelection.ToolType == "claude"
}

// openRemoteDeleteRecovery restores only the frozen request scope. The
// operation remains query-only, even if the current session list has changed.
func (a *App) openRemoteDeleteRecovery() {
	if !a.remoteDeleteRecoveryPresent() || a.remote.deleteBusy {
		return
	}
	if !sameConfigContext(a.remote.snapshot.Config, a.remote.deleteContext) {
		a.remote.deleteErr = fault.CLIContextChanged
		a.remote.deleteOpen = true
		return
	}
	a.remote.deleteOpen = true
	a.remote.deleteBusy = false
	a.remote.deleteErr = nil
	a.remote.deleteReceipt = nil
	a.remote.deleteSubmitted = true
}

func (a *App) rebuildRemoteDeleteIntent() {
	selected := a.selectedRemoteDeleteSessions()
	if len(selected) == 0 {
		a.remote.deleteSelection = remote.SessionDeleteSelection{}
		a.remote.deleteDigest, a.remote.deleteFingerprint = "", ""
		return
	}
	tool := selected[0].Tool
	ids := make([]string, 0, len(selected))
	for _, session := range selected {
		ids = append(ids, session.ID)
	}
	sort.Strings(ids)
	selection := remote.SessionDeleteSelection{ToolType: tool, Statuses: []string{"failed", "interrupted", "stopped"}}
	a.remote.deleteSelection = selection
	a.remote.deleteDigest = remote.SessionDeleteSelectionDigest(tool, ids, selection)
	a.remote.deleteFingerprint = remote.SessionDeleteIntentFingerprint(tool, ids, selection)
	// Keep the snapshot ordered by the same canonical ID order sent to the CLI.
	sort.Slice(a.remote.deleteSessions, func(i, j int) bool { return a.remote.deleteSessions[i].ID < a.remote.deleteSessions[j].ID })
}

func (a *App) selectedRemoteDeleteSessions() []cliadapter.RemoteSession {
	selected := make([]cliadapter.RemoteSession, 0, len(a.remote.deleteSelected))
	for _, session := range a.remote.deleteSessions {
		if a.remote.deleteSelected[session.ID] {
			selected = append(selected, session)
		}
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i].ID < selected[j].ID })
	return selected
}

func (a *App) remoteDeleteConfirmationCurrent() bool {
	if !a.remoteDeleteAvailable() || a.remote.deleteSubmitted || len(a.remote.deleteSessions) == 0 || len(a.remote.deleteSelected) == 0 {
		return false
	}
	if !sameConfigContext(a.remote.snapshot.Config, a.remote.deleteContext) {
		return false
	}
	for _, reviewed := range a.remote.deleteSessions {
		current, ok := a.currentRemoteSessionByID(reviewed.ID)
		if !ok || !reflect.DeepEqual(current, reviewed) {
			return false
		}
	}
	selected := a.selectedRemoteDeleteSessions()
	if len(selected) != len(a.remote.deleteSessions) {
		return false
	}
	ids := make([]string, 0, len(selected))
	for _, session := range selected {
		ids = append(ids, session.ID)
	}
	return remote.SessionDeleteSelectionDigest(a.remote.deleteSelection.ToolType, ids, a.remote.deleteSelection) == a.remote.deleteDigest &&
		remote.SessionDeleteIntentFingerprint(a.remote.deleteSelection.ToolType, ids, a.remote.deleteSelection) == a.remote.deleteFingerprint
}

func remoteSessionCanDelete(s cliadapter.RemoteSession) bool {
	return s.ID != "" && s.Tool == "claude" && (s.Status == "failed" || s.Status == "interrupted" || s.Status == "stopped")
}

func (a *App) submitRemoteDelete() {
	if a.remote.deleteBusy || !a.remoteDeleteConfirmationCurrent() {
		return
	}
	selected := a.selectedRemoteDeleteSessions()
	ids := make([]string, 0, len(selected))
	for _, session := range selected {
		ids = append(ids, session.ID)
	}
	input := remote.SessionDeleteInput{
		SchemaVersion: 1, Expected: a.remote.deleteContext, ToolType: a.remote.deleteSelection.ToolType,
		SessionIDs: ids, Selection: a.remote.deleteSelection, SelectionDigest: a.remote.deleteDigest,
		IntentFingerprint: a.remote.deleteFingerprint, Confirmed: true, Reason: "user_requested",
	}
	a.remote.deleteBusy = true
	a.remote.deleteSubmitted = true
	a.remote.deleteErr = nil
	a.remote.deleteReceipt = nil
	a.remote.deleteDone = make(chan struct{})
	a.remote.deleteResults = make(chan remoteDeleteResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.deleteCancel = cancel
	results, done, home := a.remote.deleteResults, a.remote.deleteDone, a.home
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.SessionDeleteReceipt
		if err == nil {
			receipt, err = remote.DeleteSessions(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- remoteDeleteResult{receipt: receipt, err: err}
	}()
}

func (a *App) queryRemoteDelete() {
	if a.remote.deleteBusy || strings.TrimSpace(a.remote.deleteRequestKey) == "" {
		return
	}
	a.remote.deleteBusy = true
	a.remote.deleteErr = nil
	a.remote.deleteDone = make(chan struct{})
	a.remote.deleteResults = make(chan remoteDeleteResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.deleteCancel = cancel
	requestID, results, done, home, expected := a.remote.deleteRequestKey, a.remote.deleteResults, a.remote.deleteDone, a.home, a.remote.deleteContext
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.SessionDeleteReceipt
		if err == nil {
			receipt, err = remote.QuerySessionDelete(ctx, cliadapter.Client{Binary: binary, Home: home}, remote.SessionDeleteRequestInput{RequestID: requestID, Expected: expected, Wait: true, Timeout: 60})
		}
		results <- remoteDeleteResult{receipt: receipt, err: err, query: true}
	}()
}

func (a *App) finishRemoteDelete(c *ui.Context) {
	if a.remote.deleteDone == nil || a.remote.deleteResults == nil {
		return
	}
	select {
	case result := <-a.remote.deleteResults:
		a.remote.deleteBusy = false
		a.remote.deleteCancel = nil
		a.remote.deleteDone, a.remote.deleteResults = nil, nil
		if result.err != nil {
			a.remote.deleteErr = result.err
			if errors.Is(result.err, fault.SessionDeleteOutcomeUnknown) {
				a.remote.deleteReceipt = nil
			}
		} else {
			if !a.remoteDeleteReceiptMatchesScope(result.receipt) {
				a.remote.deleteErr = fault.SessionDeleteOutcomeUnknown
				a.remote.deleteReceipt = nil
			} else {
				a.remote.deleteErr = nil
				a.remote.deleteReceipt = &result.receipt
				a.remote.deleteRequestKey = result.receipt.RequestID
				a.remote.deleteRecovery = true
				// Keep the in-memory state projection in sync with the exact
				// caller key before any later review or background persist. A
				// fresh delete review clears the transient request fields; the
				// persistence fallback intentionally reads this state pointer so
				// opening that review cannot discard the older recovery affordance.
				a.state.RemoteDeleteRecovery = a.remoteDeleteRecoveryRecord()
				// Persist only the caller key and reviewed public scope. The full
				// receipt remains in the CLI journal.
				a.persist()
				a.refreshRemote()
			}
		}
		if c != nil {
			c.After(0)
		}
	default:
		if c != nil {
			c.After(50 * time.Millisecond)
		}
	}
}

func (a *App) remoteDeleteReceiptMatchesScope(receipt remote.SessionDeleteReceipt) bool {
	if receipt.RequestID != a.remote.deleteRequestKey && a.remote.deleteRequestKey != "" {
		return false
	}
	if !sameConfigContext(receipt.Context, a.remote.deleteContext) || receipt.ToolType != a.remote.deleteSelection.ToolType ||
		receipt.SelectionDigest != a.remote.deleteDigest || receipt.IntentFingerprint != a.remote.deleteFingerprint {
		return false
	}
	want := make([]string, 0, len(a.remote.deleteSessions))
	for _, session := range a.remote.deleteSessions {
		want = append(want, session.ID)
	}
	sort.Strings(want)
	got := append([]string(nil), receipt.SessionIDs...)
	sort.Strings(got)
	return reflect.DeepEqual(got, want)
}

func (a *App) remoteDeleteDialog(c *ui.Context) {
	a.finishRemoteDelete(c)
	ui.Modal(c, &a.remote.deleteOpen, func() {
		ui.Column(c).Width(650).Gap(12).Children(func() {
			ui.Text(c, locales.RemoteDeleteQuestion()).FontSize(22).Bold()
			ui.Text(c, locales.RemoteDeleteHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, locales.RemoteDeleteCollection(float64(len(a.remote.deleteSessions)))).Bold()
			if a.remote.deleteRecovery {
				ui.Text(c, locales.RemoteDeleteRecoveryHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			ui.Scroll(c).MaxHeight(170).FillWidth().Gap(4).Children(func() {
				for _, session := range a.remote.deleteSessions {
					ui.Column(c.Key("delete-review-" + session.ID)).Gap(2).Children(func() {
						ui.Text(c, session.ID).FontSize(12).Bold()
						ui.Text(c, locales.RemoteAttachAccount(session.AccountID)).FontSize(11).TextColor(c.Theme().TextMuted)
						if session.WorkspaceID != "" {
							ui.Text(c, locales.RemoteDeleteWorkspace(session.WorkspaceID)).FontSize(11).TextColor(c.Theme().TextMuted)
						}
						if session.ProjectKey != "" {
							ui.Text(c, locales.RemoteDeleteProject(session.ProjectKey)).FontSize(11).TextColor(c.Theme().TextMuted)
						}
						ui.Text(c, locales.RemoteAttachNode(session.NodeID)).FontSize(11).TextColor(c.Theme().TextMuted)
						ui.Text(c, locales.RemoteBackend(session.Backend)).FontSize(11).TextColor(c.Theme().TextMuted)
					})
				}
			})
			if a.remote.deleteBusy {
				ui.Text(c, locales.RemoteDeletePending()).TextColor(c.Theme().TextMuted)
			}
			if a.remote.deleteErr != nil {
				a.errorText(c, a.remote.deleteErr, presentation.Error(a.remote.deleteErr))
			}
			if a.remote.deleteReceipt != nil {
				a.remoteDeleteReceiptView(c, *a.remote.deleteReceipt)
			}
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).Disabled(a.remote.deleteBusy).OnClick(func() { a.remote.deleteOpen = false })
				if a.remote.deleteRequestKey != "" && !a.remote.deleteBusy {
					ui.Button(c, locales.RemoteDeleteQuery()).OnClick(a.queryRemoteDelete)
				}
				ui.PrimaryButton(c, locales.RemoteDeleteConfirm()).Disabled(a.remote.deleteBusy || !a.remoteDeleteConfirmationCurrent()).OnClick(a.submitRemoteDelete)
			})
		})
	})
}

func (a *App) remoteDeleteReceiptView(c *ui.Context, receipt remote.SessionDeleteReceipt) {
	a.card(c, ui.Column(c).FillWidth().Padding(12).Gap(5)).Children(func() {
		ui.Text(c, locales.RemoteDeletePhase(remoteDeletePhaseText(receipt.Phase))).Bold()
		ui.Text(c, locales.RemoteDeleteRequest(receipt.RequestID)).FontSize(12)
		ui.Text(c, locales.RemoteDeleteDeleted(float64(len(receipt.DeletedSessionIDs)), float64(len(receipt.SessionIDs)))).FontSize(12)
		for _, context := range receipt.SessionContexts {
			ui.Text(c, locales.RemoteDeleteSessionContext(context.SessionID, context.NodeID, context.RuntimeBackend)).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		if receipt.CollectionMismatch {
			ui.Text(c, locales.RemoteDeleteCollectionMismatch()).TextColor(c.Theme().Danger)
		}
		for _, blocker := range receipt.RetentionBlockers {
			ui.Text(c, locales.RemoteDeleteRetentionBlocker(blocker)).TextColor(c.Theme().Danger)
		}
		if receipt.ErrorCode != nil {
			ui.Text(c, presentation.Error(&cliadapter.OperationError{Code: *receipt.ErrorCode})).TextColor(c.Theme().Danger)
		}
	})
}

func remoteDeletePhaseText(phase string) string {
	switch phase {
	case "pending", "accepted", "deleting":
		return locales.RemoteStatusPending()
	case "deleted":
		return locales.RemoteStatusStopped()
	case "unknown":
		return locales.RemoteDeleteUnknown()
	case "blocked", "failed", "interrupted":
		return locales.RemoteDeleteFailed()
	default:
		return locales.RemoteStatusOther(phase)
	}
}
