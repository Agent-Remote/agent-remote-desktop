package app

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

// toolAvailable reads a snapshot. Redraws never run executable discovery after
// the initial lookup; explicit refresh and final launch validation update it.
func (a *App) toolAvailable(tool session.Tool) bool {
	if a.tools == nil {
		a.refreshTools()
	}
	for _, location := range a.tools {
		if location.name == tool.Executable() {
			return location.path != ""
		}
	}
	return false
}

func installationGuide(tool string) string {
	switch tool {
	case "codex":
		return "https://github.com/openai/codex"
	case "claude":
		return "https://code.claude.com/docs/en/setup"
	case "fclaude", "agent-remote":
		return "https://github.com/Agent-Remote/agent-remote-cli#release-packaging"
	default:
		return ""
	}
}

func (a *App) providerAvailability(c *ui.Context, tool session.Tool) {
	available := a.toolAvailable(tool)
	if available {
		ui.Text(c, locales.ProviderReady(string(tool))).FontSize(12).TextColor(c.Theme().TextMuted)
	} else {
		ui.Text(c, locales.ProviderRequired(string(tool))).FontSize(12).TextColor(c.Theme().Danger)
		ui.Text(c, locales.ProviderInstallHelp(string(tool))).FontSize(12).TextColor(c.Theme().TextMuted)
	}
	ui.Row(c).Gap(10).Wrap().Children(func() {
		ui.Button(c, locales.RefreshTools()).OnClick(func() { a.refreshTools(); a.message = nil })
		if !available {
			a.providerGuideButton(c, string(tool))
		}
	})
}

func (a *App) providerGuideButton(c *ui.Context, tool string) {
	target := installationGuide(tool)
	if target == "" {
		return
	}
	ui.Button(c, locales.ProviderInstallGuide(tool)).Tooltip(target).OnClick(func() {
		c.OpenURLThen(target, func(err error) {
			if err != nil {
				a.message = locales.ProviderGuideOpenFailed
			}
		})
	})
}
