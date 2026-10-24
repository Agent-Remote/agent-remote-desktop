package app

import (
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/presentation"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

func (a *App) capabilitiesView(c *ui.Context) {
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(12).Children(func() {
		ui.Row(c).Gap(10).Wrap().Children(func() {
			ui.Button(c, locales.RefreshRemote()).Disabled(a.capabilities.busy).OnClick(a.refreshCapabilities)
		})
		ui.Text(c, locales.CapabilitiesHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(12).Wrap().Children(func() {
			ui.Radio(c, &a.capabilities.tab, "", locales.CapabilitiesOperations())
			ui.Radio(c, &a.capabilities.tab, "commands", locales.CapabilitiesCommands())
		})
		if a.capabilities.busy {
			ui.Text(c, locales.CapabilitiesLoading())
			ui.Button(c, locales.CancelRemoteQuery()).OnClick(a.capabilities.cancel)
			return
		}
		if !a.capabilities.loaded {
			return
		}
		if a.capabilities.tab == "commands" {
			ui.TextInput(c, &a.capabilities.filter).Label(locales.CapabilitiesFilter()).Placeholder(locales.CapabilitiesFilter())
		}
		ui.Scroll(c).Grow(1).FillWidth().Gap(12).Children(func() {
			if a.capabilities.tab == "commands" {
				a.capabilitiesCommands(c)
				return
			}
			snapshot := a.capabilities.snapshot
			if snapshot.ProtocolError != nil {
				a.errorText(c, snapshot.ProtocolError, presentation.Error(snapshot.ProtocolError))
				return
			}
			ui.Text(c, locales.CapabilitiesVersion(snapshot.Capabilities.CLIVersion, capability.HostPlatform(snapshot.Capabilities.Platform))).FontSize(12)
			if snapshot.ContextError != nil {
				a.errorText(c, snapshot.ContextError, presentation.Error(snapshot.ContextError))
			}
			ui.Text(c, locales.CapabilitiesPermissionHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			for _, operation := range capability.Operations() {
				assessment := a.operationAssessment(operation.Method)
				a.card(c, ui.Column(c.Key(operation.Method)).FillWidth().Padding(12).Gap(4)).Children(func() {
					ui.Text(c, locales.CapabilitiesOperation(strings.ReplaceAll(operation.Method, ".", "_"))).Bold()
					ui.Text(c, locales.CapabilitiesState(assessment.State)).FontSize(12).TextColor(c.Theme().TextMuted)
				})
			}
			ui.Row(c).Gap(10).Wrap().Children(func() {
				ui.Button(c, locales.RemoteManagement()).OnClick(a.openRemote)
				ui.Button(c, locales.AccountDefaultTitle()).OnClick(a.openAccountDefault)
				ui.Button(c, locales.DiagnosticsTitle()).Disabled(!a.operationAssessment("diagnostics.inspect").CanStart()).OnClick(a.openDiagnostics)
			})
		})
	})
}

func (a *App) capabilitiesCommands(c *ui.Context) {
	ui.Text(c, locales.CapabilitiesCoverageHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
	count := 0
	filter := strings.ToLower(strings.TrimSpace(a.capabilities.filter))
	for _, catalog := range a.capabilities.snapshot.Catalogs {
		ui.Text(c, catalog.Executable).Bold()
		if catalog.Err != nil {
			a.errorText(c, catalog.Err, presentation.Error(catalog.Err))
			continue
		}
		ui.Text(c, locales.CapabilitiesCommandVersion(catalog.Schema.CLIVersion)).FontSize(12)
		for _, command := range catalog.Schema.Commands {
			name := catalog.Executable
			if len(command.Path) > 0 {
				name += " " + strings.Join(command.Path, " ")
			}
			if !strings.Contains(strings.ToLower(name), filter) {
				continue
			}
			count++
			a.card(c, ui.Column(c.Key(name)).FillWidth().Gap(5).Padding(12)).Children(func() {
				ui.Text(c, name).Bold()
				platformState, declaredPlatforms := commandPlatformInfo(catalog.Executable, command)
				ui.Text(c, locales.CapabilitiesPlatform(platformState)).FontSize(12)
				if len(declaredPlatforms) != 0 {
					ui.Text(c, strings.Join(declaredPlatforms, ", ")).FontSize(12).TextColor(c.Theme().TextMuted)
				}
				ui.Text(c, locales.CapabilitiesArguments(len(command.Arguments))).FontSize(12).TextColor(c.Theme().TextMuted)
				// `forward` has an optional subcommand (`list`/`stop`) but is
				// itself an executable create operation with positional/options.
				// Treat that parent node as actionable so the catalog exposes the
				// context-bound forwarding page instead of silently hiding the
				// create form behind its child commands.
				if !command.HasSubcommands || actionableCommandNode(catalog.Executable, command.Path) {
					sensitive := cliadapter.CommandHasSensitiveArguments(command)
					pageTarget := a.commandPageTarget(catalog.Executable, command.Path)
					gate := cliadapter.CapabilityState(catalog.Executable, command.Path, a.capabilities.snapshot.Capabilities, a.capabilities.snapshot.ProtocolError)
					if workflow, ok := cliadapter.Dedicated(catalog.Executable, command.Path); ok {
						gate = cliadapter.DedicatedCapabilityState(workflow, a.capabilities.snapshot.Capabilities, a.capabilities.snapshot.ProtocolError)
					}
					if method, _, ok := cliadapter.KnownCapability(catalog.Executable, command.Path); ok {
						assessment := a.operationAssessment(method)
						// A schema command can be backed by two independent
						// contracts: a context/permission operation and a native
						// terminal workflow. Keep a hard terminal failure visible
						// when the control operation happens to be available. For
						// example, node install needs nodes.list for target
						// selection *and* the agent-remote node-install terminal
						// capability; showing `unverified` when the latter is
						// missing would make a disabled install look authorized.
						if assessment.State != "ready" && commandGateCanBeRefined(gate) {
							gate = assessment.State
						}
					}
					// Keep the permission result visible next to the platform result.
					// A disabled Open button alone is ambiguous: it cannot tell a
					// user whether the command is unavailable, needs setup, or merely
					// has not been observed against the current account yet.
					ui.Text(c, locales.CapabilitiesState(gate)).FontSize(12).TextColor(c.Theme().TextMuted)
					if sensitive {
						ui.Text(c, locales.CliFormSecureWorkflow()).FontSize(12).TextColor(c.Theme().TextMuted)
					}
					if pageTarget == "remote-sessions" || pageTarget == "remote-create" {
						ui.Text(c, locales.CliFormRemoteWorkflow()).FontSize(12).TextColor(c.Theme().TextMuted)
					}
					if pageTarget == "network-forwards" {
						ui.Text(c, locales.CliFormPortForwardWorkflow()).FontSize(12).TextColor(c.Theme().TextMuted)
					}
					openLabel := locales.CliFormOpen()
					switch pageTarget {
					case "remote-sessions", "remote-create":
						openLabel = locales.CliFormOpenRemote()
					case "network-forwards":
						openLabel = locales.CliFormOpenPortForward()
					}
					// Dedicated pages remain inspectable on hosts where the CLI
					// declares another platform. They show the platform gate and
					// keep execution disabled at the reviewed workflow boundary;
					// disabling the catalog button here would hide the structured
					// explanation and strand users on a generic command card.
					platformBlocked := platformState == "unsupported" && pageTarget == ""
					ui.Button(c, openLabel).Disabled((sensitive && pageTarget == "") || platformBlocked || genericCLIStateBlocked(gate)).OnClick(func() {
						if pageTarget != "" {
							a.openCommandPageTarget(pageTarget)
							return
						}
						a.openGenericCLI(catalog.Executable, command)
					})
				}
			})
		}
	}
	if count == 0 {
		ui.Text(c, locales.NoSearchResults()).TextColor(c.Theme().TextMuted)
	}
}

// commandGateCanBeRefined reports whether a known control-operation
// assessment is allowed to replace the current gate. Terminal workflow
// failures are authoritative because the command cannot be launched even if
// a separate list/plan operation is available. Permission and context states
// remain refinements of an otherwise usable terminal workflow.
func commandGateCanBeRefined(state string) bool {
	switch state {
	case "platform_unsupported", "cli_unsupported", "unavailable":
		return false
	default:
		return true
	}
}

// commandPlatformInfo combines the CLI schema declaration with a reviewed
// desktop workflow declaration. Older CLI schemas predate per-command
// platforms; local Bridge workflows such as ego-browser still carry a
// desktop-side platform contract so their cards show the actual gate instead
// of an ambiguous "undeclared" state.
func commandPlatformInfo(executable string, command cliadapter.Command) (string, []string) {
	if len(command.Platforms) != 0 {
		return cliadapter.PlatformState(command, runtime.GOOS), append([]string(nil), command.Platforms...)
	}
	if workflow, ok := cliadapter.Dedicated(executable, command.Path); ok && len(workflow.Platforms) != 0 {
		declared := append([]string(nil), workflow.Platforms...)
		return cliadapter.PlatformState(cliadapter.Command{Platforms: declared}, runtime.GOOS), declared
	}
	return "undeclared", nil
}

func actionableCommandNode(executable string, path []string) bool {
	if executable == "fclaude" && len(path) == 0 {
		// fclaude's root invocation defaults to run/resume, even though Clap
		// also exposes lifecycle subcommands beneath the same command group.
		return true
	}
	if len(path) != 1 || path[0] != "forward" {
		return false
	}
	return executable == "agent-remote" || executable == "fclaude"
}

// commandPageTarget routes commands whose desktop-control contract already has
// a dedicated page away from the generic argv form. The generic form remains
// available for syntax that has no dedicated workflow.
func (a *App) commandPageTarget(executable string, path []string) string {
	// `init` is a composition workflow, not a single schema request. Route the
	// discovered leaf to the wizard so setup, authentication, optional
	// WireGuard approval, diagnostics and readiness keep their explicit order.
	if executable == "agent-remote" && len(path) == 1 && path[0] == "init" {
		return "init"
	}
	// `deps status` has an optional `--fix` mutation. Route the command to the
	// structured diagnostics page so its default read-only inspection and the
	// explicit dependency repair action share one context-bound workflow. The
	// page starts in `deps.inspect`; repair is a separate user action that
	// submits the exact home and confirmation to `deps.repair`, so opening the
	// command can never trigger `--fix` implicitly.
	// Login carries password/TOTP fields in the CLI schema. Route it to the
	// dedicated credential-safe page instead of leaving the discovered command
	// disabled by the generic argv sensitive-field guard.
	if executable == "agent-remote" && len(path) == 1 && path[0] == "login" {
		return "login"
	}
	if executable == "agent-remote" && len(path) == 1 && path[0] == "logout" {
		return "logout"
	}
	// fclaude lifecycle commands are aliases for the exact context-bound
	// workflows on the Remote management page. They must not open a generic
	// argv form, which cannot freeze the selected session, node and backend or
	// recover the original stop/delete request key.
	if executable == "fclaude" {
		if len(path) == 0 || len(path) == 1 && path[0] == "run" {
			// The default and explicit run forms select an existing remote
			// session before launching the exact attach workflow.
			return "remote-sessions"
		}
		if len(path) == 1 && path[0] == "new" {
			// New sessions use the structured account/workspace form and the
			// versioned sessions.create receipt rather than an unbound argv.
			return "remote-create"
		}
		switch strings.Join(path, " ") {
		case "list":
			return "remote-sessions"
		case "attach":
			// Attach is selected from the remote session inventory so the
			// desktop can freeze the complete session/device context before
			// opening the exact PTY workflow.
			return "remote-sessions"
		case "stop", "stop-status", "delete", "delete-status":
			return "remote-sessions"
		case "forward", "forward list", "forward stop":
			return "network-forwards"
		}
	}
	if executable == "agent-remote" {
		switch strings.Join(path, " ") {
		case "node install":
			return "nodes"
		case "status":
			return "command-diagnostics:status.inspect"
		case "doctor":
			return "command-diagnostics:doctor.inspect"
		case "deps status":
			return "command-diagnostics:deps.inspect"
		case "ssh check":
			return "command-diagnostics:ssh.inspect"
		case "attach":
			// The legacy agent-remote attach command is selected from the same
			// context-bound remote session inventory as fclaude attach.
			return "remote-sessions"
		case "forward", "forward list", "forward stop":
			return "network-forwards"
		case "account list", "account create", "account bind", "account attach", "account import-config", "account verify", "account status", "account disable":
			// Account IDs and binding keys are selected from the authenticated
			// account inventory. Route all account lifecycle commands through
			// that inventory instead of manufacturing an unbound generic argv.
			return "remote-accounts"
		case "account recover-runtime", "account recovery-status":
			return "runtime-recovery"
		case "account default set", "account default get", "account default clear":
			return "account-default"
		case "credentials list":
			return "credentials"
		case "deps mutagen-environment":
			return "mutagen-environment"
		case "sync ensure", "sync status", "sync pause", "sync resume", "sync resolve", "sync reset":
			return "sync"
		case "wireguard config":
			return "wireguard"
		case "wireguard check", "wireguard status":
			return "network"
		case "wireguard up":
			return "network-tunnel-up"
		case "wireguard down":
			return "network-tunnel-down"
		case "device status", "device diagnose", "device revoke", "device rotate-token":
			// Local Device status and diagnosis both use the same structured
			// status/plan page. Keep diagnose out of the generic argv form so the
			// user sees the bounded local plan and platform gate before the
			// reviewed terminal command can run.
			return "device"
		case "ego-browser status", "ego-browser list", "ego-browser requests", "ego-browser cancel-request", "ego-browser connect", "ego-browser claim", "ego-browser delete-device", "ego-browser device-delete", "ego-browser delete-binding", "ego-browser binding-delete", "ego-browser pause", "ego-browser resume", "ego-browser stop", "ego-browser revoke":
			// Status, candidate claim, request cancellation, binding lifecycle
			// and deletion all have context-bound desktop-control contracts.
			// They share the ego-browser page so a command card cannot open an
			// unbound schema form that bypasses generation and exact-request
			// checks. Local installation and registration paths remain below as
			// reviewed terminal workflows.
			return "ego-browser"
		case "skill list", "skill info", "skill status", "skill check", "skill retry", "skill state list", "skill state conflicts", "skill state info", "skill state diff":
			return "skills"
		case "skill add", "skill update", "skill enable", "skill disable", "skill pin", "skill unpin", "skill inherit", "skill remove", "skill rollback", "skill state export":
			return "skills-review:" + strings.Join(path, " ")
		}
	}
	return a.secureCommandTarget(executable, path)
}

func (a *App) openCommandPageTarget(target string) {
	if target == "init" {
		a.openInitWizard()
		return
	}
	if strings.HasPrefix(target, "command-diagnostics:") {
		a.openCommandDiagnostics(strings.TrimPrefix(target, "command-diagnostics:"))
		return
	}
	if target == "dependencies" {
		a.openDependencies()
		return
	}
	if target == "login" {
		// Capabilities discovery is deliberately passive and does not load the
		// authenticated remote context. Load that context first, then enter the
		// credential-safe page once its platform/permission gate is observed.
		a.pendingPage = "login"
		a.openRemote()
		return
	}
	if target == "logout" {
		a.pendingPage = "logout"
		a.openRemote()
		return
	}
	if target == "remote-sessions" {
		a.pendingPage = "remote-sessions"
		a.openRemote()
		return
	}
	if target == "remote-create" {
		a.pendingPage = "remote-create"
		a.openRemote()
		return
	}
	if target == "network-forwards" {
		a.openPortForwards()
		return
	}
	if target == "remote-accounts" {
		a.pendingPage = "remote-accounts"
		a.openRemote()
		return
	}
	if target == "runtime-recovery" {
		a.openRuntimeRecovery()
		return
	}
	if target == "account-default" {
		a.openAccountDefault()
		return
	}
	if target == "credentials" {
		a.openCredentials()
		return
	}
	if target == "mutagen-environment" {
		a.openMutagenEnvironment()
		return
	}
	if target == "sync" {
		a.openSync()
		return
	}
	if target == "wireguard" {
		a.openWireGuard()
		return
	}
	if target == "network" {
		a.openNetwork()
		return
	}
	if target == "network-tunnel-up" || target == "network-tunnel-down" {
		a.openNetwork()
		a.network.section = "plan"
		if target == "network-tunnel-up" {
			a.network.action = "up"
		} else {
			a.network.action = "down"
		}
		return
	}
	if target == "device" {
		a.openDevice()
		return
	}
	if target == "ego-browser" {
		a.openEgoBrowser()
		return
	}
	if target == "nodes" {
		a.openNodes()
		return
	}
	if target == "skills" {
		a.openSkills()
		return
	}
	if strings.HasPrefix(target, "skills-review:") {
		a.openSkillCommandReview(strings.TrimPrefix(target, "skills-review:"))
		return
	}
	a.openSecureCommandTarget(target)
}

// openSkillCommandReview keeps library mutations in the dedicated reviewed
// terminal workflow. The command schema supplies every argument, Dedicated
// injects --yes where required, and the immutable argv is shown before the
// managed PTY starts; no source content or credentials enter desktop-control.
func (a *App) openSkillCommandReview(commandPath string) {
	path := strings.Fields(strings.TrimSpace(commandPath))
	if len(path) < 2 || len(path) > 3 || path[0] != "skill" {
		return
	}
	for _, catalog := range a.capabilities.snapshot.Catalogs {
		if catalog.Executable != "agent-remote" || catalog.Err != nil {
			continue
		}
		for _, command := range catalog.Schema.Commands {
			if command.HasSubcommands || len(command.Path) != len(path) || strings.Join(command.Path, " ") != commandPath {
				continue
			}
			if _, ok := cliadapter.Dedicated("agent-remote", command.Path); !ok {
				return
			}
			a.openGenericCLI(catalog.Executable, command)
			a.genericCLI.workflow = "skill_" + strings.ReplaceAll(commandPath, " ", "_")
			a.genericCLI.workflowSummary = locales.SkillsChangeReview()
			return
		}
	}
}

// secureCommandTarget maps schema commands whose arguments must never enter a
// generic argv review to the existing credential-safe GUI. Keeping this map at
// the command coverage boundary makes those commands discoverable from the
// same page while preserving the dedicated page's context and confirmation
// checks.
func (a *App) secureCommandTarget(executable string, path []string) string {
	if executable != "agent-remote" || len(path) != 2 || path[0] != "credentials" {
		return ""
	}
	switch path[1] {
	case "create", "bind", "unbind":
		return "credentials:" + path[1]
	default:
		return ""
	}
}

func (a *App) openSecureCommandTarget(target string) {
	switch target {
	case "credentials:create", "credentials:bind", "credentials:unbind":
		a.openCredentials()
		a.credentials.action = strings.TrimPrefix(target, "credentials:")
		if a.credentials.action == "create" {
			a.credentials.form = remote.CredentialCreateInput{GHMode: "remote_login", SSHMode: "agent_forwarding"}
			a.credentials.gitUserName, a.credentials.gitUserEmail = "", ""
		}
	}
}
