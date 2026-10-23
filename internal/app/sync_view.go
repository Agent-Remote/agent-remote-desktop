package app

import (
	"errors"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
	"strconv"
	"strings"
)

func (a *App) syncView(c *ui.Context) {
	p := &a.sync
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.SyncTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = "" })
		})
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			if p.localPath == "" {
				ui.Text(c, locales.SyncChooseProject())
				return
			}
			ui.Text(c, locales.SyncField("project", p.projectName)).Bold()
			ui.Text(c, p.localPath).FontSize(12)
			ui.Text(c, a.home).FontSize(12)
			if p.expected.ServerURL != nil {
				ui.Text(c, *p.expected.ServerURL).FontSize(12)
			}
			if p.expected.DeviceID != nil {
				ui.Text(c, *p.expected.DeviceID).FontSize(12)
			}
			ui.Text(c, locales.SyncHelp()).TextColor(c.Theme().TextMuted)
			if p.review && p.action == "recovery" && p.recovery.plan != nil {
				syncRecoveryPlanView(c, *p.recovery.plan)
			}
			if p.review && p.action != "recovery" {
				ui.Text(c, locales.SyncAction(p.action)).Bold()
				ui.Text(c, locales.SyncActionHelp(p.action))
				if p.status != nil {
					syncTargetView(c, p.status.Target)
					syncDetailsView(c, p.status.Sync)
				}
				if p.dryRun {
					ui.Text(c, locales.SyncSkipLocalHelp()).TextColor(c.Theme().Danger)
				} else {
					ui.Text(c, locales.SyncNormalExecution())
				}
			}
			if p.busy {
				ui.Text(c, locales.SyncWorking())
				ui.Text(c, locales.SyncCancelHelp())
				return
			}
			if p.review {
				return
			}
			for _, record := range p.history {
				syncRecordView(c, record)
			}
			syncRecordView(c, syncRecord{receipt: p.receipt, err: p.err})
			a.syncRecoveryResultsView(c)
			if p.loading {
				ui.Text(c, locales.LoadingRemote())
				return
			}
			if p.queryErr != nil {
				ui.Text(c, syncError(p.queryErr)).TextColor(c.Theme().Danger)
			}
			if p.status != nil {
				ui.Text(c, locales.SyncObservationScope()).Bold()
				syncTargetView(c, p.status.Target)
				syncDetailsView(c, p.status.Sync)
				syncMutagenView(c, p.status.Mutagen)
				ui.Text(c, locales.SyncConflictHelp()).TextColor(c.Theme().TextMuted)

			}
		})
		if p.status != nil && !p.submitted && !p.review && !p.loading && !p.busy {
			ui.Checkbox(c, &p.dryRun, locales.SyncSkipLocal()).Tooltip(locales.SyncSkipLocalHelp())
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, action := range []string{"ensure", "pause", "resume", "resolve", "reset"} {
					ui.Button(c, locales.SyncAction(action)).Disabled(!a.syncActionAvailable(action)).OnClick(func() { a.reviewSync(action) })
				}
			})
		}

		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			switch {
			case p.busy:
				ui.Button(c, locales.SyncStopWaiting()).OnClick(p.cancel)
			case p.loading:
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(p.cancel)
			case p.review:
				ui.Button(c, locales.Cancel()).OnClick(func() { p.review = false })
				if p.action == "recovery" {
					ui.PrimaryButton(c, locales.SyncRecoveryConfirm()).Disabled(!a.canApplySyncRecovery()).OnClick(a.applySyncRecovery)
				} else {
					ui.PrimaryButton(c, locales.SyncConfirm()).Disabled(!a.syncActionAvailable(p.action)).OnClick(a.submitSync)
				}
			default:
				if len(p.recovery.receipts) > 0 && p.recovery.receipts[len(p.recovery.receipts)-1].Outcome == "completed" {
					ui.Button(c, locales.SyncRecoveryFresh()).Disabled(!a.syncRecoveryAvailable("sync.status")).OnClick(a.freshSyncAfterRecovery)
				}
				if remote.SyncOperationSupported(p.capabilities, "sync.recovery.preview") {
					ui.Button(c, locales.SyncRecoveryInspect()).Disabled(!a.syncRecoveryAvailable("sync.recovery.preview")).OnClick(a.previewSyncRecovery)
				}
				ui.Button(c, locales.MutagenEnvironmentTitle()).OnClick(a.openMutagenEnvironment)
				ui.Button(c, locales.SyncDone()).OnClick(a.finishSync)
				ui.Button(c, locales.SyncRefresh()).Disabled(a.syncBlocked() || p.localPath == "").OnClick(a.refreshSync)
			}
		})
	})
}
func syncTargetView(c *ui.Context, t remote.SyncTarget) {
	ui.Text(c, locales.SyncField("project_key", t.ProjectKey)).FontSize(12)
	for _, field := range []struct {
		name  string
		value *string
	}{{"workspace_id", t.WorkspaceID}, {"sync_session_id", t.SyncSessionID}, {"fingerprint", t.Fingerprint}} {
		if field.value != nil {
			ui.Text(c, locales.SyncField(field.name, *field.value)).FontSize(12)
		}
	}
}
func syncDetailsView(c *ui.Context, s *remote.SyncDetails) {
	if s == nil {
		ui.Text(c, locales.SyncNoSession())
		return
	}
	for _, field := range []struct{ name, value string }{{"status", locales.SyncServerState(s.Status)}, {"conflict_status", locales.SyncServerConflicts(s.ConflictStatus)}, {"remote_path", s.RemotePath}, {"sync_mode", locales.SyncMode(s.SyncMode)}, {"sync_git", locales.SyncBoolean(strconv.FormatBool(s.SyncGit))}, {"exclude", strings.Join(s.Exclude, ", ")}} {
		ui.Text(c, locales.SyncField(field.name, field.value)).FontSize(12)
	}
	for _, field := range []struct {
		name  string
		value *string
	}{{"node_id", s.NodeID}, {"mutagen_session_id", s.MutagenSessionID}, {"remote_endpoint", s.RemoteEndpoint}} {
		if field.value != nil {
			ui.Text(c, locales.SyncField(field.name, *field.value)).FontSize(12)
		}
	}
}

func syncError(err error) string {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) && operation.Code == "state_unavailable" {
		return locales.SyncResultCode(operation.Code)
	}
	return presentation.Error(err)
}

func syncMutagenView(c *ui.Context, m remote.MutagenObservation) {
	ui.Text(c, locales.SyncMutagenState(m.State))
	ui.Text(c, locales.SyncConflicts(m.Conflicts))
	if m.Paused != nil {
		ui.Text(c, locales.SyncPaused(locales.SyncBoolean(strconv.FormatBool(*m.Paused))))
	}
	if m.Status != nil {
		ui.Text(c, locales.SyncLocalStatus(strings.ReplaceAll(*m.Status, "-", "_")))
	}
	if m.ConflictCount != nil && m.Conflicts != "unknown" {
		ui.Text(c, locales.SyncConflictCount(networkNumber(*m.ConflictCount)))
	}
}

func syncRecordView(c *ui.Context, record syncRecord) {
	if record.err != nil {
		ui.Text(c, syncError(record.err)).TextColor(c.Theme().Danger)
	}
	if record.receipt == nil {
		return
	}
	r := record.receipt
	ui.Text(c, locales.SyncReceiptScope()).Bold()
	ui.Text(c, r.Context.Home).FontSize(12)
	if r.Context.ServerURL != nil {
		ui.Text(c, *r.Context.ServerURL).FontSize(12)
	}
	if r.Context.DeviceID != nil {
		ui.Text(c, *r.Context.DeviceID).FontSize(12)
	}
	ui.Text(c, locales.SyncOutcome(r.Outcome)).Bold()
	ui.Text(c, locales.SyncField("operation_id", r.OperationID)).FontSize(12)
	ui.Text(c, locales.SyncAction(strings.TrimPrefix(r.Method, "sync.")))
	if r.DryRun {
		ui.Text(c, locales.SyncSkipLocalHelp()).TextColor(c.Theme().Danger)
	}
	for _, stage := range r.Stages {
		ui.Text(c, locales.SyncStage(stage.Name, stage.State))
	}
	if r.ErrorCode != nil {
		ui.Text(c, syncError(&cliadapter.OperationError{Code: *r.ErrorCode})).TextColor(c.Theme().Danger)
	}
	syncTargetView(c, r.ResultTarget)
	ui.Text(c, locales.SyncReceiptObservation()).Bold()
	if r.Sync != nil {
		syncDetailsView(c, r.Sync)
	}
	if r.Mutagen != nil {
		syncMutagenView(c, *r.Mutagen)
	}
	ui.Text(c, locales.SyncReceiptHelp()).FontSize(12)
}
