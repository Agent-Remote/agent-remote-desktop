package app

import (
	"errors"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// page is a management page of the Control Center (redesign §4.5, §7.2):
// the sheet pushes it over the grid with its title and context strip. The
// a.page string stays its key, so routes, the operation journal and
// pendingPage hand-offs are unchanged.
type page struct {
	title func() string
	view  func(*ui.Context)
	// leave replaces "Return to workbench" (close the sheet) for pages that
	// reset a draft first, as their old header button did, or go elsewhere.
	// Back and closing the sheet keep the draft, as switching sidebar items
	// did.
	leave func()
}

func (a *App) pages() map[string]page {
	plain := func(title func() string, view func(*ui.Context)) page { return page{title: title, view: view} }
	return map[string]page{
		"operations":          plain(locales.OperationsTitle, a.operationsView),
		"runtime-recovery":    plain(locales.RuntimeRecoveryTitle, a.runtimeRecoveryView),
		"mutagen-environment": plain(locales.MutagenEnvironmentTitle, a.mutagenEnvironmentView),
		"sync":                plain(locales.SyncTitle, a.syncView),
		"account-import": plain(func() string {
			if a.accountImport.history {
				return locales.AccountImportRecoveryTitle()
			}
			return locales.AccountImportTitle()
		}, a.accountImportView),
		"account":             plain(func() string { return locales.AccountActionTitle(a.account.action) }, a.accountView),
		"account-default":     plain(locales.AccountDefaultTitle, a.accountDefaultView),
		"network":             plain(locales.WireGuardNetworkTitle, a.networkView),
		"wireguard":           plain(locales.WireGuardConfigTitle, a.wireguardView),
		"setup":               plain(locales.LocalSetupTitle, a.localSetupView),
		"init":                {title: locales.InitWizardTitle, view: a.initWizardView, leave: a.cancelInitWizard},
		"capabilities":        plain(locales.CapabilitiesTitle, a.capabilitiesView),
		"device":              plain(locales.DeviceTitle, a.deviceView),
		"ego-browser":         plain(locales.EgoBrowserTitle, a.egoBrowserView),
		"nodes":               {title: locales.NodesTitle, view: a.nodesView, leave: func() { a.nodes = nodePage{}; a.closeLayer() }},
		"credentials":         plain(locales.CredentialsTitle, a.credentialsView),
		"skills":              plain(locales.SkillsTitle, a.skillsView),
		"generic-cli":         {title: locales.CliFormTitle, view: a.genericCLIView, leave: func() { a.genericCLI = genericCLIPage{}; a.closeLayer() }},
		"login":               plain(locales.SignInRemote, a.loginView),
		"logout":              plain(locales.SignOutRemote, a.logoutView),
		"remote":              plain(locales.RemoteManagement, a.remoteView),
		"diagnostics":         plain(locales.DiagnosticsTitle, a.diagnosticsView),
		"command-diagnostics": plain(a.commandDiagnosticsTitle, a.commandDiagnosticsView),
		"dependencies":        plain(locales.DependenciesTitle, a.dependenciesView),
		"settings":            plain(locales.Settings, a.settingsView),
		"project":             plain(locales.ProjectDetails, a.projectView),
	}
}

func (a *App) commandDiagnosticsTitle() string {
	switch method := a.commandDiagnostics.method; method {
	case "doctor.inspect", "doctor.repair":
		return locales.CliDoctorHelp()
	case "deps.inspect", "deps.repair":
		return locales.DependenciesTitle()
	case "status.inspect":
		return locales.CommandDiagnosticsStatus()
	case "ssh.inspect":
		return locales.CommandDiagnosticsSSH()
	default:
		return locales.CommandDiagnosticsMethod(method)
	}
}

// syncPageHost keeps a.page and the sheet in step. Whatever opens a page —
// a tile, the palette, the menu, a link on another page, a pendingPage
// hand-off — shows it in the Control Center; code that leaves a page (a
// launch, a project switch, a session focus) closes the sheet with it.
func (a *App) syncPageHost() {
	l := &a.layers
	open := a.openLayer() == layerControl
	switch {
	case open && a.page != "":
		l.hosted = a.page
	case open && l.hosted != "":
		// The page was left from code, not by Back: return to the workspace.
		l.hosted = ""
		a.closeLayer()
	case a.page != "" && !a.dialogOpen() && !a.closing:
		if _, ok := a.pages()[a.page]; ok {
			a.showLayer(layerControl)
			l.hosted = a.page
		}
	case !open && !l.controlClosing:
		l.hosted = ""
	}
}

// popPage is Back: the sheet stays and returns to its grid.
func (a *App) popPage() {
	a.page, a.pendingPage, a.layers.hosted = "", "", ""
}

// leavePage is "Return to workbench": the workspace, or the page's own
// leave (the wizard cancels a running step, else returns to Diagnostics).
func (a *App) leavePage(id string) {
	p := a.pages()[id]
	if p.leave == nil {
		a.closeLayer()
		return
	}
	p.leave()
}

// escapeLayer is Esc: a page goes back to the grid, a search clears, and
// only then does the layer close (redesign §4.5).
func (a *App) escapeLayer() {
	l := &a.layers
	if a.openLayer() == layerControl {
		switch {
		case a.page != "":
			a.popPage()
			return
		case l.ccQuery != "":
			l.ccQuery = ""
			return
		}
	}
	a.closeLayer()
}

// pageContext is the context strip of every page (redesign §4.6): the CLI
// home every page drives. Server and device are shown only where they are
// the page's own settled read (Remote management): pages that query or
// freeze a context print that one, never a cached copy. The strip is one
// line tall whatever it holds, so the page under it does not move when a
// read completes; long values ellipsize.
func (a *App) pageContext(c *ui.Context, env chrome.Env, id string) {
	ui.Row(c).FillWidth().Height(24 * c.Theme().FontSize / 14).Gap(6).Clip().Children(func() {
		env.ContextPill(c, locales.ContextHome(), a.home).Shrink(1).MinWidth(0)
		if id == "remote" && a.remote.loaded && !a.remote.busy && a.remote.err == nil {
			config := a.remote.snapshot.Config
			if config.ServerURL != nil {
				env.ContextPill(c, locales.ContextServer(), *config.ServerURL).Shrink(1).MinWidth(0)
			}
			if config.DeviceID != nil {
				env.ContextPill(c, locales.ContextDevice(), *config.DeviceID).Shrink(1).MinWidth(0)
			}
		}
		if id == "sync" || id == "project" {
			if p := a.project(); p != nil {
				env.ContextPill(c, locales.ContextProject(), p.Name).Shrink(1).MinWidth(0)
			}
		}
	})
}

// pageHost is a pushed page: Back, the title and Return to workbench, the
// context strip, then the page's own content.
func (a *App) pageHost(c *ui.Context, env chrome.Env, id string, p page) {
	ui.Column(c).Fill().Children(func() {
		// One row and the strip: the page keeps the height its full-window
		// layout had, down to the 900×600 minimum.
		ui.Column(c).FillWidth().Padding(0, 28, 8).Gap(8).Children(func() {
			ui.Row(c).FillWidth().Gap(12).Children(func() {
				back := ui.ButtonBase(c).Gap(4).Padding(4, 10, 4, 6).Radius(999).Label(locales.BackToControlCenter()).Cursor(ui.CursorPointer)
				if back.Clicked() {
					a.popPage()
				}
				chrome.Pressable(back, env).Children(func() {
					env.Text(c, "‹", theme.BodyStrong, env.P.AccentInk).FontSize(18 * c.Theme().FontSize / 14)
					env.Text(c, locales.ControlCenter(), theme.BodyStrong, env.P.AccentInk).SingleLine()
				})
				env.Text(c, p.title(), theme.PageTitle, env.P.TextPrimary).SingleLine().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
				if env.Button(c, locales.ReturnWorkbench(), chrome.Secondary).Clicked() {
					a.leavePage(id)
				}
			})
			a.pageContext(c, env, id)
		})
		ui.Column(c.Key("page-" + id)).Grow(1).MinHeight(0).FillWidth().Children(func() { p.view(c) })
	})
}

// card styles a record of a page as a well of the sheet.
func (a *App) card(c *ui.Context, el ui.Element) ui.Element {
	return a.chromeEnv(c).Well(el)
}

// receiptStages draws the stages a CLI receipt or plan reports with the
// operation page's marks (redesign §4.6). label(i) is the stage's localized
// "name: state" line; state is the CLI's own word for it.
func (a *App) receiptStages(c *ui.Context, n int, stage func(i int) (label, state string)) {
	stages := make([]chrome.Stage, n)
	for i := range stages {
		label, state := stage(i)
		stages[i] = chrome.Stage{Name: label, State: stageState(state)}
	}
	a.chromeEnv(c).Stages(c, stages)
}

func stageState(state string) chrome.StageState {
	switch state {
	case "completed", "succeeded", "done":
		return chrome.StageDone
	case "failed":
		return chrome.StageFailed
	case "running", "in_progress", "active":
		return chrome.StageRunning
	case "planned", "pending", "skipped", "waiting":
		return chrome.StageWaiting
	}
	return chrome.StageUnknown
}

// errorText shows a page's error: an unknown outcome as the recovery
// banner (redesign §4.6: query the original, never replay), anything else
// as danger text.
func (a *App) errorText(c *ui.Context, err error, message string) {
	if outcomeUnknown(err) {
		a.chromeEnv(c).Banner(c, message, nil)
		return
	}
	ui.Text(c, message).TextColor(c.Theme().Danger)
}

// outcomeUnknown is true for errors saying a request may or may not have
// taken effect: the desktop's fault codes and the CLI's error codes alike.
func outcomeUnknown(err error) bool {
	var code fault.Code
	if errors.As(err, &code) {
		return strings.HasSuffix(string(code), "outcome_unknown")
	}
	var operation *cliadapter.OperationError
	return errors.As(err, &operation) && strings.HasSuffix(operation.Code, "outcome_unknown")
}
