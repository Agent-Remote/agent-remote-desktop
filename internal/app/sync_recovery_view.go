package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func syncRecoveryPlanView(c *ui.Context, plan remote.SyncRecoveryPlan) {
	ui.Text(c, locales.SyncRecoveryTitle()).Bold()
	ui.Text(c, locales.SyncRecoveryHelp())
	syncRecoveryMappingView(c, plan)
	ui.Text(c, locales.SyncRecoveryEffects()).TextColor(c.Theme().Danger)
	if !plan.Recoverable {
		ui.Text(c, locales.SyncRecoveryBlocked()).TextColor(c.Theme().Danger)
	}
	if plan.ErrorCode != nil {
		ui.Text(c, syncError(&cliadapter.OperationError{Code: *plan.ErrorCode})).TextColor(c.Theme().Danger)
	}
}
func syncRecoveryMappingView(c *ui.Context, plan remote.SyncRecoveryPlan) {
	ui.Text(c, plan.Context.Home).FontSize(12)
	if plan.Context.ServerURL != nil {
		ui.Text(c, *plan.Context.ServerURL).FontSize(12)
	}
	if plan.Context.DeviceID != nil {
		ui.Text(c, *plan.Context.DeviceID).FontSize(12)
	}
	ui.Text(c, locales.SyncRecoveryScope(plan.ServerState))
	ui.Text(c, locales.SyncRecoveryMapping()).Bold()
	syncTargetView(c, plan.Target)
	w := plan.Mapping.Workspace
	ui.Text(c, locales.SyncField("project", w.DisplayName)).FontSize(12)
	ui.Text(c, locales.SyncField("local_path", w.LocalPath)).FontSize(12)
	ui.Text(c, locales.SyncField("fingerprint", plan.MappingFingerprint)).FontSize(12)
	if w.RemotePath != nil {
		ui.Text(c, locales.SyncField("remote_path", *w.RemotePath)).FontSize(12)
	}
	if s := plan.Mapping.Sync; s != nil {
		ui.Text(c, locales.SyncField("status", locales.SyncServerState(s.Status))).FontSize(12)
		ui.Text(c, locales.SyncField("conflict_status", locales.SyncServerConflicts(s.ConflictStatus))).FontSize(12)
		for _, field := range []struct {
			name  string
			value *string
		}{{"node_id", s.NodeID}, {"mutagen_session_id", s.MutagenSessionID}, {"remote_endpoint", s.RemoteEndpoint}} {
			if field.value != nil {
				ui.Text(c, locales.SyncField(field.name, *field.value)).FontSize(12)
			}
		}
	}
	syncRecoveryMutagenView(c, plan.Mutagen)
}
func syncRecoveryMutagenView(c *ui.Context, observation remote.SyncRecoveryMutagen) {
	ui.Text(c, locales.SyncRecoveryMutagenState(observation.State))
	if observation.NamespaceFingerprint != nil {
		ui.Text(c, locales.SyncField("namespace_fingerprint", *observation.NamespaceFingerprint)).FontSize(12)
	}
	if target := observation.Target; target != nil {
		for _, field := range []struct{ name, value string }{{"mutagen_identifier", target.Identifier}, {"mutagen_session_id", target.Name}, {"local_path", target.LocalPath}, {"remote_path", target.RemotePath}, {"remote_endpoint", target.Endpoint}, {"configuration_fingerprint", target.ConfigurationFingerprint}} {
			ui.Text(c, locales.SyncField(field.name, field.value)).FontSize(12)
		}
	}
}
func (a *App) syncRecoveryResultsView(c *ui.Context) {
	p := &a.sync.recovery
	if p.err != nil {
		a.errorText(c, p.err, syncError(p.err))
	}
	for _, failure := range p.failures {
		ui.Text(c, locales.SyncRecoveryReceipt()).Bold()
		if failure.plan != nil {
			syncRecoveryMappingView(c, *failure.plan)
		}
		a.errorText(c, failure.err, syncError(failure.err))
	}
	for _, receipt := range p.receipts {
		ui.Text(c, locales.SyncRecoveryReceipt()).Bold()
		ui.Text(c, locales.SyncOutcome(receipt.Outcome)).Bold()
		ui.Text(c, locales.SyncField("operation_id", receipt.OperationID)).FontSize(12)
		syncRecoveryMappingView(c, receipt.Plan)
		a.receiptStages(c, len(receipt.Stages), func(i int) (string, string) {
			return locales.SyncStage(receipt.Stages[i].Name, receipt.Stages[i].State), receipt.Stages[i].State
		})
		if receipt.ErrorCode != nil {
			ui.Text(c, syncError(&cliadapter.OperationError{Code: *receipt.ErrorCode})).TextColor(c.Theme().Danger)
		}
		ui.Text(c, locales.SyncRecoveryResultTarget()).Bold()
		syncTargetView(c, receipt.ResultTarget)
		if receipt.Mutagen != nil {
			syncRecoveryMutagenView(c, *receipt.Mutagen)
		}
		if receipt.Outcome == "completed" {
			ui.Text(c, locales.SyncRecoveryFreshHelp())
		}
	}
}
