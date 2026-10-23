package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// projectView is the Control Center's project details (redesign §4.5),
// which replaces the removed details column: what the ⌥X wheel probes,
// read from local files only, and the project's own destinations.
func (a *App) projectView(c *ui.Context) {
	env := a.chromeEnv(c)
	ui.Scroll(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		project := a.project()
		if project == nil {
			env.Text(c, locales.ProjectDetailsEmpty(), theme.Body, env.P.TextSecondary)
			if env.Button(c, locales.OpenProjectDirectory(), chrome.Primary).Clicked() {
				a.closeLayer()
				a.projectOpen = true
			}
			return
		}
		branch, language := locales.ProjectReading(), locales.ProjectReading()
		if facts, ok := a.layers.probes.Get(project.Path); ok {
			switch {
			case facts.Branch == "":
				branch = locales.ProjectNotRepository()
			case facts.Detached:
				branch = locales.ProjectDetached(facts.Branch)
			default:
				branch = facts.Branch
			}
			language = languageName(facts.Language)
		}
		agents := locales.ProjectAgents(a.projectAgentCount(project.ID))
		env.Panel(c, project.Name, false, func() {
			env.KV(c, [][2]string{
				{locales.ProjectFieldPath(), project.Path},
				{locales.ProjectFieldBranch(), branch},
				{locales.ProjectFieldLanguage(), language},
				{locales.ProjectFieldAgents(), agents},
			})
		})
		env.Text(c, locales.ProjectDetailsHelp(), theme.Caption, env.P.TextSecondary)
		ui.Row(c).Gap(10).Wrap().Children(func() {
			if env.Button(c, locales.SyncOpen(), chrome.Secondary).Clicked() {
				a.openSync()
			}
			if env.Button(c, locales.PortForwardTitle(), chrome.Secondary).Clicked() {
				a.openPortForwards()
			}
			if env.Button(c, locales.SwitchProject(), chrome.Secondary).Clicked() {
				a.showLayer(layerProjects)
			}
		})
	})
}
