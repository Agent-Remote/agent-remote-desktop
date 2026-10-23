package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) remoteStopAvailable() bool {
	return a.remote.loaded && !a.remote.busy && a.remote.err == nil && a.remote.snapshot.SessionsError == nil &&
		remote.SessionStopSupported(a.remote.snapshot.Capabilities) && !a.remote.stopBusy && !a.remote.createBusy && !a.remote.deleteBusy && !a.login.busy && !a.logout.busy && !a.quitting.Load()
}

func (a *App) beginRemoteStop(selected cliadapter.RemoteSession) {
	if !a.remoteStopAvailable() || !remoteSessionCanStop(selected) {
		return
	}
	current, ok := a.currentRemoteSessionByID(selected.ID)
	if !ok || current.NodeID != selected.NodeID || current.Backend != selected.Backend {
		a.remote.stopErr = fault.CLIContextChanged
		a.remote.stopOpen = true
		return
	}
	a.remote.stopSession = selected
	a.remote.stopContext = copyConfigContext(a.remote.snapshot.Config)
	a.remote.stopOpen = true
	a.remote.stopBusy = false
	a.remote.stopErr = nil
	a.remote.stopReceipt = nil
	a.remote.stopRequestKey = ""
	a.remote.stopSubmitted = false
}

func remoteSessionCanStop(s cliadapter.RemoteSession) bool {
	return s.ID != "" && s.Tool == "claude" && s.Status != "stopped" && s.Status != "failed" && s.Status != "interrupted"
}

func (a *App) remoteStopRecoveryPresent() bool {
	return strings.TrimSpace(a.remote.stopRequestKey) != "" && a.remote.stopSubmitted && a.remote.stopSession.ID != "" && a.remote.stopContext.ServerURL != nil && a.remote.stopContext.DeviceID != nil
}

func (a *App) openRemoteStopRecovery() {
	if !a.remoteStopRecoveryPresent() || a.remote.stopBusy {
		return
	}
	a.remote.stopOpen = true
	a.remote.stopErr = nil
}

func (a *App) currentRemoteSessionByID(id string) (cliadapter.RemoteSession, bool) {
	var found cliadapter.RemoteSession
	count := 0
	for _, candidate := range a.remote.snapshot.Sessions {
		if candidate.ID == id {
			found, count = candidate, count+1
		}
	}
	return found, count == 1
}

func (a *App) remoteStopConfirmationCurrent() bool {
	if !a.remoteStopAvailable() || !remoteSessionCanStop(a.remote.stopSession) {
		return false
	}
	current, ok := a.currentRemoteSessionByID(a.remote.stopSession.ID)
	return !a.remote.stopSubmitted && ok && sameConfigContext(a.remote.snapshot.Config, a.remote.stopContext) && current.NodeID == a.remote.stopSession.NodeID && current.Backend == a.remote.stopSession.Backend && current.AccountID == a.remote.stopSession.AccountID && remoteSessionCanStop(current)
}

func (a *App) submitRemoteStop() {
	if a.remote.stopBusy || !a.remoteStopConfirmationCurrent() {
		return
	}
	selected := a.remote.stopSession
	config := a.remote.stopContext
	if config.ServerURL == nil || config.DeviceID == nil || config.Home != a.home {
		a.remote.stopErr = fault.CLIContextChanged
		return
	}
	input := remote.SessionStopInput{
		SchemaVersion: 1, Expected: config, SessionID: selected.ID,
		ExpectedNodeID: selected.NodeID, ExpectedBackend: selected.Backend,
		Confirmed: true,
	}
	if selected.StopTaskID != nil {
		input.ExpectedStopTaskID = *selected.StopTaskID
	}
	a.remote.stopBusy = true
	a.remote.stopSubmitted = true
	a.remote.stopErr = nil
	a.remote.stopReceipt = nil
	a.remote.stopDone = make(chan struct{})
	a.remote.stopResults = make(chan remoteStopResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.stopCancel = cancel
	results, done, home := a.remote.stopResults, a.remote.stopDone, a.home
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.SessionStopReceipt
		if err == nil {
			receipt, err = remote.StopSession(ctx, cliadapter.Client{Binary: binary, Home: home}, input)
		}
		results <- remoteStopResult{receipt: receipt, err: err}
	}()
}

func (a *App) queryRemoteStop() {
	if a.remote.stopBusy || strings.TrimSpace(a.remote.stopRequestKey) == "" {
		return
	}
	a.remote.stopBusy = true
	a.remote.stopErr = nil
	a.remote.stopDone = make(chan struct{})
	a.remote.stopResults = make(chan remoteStopResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	a.remote.stopCancel = cancel
	requestID, results, done, home, expected := a.remote.stopRequestKey, a.remote.stopResults, a.remote.stopDone, a.home, a.remote.stopContext
	go func() {
		defer close(done)
		defer cancel()
		binary, err := a.findExecutable("agent-remote")
		var receipt remote.SessionStopReceipt
		if err == nil {
			receipt, err = remote.QuerySessionStop(ctx, cliadapter.Client{Binary: binary, Home: home}, remote.SessionStopRequestInput{RequestID: requestID, Expected: expected, Wait: true, Timeout: 60})
		}
		results <- remoteStopResult{receipt: receipt, err: err, query: true}
	}()
}

func copyConfigContext(value cliadapter.ConfigContext) cliadapter.ConfigContext {
	copy := value
	if value.ServerURL != nil {
		server := *value.ServerURL
		copy.ServerURL = &server
	}
	if value.DeviceID != nil {
		device := *value.DeviceID
		copy.DeviceID = &device
	}
	return copy
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	copy := value
	return &copy
}

func sameConfigContext(a, b cliadapter.ConfigContext) bool {
	if a.Home != b.Home {
		return false
	}
	if (a.ServerURL == nil) != (b.ServerURL == nil) || (a.ServerURL != nil && *a.ServerURL != *b.ServerURL) {
		return false
	}
	return (a.DeviceID == nil) == (b.DeviceID == nil) && (a.DeviceID == nil || *a.DeviceID == *b.DeviceID)
}

func (a *App) finishRemoteStop(c *ui.Context) {
	if a.remote.stopDone == nil || a.remote.stopResults == nil {
		return
	}
	select {
	case result := <-a.remote.stopResults:
		a.remote.stopBusy = false
		a.remote.stopCancel = nil
		a.remote.stopDone, a.remote.stopResults = nil, nil
		if result.err != nil {
			a.remote.stopErr = result.err
			if errors.Is(result.err, fault.SessionStopOutcomeUnknown) {
				a.remote.stopReceipt = nil
			}
		} else {
			a.remote.stopErr = nil
			a.remote.stopReceipt = &result.receipt
			a.remote.stopRequestKey = result.receipt.RequestID
			if result.receipt.StopTaskID != nil {
				value := *result.receipt.StopTaskID
				a.remote.stopSession.StopTaskID = &value
			}
			// Save the exact caller key as soon as the CLI returns it. A later
			// restart can query this key, while the receipt itself remains owned
			// by the CLI journal.
			a.state.RemoteStopRecovery = a.remoteStopRecoveryRecord()
			a.persist()
			// Keep the list observation separate from the immutable stop receipt;
			// a refresh may show stopping while the receipt still reports save work.
			a.refreshRemote()
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

func (a *App) remoteStopDialog(c *ui.Context) {
	a.finishRemoteStop(c)
	ui.Modal(c, &a.remote.stopOpen, func() {
		ui.Column(c).Width(560).Gap(14).Children(func() {
			ui.Text(c, locales.RemoteStopQuestion()).FontSize(22).Bold()
			ui.Text(c, a.remote.stopSession.ID).Bold()
			ui.Text(c, locales.RemoteStopHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			if a.remote.stopBusy {
				ui.Text(c, locales.RemoteStopPending()).TextColor(c.Theme().TextMuted)
			}
			if a.remote.stopErr != nil {
				a.errorText(c, a.remote.stopErr, presentation.Error(a.remote.stopErr))
			}
			if a.remote.stopReceipt != nil {
				a.remoteStopReceiptView(c, *a.remote.stopReceipt)
			}
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).Disabled(a.remote.stopBusy).OnClick(func() { a.remote.stopOpen = false })
				if a.remote.stopRequestKey != "" && !a.remote.stopBusy {
					ui.Button(c, locales.RemoteStopQuery()).OnClick(a.queryRemoteStop)
				}
				ui.PrimaryButton(c, locales.RemoteStopConfirm()).Disabled(a.remote.stopBusy || !a.remoteStopConfirmationCurrent()).OnClick(a.submitRemoteStop)
			})
		})
	})
}

func (a *App) remoteStopReceiptView(c *ui.Context, receipt remote.SessionStopReceipt) {
	a.card(c, ui.Column(c).FillWidth().Padding(12).Gap(6)).Children(func() {
		phase := receipt.Phase
		switch phase {
		case "pending", "accepted", "running", "saving":
			phase = locales.RemoteStatusPending()
		case "stopping":
			phase = locales.RemoteStatusStopping()
		case "stopped":
			phase = locales.RemoteStatusStopped()
		case "saved":
			phase = locales.RemoteStopSaved()
		case "settled":
			phase = locales.RemoteStatusStopped()
		case "unresolved":
			phase = locales.RemoteStopUnknown()
		case "failed", "interrupted":
			phase = locales.RemoteStatusFailed()
		case "deleted":
			phase = locales.RemoteStopDeleted()
		case "unknown":
			phase = locales.RemoteStopUnknown()
		}
		ui.Text(c, locales.RemoteStopPhase(phase)).Bold()
		process := "unknown"
		if receipt.ProcessStopped != nil {
			if *receipt.ProcessStopped {
				process = "stopped"
			} else {
				process = "stopping"
			}
		} else if receipt.SessionStatus != nil {
			process = *receipt.SessionStatus
		} else if receipt.StopTaskStatus != nil {
			process = *receipt.StopTaskStatus
		}
		ui.Text(c, locales.RemoteStopProcess(remoteStopStatusText(process)))
		save := locales.RemoteStopSaved()
		if receipt.SaveStatus != nil {
			save = remoteStopStatusText(*receipt.SaveStatus)
		} else if receipt.SkillFinalizationOperationID != nil {
			save = locales.RemoteStatusPending()
		}
		if receipt.Phase == "unknown" {
			save = locales.RemoteStopUnknown()
		}
		ui.Text(c, locales.RemoteStopSave(save))
		if receipt.RequestID != "" {
			ui.Text(c, receipt.RequestID).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if receipt.ErrorCode != nil {
			ui.Text(c, presentation.Error(&cliadapter.OperationError{Code: *receipt.ErrorCode})).TextColor(c.Theme().Danger)
		}
	})
}

func remoteStopStatusText(status string) string {
	switch status {
	case "saved", "succeeded", "complete", "completed":
		return locales.RemoteStopSaved()
	case "saving", "pending", "accepted", "running":
		return locales.RemoteStatusPending()
	case "unknown", "unresolved":
		return locales.RemoteStopUnknown()
	case "not_requested", "none":
		return locales.RemoteStatusOther(status)
	default:
		return presentation.RemoteStatus(status)
	}
}
