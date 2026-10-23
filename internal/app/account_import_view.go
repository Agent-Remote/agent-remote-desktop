package app

import (
	"errors"
	"slices"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) accountImportView(c *ui.Context) {
	p := &a.accountImport
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(12).Children(func() {
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			if p.loading || p.busy {
				ui.Text(c, locales.AccountImportWorking())
				ui.Text(c, locales.AccountImportCancelHelp())
			}
			if p.err != nil {
				a.errorText(c, p.err, accountImportError(p.err))
			}
			if p.review == "forget" {
				ui.Text(c, locales.AccountImportForgetHelp()).Bold()
			}
			if p.history {
				ui.Text(c, locales.AccountImportPendingHelp())
				if p.pendingLoaded && len(p.records) == 0 {
					ui.Text(c, locales.AccountImportNoPending())
				}
			} else {
				ui.Text(c, locales.AccountImportHelp())
				importContextView(c, p.expected.Home, p.expected.ServerURL, p.expected.DeviceID, p.accountID)
				ui.Text(c, locales.AccountImportPrivacy()).TextColor(c.Theme().Danger)
				if !p.submitted && !p.loading && !p.busy && p.review == "" {
					if ui.Checkbox(c, &p.selection.IncludeResumeHistory, locales.AccountImportHistory()).Changed() {
						a.changeImportSelection()
					}
					if ui.Checkbox(c, &p.selection.ExcludeSkills, locales.AccountImportExcludeSkills()).Changed() {
						a.changeImportSelection()
					}
				}
				ui.Text(c, locales.AccountImportSkillsHelp()).FontSize(12)
				ui.Text(c, locales.AccountImportDiscoverHelp()).FontSize(12)
				if p.discovery != nil {
					ui.Text(c, locales.AccountImportSource(p.discovery.SourceHome)).FontSize(12)
					importScopeView(c, p.discovery.Include, p.discovery.Exclude)
					ui.Text(c, locales.AccountImportPrepareHelp())
				}
				if !p.submitted && a.accountImportHasRetainedRequest() {
					ui.Text(c, locales.AccountImportRetainedHelp()).TextColor(c.Theme().Danger)
				}
				if p.plan != nil {
					a.accountImportManifestView(c, *p.plan)
				}
				if p.validation != nil {
					accountImportValidationView(c, *p.validation)
				}
				switch p.review {
				case "validate":
					ui.Text(c, locales.AccountImportValidateHelp()).Bold()
				case "apply":
					ui.Text(c, locales.AccountImportApplyHelp()).Bold()
				}
			}
			for _, record := range p.records {
				a.accountImportRecordView(c, record)
			}
		})
		a.accountImportFooter(c)
	})
}
func importContextView(c *ui.Context, home string, server, device *string, account string) {
	ui.Text(c, home).FontSize(12)
	if server != nil {
		ui.Text(c, *server).FontSize(12)
	}
	if device != nil {
		ui.Text(c, *device).FontSize(12)
	}
	if account != "" {
		ui.Text(c, locales.AccountExactID(account)).Bold()
	}
}
func importScopeView(c *ui.Context, include, exclude []string) {
	ui.Text(c, locales.AccountImportInclude()).Bold()
	if len(include) == 0 {
		ui.Text(c, locales.AccountImportEmpty())
	}
	for _, path := range include {
		ui.Text(c, path).FontSize(12)
	}
	ui.Text(c, locales.AccountImportExclude()).Bold()
	for _, path := range exclude {
		ui.Text(c, path).FontSize(12)
	}
}
func (a *App) accountImportManifestView(c *ui.Context, plan remote.AccountImportPlan) {
	ui.Text(c, locales.AccountImportManifest()).Bold()
	ui.Text(c, locales.AccountImportDigest(plan.Digest)).FontSize(12)
	ui.Text(c, locales.AccountImportCount(networkNumber(uint64(len(plan.Manifest))), networkNumber(plan.TotalBytes)))
	if len(plan.Manifest) == 0 {
		ui.Text(c, locales.AccountImportEmpty())
		return
	}
	const pageSize = 8
	pages := (len(plan.Manifest) + pageSize - 1) / pageSize
	page := min(max(a.accountImport.manifestPage, 0), pages-1)
	for _, file := range plan.Manifest[page*pageSize : min((page+1)*pageSize, len(plan.Manifest))] {
		ui.Column(c.Key(file.Path)).FillWidth().Gap(3).Children(func() {
			ui.Text(c, locales.AccountImportFile(file.Path, networkNumber(file.Size))).FontSize(12)
			ui.Text(c, file.SHA256).FontSize(12).TextColor(c.Theme().TextMuted)
		})
	}
	if pages > 1 {
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Button(c, locales.AccountImportPreviousPage()).Disabled(page == 0).OnClick(func() { a.accountImport.manifestPage = page - 1 })
			ui.Text(c, locales.AccountImportPage(networkNumber(uint64(page+1)), networkNumber(uint64(pages))))
			ui.Button(c, locales.AccountImportNextPage()).Disabled(page+1 == pages).OnClick(func() { a.accountImport.manifestPage = page + 1 })
		})
	}
}
func accountImportValidationView(c *ui.Context, value remote.AccountImportValidation) {
	ui.Text(c, locales.AccountImportValidateHelp()).FontSize(12)
	ui.Text(c, locales.AccountImportValidationCounts(networkNumber(uint64(len(value.Accepted))), networkNumber(uint64(len(value.Rejected))))).Bold()
	for _, path := range value.Rejected {
		ui.Text(c, path).FontSize(12).TextColor(c.Theme().Danger)
	}
	for _, code := range value.WarningCodes {
		ui.Text(c, locales.AccountImportWarning(code)).TextColor(c.Theme().Danger)
	}
}
func (a *App) accountImportRecordView(c *ui.Context, record accountImportRecord) {
	ref := record.reference
	ui.Column(c.Key(ref.RequestID)).FillWidth().Gap(6).Padding(10).Children(func() {
		ui.Text(c, locales.AccountImportRequest(ref.RequestID)).Bold()
		importContextView(c, ref.Context.Home, ref.Context.ServerURL, ref.Context.DeviceID, ref.AccountID)
		ui.Text(c, locales.AccountImportDigest(ref.Digest)).FontSize(12)
		if !record.notSubmitted {
			ui.Text(c, locales.AccountImportPhase(ref.Phase))
		}
		if ref.TaskID != nil {
			ui.Text(c, locales.AccountImportTask(*ref.TaskID)).FontSize(12)
		}
		if record.err != nil {
			a.errorText(c, record.err, accountImportError(record.err))
		}
		if record.notSubmitted {
			ui.Text(c, locales.AccountImportNotSubmitted())
			return
		}
		if record.receipt != nil {
			for _, code := range record.receipt.WarningCodes {
				ui.Text(c, locales.AccountImportWarning(code)).TextColor(c.Theme().Danger)
			}
		}
		ui.Text(c, locales.AccountImportNoReplay()).FontSize(12)
		if slices.Contains([]string{"rejected", "succeeded", "failed", "cancelled", "expired"}, ref.Phase) {
			ui.Button(c, locales.AccountImportForget()).Disabled(a.accountImport.loading || a.accountImport.busy || a.importBlocked()).OnClick(func() { a.accountImport.forget = &ref; a.accountImport.review = "forget" })
		}
		ui.Button(c, locales.AccountImportStatus()).Disabled(a.accountImport.loading || a.accountImport.busy || a.importBlocked()).OnClick(func() { a.queryAccountImport(ref) })
	})
}
func (a *App) accountImportFooter(c *ui.Context) {
	p := &a.accountImport
	ui.Row(c).Gap(8).Wrap().Justify(ui.End).Children(func() {
		switch {
		case p.loading || p.busy:
			ui.Button(c, locales.AccountImportStopWaiting()).OnClick(p.cancel)
		case p.review == "forget":
			ui.Button(c, locales.Cancel()).OnClick(func() { p.review = ""; p.forget = nil })
			ui.PrimaryButton(c, locales.AccountImportForgetConfirm()).Disabled(a.importBlocked()).OnClick(a.forgetAccountImport)
		case p.history:
			if p.accountID != "" {
				ui.Button(c, locales.AccountImportShowReview()).OnClick(func() { p.history = false })
			}
			ui.Button(c, locales.AccountImportPending()).Disabled(a.importBlocked()).OnClick(a.loadAccountImportPending)
		case p.review != "":
			ui.Button(c, locales.Cancel()).OnClick(func() { p.review = "" })
			if p.review == "validate" {
				ui.PrimaryButton(c, locales.AccountImportValidateConfirm()).Disabled(a.importBlocked()).OnClick(a.validateAccountImport)
			} else {
				ui.PrimaryButton(c, locales.AccountImportApplyConfirm()).Disabled(a.importBlocked()).OnClick(a.applyAccountImport)
			}
		default:
			ui.Button(c, locales.AccountImportNew()).OnClick(a.finishAccountImport)
			if p.submitted {
				return
			}
			ui.Button(c, locales.AccountImportDiscover()).Disabled(a.importBlocked() || p.accountID == "").OnClick(a.discoverAccountImport)
			if p.discovery != nil {
				ui.Button(c, locales.AccountImportPrepare()).Disabled(a.importBlocked()).OnClick(a.prepareAccountImport)
				ui.Button(c, locales.AccountImportValidate()).Disabled(a.importBlocked()).OnClick(func() { p.review = "validate" })
			}
			if p.plan != nil {
				ui.PrimaryButton(c, locales.AccountImportApply()).Disabled(a.importBlocked() || len(p.plan.Manifest) == 0 || p.err != nil || a.accountImportHasRetainedRequest() || p.validation != nil && len(p.validation.Rejected) > 0).OnClick(func() { p.review = "apply" })
			}
		}
	})
}
func accountImportError(err error) string {
	if errors.Is(err, fault.AccountOutcomeUnknown) {
		return locales.AccountImportUnknown()
	}
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		if operation.Code == "unsupported_method" {
			return locales.AccountImportUnavailable()
		}
		return locales.AccountImportErrorCode(operation.Code)
	}
	return presentation.Error(err)
}
