package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type commandDiagnosticsPage struct {
	method       string
	options      remote.CommandDiagnosticOptions
	report       remote.CommandDiagnosticReport
	ssh          remote.SSHInspectReport
	err          error
	busy, loaded bool
	confirmed    bool
	repairOpen   bool
	cancel       context.CancelFunc
	done         chan struct{}
	results      chan commandDiagnosticsResult
	generation   uint64
}

type commandDiagnosticsResult struct {
	generation uint64
	report     remote.CommandDiagnosticReport
	ssh        remote.SSHInspectReport
	err        error
}

func (a *App) openCommandDiagnostics(method string) {
	a.page = "command-diagnostics"
	if method != a.commandDiagnostics.method {
		a.commandDiagnostics = commandDiagnosticsPage{method: method, options: commandDiagnosticDefaults(method)}
	}
	if !a.commandDiagnostics.busy {
		a.refreshCommandDiagnostics()
	}
}

func commandDiagnosticDefaults(method string) remote.CommandDiagnosticOptions {
	switch method {
	case "doctor.inspect":
		return remote.CommandDiagnosticOptions{Online: true, DeepCheck: true, VerifyVersions: true}
	case "doctor.repair":
		return remote.CommandDiagnosticOptions{Online: true, DeepCheck: true, VerifyVersions: true}
	case "deps.inspect", "deps.repair":
		return remote.CommandDiagnosticOptions{VerifyVersions: true}
	default:
		return remote.CommandDiagnosticOptions{}
	}
}

func (a *App) refreshCommandDiagnostics() {
	p := &a.commandDiagnostics
	if p.busy || a.closing || a.quitting.Load() || p.method == "" || p.method == "doctor.repair" || p.method == "deps.repair" {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	p.results = make(chan commandDiagnosticsResult, 1)
	p.busy, p.loaded, p.err = true, false, nil
	results, done, home, generation, find, method, options := p.results, p.done, a.home, p.generation, a.findExecutable, p.method, p.options
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var report remote.CommandDiagnosticReport
		var ssh remote.SSHInspectReport
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			switch method {
			case "status.inspect":
				report, err = remote.InspectStatus(ctx, client, options.Online)
			case "doctor.inspect":
				report, err = remote.InspectDoctor(ctx, client, options)
			case "deps.inspect":
				report, err = remote.InspectDependenciesCommand(ctx, client, options.VerifyVersions)
			case "ssh.inspect":
				ssh, err = remote.InspectSSH(ctx, client, nil, nil, nil)
			default:
				err = fault.CLIProtocol
			}
		}
		results <- commandDiagnosticsResult{generation: generation, report: report, ssh: ssh, err: err}
	}()
}

func (a *App) repairCommandDiagnostics() {
	p := &a.commandDiagnostics
	if p.busy || a.closing || a.quitting.Load() || !p.confirmed || (p.method != "doctor.repair" && p.method != "deps.repair") {
		return
	}
	p.confirmed = false
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.done = make(chan struct{})
	p.results = make(chan commandDiagnosticsResult, 1)
	p.busy, p.loaded, p.err = true, false, nil
	results, done, home, generation, find, method, options := p.results, p.done, a.home, p.generation, a.findExecutable, p.method, p.options
	go func() {
		defer close(done)
		defer cancel()
		binary, err := find("agent-remote")
		var report remote.CommandDiagnosticReport
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			repair := remote.CommandRepairOptions{CommandDiagnosticOptions: options, Confirmed: true, ExpectedHome: home}
			if method == "doctor.repair" {
				report, err = remote.RepairDoctor(ctx, client, repair)
			} else {
				report, err = remote.RepairDependenciesCommand(ctx, client, repair)
			}
		}
		results <- commandDiagnosticsResult{generation: generation, report: report, err: err}
	}()
}

func (a *App) pollCommandDiagnostics(c *ui.Context) {
	p := &a.commandDiagnostics
	if !p.busy {
		return
	}
	select {
	case result := <-p.results:
		p.busy = false
		if result.generation == p.generation {
			p.loaded, p.report, p.ssh, p.err = true, result.report, result.ssh, result.err
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) invalidateCommandDiagnostics() {
	p := &a.commandDiagnostics
	p.generation++
	p.loaded, p.err = false, nil
	if p.busy && p.cancel != nil {
		if p.method == "doctor.repair" || p.method == "deps.repair" {
			p.err = fault.CLIContextChanged
		}
		p.cancel()
	} else if a.page == "command-diagnostics" && p.method != "doctor.repair" && p.method != "deps.repair" {
		a.refreshCommandDiagnostics()
	} else if p.method == "doctor.repair" || p.method == "deps.repair" {
		// A context change after a repair request must not replay the mutation.
		p.err = fault.CLIContextChanged
	}
}

func (a *App) commandDiagnosticsView(c *ui.Context) {
	p := &a.commandDiagnostics
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Button(c, locales.ReturnDiagnostics()).OnClick(a.openDiagnostics)
		})
		ui.Text(c, locales.CommandDiagnosticsMethod(p.method)).FontSize(12).TextColor(c.Theme().TextMuted)
		if p.method == "doctor.inspect" || p.method == "doctor.repair" {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Checkbox(c, &p.options.Online, locales.DiagnosticsOnline()).Disabled(p.busy)
				ui.Checkbox(c, &p.options.DeepCheck, locales.DiagnosticsDeepHelp()).Disabled(p.busy)
				ui.Checkbox(c, &p.options.VerifyVersions, locales.DiagnosticsVersions()).Disabled(p.busy)
			})
		} else if p.method == "status.inspect" {
			ui.Checkbox(c, &p.options.Online, locales.DiagnosticsOnline()).Disabled(p.busy)
		} else if p.method == "deps.inspect" || p.method == "deps.repair" {
			ui.Checkbox(c, &p.options.VerifyVersions, locales.DiagnosticsVersions()).Disabled(p.busy)
		}
		if p.method == "doctor.inspect" || p.method == "deps.inspect" || p.method == "status.inspect" || p.method == "ssh.inspect" {
			ui.Button(c, locales.DiagnosticsRefresh()).Disabled(p.busy).OnClick(a.refreshCommandDiagnostics)
		}
		if p.method == "doctor.inspect" || p.method == "doctor.repair" {
			ui.Button(c, locales.CommandDiagnosticsRepairDoctor()).Disabled(p.busy).OnClick(func() {
				p.method = "doctor.repair"
				p.confirmed = false
				p.repairOpen = true
			})
		}
		if p.method == "deps.inspect" || p.method == "deps.repair" {
			ui.Button(c, locales.CommandDiagnosticsRepairDependencies()).Disabled(p.busy).OnClick(func() {
				p.method = "deps.repair"
				p.confirmed = false
				p.repairOpen = true
			})
		}
		if p.busy {
			ui.Text(c, locales.DiagnosticsBusy()).TextColor(c.Theme().TextMuted)
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(p.cancel)
			return
		}
		if p.err != nil {
			a.errorText(c, p.err, presentation.Error(p.err))
			return
		}
		if !p.loaded {
			return
		}
		if p.method == "ssh.inspect" {
			a.renderSSHInspection(c, p.ssh)
			return
		}
		a.renderCommandDiagnosticReport(c, p.report)
	})
	a.commandDiagnosticsRepairDialog(c)
}

// commandDiagnosticsRepairDialog is the explicit confirmation boundary for
// the mutating doctor/dependency workflows. Opening a diagnostics page only
// performs a read; the control request is issued after the user reviews the
// exact CLI operation and managed home in this dialog.
func (a *App) commandDiagnosticsRepairDialog(c *ui.Context) {
	p := &a.commandDiagnostics
	ui.Modal(c, &p.repairOpen, func() {
		command := "agent-remote doctor --fix"
		if p.method == "deps.repair" {
			command = "agent-remote deps status --fix"
		}
		ui.Column(c).Width(560).Gap(14).Children(func() {
			ui.Text(c, locales.CommandDiagnosticsRepairReview(command)).FontSize(22).Bold()
			ui.Text(c, locales.CommandDiagnosticsRepairHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Text(c, locales.CommandDiagnosticsRepairHome(a.home)).FontSize(12)
			ui.Text(c, locales.CommandDiagnosticsMethod(p.method)).FontSize(12).TextColor(c.Theme().TextMuted)
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).OnClick(func() {
					p.repairOpen = false
					p.confirmed = false
					if p.method == "doctor.repair" {
						p.method = "doctor.inspect"
					} else {
						p.method = "deps.inspect"
					}
				})
				ui.PrimaryButton(c, locales.CommandDiagnosticsRepairConfirm()).OnClick(func() {
					p.repairOpen = false
					p.confirmed = true
					a.repairCommandDiagnostics()
				})
			})
		})
	})
}

func (a *App) renderCommandDiagnosticReport(c *ui.Context, report remote.CommandDiagnosticReport) {
	ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
		ui.Text(c, report.Home).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.DiagnosticsManifest(report.ManifestSource)).FontSize(12)
		ui.Text(c, locales.DiagnosticsChecks()).Bold()
		for _, check := range report.Checks {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Text(c, locales.DiagnosticsCheckLabel(check.ID)).Bold()
				ui.Text(c, locales.DiagnosticsStatus(check.Status)).TextColor(c.Theme().TextMuted)
			})
		}
		ui.Text(c, locales.DiagnosticsDependencies()).Bold()
		for _, dependency := range report.Dependencies {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Text(c, dependency.Name).Bold()
				ui.Text(c, locales.DiagnosticsStatus(dependency.Status)).TextColor(c.Theme().TextMuted)
				ui.Text(c, locales.DiagnosticsVersionStatus(dependency.VersionStatus))
			})
		}
		if report.Remote != nil {
			ui.Text(c, locales.DiagnosticsCheckLabel("server")+": "+locales.DiagnosticsStatus(report.Remote.Server.Status)).FontSize(12)
			ui.Text(c, locales.DiagnosticsCheckLabel("device")+": "+locales.DiagnosticsStatus(report.Remote.Device.Status)).FontSize(12)
		}
		if report.Setup != nil {
			ui.Text(c, locales.LocalSetupStatus(boolStatus(report.Setup.Complete))).FontSize(12)
		}
		if report.Repair != nil {
			ui.Text(c, locales.LocalSetupStatus(boolStatus(report.Repair.Complete))).FontSize(12)
		}
	})
}

func (a *App) renderSSHInspection(c *ui.Context, report remote.SSHInspectReport) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, report.Home).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.CommandDiagnosticsSSHVersion(report.SSH.Version)).FontSize(12)
		ui.Text(c, locales.CommandDiagnosticsAuthorization(report.Authorization.Status)).FontSize(12)
	})
}

func boolStatus(complete bool) string {
	if complete {
		return "ready"
	}
	return "failed"
}
