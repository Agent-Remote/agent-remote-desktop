package app

import (
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type paletteItem struct {
	id, title, detail string
	run               func()
}

func (a *App) openPalette() { a.paletteQuery = ""; a.paletteOpen = true }

func (a *App) paletteItems() []paletteItem {
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
	query := strings.ToLower(strings.TrimSpace(a.paletteQuery))
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

func (a *App) palette(c *ui.Context) {
	ui.Modal(c, &a.paletteOpen, func() {
		ui.Column(c).Width(620).Gap(14).Children(func() {
			ui.Text(c, locales.CommandPalette()).FontSize(24).Bold()
			ui.TextInput(c, &a.paletteQuery).Label(locales.PaletteSearch()).Placeholder(locales.PaletteSearch()).AutoFocus().OnSubmit(func() {
				if items := a.paletteItems(); len(items) > 0 {
					a.choosePaletteItem(items[0])
				}
			})
			items := a.paletteItems()
			ui.Scroll(c).Height(320).Gap(4).Children(func() {
				if len(items) == 0 {
					ui.Text(c, locales.NoSearchResults()).TextColor(c.Theme().TextMuted)
				}
				for _, item := range items {
					ui.ButtonBase(c.Key(item.id)).FillWidth().Padding(10, 12).Radius(8).Children(func() {
						ui.Column(c).Gap(4).Children(func() { ui.Text(c, item.title); ui.Text(c, item.detail).FontSize(11).TextColor(c.Theme().TextMuted) })
					}).OnClick(func() { a.choosePaletteItem(item) })
				}
			})
			ui.Row(c).Justify(ui.End).Children(func() { ui.Button(c, locales.Cancel()).OnClick(func() { a.paletteOpen = false }) })
		})
	})
}
