package app

import (
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func (a *App) dialogs(c *ui.Context) {
	ui.Modal(c, &a.projectOpen, func() {
		ui.Column(c).Width(520).Gap(16).Children(func() {
			ui.Text(c, locales.OpenProject()).FontSize(24).Bold()
			ui.Text(c, locales.ProjectLocationHelp()).TextColor(c.Theme().TextMuted)
			ui.TextInput(c, &a.projectPath).Placeholder(locales.ProjectPathPlaceholder()).Label(locales.ProjectDirectory()).AutoFocus()
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Button(c, locales.Browse()).OnClick(a.selectDirectory)
				ui.Spacer(c)
				ui.Button(c, locales.Cancel()).OnClick(func() { a.projectOpen = false })
				ui.PrimaryButton(c, locales.Open()).Disabled(strings.TrimSpace(a.projectPath) == "").OnClick(func() { a.addProject(a.projectPath) })
			})
			if a.message != nil {
				ui.Text(c, a.message()).TextColor(c.Theme().Danger)
			}
		})
	})
	ui.Modal(c, &a.newOpen, func() {
		_, height := c.Size()
		available := a.toolAvailable(a.newTool)
		ui.Column(c).Width(560).Height(min(520, height-100)).Gap(14).Children(func() {
			ui.Text(c, locales.NewAgentSession()).FontSize(24).Bold()
			ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(12).Children(func() {
				if p := a.project(); p != nil {
					ui.Text(c, p.Path).TextColor(c.Theme().TextMuted)
				}
				for _, tool := range []session.Tool{session.Codex, session.Claude, session.RemoteClaude} {
					label := locales.LocalTool(string(tool))
					if tool == session.RemoteClaude {
						label = locales.RemoteTool(string(tool))
					}
					ui.Radio(c, &a.newTool, tool, label)
				}
				a.providerAvailability(c, a.newTool)
				ui.TextInput(c, &a.newName).Label(locales.SessionName()).Placeholder(locales.SessionName())
				ui.Text(c, locales.LaunchArgsHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
				ui.TextInput(c, &a.newArgs).Placeholder(locales.LaunchArgsPlaceholder()).Label(locales.LaunchArgs())
				ui.Text(c, locales.LaunchToolHelp()).TextColor(c.Theme().TextMuted).FontSize(12)
				if a.message != nil {
					ui.Text(c, a.message()).TextColor(c.Theme().Danger)
				}
			})
			ui.Row(c).Justify(ui.End).Gap(10).Wrap().Children(func() {
				ui.Button(c, locales.Cancel()).OnClick(func() { a.newOpen = false })
				ui.PrimaryButton(c, locales.StartSession()).Disabled(a.starting || !available).OnClick(a.startSession)
			})
		})
	})
	ui.Modal(c, &a.stopOpen, func() {
		ui.Column(c).Width(480).Gap(16).Children(func() {
			ui.Text(c, locales.CloseSessionQuestion()).FontSize(22).Bold()
			if saved, ok := a.savedSession(a.stopID); ok {
				name := saved.Name
				if name == "" {
					name = saved.Tool
				}
				ui.Text(c, name).Bold()
				ui.Text(c, saved.Directory).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			remote := false
			for _, r := range a.sessions.Records() {
				if r.ID == a.stopID {
					remote = r.Remote
				}
			}
			message := locales.StopLocalHelp()
			if remote {
				message = locales.DetachRemoteHelp()
			}
			if a.stopForce {
				message = locales.ForceStopHelp()
			}
			ui.Text(c, message).TextColor(c.Theme().TextMuted)
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.Cancel()).OnClick(func() { a.stopOpen = false })
				ui.Button(c, locales.HideOnly()).OnClick(func() { a.hideSession(a.stopID); a.stopOpen = false })
				label := locales.ConfirmClose()
				if a.stopForce {
					label = locales.ForceStop()
				}
				ui.PrimaryButton(c, label).OnClick(a.removeSession)
			})
		})
	})
	ui.Modal(c, &a.quitOpen, func() {
		ui.Column(c).Width(520).Gap(16).Children(func() {
			ui.Text(c, locales.QuitQuestion()).FontSize(24).Bold()
			if a.runtimeRecovery.busy {
				ui.Text(c, locales.RuntimeRecoveryCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.mutagenEnvironment.busy {
				ui.Text(c, locales.MutagenEnvironmentCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.sync.busy {
				ui.Text(c, locales.SyncCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.accountImport.busy {
				ui.Text(c, locales.AccountImportCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.account.busy {
				ui.Text(c, locales.AccountCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.accountDefault.busy {
				ui.Text(c, locales.AccountDefaultCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.wireguard.busy {
				ui.Text(c, locales.WireGuardCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.setup.busy {
				ui.Text(c, locales.LocalSetupCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.logout.busy {
				ui.Text(c, locales.LogoutCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			if a.login.busy {
				ui.Text(c, locales.LoginCancelHelp()).TextColor(c.Theme().TextMuted)
			}
			local := 0
			for _, r := range a.sessions.Records() {
				if !r.Remote && !r.Exited {
					local++
					name := string(r.Tool)
					if saved, ok := a.savedSession(r.ID); ok && saved.Name != "" {
						name = locales.NamedSessionSummary(saved.Name, string(r.Tool))
					}
					ui.Text(c, locales.QuitSessionDetail(name, r.Directory)).FontSize(12)
				}
			}
			ui.Text(c, locales.QuitSessions(local)).TextColor(c.Theme().TextMuted)
			if a.quitForce {
				ui.Text(c, locales.QuitTimeoutHelp()).TextColor(c.Theme().Danger)
			}
			ui.Row(c).Gap(10).Wrap().Justify(ui.End).Children(func() {
				ui.Button(c, locales.KeepWorking()).OnClick(func() { a.quitOpen = false })
				label := locales.StopAndQuit()
				if a.quitForce {
					label = locales.ForceQuit()
				}
				ui.PrimaryButton(c, label).Disabled(a.quitting.Load() || a.starting).OnClick(a.confirmQuit)
			})
		})
	})
	a.remoteStopDialog(c)
}

func (a *App) settingsView(c *ui.Context) {
	if a.tools == nil {
		a.refreshTools()
	}
	env := a.chromeEnv(c)
	ui.Scroll(c).Fill().Padding(4, 28, 24).Gap(14).Children(func() {
		env.Panel(c, locales.Language(), false, func() {
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Button(c, locales.FollowSystem()).OnClick(func() { mygo.App.SetLocale("") })
				for _, tag := range mygo.App.Languages() {
					button := ui.Button(c.Key(tag), i18n.Autonym(tag))
					if tag == i18n.Locale() {
						button.Background(env.P.Selected()).TextColor(env.P.AccentInk)
					}
					button.OnClick(func() { mygo.App.SetLocale(tag) })
				}
			})
		})
		env.Panel(c, locales.Tools(), false, func() {
			env.Text(c, locales.SharedConfigHelp(), theme.Caption, env.P.TextSecondary)
			ui.Row(c).Children(func() { ui.Button(c, locales.RefreshTools()).OnClick(a.refreshTools) })
			for _, tool := range a.tools {
				name, path := tool.name, tool.path
				if path == "" {
					path = locales.ToolMissing()
				}
				env.Well(ui.Column(c.Key(name)).FillWidth().Padding(12).Gap(6)).Children(func() {
					env.Text(c, name, theme.BodyStrong, env.P.TextPrimary)
					env.Text(c, path, theme.MonoSmall, env.P.TextSecondary)
					if tool.path == "" {
						env.Text(c, locales.ProviderInstallHelp(name), theme.Caption, env.P.TextPrimary)
						ui.Row(c).Children(func() { a.providerGuideButton(c, name) })
					}
				})
			}
		})
		env.Panel(c, locales.ChordModifierTitle(), false, func() {
			// The layers' chord: ⌥/Alt by default, ⌃⌥/Ctrl+Alt where ⌥ +
			// letter must type text (redesign §7.3).
			choice := 0
			if a.state.Preferences.ChordModifier == chordModifierCtrlAlt {
				choice = 1
			}
			option, both := "Alt", "Ctrl+Alt"
			if runtime.GOOS == "darwin" {
				option, both = "⌥", "⌃⌥"
			}
			if env.Segmented(c, &choice, locales.ChordModifierTitle(), option, both) {
				a.state.Preferences.ChordModifier = map[int]string{0: "", 1: chordModifierCtrlAlt}[choice]
				a.chordCtrlAlt = choice == 1
				a.persist()
			}
			env.Text(c, locales.ChordModifierHelp(), theme.Caption, env.P.TextSecondary)
		})
		env.Panel(c, locales.Appearance(), false, func() {
			// Segment order follows the redesign: follow system, light, dark, kawaii.
			a.appearanceSwitch(c, env)
			if ui.Checkbox(c, &a.state.Preferences.FocusMode, locales.FocusMode()).Changed() {
				a.persist()
			}
			if ui.Checkbox(c, &a.state.Preferences.ReduceMotion, locales.ReduceMotion()).Changed() {
				a.persist()
			}
			env.Text(c, locales.ReduceMotionHelp(), theme.Caption, env.P.TextSecondary)
			if ui.Checkbox(c, &a.state.Preferences.ReduceTransparency, locales.ReduceTransparency()).Changed() {
				a.persist()
			}
			env.Text(c, locales.ReduceTransparencyHelp(), theme.Caption, env.P.TextSecondary)
			ui.Row(c).Gap(12).Wrap().Children(func() {
				ui.Text(c, locales.FontSize(int(a.state.Preferences.FontSize)))
				ui.Button(c, "－").Label(locales.DecreaseFont()).Disabled(a.state.Preferences.FontSize <= 10).OnClick(func() { a.changeFont(-1) })
				ui.Button(c, "＋").Label(locales.IncreaseFont()).Disabled(a.state.Preferences.FontSize >= 32).OnClick(func() { a.changeFont(1) })
			})
		})
	})
}

type toolLocation struct{ name, path string }

func (a *App) refreshTools() {
	a.tools = nil
	for _, name := range []string{"codex", "claude", "fclaude", "agent-remote"} {
		path, _ := a.findExecutable(name)
		a.tools = append(a.tools, toolLocation{name: name, path: path})
	}
}

func (a *App) changeFont(delta float32) {
	a.state.Preferences.FontSize += delta
	for _, r := range a.sessions.Records() {
		if t, ok := r.Process.(*terminalhost.Host); ok {
			t.Terminal.SetFont(terminal.Font{Size: a.state.Preferences.FontSize})
		}
	}
	a.persist()
}
