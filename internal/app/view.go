package app

import (
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// View renders the workbench into a native UI context.
func (a *App) View(c *ui.Context) {
	frameStarted := time.Now()
	defer func() { a.motionFrames.Observe(time.Since(frameStarted)) }()
	a.pollLaunch()
	a.pollExit()
	a.pollSync(c)
	a.pollMutagenEnvironment(c)
	a.pollAccount(c)
	a.pollAccountImport(c)
	a.pollRuntimeRecovery(c)
	a.pollAccountDefault(c)
	a.pollRemote(c)
	a.pollRemoteCreate(c)
	a.pollLogin(c)
	a.pollLogout(c)
	a.pollLocalSetup(c)
	a.pollInitWizard(c)
	a.pollWireGuard(c)
	a.pollNetwork(c)
	a.pollDiagnostics(c)
	a.pollDependencies(c)
	a.pollCommandDiagnostics(c)
	a.pollCapabilities(c)
	a.pollDevice(c)
	a.pollEgoBrowser(c)
	a.pollNodes(c)
	a.pollCredentials(c)
	a.pollSkills(c)
	// Persist safe lifecycle projections after workers publish their latest
	// result. This never performs a CLI query or retries a mutation.
	a.syncOperationJournal()
	// Keep one final frame scheduled while a background save is outstanding.
	// A failed save then appears without continuously polling an idle workbench.
	if a.savePending.Load() > 0 {
		c.After(50 * time.Millisecond)
	}
	a.resolveTheme(c.Theme().Dark)
	t := theme.UI(a.shownTheme)
	c.SetTheme(&t)
	ui.Box(c).Fill().Background(t.Background).Children(func() {
		a.backgroundDecoration(c, t)
		ui.Column(c).Fill().Children(func() {
			a.toolbar(c)
			ui.Row(c).Grow(1).MinHeight(0).Children(func() {
				if !a.state.Preferences.FocusMode {
					if ui.Split(c, &a.state.Preferences.SidebarWidth, func() { a.sidebar(c) }, func() { a.content(c) }).Fill().Changed() {
						a.state.Preferences.SidebarWidth = min(400, max(180, a.state.Preferences.SidebarWidth))
						a.layoutChanged()
					}
				} else {
					a.content(c)
				}
			})
			a.statusbar(c)
		})
	})
	if focused := ui.FocusedValue(c, &a.focusValue); focused != "" && focused == a.focusValue {
		a.focusPane(focused, false)
	}
	a.shortcuts(c)
	a.dialogs(c)
	a.recoveryDialog(c)
	a.remoteAttachDialog(c)
	a.remoteCreateDialog(c)
	a.remoteDeleteDialog(c)
	a.palette(c)
	a.saveLayoutWhenIdle(c)
}

// backgroundDecoration adds a small, pass-through accent layer behind the
// workbench chrome. It is intentionally a one-shot Timeline: after the
// 180 ms entrance it settles and no longer causes frames while a terminal is
// streaming output. Opaque terminal panes retain their own readable palette.
func (a *App) backgroundDecoration(c *ui.Context, t ui.Theme) {
	policy := theme.BackgroundDecor(theme.Reduced(c, a.state.Preferences.ReduceMotion), a.state.Preferences.FocusMode)
	if !policy.Enabled {
		return
	}
	anchor := ui.Box(c).Key("workbench-background-motion").Absolute().Fill().PassThrough()
	progress := float32(1)
	if policy.Animated {
		timeline := anchor.Timeline("intro", ui.TimelineOptions{})
		progress = timeline.Tween(ui.Tween{From: 0, To: 1, Duration: theme.BackgroundMotionDuration})
	} else {
		// Supplying a settled progress value makes the user preference take
		// effect even when the native accessibility preference is unavailable.
		anchor.Timeline("intro", ui.TimelineOptions{Progress: &progress})
	}

	// The circles sit in the spare workbench canvas. Very low alpha keeps
	// localized text and terminal glyphs readable in both theme families.
	accent := t.Accent.Alpha(policy.Opacity)
	secondary := t.AccentHover.Alpha(policy.Opacity * 0.72)
	ui.Box(c).Key("workbench-background-orb-a").Absolute().Left(-90+progress*18).Top(-120+progress*12).Size(300, 300).Radius(150).Background(accent).PassThrough()
	ui.Box(c).Key("workbench-background-orb-b").Absolute().Right(-140+progress*24).Bottom(-160+progress*16).Size(390, 390).Radius(195).Background(secondary).PassThrough()
	ui.Box(c).Key("workbench-background-orb-c").Absolute().Right(170-progress*10).Top(120+progress*8).Size(120, 120).Radius(60).Background(t.Accent.Alpha(policy.Opacity * 0.52)).PassThrough()
}

func (a *App) toolbar(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	ui.Row(c).Height(66).Padding(16, 24).Gap(12).Children(func() {
		ui.Text(c, locales.BrandMark()).FontSize(17).Bold().TextColor(a.tokens().AccentInk)
		ui.Text(c, locales.BrandName()).FontSize(16).Bold()
		if width >= 1100 {
			ui.Text(c, locales.ToolbarWorkbench()).TextColor(t.TextMuted)
		}
		ui.Button(c, locales.SearchWorkbench()).Tooltip(locales.CommandPalette()).OnClick(a.openPalette)
		ui.Spacer(c)
		ui.Button(c, locales.Focus()).Tooltip(locales.HideDecorations()).OnClick(func() { a.state.Preferences.FocusMode = !a.state.Preferences.FocusMode; a.persist() })
		ui.Button(c, locales.Theme()).OnClick(func() {
			// Cycle from what is on screen; an explicit choice stops following
			// the system appearance.
			a.setTheme(theme.Next(a.shownTheme), false)
		})
		label := locales.NewSessionAction()
		if a.starting {
			label = locales.Starting()
		}
		ui.PrimaryButton(c, label).Disabled(a.starting).OnClick(func() { a.newSession("", false) })
	})
}

func (a *App) sidebar(c *ui.Context) {
	t := c.Theme()
	width, _ := c.Size()
	navHeight := float32(26)
	if width < 1100 {
		// Preserve the project/session rows in the minimum 900x600 window while
		// keeping every primary destination in the fixed sidebar region.
		navHeight = 22
	}
	records := map[string]session.Record{}
	for _, record := range a.sessions.Records() {
		records[record.ID] = record
	}
	ui.Column(c).Fill().Padding(10, 16).Gap(6).Background(t.Surface.Alpha(.35)).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, locales.Projects()).FontSize(12).TextColor(t.TextMuted).Grow(1)
			ui.Button(c, "＋").Label(locales.AddProject()).OnClick(func() { a.projectOpen = true })
		})
		// Keep project switching available at the minimum window size. The
		// detailed session list remains scrollable below, while this compact row
		// avoids making a project change depend on the sidebar scroll position.
		ui.Row(c).Gap(4).Wrap().Children(func() {
			for _, project := range a.state.Projects {
				button := ui.Button(c, project.Name).Height(24).Padding(6, 8).Radius(6)
				if project.ID == a.state.Preferences.SelectedProject {
					button = button.Background(a.tokens().Selected())
				}
				button.OnClick(func() { a.selectProject(project.ID) })
			}
		})
		// Keep primary navigation outside the project list so it remains
		// reachable at the minimum window size and with long localized labels.
		ui.Button(c, locales.SyncOpen()).Tooltip(locales.SyncTitle()).FillWidth().Height(navHeight).OnClick(a.openSync)
		ui.Button(c, locales.OperationsSidebar()).Tooltip(locales.OperationsTitle()).FillWidth().Height(navHeight).OnClick(a.openOperations)
		ui.Button(c, locales.RemoteManagement()).Tooltip(locales.RemoteManagement()).FillWidth().Height(navHeight).OnClick(a.openRemote)
		ui.Button(c, locales.DiagnosticsTitle()).Tooltip(locales.DiagnosticsTitle()).FillWidth().Height(navHeight).OnClick(a.openDiagnostics)
		ui.Button(c, locales.InitWizardTitle()).Tooltip(locales.InitWizardTitle()).FillWidth().Height(navHeight).OnClick(a.openInitWizard)
		ui.Button(c, locales.CapabilitiesTitle()).Tooltip(locales.CapabilitiesTitle()).FillWidth().Height(navHeight).OnClick(a.openCapabilities)
		ui.Button(c, locales.SkillsTitle()).Tooltip(locales.SkillsTitle()).FillWidth().Height(navHeight).OnClick(a.openSkills)
		ui.Button(c, locales.NodesTitle()).Tooltip(locales.NodesHelp()).FillWidth().Height(navHeight).OnClick(a.openNodes)
		ui.Button(c, locales.EgoBrowserTitle()).Tooltip(locales.EgoBrowserHelp()).FillWidth().Height(navHeight).OnClick(a.openEgoBrowser)
		ui.Button(c, locales.Settings()).FillWidth().Height(navHeight).OnClick(func() { a.page = "settings" })
		ui.Scroll(c).Grow(1).Gap(14).Children(func() {
			for _, project := range a.state.Projects {
				// Keep the project container content-sized.  A fixed height here
				// also constrained the session children (and made their labels
				// report a zero hit box in mygo's headless layout).
				ui.Column(c.Key(project.ID)).Gap(1).FillWidth().Children(func() {
					button := ui.Button(c, project.Name).FillWidth().Padding(10, 12).Radius(8)
					bg := ui.Transparent
					if project.ID == a.state.Preferences.SelectedProject {
						bg = a.tokens().Selected()
					} else if button.Hovered() {
						bg = t.SurfaceHover
					}
					ui.Row(c).Height(30).FillWidth().Children(func() {
						button.Background(button.AnimateColor("surface", bg, theme.Animation(theme.Reduced(c, a.state.Preferences.ReduceMotion)))).OnClick(func() { a.selectProject(project.ID) })
					})
					for _, rec := range a.state.Sessions {
						if rec.ProjectID == project.ID {
							status := locales.SavedSessionStatus()
							if live, ok := records[rec.ID]; ok {
								status = locales.Running()
								if live.Exited {
									status = locales.Exited()
								}
							}
							location := locales.Local()
							if session.Tool(rec.Tool).IsRemote() {
								location = locales.Remote()
							}
							// The button owns the row height; the wrapper must remain
							// content-sized so it can be hit by accessibility/test Find.
							ui.Column(c.Key(rec.ID)).Gap(2).FillWidth().Children(func() {
								if rec.Name != "" {
									ui.Text(c, rec.Name).TextColor(t.Text)
								}
								ui.Button(c, locales.SessionSummary(rec.Tool, location, status)).FontSize(12).TextColor(t.TextMuted).MinWidth(0).FillWidth().Height(30).OnClick(func() { a.focusSession(rec.ID) })
							})
						}
					}
				})
			}
		})
		ui.Text(c, locales.SidebarTagline()).TextColor(t.TextMuted).FontSize(12)
		if a.mutagenEnvironment.hasOperation() {
			ui.Button(c, locales.MutagenEnvironmentOperation()).FillWidth().OnClick(a.openMutagenEnvironment)
		}
		if a.sync.hasOperation() {
			ui.Button(c, locales.SyncOperationStatus()).FillWidth().OnClick(func() { a.page = "sync" })
		}
		if a.accountImport.submitted || len(a.accountImport.records) > 0 {
			ui.Button(c, locales.AccountImportRecoveryTitle()).FillWidth().OnClick(a.openAccountImportHistory)
		}
		if a.runtimeRecovery.submitted || len(a.runtimeRecovery.records) > 0 {
			ui.Button(c, locales.RuntimeRecoveryTitle()).FillWidth().OnClick(a.openRuntimeRecovery)
		}
		if a.account.submitted {
			ui.Button(c, locales.AccountOperationStatus()).Tooltip(locales.AccountOperationStatus()).FillWidth().OnClick(a.openAccountOperation)
		}
		if a.accountDefault.submitted {
			ui.Button(c, locales.AccountDefaultStatus()).Tooltip(locales.AccountDefaultStatus()).FillWidth().OnClick(a.openAccountDefault)
		}
		if a.login.hasOperation() {
			ui.Button(c, locales.LoginOperationStatus()).Tooltip(locales.LoginOperationStatus()).FillWidth().OnClick(a.openLogin)
		}
		if a.logout.hasOperation() {
			ui.Button(c, locales.LogoutOperationStatus()).Tooltip(locales.LogoutOperationStatus()).FillWidth().OnClick(a.openLogout)
		}
		if a.setup.hasOperation() {
			ui.Button(c, locales.LocalSetupOperationStatus()).Tooltip(locales.LocalSetupOperationStatus()).FillWidth().OnClick(a.openLocalSetup)
		}
		if a.wireguard.hasOperation() {
			ui.Button(c, locales.WireGuardOperationStatus()).Tooltip(locales.WireGuardOperationStatus()).FillWidth().OnClick(a.openWireGuard)
		}
	})
}

func (a *App) content(c *ui.Context) {
	ui.Column(c).Fill().MinWidth(0).Children(func() {
		if a.page == "operations" {
			a.operationsView(c)
			return
		}
		if a.page == "runtime-recovery" {
			a.runtimeRecoveryView(c)
			return
		}
		if a.page == "mutagen-environment" {
			a.mutagenEnvironmentView(c)
			return
		}
		if a.page == "sync" {
			a.syncView(c)
			return
		}
		if a.page == "account-import" {
			a.accountImportView(c)
			return
		}
		if a.page == "account" {
			a.accountView(c)
			return
		}
		if a.page == "account-default" {
			a.accountDefaultView(c)
			return
		}
		if a.page == "network" {
			a.networkView(c)
			return
		}
		if a.page == "wireguard" {
			a.wireguardView(c)
			return
		}
		if a.page == "setup" {
			a.localSetupView(c)
			return
		}
		if a.page == "init" {
			a.initWizardView(c)
			return
		}
		if a.page == "capabilities" {
			a.capabilitiesView(c)
			return
		}
		if a.page == "device" {
			a.deviceView(c)
			return
		}
		if a.page == "ego-browser" {
			a.egoBrowserView(c)
			return
		}
		if a.page == "nodes" {
			a.nodesView(c)
			return
		}
		if a.page == "credentials" {
			a.credentialsView(c)
			return
		}
		if a.page == "skills" {
			a.skillsView(c)
			return
		}
		if a.page == "generic-cli" {
			a.genericCLIView(c)
			return
		}
		if a.page == "login" {
			a.loginView(c)
			return
		}
		if a.page == "logout" {
			a.logoutView(c)
			return
		}
		if a.page == "remote" {
			a.remoteView(c)
			return
		}
		if a.page == "diagnostics" {
			a.diagnosticsView(c)
			return
		}
		if a.page == "command-diagnostics" {
			a.commandDiagnosticsView(c)
			return
		}
		if a.page == "dependencies" {
			a.dependenciesView(c)
			return
		}
		if a.page == "settings" {
			a.settingsView(c)
			return
		}
		project := a.project()
		if project == nil {
			a.welcome(c)
			return
		}
		ui.Row(c).Height(52).Padding(12, 20).Gap(12).Children(func() {
			ui.Text(c, project.Name).Bold()
			ui.Spacer(c)
			ui.Button(c, locales.SplitHorizontal()).Disabled(a.active == "" || a.starting).OnClick(func() { a.newSession(a.active, false) })
			ui.Button(c, locales.SplitVertical()).Disabled(a.active == "" || a.starting).OnClick(func() { a.newSession(a.active, true) })
			ui.Button(c, locales.SyncOpen()).OnClick(a.openSync)
			ui.Button(c, locales.Details()).OnClick(func() { a.state.Preferences.ShowDetails = !a.state.Preferences.ShowDetails; a.persist() })
		})
		a.tabbar(c, project.ID)
		ui.Row(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Grow(1).FillHeight().MinWidth(0).Padding(0, 12, 12).Children(func() {
				tab := a.state.Board.Current(project.ID)
				if tab == nil {
					a.projectEmpty(c, *project)
				} else {
					a.pane(c, tab.Root)
				}
			})
			if a.state.Preferences.ShowDetails {
				a.details(c, *project)
			}
		})
	})
}

func (a *App) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(64).Justify(ui.Center).Gap(18).Children(func() {
		ui.Text(c, locales.WelcomeEyebrow()).FontSize(12).TextColor(a.tokens().AccentInk)
		ui.Text(c, locales.WelcomeTitle()).FontSize(38).Bold()
		ui.Text(c, locales.WelcomeHelp()).FontSize(16).TextColor(t.TextMuted)
		ui.Row(c).Margin(16, 0).Gap(12).Wrap().Children(func() {
			ui.PrimaryButton(c, locales.OpenProjectDirectory()).Padding(12, 22).OnClick(func() { a.projectOpen = true })
			ui.Button(c, locales.ViewEnvironment()).Padding(12, 22).OnClick(func() { a.page = "settings" })
		})
		ui.Text(c, locales.SupportedAgents()).FontSize(12).TextColor(t.TextMuted)
	})
}

func (a *App) projectEmpty(c *ui.Context, p workspace.Project) {
	ui.Column(c).Fill().Center().Gap(16).Children(func() {
		ui.Text(c, locales.ChooseAgent()).FontSize(24).Bold()
		ui.Text(c, p.Path).TextColor(c.Theme().TextMuted)
		ui.PrimaryButton(c, locales.NewProjectSession()).OnClick(func() { a.newSession("", false) })
		ui.Text(c, locales.HiddenSessionsHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}

func (a *App) pane(c *ui.Context, p *workspace.Pane) {
	if p == nil {
		return
	}
	if p.First != nil {
		first := func() { a.pane(c, p.First) }
		second := func() { a.pane(c, p.Second) }
		if p.Vertical {
			if ui.SplitVerticalPercent(c.Key(p), &p.Ratio, first, second).Fill().Changed() {
				a.layoutChanged()
			}
		} else {
			if ui.SplitPercent(c.Key(p), &p.Ratio, first, second).Fill().Changed() {
				a.layoutChanged()
			}
		}
		return
	}
	var found *session.Record
	for _, r := range a.sessions.Records() {
		if r.ID == p.SessionID {
			copy := r
			found = &copy
			break
		}
	}
	if found == nil {
		if saved, ok := a.savedSession(p.SessionID); ok {
			a.savedPane(c, saved)
		}
		return
	}
	r := *found
	t := c.Theme()
	host, _ := r.Process.(*terminalhost.Host)
	// The pane matches its opaque terminal so no canvas seam shows round it.
	ui.Column(c.Key(r.ID)).Fill().MinWidth(0).MinHeight(0).Radius(10).Clip().Background(a.tokens().TerminalBG).Children(func() {
		ui.Row(c).Height(40).Padding(6, 12).Gap(8).Background(t.Surface).Children(func() {
			label := locales.LocalTool(string(r.Tool))
			if r.Remote {
				label = locales.RemoteTool(string(r.Tool))
			}
			ui.ButtonBase(c).Children(func() {
				title := ""
				if host != nil {
					title = host.Title()
				}
				if saved, ok := a.savedSession(r.ID); ok && saved.Name != "" {
					ui.Column(c).Children(func() {
						ui.Text(c, saved.Name).FontSize(13)
						if title == "" {
							title = label
						}
						ui.Text(c, title).FontSize(11).TextColor(t.TextMuted)
					})
				} else if title != "" {
					ui.Column(c).Children(func() {
						ui.Text(c, title).FontSize(13)
						ui.Text(c, label).FontSize(11).TextColor(t.TextMuted)
					})
				} else {
					ui.Text(c, label)
				}
			}).Grow(1).OnClick(func() { a.focusPane(r.ID, true) })
			if r.Exited {
				ui.Text(c, locales.ExitCode(r.ExitCode)).FontSize(12).TextColor(t.TextMuted)
			}
			ui.Button(c, locales.Hide()).FontSize(12).OnClick(func() { a.hideSession(r.ID) })
			ui.Button(c, "×").Label(locales.CloseSessionLabel(r.ID)).OnClick(func() { a.stopID = r.ID; a.stopForce = false; a.stopOpen = true })
		})
		if term, ok := r.Process.(*terminalhost.Host); ok {
			terminal.View(c.Key("terminal-"+r.ID), term.Terminal).Grow(1).FillWidth().FocusBind(&a.focusValue, r.ID)
		}
	})
	// Update exit metadata while visible, without busy polling in the background.
	if !r.Exited {
		c.After(time.Second)
	}
}

func (a *App) details(c *ui.Context, p workspace.Project) {
	details := ui.Column(c.Key("details")).Width(245).FillHeight().Padding(20).Gap(14).Background(c.Theme().Surface.Alpha(.3))
	if transition := theme.Transition(theme.Reduced(c, a.state.Preferences.ReduceMotion)); transition != nil {
		details = details.Transition(*transition)
	}
	details.Children(func() {
		ui.Text(c, locales.ProjectDetails()).Bold()
		ui.Text(c, p.Name)
		ui.Text(c, p.Path).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.ProjectDetailsHelp()).FontSize(13).TextColor(c.Theme().TextMuted)
	})
}

func (a *App) statusbar(c *ui.Context) {
	t := c.Theme()
	text := locales.ReadyBackground()
	if failure := a.saveError.Load(); failure != nil {
		text = locales.ErrorSavePreferences(presentation.Error(failure.err))
	}
	if a.message != nil {
		text = a.message()
	}
	ui.Row(c).Height(34).Padding(8, 20).Gap(12).Children(func() {
		ui.Text(c, text).FontSize(12).TextColor(t.TextMuted).Grow(1)
		ui.Text(c, locales.SessionCount(len(a.state.Sessions))).FontSize(12).TextColor(t.TextMuted)
	})
}
