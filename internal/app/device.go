package app

import (
	"context"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type devicePage struct {
	snapshot  *remote.DeviceOverview
	local     *remote.DeviceLocalStatus
	localErr  error
	plan      *remote.DeviceLocalPlan
	planErr   error
	review    *remote.DeviceActionInput
	action    string
	loading   bool
	busy      bool
	submitted bool
	err       error
	receipt   *remote.DeviceReceipt
	cancel    context.CancelFunc
	results   chan deviceResult
}

type deviceResult struct {
	snapshot remote.DeviceOverview
	local    remote.DeviceLocalStatus
	localErr error
	plan     remote.DeviceLocalPlan
	planErr  error
	receipt  remote.DeviceReceipt
	err      error
}

func (a *App) openDevice() {
	a.page = "device"
	if a.device.snapshot == nil && !a.device.loading && !a.device.busy && !a.device.submitted {
		a.loadDevice()
	}
}

func (a *App) deviceBlocked() bool {
	return a.closing || a.quitting.Load() || a.device.loading || a.device.busy
}

func (a *App) loadDevice() {
	p := &a.device
	if a.deviceBlocked() {
		return
	}
	p.snapshot, p.local, p.review, p.receipt, p.plan, p.err, p.localErr, p.planErr = nil, nil, nil, nil, nil, nil, nil, nil
	p.submitted = false
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.results, p.loading = cancel, make(chan deviceResult, 1), true
	results, home, find := p.results, a.home, a.findExecutable
	go func() {
		binary, err := find("agent-remote")
		var snapshot remote.DeviceOverview
		var local remote.DeviceLocalStatus
		var localErr error
		var plan remote.DeviceLocalPlan
		var planErr error
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			snapshot, err = remote.LoadDevice(ctx, client)
			local, localErr = remote.LoadDeviceLocalStatus(ctx, client)
			plan, planErr = remote.LoadDeviceLocalPlan(ctx, client, remote.DeviceLocalPlanInput{ExpectedHome: home, Action: "diagnose"})
		}
		results <- deviceResult{snapshot: snapshot, local: local, localErr: localErr, plan: plan, planErr: planErr, err: err}
	}()
}

func (a *App) reviewDevice(action string) {
	p := &a.device
	if p.snapshot == nil || a.deviceBlocked() || p.submitted || !remote.DeviceSupported(p.snapshot.Capabilities, "device."+action) {
		return
	}
	p.action = action
	p.review = &remote.DeviceActionInput{Expected: p.snapshot.Context, DeviceID: p.snapshot.Device.ID}
	p.err = nil
}

func (a *App) submitDevice() {
	p := &a.device
	if p.review == nil || a.deviceBlocked() || p.submitted {
		return
	}
	input := *p.review
	input.Confirmed = true
	ctx, cancel := context.WithCancel(context.Background())
	p.cancel, p.results, p.busy, p.submitted = cancel, make(chan deviceResult, 1), true, true
	results, home, find, action := p.results, a.home, a.findExecutable, p.action
	go func() {
		binary, err := find("agent-remote")
		var receipt remote.DeviceReceipt
		if err == nil {
			client := cliadapter.Client{Binary: binary, Home: home}
			if action == "revoke" {
				receipt, err = remote.RevokeDevice(ctx, client, input)
			} else {
				receipt, err = remote.RotateDeviceToken(ctx, client, input)
			}
		}
		results <- deviceResult{receipt: receipt, err: err}
	}()
}

func (a *App) pollDevice(c *ui.Context) {
	p := &a.device
	if !p.loading && !p.busy {
		return
	}
	select {
	case result := <-p.results:
		mutation := p.busy
		p.loading, p.busy = false, false
		p.err = result.err
		if mutation {
			if result.err == nil {
				p.receipt = &result.receipt
			}
			a.invalidateCapabilities()
		} else {
			p.localErr = result.localErr
			p.planErr = result.planErr
			if result.localErr == nil {
				p.local = &result.local
			}
			if result.planErr == nil {
				p.plan = &result.plan
			}
			if result.err == nil {
				p.snapshot = &result.snapshot
			}
		}
		if p.cancel != nil {
			p.cancel()
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) cancelDeviceWait() {
	if a.device.cancel != nil {
		a.device.cancel()
	}
}

func (a *App) deviceView(c *ui.Context) {
	p := &a.device
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.DeviceTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(func() { a.page = "" })
		})
		ui.Text(c, locales.DeviceHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Scroll(c).Grow(1).FillWidth().Gap(10).Children(func() {
			if p.loading {
				ui.Text(c, locales.DeviceLoading())
				return
			}
			if p.err != nil {
				ui.Text(c, presentation.Error(p.err)).TextColor(c.Theme().Danger)
				if p.local != nil {
					a.deviceLocalDetailsView(c, p.local)
				}
				return
			}
			if p.receipt != nil {
				a.deviceReceiptView(c, p.receipt)
				return
			}
			if p.snapshot == nil {
				return
			}
			if p.review != nil {
				a.deviceReviewView(c, p.review)
				return
			}
			a.deviceDetailsView(c, p.snapshot)
			if p.local != nil {
				a.deviceLocalDetailsView(c, p.local)
			} else if p.localErr != nil {
				ui.Text(c, locales.DeviceLocalStatusUnavailable(presentation.Error(p.localErr))).FontSize(12).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
			switch {
			case p.loading:
				ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.cancelDeviceWait)
			case p.busy:
				ui.Button(c, locales.DeviceStopWaiting()).OnClick(a.cancelDeviceWait)
			case p.review != nil:
				ui.Button(c, locales.Cancel()).OnClick(func() { p.review = nil })
				ui.PrimaryButton(c, locales.DeviceConfirm()).OnClick(a.submitDevice)
			case p.receipt != nil || p.err != nil:
				ui.Button(c, locales.DeviceRefresh()).OnClick(a.loadDevice)
			default:
				ui.Button(c, locales.DeviceRefresh()).OnClick(a.loadDevice)
			}
		})
	})
}

func (a *App) deviceLocalDetailsView(c *ui.Context, status *remote.DeviceLocalStatus) {
	ui.Column(c).FillWidth().Padding(14).Gap(6).Background(c.Theme().Surface).Radius(8).Children(func() {
		ui.Text(c, locales.DeviceLocalTitle()).Bold()
		ui.Text(c, locales.DeviceLocalInstallation(status.App.Installation)).FontSize(12)
		ui.Text(c, locales.DeviceLocalPath(status.App.Path)).FontSize(12)
		if status.App.Version != nil {
			ui.Text(c, locales.DeviceLocalVersion(*status.App.Version)).FontSize(12)
		}
		ui.Text(c, locales.DeviceLocalChecks(status.App.BundleIdentifier, status.App.CodeSignature, status.App.SigningIdentity, status.App.Gatekeeper)).FontSize(12)
		ui.Text(c, locales.DeviceLocalXPC(status.App.NetworkBrokerXPC, status.App.GUIExecutorXPC)).FontSize(12)
		ui.Text(c, locales.DeviceLocalRunning(boolText(status.App.Running))).FontSize(12)
		if a.device.plan != nil {
			ui.Text(c, locales.DeviceLocalPlanTitle(a.device.plan.Action)).Bold().FontSize(12)
			ui.Text(c, locales.DeviceLocalPlanHelp(a.device.plan.TerminalMethod)).FontSize(11).TextColor(c.Theme().TextMuted)
			for _, stage := range a.device.plan.Stages {
				ui.Text(c, locales.DeviceLocalPlanStage(stage.ID, stage.State)).FontSize(11)
			}
		} else if a.device.planErr != nil {
			ui.Text(c, locales.DeviceLocalPlanUnavailable(presentation.Error(a.device.planErr))).FontSize(11).TextColor(c.Theme().TextMuted)
		}
		// `device diagnose` is a schema command with a reviewed native-terminal
		// workflow. Keep its command card on this structured page, but retain an
		// explicit action that opens the exact discovered argv after the local
		// status/plan has been observed.
		if a.device.plan != nil && a.device.plan.Action == "diagnose" {
			ui.Button(c, locales.CliFormReview()+" "+locales.DeviceAction("diagnose")).OnClick(func() {
				a.openDeviceLocalCommand("diagnose")
			})
		}
	})
}

// openDeviceLocalCommand resolves the exact command from the current schema
// catalog before entering the generic form. The form still applies the
// dedicated workflow's platform, capability, confirmation and argv checks;
// this helper only preserves the Device page context for the diagnosis entry.
func (a *App) openDeviceLocalCommand(action string) {
	if action == "" || a.capabilities.snapshot.ProtocolError != nil {
		return
	}
	for _, catalog := range a.capabilities.snapshot.Catalogs {
		if catalog.Executable != "agent-remote" || catalog.Err != nil {
			continue
		}
		for _, command := range catalog.Schema.Commands {
			if command.HasSubcommands || len(command.Path) != 2 || command.Path[0] != "device" || command.Path[1] != action {
				continue
			}
			a.openGenericCLI(catalog.Executable, command)
			return
		}
	}
}

func (a *App) deviceDetailsView(c *ui.Context, snapshot *remote.DeviceOverview) {
	ui.Column(c).FillWidth().Padding(14).Gap(6).Background(c.Theme().Surface).Radius(8).Children(func() {
		ui.Text(c, snapshot.Device.Name).Bold()
		ui.Text(c, locales.DeviceID(snapshot.Device.ID)).FontSize(12)
		ui.Text(c, locales.DevicePlatform(snapshot.Device.Platform)).FontSize(12)
		ui.Text(c, locales.DeviceStatus(snapshot.Device.Status)).FontSize(12)
		if snapshot.Context.ServerURL != nil {
			ui.Text(c, locales.DeviceServer(*snapshot.Context.ServerURL)).FontSize(12)
		}
		ui.Text(c, locales.DeviceContext(snapshot.Context.Home)).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Wrap().Children(func() {
			ui.Button(c, locales.DeviceRotate()).Disabled(!remote.DeviceSupported(snapshot.Capabilities, "device.rotate_token") || a.deviceBlocked()).OnClick(func() { a.reviewDevice("rotate_token") })
			ui.Button(c, locales.DeviceRevoke()).Disabled(!remote.DeviceSupported(snapshot.Capabilities, "device.revoke") || a.deviceBlocked()).OnClick(func() { a.reviewDevice("revoke") })
		})
	})
}

func (a *App) deviceReviewView(c *ui.Context, input *remote.DeviceActionInput) {
	ui.Column(c).FillWidth().Padding(14).Gap(6).Background(c.Theme().Surface).Radius(8).Children(func() {
		ui.Text(c, locales.DeviceReview()).Bold()
		ui.Text(c, locales.DeviceAction(a.device.action)).Bold()
		ui.Text(c, locales.DeviceID(input.DeviceID)).FontSize(12)
		ui.Text(c, locales.DeviceServer(defaultDeviceServer(input.Expected.ServerURL))).FontSize(12)
		ui.Text(c, locales.DeviceContext(input.Expected.Home)).FontSize(12)
		ui.Text(c, locales.DeviceConfirmHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}

func (a *App) deviceReceiptView(c *ui.Context, receipt *remote.DeviceReceipt) {
	ui.Text(c, locales.DeviceReceipt()).Bold()
	ui.Text(c, locales.DeviceAction(a.device.action)).FontSize(12)
	ui.Text(c, locales.DeviceID(receipt.DeviceID)).FontSize(12)
	ui.Text(c, locales.DeviceRemoteStatus(receipt.RemoteStatus)).FontSize(12)
	if receipt.LocalCleanupComplete != nil {
		ui.Text(c, locales.DeviceLocalCleanup(boolText(*receipt.LocalCleanupComplete))).FontSize(12)
	}
	if receipt.LocalCredentialComplete != nil {
		ui.Text(c, locales.DeviceLocalCredential(boolText(*receipt.LocalCredentialComplete))).FontSize(12)
	}
}

func defaultDeviceServer(value *string) string {
	if value == nil {
		return locales.DeviceNotConfigured()
	}
	return *value
}
