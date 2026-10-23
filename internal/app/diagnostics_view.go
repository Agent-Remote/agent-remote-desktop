package app

import (
	"context"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) diagnosticsView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		ui.Text(c, locales.DiagnosticsHelp()).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.DiagnosticsDeepHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Checkbox(c, &a.diagnostics.online, locales.DiagnosticsOnline()).Disabled(a.diagnostics.busy)
			ui.Button(c, locales.DiagnosticsRefresh()).Disabled(a.diagnostics.busy).OnClick(func() { a.refreshDiagnostics(a.diagnostics.online, false, true) })
			ui.Button(c, locales.DiagnosticsVersions()).Disabled(a.diagnostics.busy).OnClick(func() { a.refreshDiagnostics(a.diagnostics.online, true, true) })
		})
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Button(c, locales.CommandDiagnosticsStatus()).OnClick(func() { a.openCommandDiagnostics("status.inspect") })
			ui.Button(c, locales.CommandDiagnosticsDoctor()).OnClick(func() { a.openCommandDiagnostics("doctor.inspect") })
			ui.Button(c, locales.CommandDiagnosticsSSH()).OnClick(func() { a.openCommandDiagnostics("ssh.inspect") })
			ui.Button(c, locales.DependenciesTitle()).OnClick(a.openDependencies)
			ui.Button(c, locales.MutagenEnvironmentTitle()).OnClick(a.openMutagenEnvironment)
			ui.Button(c, locales.LocalSetupTitle()).OnClick(a.openLocalSetup)
			ui.Button(c, locales.WireGuardConfigTitle()).OnClick(a.openWireGuard)
			ui.Button(c, locales.WireGuardNetworkTitle()).OnClick(a.openNetwork)
		})
		if a.diagnostics.busy {
			ui.Text(c, locales.DiagnosticsBusy()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.diagnostics.cancel)
			return
		}
		if err := a.diagnostics.err; err != nil {
			text := presentation.Error(err)
			var operation *cliadapter.OperationError
			if errors.Is(err, context.Canceled) {
				text = locales.DiagnosticsCanceled()
			} else if errors.As(err, &operation) && operation.Code == "unsupported_method" {
				text = locales.DiagnosticsUnsupported()
			}
			ui.Text(c, text).TextColor(c.Theme().Danger)
			return
		}
		if !a.diagnostics.loaded {
			return
		}
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			ui.Text(c, a.diagnostics.report.Home).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, locales.DiagnosticsChecks()).Bold()
			for _, check := range a.diagnostics.report.Checks {
				a.card(c, ui.Column(c.Key(check.ID)).FillWidth().Gap(4).Padding(10)).Children(func() {
					ui.Row(c).Gap(12).Wrap().Children(func() {
						ui.Text(c, locales.DiagnosticsCheckLabel(check.ID)).Bold()
						ui.Text(c, locales.DiagnosticsStatus(check.Status)).TextColor(c.Theme().TextMuted)
					})
					if check.Path != nil {
						ui.Text(c, *check.Path).FontSize(12).TextColor(c.Theme().TextMuted)
					}
					if check.Suggestion != "none" {
						ui.Text(c, locales.DiagnosticsSuggestion(check.Suggestion)).FontSize(12)
					}
				})
			}
			ui.Text(c, locales.DiagnosticsDependencies()).Bold()
			ui.Text(c, locales.DiagnosticsManifest(a.diagnostics.report.ManifestSource)).FontSize(12)
			ui.Text(c, locales.DiagnosticsVersionHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			for index, dependency := range a.diagnostics.report.Dependencies {
				a.card(c, ui.Column(c.Key(index)).FillWidth().Gap(5).Padding(12)).Children(func() {
					ui.Row(c).Gap(12).Wrap().Children(func() {
						ui.Text(c, dependency.Name).Bold()
						ui.Text(c, locales.DiagnosticsStatus(dependency.Status)).TextColor(c.Theme().TextMuted)
					})
					ui.Text(c, dependency.Path).FontSize(12).TextColor(c.Theme().TextMuted)
					ui.Text(c, locales.DiagnosticsVersionRequirement(dependency.RequiredVersion)).FontSize(12)
					ui.Text(c, locales.DiagnosticsVersionStatus(dependency.VersionStatus)).FontSize(12)
					if dependency.VersionOutput != nil {
						ui.Text(c, *dependency.VersionOutput).FontSize(12)
					}
					ui.Text(c, locales.DiagnosticsLicense(dependency.License)).FontSize(12)
					ui.Text(c, dependency.LicenseNotice).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
		})
	})
}
