package app

import (
	"context"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) dependenciesView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Button(c, locales.ReturnDiagnostics()).OnClick(a.openDiagnostics)
		})
		ui.Text(c, locales.DependenciesHelp()).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.DependenciesReadOnly()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Button(c, locales.CommandDiagnosticsDependencies()).OnClick(func() { a.openCommandDiagnostics("deps.inspect") })
			ui.Checkbox(c, &a.dependencies.verifyVersions, locales.DiagnosticsVersions()).Disabled(a.dependencies.busy)
			ui.Button(c, locales.DependenciesRefresh()).Disabled(a.dependencies.busy).OnClick(func() { a.refreshDependencies(a.dependencies.verifyVersions) })
			ui.Button(c, locales.DependenciesVersions()).Disabled(a.dependencies.busy).OnClick(func() {
				a.dependencies.verifyVersions = true
				a.refreshDependencies(true)
			})
		})
		if a.dependencies.busy {
			ui.Text(c, locales.DependenciesBusy()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.dependencies.cancel)
			return
		}
		if err := a.dependencies.err; err != nil {
			text := presentation.Error(err)
			var operation *cliadapter.OperationError
			if errors.Is(err, context.Canceled) {
				text = locales.DependenciesCanceled()
			} else if errors.As(err, &operation) && operation.Code == "unsupported_method" {
				text = locales.DependenciesUnsupported()
			}
			ui.Text(c, text).TextColor(c.Theme().Danger)
			return
		}
		if !a.dependencies.loaded {
			return
		}
		report := a.dependencies.report
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			ui.Text(c, locales.DependenciesPlatform(report.Platform)).FontSize(12)
			ui.Text(c, report.Home).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, locales.DiagnosticsManifest(report.ManifestSource)).FontSize(12)
			ui.Text(c, locales.DiagnosticsVersionHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			if len(report.Dependencies) == 0 {
				ui.Text(c, locales.DependenciesEmpty()).TextColor(c.Theme().TextMuted)
			}
			for index, dependency := range report.Dependencies {
				dependency := dependency
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
