package app

import (
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) runtimeRecoveryView(c *ui.Context) {
	p := &a.runtimeRecovery
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(12).Children(func() {
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			ui.Text(c, locales.RuntimeRecoveryHelp())
			if p.loading || p.busy {
				ui.Text(c, locales.RuntimeRecoveryWorking())
				ui.Text(c, locales.RuntimeRecoveryCancelHelp())
			}
			if p.err != nil {
				a.errorText(c, p.err, runtimeRecoveryError(p.err))
			}
			if !p.submitted {
				ui.Button(c, locales.RuntimeRecoveryAuthority()).Disabled(p.loading || p.busy || p.review != "" || a.runtimeRecoveryBlocked()).OnClick(a.checkRuntimeAuthority)
			}
			if p.authority == nil {
				ui.Text(c, locales.RuntimeRecoveryAuthorityUnknown())
			} else {
				runtimeContextView(c, p.authority.Context)
				if p.authority.Admin {
					ui.Text(c, locales.RuntimeRecoveryAuthorityAdmin())
				} else {
					ui.Text(c, locales.RuntimeRecoveryAuthorityDenied()).TextColor(c.Theme().Danger)
				}
			}
			if p.review == "forget" {
				ui.Text(c, locales.RuntimeRecoveryForgetHelp()).Bold()
				if p.forget != nil {
					runtimeContextView(c, p.forget.Expected)
					ui.Text(c, locales.AccountExactID(p.forget.AccountID)).Bold()
					ui.Text(c, locales.RuntimeRecoveryRequest(p.forget.RequestID)).Bold()
					if p.forget.ExpectedDigest != nil {
						ui.Text(c, locales.RuntimeRecoveryDigest(*p.forget.ExpectedDigest)).FontSize(12)
					}
				}
			}
			if p.authority != nil && p.authority.Admin && !p.submitted {
				locked := p.loading || p.busy || p.review != ""
				ui.TextInput(c, &p.accountID).Label(locales.RuntimeRecoveryAccount()).FillWidth().Disabled(locked).OnChange(a.clearRuntimeReview)
				ui.TextInput(c, &p.originalTask).Label(locales.RuntimeRecoveryOriginalTask()).FillWidth().Disabled(locked).OnChange(a.clearRuntimeReview)
				ui.Row(c).Gap(8).Wrap().Children(func() {
					for _, action := range []string{"verify_target", "verify_source", "repair_source"} {
						if ui.Radio(c, &p.action, action, locales.RuntimeRecoveryAction(action)).Disabled(locked).Changed() {
							a.clearRuntimeReview()
						}
					}
				})
				if a.runtimeHasRetainedRequest() {
					ui.Text(c, locales.RuntimeRecoveryRetained()).TextColor(c.Theme().Danger)
				}
			}
			if p.plan != nil {
				a.runtimePlanView(c, *p.plan)
			}
			if p.review == "apply" {
				ui.Text(c, locales.RuntimeRecoveryConfirmHelp(p.plan.Action)).Bold().TextColor(c.Theme().Danger)
			}
			ui.Button(c, locales.RuntimeRecoveryPending()).Disabled(p.loading || p.busy || p.review != "" || a.runtimeRecoveryBlocked()).OnClick(a.loadRuntimePending)
			for _, record := range p.records {
				a.runtimeRecordView(c, record)
			}
			if p.pendingLoaded && len(p.records) == 0 {
				ui.Text(c, locales.RuntimeRecoveryNoPending())
			}
			if p.authority != nil && p.authority.Admin && !p.submitted && p.review == "" {
				a.runtimeManualView(c)
			}
		})
		a.runtimeRecoveryFooter(c)
	})
}
func runtimeContextView(c *ui.Context, value remote.RuntimeRecoveryContext) {
	ui.Text(c, value.Home).FontSize(12)
	ui.Text(c, value.ServerURL).FontSize(12)
	ui.Text(c, locales.RuntimeRecoveryActor(value.ActorUserID)).FontSize(12)
}
func runtimeOriginalView(c *ui.Context, value remote.RuntimeRecoveryOriginal) {
	ui.Text(c, locales.RuntimeRecoveryOwner(value.UserID)).FontSize(12)
	ui.Text(c, locales.RuntimeRecoveryOriginal(value.TaskID, value.TaskRecordID)).FontSize(12)
	ui.Text(c, locales.RuntimeRecoveryBackends(value.NodeID, value.SourceRuntimeBackend, value.TargetRuntimeBackend)).FontSize(12)
}
func (a *App) runtimePlanView(c *ui.Context, plan remote.RuntimeRecoveryPlan) {
	ui.Text(c, locales.RuntimeRecoveryObservation()).Bold()
	runtimeContextView(c, plan.Context)
	ui.Text(c, locales.AccountExactID(plan.AccountID))
	runtimeOriginalView(c, plan.Original)
	ui.Text(c, locales.RuntimeRecoveryAction(plan.Action))
	ui.Text(c, locales.RuntimeRecoveryObservedStatuses(locales.RuntimeRecoveryStatus(plan.OriginalStatus), locales.AccountState(plan.AccountStatus))).FontSize(12)
	ui.Text(c, locales.RuntimeRecoveryDigest(plan.Digest)).FontSize(12)
}
func runtimeReceiptView(c *ui.Context, value remote.RuntimeRecoveryReceipt) {
	ui.Text(c, locales.RuntimeRecoveryAction(value.Recovery.Binding.RecoveryAction()))
	ui.Text(c, locales.RuntimeRecoveryTask(value.Recovery.Binding.TaskID, value.Recovery.Binding.TaskRecordID)).FontSize(12)
	ui.Text(c, locales.RuntimeRecoveryStatus(value.Recovery.Status)).Bold()
	switch {
	case value.TargetCompletionConfirmed:
		ui.Text(c, locales.RuntimeRecoveryTargetConfirmed())
	case value.SourceRestorationConfirmed:
		ui.Text(c, locales.RuntimeRecoverySourceConfirmed())
	default:
		ui.Text(c, locales.RuntimeRecoveryUnconfirmed())
	}
}
func (a *App) runtimeRecordView(c *ui.Context, record runtimeRecoveryRecord) {
	ref := record.reference
	ui.Column(c.Key(ref.Context.ServerURL + ref.Context.ActorUserID + ref.RequestID)).FillWidth().Gap(6).Padding(10).Children(func() {
		ui.Text(c, locales.RuntimeRecoveryRequest(ref.RequestID)).Bold()
		runtimeContextView(c, ref.Context)
		ui.Text(c, locales.AccountExactID(ref.AccountID))
		runtimeOriginalView(c, ref.Original)
		ui.Text(c, locales.RuntimeRecoveryAction(ref.Action))
		if record.err != nil {
			a.errorText(c, record.err, runtimeRecoveryError(record.err))
		}
		if record.notSubmitted {
			ui.Text(c, locales.RuntimeRecoveryNotSubmitted())
			return
		}
		ui.Text(c, locales.RuntimeRecoverySubmission(ref.Phase))
		if ref.RejectionCode != nil {
			ui.Text(c, locales.RuntimeRecoveryError(*ref.RejectionCode))
		}
		if record.receipt != nil {
			runtimeReceiptView(c, *record.receipt)
		}
		ui.Text(c, locales.RuntimeRecoveryNoReplay()).FontSize(12)
		locked := a.runtimeRecovery.loading || a.runtimeRecovery.busy || a.runtimeRecoveryBlocked()
		ui.Button(c, locales.RuntimeRecoveryQuery()).Disabled(locked).OnClick(func() { a.queryRuntimeRecord(record) })
		if ref.Phase == "rejected" || record.receipt != nil && remote.RuntimeRecoveryTerminal(record.receipt.Recovery.Status) {
			ui.Button(c, locales.RuntimeRecoveryForget()).Disabled(locked).OnClick(func() { a.reviewRuntimeForget(record) })
		}
	})
}
func (a *App) runtimeManualView(c *ui.Context) {
	p := &a.runtimeRecovery
	locked := p.loading || p.busy || a.runtimeRecoveryBlocked()
	ui.Text(c, locales.RuntimeRecoveryManualTitle()).Bold()
	ui.Text(c, locales.RuntimeRecoveryManualHelp())
	clear := func() { p.manualReceipt = nil }
	ui.TextInput(c, &p.manualAccount).Label(locales.RuntimeRecoveryManualAccount()).FillWidth().Disabled(locked).OnChange(clear)
	ui.TextInput(c, &p.manualKey).Label(locales.RuntimeRecoveryManualKey()).FillWidth().Disabled(locked).OnChange(clear)
	ui.Button(c, locales.RuntimeRecoveryManualQuery()).Disabled(locked).OnClick(a.queryRuntimeManual)
	if p.manualReceipt != nil {
		ui.Text(c, locales.RuntimeRecoveryRequest(p.manualReceipt.RequestID))
		runtimeOriginalView(c, p.manualReceipt.Recovery.Binding.Original())
		runtimeReceiptView(c, *p.manualReceipt)
	}
}
func (a *App) runtimeRecoveryFooter(c *ui.Context) {
	p := &a.runtimeRecovery
	ui.Row(c).Gap(8).Wrap().Justify(ui.End).Children(func() {
		switch {
		case p.loading || p.busy:
			ui.Button(c, locales.RuntimeRecoveryStopWaiting()).OnClick(p.cancel)
		case p.review == "forget":
			ui.Button(c, locales.Cancel()).OnClick(func() { p.review = ""; p.forget = nil })
			ui.PrimaryButton(c, locales.RuntimeRecoveryForgetConfirm()).Disabled(a.runtimeRecoveryBlocked()).OnClick(a.forgetRuntimeRecord)
		case p.review == "apply":
			ui.Button(c, locales.Cancel()).OnClick(func() { p.review = "" })
			ui.PrimaryButton(c, locales.RuntimeRecoveryApplyConfirm()).Disabled(a.runtimeRecoveryBlocked()).OnClick(a.applyRuntimeRecovery)
		default:
			ui.Button(c, locales.RuntimeRecoveryFinish()).OnClick(a.finishRuntimeReview)
			if p.submitted {
				return
			}
			if p.authority != nil && p.authority.Admin {
				if p.plan == nil {
					ui.Button(c, locales.RuntimeRecoveryInspect()).Disabled(a.runtimeRecoveryBlocked()).OnClick(a.inspectRuntimeRecovery)
				} else {
					ui.PrimaryButton(c, locales.RuntimeRecoveryApply()).Disabled(a.runtimeRecoveryBlocked() || p.err != nil || a.runtimeHasRetainedRequest()).OnClick(func() { p.review = "apply" })
				}
			}
		}
	})
}
func runtimeRecoveryError(err error) string {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		switch operation.Code {
		case "unsupported_method", "runtime_recovery_response_invalid", "runtime_recovery_status_unknown", "runtime_recovery_outcome_unknown", "runtime_recovery_rejected", "runtime_recovery_review_changed", "runtime_recovery_registry_unavailable", "runtime_recovery_pending", "runtime_recovery_context_changed", "runtime_recovery_receipt_changed", "runtime_recovery_not_terminal", "invalid_params", "confirmation_required", "configuration_changed", "configuration_unavailable", "credential_unavailable", "authentication_required", "permission_denied", "canceled", "timeout":
			return locales.RuntimeRecoveryError(operation.Code)
		default:
			return presentation.Error(fault.CLIProtocol)
		}
	}
	return presentation.Error(err)
}
