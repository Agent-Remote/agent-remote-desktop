package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type initStage string

const (
	initForm            initStage = "form"
	initRunning         initStage = "running"
	initApprovalStage   initStage = "approval"
	initWireGuardReview initStage = "wireguard_review"
	initFinalizing      initStage = "finalizing"
	initComplete        initStage = "complete"
	initFailed          initStage = "failed"
)

// initWizardPage is intentionally independent from the individual login,
// setup and WireGuard pages. A wizard owns one ordered operation, so a late
// result cannot be mistaken for a new submission on another page.
type initWizardPage struct {
	server, username, password, totp string
	device, sshKey, wireguardKey     string
	browser, skipDevice, configureWG bool
	output                           string
	review                           bool
	stage                            initStage
	busy                             bool
	approval                         *remote.DeviceCodeApproval
	approvalDeadline                 time.Time
	browserOpenFailed                bool
	plan                             *remote.WireGuardConfigPlan
	setup                            *remote.LocalSetupReceipt
	login                            *remote.LoginReceipt
	wireguard                        *remote.WireGuardConfigReceipt
	diagnostics                      *remote.DiagnosticReport
	readiness                        *remote.InitReadiness
	err                              error
	cancel                           context.CancelFunc
	results                          chan initResult
	approvals                        chan initApprovalEvent
}

type initApprovalEvent struct {
	event    remote.DeviceCodeApproval
	deadline time.Time
}

type initResult struct {
	stage       initStage
	setup       *remote.LocalSetupReceipt
	login       *remote.LoginReceipt
	plan        *remote.WireGuardConfigPlan
	wireguard   *remote.WireGuardConfigReceipt
	diagnostics *remote.DiagnosticReport
	readiness   *remote.InitReadiness
	err         error
}

func boolText(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func intText(value int) string { return strconv.Itoa(value) }

func (a *App) openInitWizard() {
	if a.initWizard.busy {
		a.page = "init"
		return
	}
	config := a.remote.snapshot.Config
	browser := false
	if a.remote.loaded {
		browser = !a.remote.snapshot.Capabilities.SupportsEffect("login.password", "write")
	}
	a.initWizard = initWizardPage{stage: initForm, browser: browser, configureWG: false}
	if config.ServerURL != nil {
		a.initWizard.server = *config.ServerURL
	}
	a.page = "init"
}

func (a *App) initWizardMethod() string {
	if a.initWizard.browser {
		return "login.device_code"
	}
	return "login.password"
}

func (a *App) initWizardCanSubmit() bool {
	w := &a.initWizard
	if w.busy || w.stage != initForm || !w.review || strings.TrimSpace(w.server) == "" {
		return false
	}
	if a.remote.loaded && (!a.remoteWorkflowAvailable("setup.local") || !a.remoteWorkflowAvailable(a.initWizardMethod()) || !a.remoteWorkflowAvailable("diagnostics.inspect")) {
		return false
	}
	if !w.browser && (strings.TrimSpace(w.username) == "" || w.password == "") {
		return false
	}
	return true
}

func (a *App) submitInitWizard() {
	if !a.initWizardCanSubmit() || a.closing || a.quitting.Load() || a.synchronizationBusy() || a.setup.busy || a.login.busy || a.wireguard.busy {
		return
	}
	w := &a.initWizard
	w.password, w.totp = strings.TrimSpace(w.password), strings.TrimSpace(w.totp)
	w.stage, w.busy, w.err = initRunning, true, nil
	w.approval, w.plan, w.setup, w.login, w.wireguard, w.diagnostics, w.readiness = nil, nil, nil, nil, nil, nil, nil
	w.results, w.approvals = make(chan initResult, 1), make(chan initApprovalEvent, 1)
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	input := remote.PasswordLoginInput{
		ServerURL: strings.TrimSpace(w.server), Username: strings.TrimSpace(w.username), Password: w.password,
		TOTP: optionalLoginText(w.totp), DeviceName: optionalLoginText(w.device), SSHKey: optionalLoginText(w.sshKey),
		WireGuardKey: optionalLoginText(w.wireguardKey), SkipDevice: w.skipDevice, Confirmed: true,
	}
	deviceInput := remote.DeviceCodeLoginInput{ServerURL: input.ServerURL, DeviceName: input.DeviceName, SSHKey: input.SSHKey,
		WireGuardKey: input.WireGuardKey, SkipDevice: input.SkipDevice, Confirmed: true}
	results, approvals, home, find, browser, configureWG, output := w.results, w.approvals, a.home, a.findExecutable, w.browser, w.configureWG, strings.TrimSpace(w.output)
	// Clear the UI's copy before starting the worker. The worker has its own
	// transient copy and clears it before publishing a result.
	w.password, w.totp = "", ""
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		if err != nil {
			results <- initResult{stage: initFailed, err: err}
			return
		}
		client := cliadapter.Client{Binary: binary, Home: home}
		setup, err := remote.SetupLocal(ctx, client, remote.LocalSetupInput{Confirmed: true, ExpectedHome: home})
		if err != nil {
			results <- initResult{stage: initFailed, err: err}
			return
		}
		if !setup.Complete {
			results <- initResult{stage: initFailed, setup: &setup, err: fault.LocalSetupIncomplete}
			return
		}
		current, err := remote.InitContext(ctx, client)
		if err != nil {
			results <- initResult{stage: initFailed, setup: &setup, err: err}
			return
		}
		input.Expected = remote.LoginContext{ServerURL: current.ServerURL, DeviceID: current.DeviceID}
		deviceInput.Expected = input.Expected
		var receipt remote.LoginReceipt
		if browser {
			receipt, err = remote.DeviceCodeLogin(ctx, client, deviceInput, func(event remote.DeviceCodeApproval) {
				select {
				case approvals <- initApprovalEvent{event: event, deadline: time.Now().Add(time.Duration(event.ExpiresIn) * time.Second)}:
				case <-ctx.Done():
				}
			})
		} else {
			receipt, err = remote.PasswordLogin(ctx, client, input)
		}
		input.Password = ""
		input.TOTP = nil
		if err != nil {
			results <- initResult{stage: initFailed, setup: &setup, err: err}
			return
		}
		if configureWG {
			planInput := remote.WireGuardPlanInput{ExpectedHome: home}
			if output != "" {
				planInput.Output = &output
			}
			plan, planErr := remote.PlanWireGuardConfig(ctx, client, planInput)
			if planErr != nil {
				results <- initResult{stage: initFailed, setup: &setup, login: &receipt, err: planErr}
				return
			}
			results <- initResult{stage: initWireGuardReview, setup: &setup, login: &receipt, plan: &plan}
			return
		}
		a.finishInit(ctx, client, results, &setup, &receipt)
	}()
}

func (a *App) finishInit(ctx context.Context, client cliadapter.Client, results chan<- initResult, setup *remote.LocalSetupReceipt, login *remote.LoginReceipt) {
	diagnostics, err := remote.InspectDiagnostics(ctx, client, true, true, true)
	if err != nil {
		results <- initResult{stage: initFailed, setup: setup, login: login, err: err}
		return
	}
	readiness, err := remote.AssessInitReadiness(ctx, client)
	if err != nil {
		results <- initResult{stage: initFailed, setup: setup, login: login, diagnostics: &diagnostics, err: err}
		return
	}
	results <- initResult{stage: initComplete, setup: setup, login: login, diagnostics: &diagnostics, readiness: &readiness}
}

func (a *App) submitInitWireGuard() {
	w := &a.initWizard
	if w.busy || w.stage != initWireGuardReview || w.plan == nil || a.closing || a.quitting.Load() {
		return
	}
	plan := *w.plan
	w.busy, w.stage, w.err = true, initFinalizing, nil
	w.results = make(chan initResult, 1)
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	results, home, find := w.results, a.home, a.findExecutable
	go func() {
		defer cancel()
		binary, err := find("agent-remote")
		client := cliadapter.Client{Binary: binary, Home: home}
		var receipt remote.WireGuardConfigReceipt
		if err == nil {
			receipt, err = remote.ApplyWireGuardConfig(ctx, client, remote.WireGuardApplyInput{Confirmed: true, ExpectedPlan: plan})
		}
		if err != nil {
			results <- initResult{stage: initFailed, setup: w.setup, login: w.login, plan: &plan, err: err}
			return
		}
		diagnostics, err := remote.InspectDiagnostics(ctx, client, true, true, true)
		if err != nil {
			results <- initResult{stage: initFailed, setup: w.setup, login: w.login, plan: &plan, wireguard: &receipt, err: err}
			return
		}
		readiness, err := remote.AssessInitReadiness(ctx, client)
		if err != nil {
			results <- initResult{stage: initFailed, setup: w.setup, login: w.login, plan: &plan, wireguard: &receipt, diagnostics: &diagnostics, err: err}
			return
		}
		results <- initResult{stage: initComplete, setup: w.setup, login: w.login, plan: &plan, wireguard: &receipt, diagnostics: &diagnostics, readiness: &readiness}
	}()
}

func (a *App) pollInitWizard(c *ui.Context) {
	w := &a.initWizard
	if !w.busy {
		return
	}
	select {
	case approval := <-w.approvals:
		w.approval, w.approvalDeadline = &approval.event, approval.deadline
		w.stage = initApprovalStage
	default:
	}
	select {
	case result := <-w.results:
		w.busy, w.stage, w.err = false, result.stage, result.err
		w.setup, w.login, w.plan, w.wireguard, w.diagnostics, w.readiness = result.setup, result.login, result.plan, result.wireguard, result.diagnostics, result.readiness
		w.approval = nil
		if result.stage == initFailed {
			a.invalidateRemoteContext()
		} else if result.stage == initComplete {
			a.invalidateRemoteContext()
			a.invalidateDiagnostics()
		}
	default:
		c.After(50 * time.Millisecond)
	}
}

func (a *App) cancelInitWizard() {
	if a.initWizard.busy && a.initWizard.cancel != nil {
		a.initWizard.cancel()
		a.initWizard.stage, a.initWizard.busy = initFailed, false
		a.initWizard.err = context.Canceled
		return
	}
	a.initWizard = initWizardPage{}
	a.openDiagnostics()
}

func (a *App) initWizardView(c *ui.Context) {
	w := &a.initWizard
	ui.Column(c).Fill().Padding(24).Gap(14).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Text(c, locales.InitWizardTitle()).FontSize(24).Bold().Grow(1)
			ui.Button(c, locales.ReturnWorkbench()).OnClick(a.cancelInitWizard)
		})
		ui.Text(c, locales.InitWizardHelp()).TextColor(c.Theme().TextMuted)
		if w.busy || w.stage == initApprovalStage || w.stage == initFinalizing {
			a.initWizardProgressView(c)
			return
		}
		if w.stage == initWireGuardReview {
			a.initWizardWireGuardView(c)
			return
		}
		if w.stage == initComplete {
			a.initWizardCompleteView(c)
			return
		}
		if w.stage == initFailed && w.err != nil {
			ui.Text(c, presentation.Error(w.err)).TextColor(c.Theme().Danger)
			ui.Button(c, locales.InitWizardRetry()).OnClick(func() { w.stage, w.err, w.review = initForm, nil, false })
			return
		}
		a.initWizardFormView(c)
	})
}

func (a *App) initWizardProgressView(c *ui.Context) {
	w := &a.initWizard
	if w.stage == initApprovalStage && w.approval != nil {
		remaining := int(time.Until(w.approvalDeadline).Seconds())
		ui.Text(c, locales.InitWizardApproval()).FontSize(20).Bold()
		ui.Text(c, w.approval.VerificationURL)
		ui.Text(c, w.approval.UserCode).FontSize(26).Bold()
		if remaining > 0 {
			ui.Text(c, locales.LoginCodeExpires(remaining)).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, locales.LoginCodeElapsed()).TextColor(c.Theme().TextMuted)
		}
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Button(c, locales.LoginCopyCode()).Disabled(remaining <= 0).OnClick(func() { c.WriteClipboard(w.approval.UserCode) })
			ui.PrimaryButton(c, locales.LoginOpenBrowser()).Disabled(remaining <= 0).OnClick(func() {
				c.OpenURLThen(w.approval.VerificationURL, func(err error) { w.browserOpenFailed = err != nil })
			})
		})
		if w.browserOpenFailed {
			ui.Text(c, locales.LoginBrowserOpenFailed()).TextColor(c.Theme().Danger)
		}
	} else {
		ui.Text(c, locales.InitWizardProgress()).FontSize(20).Bold()
		ui.Text(c, locales.InitWizardStage(string(w.stage))).TextColor(c.Theme().TextMuted)
	}
	ui.Text(c, locales.InitWizardCancelHelp()).TextColor(c.Theme().TextMuted)
	ui.Button(c, locales.StopLoginWaiting()).OnClick(w.cancel)
}

func (a *App) initWizardFormView(c *ui.Context) {
	w := &a.initWizard
	ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
		ui.TextInput(c, &w.server).Label(locales.LoginServer()).Placeholder(locales.LoginServer()).AutoFocus()
		ui.Row(c).Gap(10).Wrap().Children(func() {
			if a.remoteWorkflowAvailable("login.password") {
				ui.Button(c, locales.LoginWithPassword()).Disabled(w.browser).OnClick(func() { w.browser = false })
			}
			if a.remoteWorkflowAvailable("login.device_code") {
				ui.Button(c, locales.LoginWithBrowser()).Disabled(!w.browser).OnClick(func() { w.browser = true; w.password, w.totp = "", "" })
			}
		})
		if !w.browser {
			ui.TextInput(c, &w.username).Label(locales.LoginUsername()).Placeholder(locales.LoginUsername())
			ui.TextInput(c, &w.password).Label(locales.LoginPassword()).Placeholder(locales.LoginPassword()).Password()
			ui.TextInput(c, &w.totp).Label(locales.LoginTOTP()).Placeholder(locales.LoginTOTP()).Password()
		} else {
			ui.Text(c, locales.LoginBrowserHelp()).TextColor(c.Theme().TextMuted)
		}
		ui.Checkbox(c, &w.skipDevice, locales.LoginUserOnly())
		if !w.skipDevice {
			ui.TextInput(c, &w.device).Label(locales.LoginDeviceName()).Placeholder(locales.LoginDeviceName())
			ui.TextInput(c, &w.sshKey).Label(locales.LoginSSHKey()).Placeholder(locales.LoginSSHKey())
			ui.TextInput(c, &w.wireguardKey).Label(locales.LoginWireGuardKey()).Placeholder(locales.LoginWireGuardKey())
		}
		ui.Checkbox(c, &w.configureWG, locales.InitWizardConfigureWireGuard())
		if w.configureWG {
			ui.TextInput(c, &w.output).Label(locales.InitWizardWireGuardOutput()).Placeholder(locales.InitWizardDefaultOutput())
		}
		if w.review {
			ui.Column(c).FillWidth().Padding(12).Gap(6).Background(c.Theme().Surface).Radius(8).Children(func() {
				ui.Text(c, locales.InitWizardReview()).Bold()
				ui.Text(c, w.server)
				ui.Text(c, locales.InitWizardScope()).TextColor(c.Theme().TextMuted)
				ui.Text(c, locales.InitWizardDeviceScope(boolText(w.skipDevice))).TextColor(c.Theme().TextMuted)
				if w.configureWG {
					ui.Text(c, locales.InitWizardWireGuardScope(w.output)).TextColor(c.Theme().TextMuted)
				}
			})
		}
	})
	ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
		ui.Button(c, locales.Cancel()).OnClick(a.cancelInitWizard)
		if w.review {
			ui.Button(c, locales.EditLogin()).OnClick(func() { w.review = false })
			ui.PrimaryButton(c, locales.InitWizardConfirm()).Disabled(!a.initWizardCanSubmit()).OnClick(a.submitInitWizard)
		} else {
			ui.PrimaryButton(c, locales.ReviewLogin()).Disabled(strings.TrimSpace(w.server) == "" || (!w.browser && (strings.TrimSpace(w.username) == "" || w.password == ""))).OnClick(func() { w.review = true })
		}
	})
}

func (a *App) initWizardWireGuardView(c *ui.Context) {
	w := &a.initWizard
	ui.Text(c, locales.InitWizardWireGuardReview()).FontSize(20).Bold()
	ui.Text(c, locales.InitWizardWireGuardReviewHelp()).TextColor(c.Theme().TextMuted)
	if w.plan != nil {
		ui.Text(c, w.plan.Output.Path).Bold()
		ui.Text(c, locales.InitWizardWireGuardAction(w.plan.Output.Action)).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.InitWizardWireGuardDirectories(intText(len(w.plan.Output.MissingDirectories)))).TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
		ui.Button(c, locales.Cancel()).OnClick(a.cancelInitWizard)
		ui.PrimaryButton(c, locales.InitWizardConfirmWireGuard()).OnClick(a.submitInitWireGuard)
	})
}

func (a *App) initWizardCompleteView(c *ui.Context) {
	w := &a.initWizard
	ui.Text(c, locales.InitWizardComplete()).FontSize(20).Bold()
	if w.setup != nil {
		ui.Text(c, locales.InitWizardLocalReady(boolText(w.setup.Complete)))
	}
	if w.login != nil {
		ui.Text(c, locales.InitWizardLoginReady(w.login.ServerURL))
	}
	if w.wireguard != nil {
		ui.Text(c, locales.InitWizardWireGuardReady(boolText(w.wireguard.Complete), w.wireguard.OutputPath))
	} else {
		ui.Text(c, locales.InitWizardWireGuardSkipped()).TextColor(c.Theme().TextMuted)
	}
	if w.diagnostics != nil {
		ui.Text(c, locales.InitWizardDiagnosticsReady(intText(len(w.diagnostics.Checks)), intText(len(w.diagnostics.Dependencies))))
	}
	if w.readiness != nil {
		ui.Text(c, locales.InitWizardReadiness(boolText(w.readiness.Ready), intText(len(w.readiness.Overview.Accounts)), intText(len(w.readiness.Overview.Sessions)))).TextColor(c.Theme().TextMuted)
	}
	ui.PrimaryButton(c, locales.ReturnWorkbench()).OnClick(func() { a.initWizard = initWizardPage{}; a.page = "" })
}
