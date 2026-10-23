package app

import (
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type paletteItem struct {
	id, title, detail string
	run               func()
}

func (a *App) openPalette() { a.paletteQuery = ""; a.paletteOpen = true }

func (a *App) paletteItems() []paletteItem { return a.searchIndex(a.paletteQuery) }

// searchIndex is the one index of the palette and the Control Center's
// search (redesign §4.5, §4.7): pages, commands, projects and sessions.
func (a *App) searchIndex(q string) []paletteItem {
	items := []paletteItem{{"control-center", locales.ControlCenter(), "", func() { a.showLayer(layerControl) }},
		{"sync", locales.SyncTitle(), "", a.openSync}, {"diagnostics", locales.DiagnosticsTitle(), "", a.openDiagnostics},
		{"theme", locales.Theme(), "", func() { a.setTheme(theme.Next(a.shownTheme), false) }},
		{"operations", locales.OperationsTitle(), locales.OperationsHelp(), a.openOperations}, {"device", locales.DeviceTitle(), locales.DeviceHelp(), a.openDevice}, {"ego-browser", locales.EgoBrowserTitle(), locales.EgoBrowserHelp(), a.openEgoBrowser}, {"init", locales.InitWizardTitle(), locales.InitWizardHelp(), a.openInitWizard}, {"runtime-recovery", locales.RuntimeRecoveryTitle(), "", a.openRuntimeRecovery}, {"nodes", locales.NodesTitle(), locales.NodesHelp(), a.openNodes}, {"account-import-history", locales.AccountImportRecoveryTitle(), "", a.openAccountImportHistory}, {"mutagen-environment", locales.MutagenEnvironmentTitle(), "", a.openMutagenEnvironment}, {"dependencies", locales.DependenciesTitle(), locales.DependenciesHelp(), a.openDependencies}, {"account-default", locales.AccountDefaultTitle(), "", a.openAccountDefault}, {"capabilities", locales.CapabilitiesTitle(), "", a.openCapabilities}, {"skills", locales.SkillsTitle(), "", a.openSkills}, {"network", locales.WireGuardNetworkTitle(), "", a.openNetwork}}
	if a.account.submitted {
		items = append(items, paletteItem{"operation-account", locales.AccountOperationStatus(), "", a.openAccountOperation})
	}
	if a.login.hasOperation() {
		items = append(items, paletteItem{"operation-login", locales.LoginOperationStatus(), "", a.openLogin})
	}
	if a.logout.hasOperation() {
		items = append(items, paletteItem{"operation-logout", locales.LogoutOperationStatus(), "", a.openLogout})
	}
	if a.setup.hasOperation() {
		items = append(items, paletteItem{"operation-setup", locales.LocalSetupOperationStatus(), "", a.openLocalSetup})
	} else {
		items = append(items, paletteItem{"setup", locales.LocalSetupTitle(), "", a.openLocalSetup})
	}
	if a.wireguard.hasOperation() {
		items = append(items, paletteItem{"operation-wireguard", locales.WireGuardOperationStatus(), "", a.openWireGuard})
	} else {
		items = append(items, paletteItem{"wireguard", locales.WireGuardConfigTitle(), "", a.openWireGuard})
	}
	for _, command := range a.commands() {
		if command.id == "palette" {
			continue
		}
		modifier := "Ctrl"
		if runtime.GOOS == "darwin" {
			modifier = "Cmd"
		}
		shortcut := strings.ReplaceAll(command.accelerator, "CmdOrCtrl", modifier)
		items = append(items, paletteItem{"command-" + command.id, command.label(), shortcut, command.run})
	}
	for _, project := range a.state.Projects {
		items = append(items, paletteItem{"project-" + project.ID, project.Name, project.Path, func() { a.selectProject(project.ID) }})
	}
	for _, session := range a.state.Sessions {
		name := session.Name
		if name == "" {
			name = session.Tool
		}
		items = append(items, paletteItem{"session-" + session.ID, name, session.Directory, func() { a.focusSession(session.ID) }})
	}
	items = append(items, paletteItem{"project-details", locales.ProjectDetails(), "", func() { a.page = "project" }})
	query := strings.ToLower(strings.TrimSpace(q))
	if query == "" {
		return items
	}
	filtered := items[:0]
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.title+" "+item.detail+" "+item.id), query) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func (a *App) choosePaletteItem(item paletteItem) { a.paletteOpen = false; item.run() }

// palette is ⌘⇧P (redesign §4.7): a HUD search field a third of the way
// down, over the scrim, its results growing from under it. The keyboard
// stays in the field: arrows move the highlight, Enter runs it, Esc closes.
func (a *App) palette(c *ui.Context) {
	if !a.paletteOpen {
		a.paletteHot = 0
	}
	env := a.chromeEnv(c)
	_, height := c.Size()
	ui.DialogBase(c, &a.paletteOpen, func(backdrop, panel ui.Element) {
		scrim := env.P.Scrim
		if env.S.ReduceTransparency {
			scrim = scrim.Alpha(2)
		}
		backdrop.Background(scrim).Justify(ui.Start).Padding(max(24, height/3-60), 24, 24)
		env.Glass(panel.Width(620).Padding(10).Gap(8), chrome.HUD, 24, false).
			Transition(ui.ElementTransition{Animation: ptr(env.Move(theme.Snappy)), Enter: &ui.Motion{Y: -8, Scale: 0.96}, Exit: &ui.Motion{Y: -8, Scale: 0.96}})
		items := a.paletteItems()
		a.paletteHot = max(0, min(a.paletteHot, len(items)-1))
		in := ui.TextInput(c, &a.paletteQuery).Label(locales.PaletteSearch()).Placeholder(locales.PaletteSearch()).AutoFocus().
			FontSize(17*c.Theme().FontSize/14).Padding(10, 14).OnSubmit(func() {
			if len(items) > 0 {
				a.choosePaletteItem(items[a.paletteHot])
			}
		})
		if in.Changed() {
			a.paletteHot = 0
		}
		in.OnShortcut(0, ui.KeyDown, func() { a.paletteHot++ })
		in.OnShortcut(0, ui.KeyUp, func() { a.paletteHot = max(0, a.paletteHot-1) })
		ui.Scroll(c).FillWidth().MaxHeight(min(360, height*2/3-90)).Gap(2).Children(func() {
			if len(items) == 0 {
				env.Text(c, locales.NoSearchResults(), theme.Body, env.P.TextSecondary).Padding(10, 14)
			}
			for i, item := range items {
				row := ui.ButtonBase(c.Key(item.id)).FillWidth().Padding(9, 14).Radius(14).Justify(ui.Start).Label(item.title)
				if i == a.paletteHot || row.Hovered() {
					row = row.Background(env.P.Selected())
				}
				if row.Clicked() {
					a.choosePaletteItem(item)
				}
				row.Children(func() {
					ui.Column(c).Gap(2).Shrink(1).MinWidth(0).Children(func() {
						env.Text(c, item.title, theme.BodyStrong, env.P.TextPrimary).SingleLine().Ellipsis("…")
						if item.detail != "" {
							env.Text(c, item.detail, theme.Caption, env.P.TextSecondary).SingleLine().Ellipsis("…")
						}
					})
				})
			}
		})
	})
}
