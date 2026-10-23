package app

import (
	"context"
	"errors"
	"strconv"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) mutagenEnvironmentView(c *ui.Context) {
	p := &a.mutagenEnvironment
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.MutagenEnvironmentTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = "" })
		})
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			ui.Text(c, locales.MutagenEnvironmentHelp())
			ui.Text(c, a.home).FontSize(12)
			if p.loading {
				ui.Text(c, locales.MutagenEnvironmentInspecting())
			}
			if p.busy {
				ui.Text(c, locales.MutagenEnvironmentWorking())
				ui.Text(c, locales.MutagenEnvironmentCancelHelp())
			}
			if p.err != nil {
				ui.Text(c, mutagenEnvironmentError(p.err)).TextColor(c.Theme().Danger)
			}
			if p.plan != nil {
				ui.Text(c, locales.MutagenEnvironmentReview()).Bold()
				ui.Text(c, locales.MutagenEnvironmentImpact()).TextColor(c.Theme().Danger)
				a.mutagenEnvironmentPlanView(c, *p.plan, "preview")
			}
			for index, record := range p.records {
				ui.Column(c.Key(index)).FillWidth().Gap(8).Children(func() {
					ui.Text(c, locales.MutagenEnvironmentOperation()).Bold()
					if record.err != nil {
						ui.Text(c, mutagenEnvironmentError(record.err)).TextColor(c.Theme().Danger)
					}
					if receipt := record.receipt; receipt != nil {
						ui.Text(c, locales.MutagenEnvironmentOutcome(receipt.Outcome)).Bold()
						ui.Text(c, locales.SyncField("operation_id", receipt.OperationID)).FontSize(12)
						for _, stage := range receipt.Stages {
							ui.Text(c, locales.MutagenEnvironmentStage(stage.Name, stage.State))
						}
						if receipt.ErrorCode != nil {
							ui.Text(c, mutagenEnvironmentError(&cliadapter.OperationError{Code: *receipt.ErrorCode})).TextColor(c.Theme().Danger)
						}
						if receipt.Environment != nil {
							ui.Text(c, locales.MutagenEnvironmentReady()).TextColor(c.Theme().TextMuted)
						}
					}
					ui.Text(c, locales.MutagenEnvironmentReceiptScope()).Bold()
					a.mutagenEnvironmentPlanView(c, record.plan, "record-"+strconv.Itoa(index))
					ui.Text(c, locales.MutagenEnvironmentNoReplay()).FontSize(12)
				})
			}
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			switch {
			case p.busy:
				ui.Button(c, locales.SyncStopWaiting()).OnClick(p.cancel)
			case p.loading:
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.cancelMutagenEnvironmentPreview)
			default:
				ui.Button(c, locales.SyncDone()).OnClick(a.finishMutagenEnvironment)
				ui.Button(c, locales.MutagenEnvironmentInspect()).Disabled(a.mutagenEnvironmentBlocked()).OnClick(a.previewMutagenEnvironment)
				if p.plan != nil {
					ui.PrimaryButton(c, locales.MutagenEnvironmentConfirm()).Disabled(!p.plan.Repairable || p.submitted || a.mutagenEnvironmentBlocked()).OnClick(a.applyMutagenEnvironment)
				}
			}
		})
	})
}
func (a *App) mutagenEnvironmentPlanView(c *ui.Context, plan remote.MutagenEnvironmentPlan, key string) {
	ui.Text(c, locales.MutagenEnvironmentState(plan.DaemonState, plan.Registration))
	for _, field := range []struct{ name, value string }{{"data_directory", plan.DataDirectory}, {"fingerprint", plan.Fingerprint}, {"inventory_fingerprint", plan.InventoryFingerprint}} {
		ui.Text(c, locales.MutagenEnvironmentField(field.name, field.value)).FontSize(12)
	}
	for _, field := range []struct {
		name  string
		value *string
	}{{"namespace_fingerprint", plan.NamespaceFingerprint}, {"running_fingerprint", plan.RunningFingerprint}, {"marker_fingerprint", plan.MarkerFingerprint}} {
		if field.value != nil {
			ui.Text(c, locales.MutagenEnvironmentField(field.name, *field.value)).FontSize(12)
		}
	}
	ui.Text(c, locales.MutagenEnvironmentPrevious()).Bold()
	if plan.Previous == nil {
		ui.Text(c, locales.MutagenEnvironmentPreviousUnknown())
	} else {
		mutagenBindingView(c, *plan.Previous)
	}
	ui.Text(c, locales.MutagenEnvironmentDesired()).Bold()
	mutagenBindingView(c, plan.Desired)
	for _, tool := range plan.Tools {
		ui.Text(c, locales.MutagenEnvironmentTool(tool.Name, tool.Path)).FontSize(12)
		ui.Text(c, locales.MutagenEnvironmentField("tool_fingerprint", tool.Fingerprint)).FontSize(12)
	}
	if plan.ErrorCode != nil {
		ui.Text(c, mutagenEnvironmentError(&cliadapter.OperationError{Code: *plan.ErrorCode})).TextColor(c.Theme().Danger)
	}
	ui.Text(c, locales.MutagenEnvironmentSessions()).Bold()
	ui.Text(c, locales.MutagenEnvironmentSessionCount(networkNumber(uint64(len(plan.Sessions)))))
	if len(plan.Sessions) == 0 {
		ui.Text(c, locales.MutagenEnvironmentNoSessions())
		return
	}
	const pageSize = 8
	pages := (len(plan.Sessions) + pageSize - 1) / pageSize
	if a.mutagenEnvironment.sessionPages == nil {
		a.mutagenEnvironment.sessionPages = map[string]int{}
	}
	page := min(max(a.mutagenEnvironment.sessionPages[key], 0), pages-1)
	for index, session := range plan.Sessions[page*pageSize : min((page+1)*pageSize, len(plan.Sessions))] {
		ui.Column(c.Key(key + session.Identifier)).FillWidth().Gap(4).Padding(8).Children(func() {
			ui.Text(c, locales.MutagenEnvironmentSession(session.Kind, networkNumber(uint64(page*pageSize+index+1)))).Bold()
			ui.Text(c, locales.MutagenEnvironmentField("identifier", session.Identifier)).FontSize(12)
			if session.Name != "" {
				ui.Text(c, locales.MutagenEnvironmentField("name", session.Name)).FontSize(12)
			}
			ui.Text(c, locales.MutagenEnvironmentPaused(strconv.FormatBool(session.Paused)))
			mutagenEndpointView(c, "source", session.Source)
			mutagenEndpointView(c, "destination", session.Destination)
			ui.Text(c, locales.MutagenEnvironmentField("configuration_fingerprint", session.ConfigurationFingerprint)).FontSize(12)
		})
	}
	if pages > 1 {
		ui.Row(c.Key(key)).Gap(8).Wrap().Children(func() {
			ui.Button(c, locales.MutagenEnvironmentPreviousPage()).Disabled(page == 0).OnClick(func() { a.mutagenEnvironment.sessionPages[key] = page - 1 })
			ui.Text(c, locales.MutagenEnvironmentPage(networkNumber(uint64(page+1)), networkNumber(uint64(pages))))
			ui.Button(c, locales.MutagenEnvironmentNextPage()).Disabled(page+1 == pages).OnClick(func() { a.mutagenEnvironment.sessionPages[key] = page + 1 })
		})
	}
}
func mutagenBindingView(c *ui.Context, binding remote.MutagenEnvironmentBinding) {
	ui.Text(c, locales.MutagenEnvironmentField("home", binding.Home)).FontSize(12)
	ui.Text(c, locales.MutagenEnvironmentField("ssh_directory", binding.SSHDirectory)).FontSize(12)
	ui.Text(c, locales.MutagenEnvironmentField("binding_fingerprint", binding.Fingerprint)).FontSize(12)
}
func mutagenEndpointView(c *ui.Context, side string, endpoint remote.MutagenEnvironmentEndpoint) {
	ui.Text(c, locales.MutagenEnvironmentEndpoint(side, endpoint.Protocol, endpoint.User, endpoint.Host, networkNumber(uint64(endpoint.Port)), endpoint.Path)).FontSize(12)
}
func mutagenEnvironmentError(err error) string {
	if errors.Is(err, context.Canceled) {
		return locales.MutagenEnvironmentQueryCanceled()
	}
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		if operation.Code == "unsupported_method" {
			return locales.MutagenEnvironmentUnavailable()
		}
		if operation.Code == "sync_outcome_unknown" || operation.Code == "sync_mutagen_environment_outcome_unknown" {
			return locales.MutagenEnvironmentUnknown()
		}
		return locales.MutagenEnvironmentResultCode(operation.Code)
	}
	return presentation.Error(err)
}
