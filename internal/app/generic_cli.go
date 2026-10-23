package app

import (
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/egoist/mygo/ui"
)

type genericCLIPage struct {
	executable      string
	command         cliadapter.Command
	requiresProject bool
	dedicated       cliadapter.DedicatedWorkflow
	dedicatedState  string
	values          cliadapter.FormValues
	text            map[string]string
	repeat          map[string][]string
	choice          map[string]string
	platformState   string
	capabilityState string
	sensitive       bool
	argv            []string
	review          bool
	workflow        string
	workflowSummary string
	err             error
}

// dedicatedOperation is the safe in-memory receipt projection for a reviewed
// node/Skill/ego-browser terminal request. The exact argv and CLI output remain in
// the managed terminal; the operation center only needs lifecycle state.
type dedicatedOperation struct {
	id        string
	command   string
	session   string
	status    string
	exitCode  int
	err       error
	updatedAt time.Time
	recovered bool
}

func (a *App) dedicatedOperation(id string) *dedicatedOperation {
	for i := range a.dedicatedOperations {
		if a.dedicatedOperations[i].id == id {
			return &a.dedicatedOperations[i]
		}
	}
	return nil
}

func (a *App) openGenericCLI(executable string, command cliadapter.Command) {
	capabilityState := cliadapter.CapabilityState(executable, command.Path, a.capabilities.snapshot.Capabilities, a.capabilities.snapshot.ProtocolError)
	dedicated, isDedicated := cliadapter.Dedicated(executable, command.Path)
	dedicatedState := ""
	if isDedicated {
		dedicatedState = cliadapter.DedicatedCapabilityState(dedicated, a.capabilities.snapshot.Capabilities, a.capabilities.snapshot.ProtocolError)
		capabilityState = dedicatedState
	}
	if method, _, ok := cliadapter.KnownCapability(executable, command.Path); ok {
		assessment := a.operationAssessment(method)
		if assessment.State != "ready" {
			capabilityState = assessment.State
		}
	}
	formValues, _ := cliadapter.InitialDedicatedFormValues(executable, command)
	a.genericCLI = genericCLIPage{
		executable:      executable,
		command:         command,
		requiresProject: genericCLIRequiresProject(executable, command.Path),
		dedicated:       dedicated,
		dedicatedState:  dedicatedState,
		values:          formValues,
		text:            map[string]string{},
		repeat:          map[string][]string{},
		choice:          map[string]string{},
		platformState:   commandPlatformState(executable, command),
		capabilityState: capabilityState,
		sensitive:       cliadapter.CommandHasSensitiveArguments(command),
	}
	for _, argument := range command.Arguments {
		values := a.genericCLI.values[argument.ID]
		if cliadapter.IsBooleanArgument(argument) {
			if len(values) == 0 {
				a.genericCLI.values[argument.ID] = []string{"false"}
			}
			continue
		}
		if len(argument.Choices) != 0 {
			if len(values) > 0 {
				a.genericCLI.choice[argument.ID] = values[0]
			}
			continue
		}
		if genericCLIRepeatable(argument) {
			a.genericCLI.repeat[argument.ID] = append([]string(nil), values...)
			if argument.Required && len(a.genericCLI.repeat[argument.ID]) == 0 {
				a.genericCLI.repeat[argument.ID] = []string{""}
			}
			continue
		}
		a.genericCLI.text[argument.ID] = strings.Join(values, "\n")
	}
	a.page = "generic-cli"
}

// genericCLIRequiresProject separates commands that operate on the selected
// workspace from local/home-scoped utility commands. The latter still run in
// a managed terminal, but use a desktop-created CLI utilities project when
// the user has not opened a workspace yet.
func genericCLIRequiresProject(executable string, path []string) bool {
	if executable == "fclaude" {
		return len(path) == 0 || (len(path) > 0 && (path[0] == "run" || path[0] == "new"))
	}
	if executable == "agent-remote" && len(path) > 0 {
		switch path[0] {
		case "status", "doctor", "deps", "ssh", "skill", "node", "device", "ego-browser":
			return false
		}
	}
	return true
}

func (a *App) ensureCLIUtilityProject() *workspace.Project {
	root, err := filepath.Abs(a.home)
	if err != nil {
		return nil
	}
	if resolved, resolveErr := filepath.EvalSymlinks(root); resolveErr == nil {
		root = resolved
	}
	for i := range a.state.Projects {
		if filepath.Clean(a.state.Projects[i].Path) == filepath.Clean(root) {
			return &a.state.Projects[i]
		}
	}
	project, err := workspace.NewProject(root)
	if err != nil {
		return nil
	}
	project.Name = locales.CliUtilityProject()
	a.state.Projects = append(a.state.Projects, project)
	a.persist()
	return &a.state.Projects[len(a.state.Projects)-1]
}

func commandPlatformState(executable string, command cliadapter.Command) string {
	state, _ := commandPlatformInfo(executable, command)
	return state
}

func (a *App) genericCLIBlocked() bool {
	p := &a.genericCLI
	return p.platformState == "unsupported" || p.sensitive || genericCLIStateBlocked(p.capabilityState) || p.dedicatedState == "platform_unsupported" || p.dedicatedState == "cli_unsupported" || p.dedicatedState == "unavailable" || a.starting || a.quitting.Load() || a.closing
}

func genericCLIStateBlocked(state string) bool {
	switch state {
	case "unavailable", "unsupported", "platform_unsupported", "cli_unsupported", "context_unavailable", "setup_required", "device_required", "denied", "sign_in_required":
		return true
	default:
		return false
	}
}

func genericCLIRepeatable(argument cliadapter.Argument) bool {
	return strings.EqualFold(argument.Action, "append") || argument.MaxValues != nil && *argument.MaxValues > 1
}

func (a *App) reviewGenericCLI() {
	p := &a.genericCLI
	if a.genericCLIBlocked() {
		if p.sensitive {
			p.err = nil
		} else if p.platformState == "unsupported" {
			p.err = fault.CLIUnavailable
		} else if p.capabilityState == "unavailable" || p.capabilityState == "unsupported" {
			p.err = fault.CLIUnavailable
		}
		return
	}
	argv, err := cliadapter.BuildArgv(p.command, p.values)
	if err != nil {
		p.err = err
		p.review = false
		p.argv = nil
		return
	}
	if p.dedicated.Method != "" {
		if err := cliadapter.ValidateDedicatedArgv(p.dedicated, argv, true); err != nil {
			p.err = err
			p.review = false
			p.argv = nil
			return
		}
	}
	p.err = nil
	p.argv = append([]string(nil), argv...)
	p.review = true
}

func (a *App) startGenericCLI() {
	p := &a.genericCLI
	if !p.review || len(p.argv) == 0 || a.genericCLIBlocked() {
		return
	}
	project := a.project()
	if project == nil && p.requiresProject {
		p.err = fault.ProjectDirectory
		return
	}
	if project == nil {
		project = a.ensureCLIUtilityProject()
		if project == nil {
			p.err = fault.ProjectDirectory
			return
		}
		a.state.Preferences.SelectedProject = project.ID
	}
	tool := session.Tool(p.executable)
	if tool != session.AgentRemote && tool != session.RemoteClaude {
		p.err = fault.UnsupportedAgent
		return
	}
	// Copy the complete argv before entering the asynchronous launch path. The
	// form can be edited only after this immutable terminal request exists.
	args := append([]string(nil), p.argv...)
	name := strings.TrimSpace(p.executable + " " + cliadapter.CommandPath(p.command))
	opID := workspace.NewID()
	a.dedicatedOperations = append(a.dedicatedOperations, dedicatedOperation{id: opID, command: name, status: "pending", updatedAt: time.Now().UTC()})
	// Persist only the fixed reviewed command label and lifecycle state. The
	// immutable argv remains in the managed terminal and is never part of the
	// desktop snapshot.
	a.persist()
	a.page = ""
	a.genericCLI = genericCLIPage{}
	a.launch(launchRequest{projectID: project.ID, directory: project.Path, tool: tool, args: args, name: name, operationID: opID})
}

func (a *App) genericCLIView(c *ui.Context) {
	p := &a.genericCLI
	ui.Column(c).Fill().Padding(4, 28, 24).Gap(12).Children(func() {
		ui.Text(c, locales.CliFormHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Text(c, locales.CliFormHomeManaged()).FontSize(12).TextColor(c.Theme().TextMuted)
		if p.dedicated.Method == "fclaude run" || p.dedicated.Method == "fclaude new" {
			ui.Text(c, locales.CliFclaudeTerminalLifecycleHelp()).FontSize(12).TextColor(a.tokens().AccentInk)
		}
		ui.Text(c, locales.CliFormExecutable()+": "+p.executable).Bold()
		commandName := cliadapter.CommandPath(p.command)
		if commandName == "" && p.dedicated.Method != "" {
			// fclaude's root invocation defaults to the run workflow while
			// still accepting passthrough args. Show the reviewed capability
			// label so the command group is not presented as an unnamed form.
			commandName = p.dedicated.Method
		}
		ui.Text(c, locales.CliFormCommand()+": "+commandName).Bold()
		if p.dedicated.Method == "agent-remote doctor" {
			ui.Text(c, locales.CliDoctorHelp()).FontSize(12).TextColor(c.Theme().TextMuted)
			if valuesContainTrue(p.values, "fix") {
				ui.Text(c, locales.CliDoctorFixHelp()).FontSize(12).TextColor(a.tokens().AccentInk)
			}
		}
		ui.Text(c, locales.CliFormPlatform(p.platformState)).FontSize(12).TextColor(c.Theme().TextMuted)
		if p.capabilityState == "unknown" {
			ui.Text(c, locales.CliFormUnknownCapability()).FontSize(12).TextColor(c.Theme().TextMuted)
		} else {
			ui.Text(c, locales.CliFormCapability(p.capabilityState)).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if p.command.Description != "" {
			ui.Text(c, p.command.Description).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if p.workflowSummary != "" {
			ui.Text(c, p.workflowSummary).FontSize(12).TextColor(a.tokens().AccentInk)
		}
		if p.dedicated.Method != "" && p.dedicated.RequiresConfirmation {
			ui.Text(c, locales.CliDedicatedConfirmationRequired()).FontSize(12).TextColor(a.tokens().AccentInk)
		}
		if p.err != nil {
			if p.err == fault.ProjectDirectory {
				ui.Text(c, locales.CliFormProjectRequired()).TextColor(c.Theme().Danger)
			} else if p.platformState == "unsupported" {
				ui.Text(c, locales.CliFormPlatformUnsupported()).TextColor(c.Theme().Danger)
			} else if genericCLIStateBlocked(p.capabilityState) {
				ui.Text(c, locales.CliFormCapabilityUnavailable()).TextColor(c.Theme().Danger)
			} else if errors.Is(p.err, cliadapter.ErrDedicatedWorkflowUnconfirmed) {
				ui.Text(c, locales.CliDedicatedConfirmationRequired()).TextColor(c.Theme().Danger)
			} else if errors.Is(p.err, cliadapter.ErrDedicatedWorkflowArgvMismatch) {
				ui.Text(c, locales.CliDedicatedArgvChanged()).TextColor(c.Theme().Danger)
			} else if errors.Is(p.err, cliadapter.ErrDedicatedWorkflowMissingYes) {
				ui.Text(c, locales.CliDedicatedYesRequired()).TextColor(c.Theme().Danger)
			} else if errors.Is(p.err, cliadapter.ErrDedicatedWorkflowUnsafePath) {
				ui.Text(c, locales.CliDedicatedUnsafePath()).TextColor(c.Theme().Danger)
			} else {
				ui.Text(c, locales.CliFormCommandInvalid(genericCLIValidationError(p.err))).TextColor(c.Theme().Danger)
			}
		} else if p.sensitive {
			ui.Text(c, locales.CliFormSecureWorkflow()).TextColor(c.Theme().Danger)
		}
		if p.review {
			a.genericCLIReview(c)
			return
		}
		ui.Scroll(c).Grow(1).MinHeight(0).FillWidth().Gap(10).Children(func() {
			for _, argument := range p.command.Arguments {
				a.genericCLIArgument(c, argument)
			}
			if len(p.command.Arguments) == 0 {
				ui.Text(c, locales.CliFormNoArguments()).TextColor(c.Theme().TextMuted)
			}
		})
		ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
			ui.Button(c, locales.Cancel()).OnClick(func() { a.page = ""; a.genericCLI = genericCLIPage{} })
			ui.PrimaryButton(c, locales.CliFormReview()).Disabled(a.genericCLIBlocked()).OnClick(a.reviewGenericCLI)
		})
	})
}

func valuesContainTrue(values cliadapter.FormValues, id string) bool {
	items := values[id]
	return len(items) > 0 && strings.EqualFold(items[0], "true")
}

func genericCLIValidationError(err error) string {
	switch {
	case errors.Is(err, cliadapter.ErrFormArgumentMissing):
		return locales.CliFormArgumentMissing()
	case errors.Is(err, cliadapter.ErrFormArgumentCount):
		return locales.CliFormArgumentCount()
	case errors.Is(err, cliadapter.ErrFormArgumentChoice):
		return locales.CliFormArgumentChoice()
	case errors.Is(err, cliadapter.ErrFormArgumentUnknown):
		return locales.CliFormArgumentUnknown()
	case errors.Is(err, cliadapter.ErrFormArgumentFlag):
		return locales.CliFormArgumentFlag()
	case errors.Is(err, cliadapter.ErrFormArgumentManaged):
		return locales.CliFormArgumentManaged()
	default:
		return locales.CliFormArgumentFlag()
	}
}

func (a *App) genericCLIArgument(c *ui.Context, argument cliadapter.Argument) {
	p := &a.genericCLI
	// The global --home field is owned by the desktop launch environment. It
	// remains part of the discovered schema for compatibility, but exposing it
	// as an editable control would allow a command to escape the selected home
	// and the capability/context checks bound to it.
	if cliadapter.IsDesktopManagedArgument(argument) {
		return
	}
	// `skill add --list` is the one dedicated path with an explicit read-only
	// mode that conflicts with --yes. Keep its checkbox visible so the user
	// can clear the mutation confirmation before reviewing discovery; all
	// mutating variants are still rejected without --yes by the adapter.
	if p.dedicated.Method != "" && p.dedicated.ForceYes && p.dedicated.Method != "agent-remote skill add" && (argument.ID == "yes" || argument.Long == "yes") {
		return
	}
	label := argument.ID
	if argument.Long != "" {
		label = "--" + argument.Long
	} else if argument.Short != "" {
		label = "-" + argument.Short
	}
	if argument.Required {
		label = locales.CliFormRequiredLabel(label, locales.CliFormRequired())
	}
	ui.Column(c.Key("generic-arg-" + argument.ID)).Gap(4).Children(func() {
		if cliadapter.IsBooleanArgument(argument) {
			selected := len(p.values[argument.ID]) > 0 && strings.EqualFold(p.values[argument.ID][0], "true")
			if ui.Checkbox(c, &selected, label).Changed() {
				if selected {
					p.values[argument.ID] = []string{"true"}
				} else {
					p.values[argument.ID] = []string{"false"}
				}
			}
			return
		}
		if len(argument.Choices) != 0 {
			ui.Text(c, label).Bold()
			selected := p.choice[argument.ID]
			for _, choice := range argument.Choices {
				ui.Radio(c, &selected, choice, choice).OnChange(func() {
					p.choice[argument.ID] = selected
					p.values[argument.ID] = []string{choice}
				})
			}
			return
		}
		if genericCLIRepeatable(argument) {
			a.genericCLIRepeatableArgument(c, argument, label)
			if argument.Description != "" {
				ui.Text(c, argument.Description).FontSize(12).TextColor(c.Theme().TextMuted)
			}
			return
		}
		value := p.text[argument.ID]
		input := ui.TextInput(c, &value).Label(label).Placeholder(strings.Join(argument.ValueNames, "/"))
		if input.Changed() {
			p.text[argument.ID] = value
			if strings.Contains(value, "\n") || strings.EqualFold(argument.Action, "append") {
				if value == "" {
					p.values[argument.ID] = nil
				} else {
					p.values[argument.ID] = strings.Split(value, "\n")
				}
			} else {
				p.values[argument.ID] = []string{value}
			}
		}
		if argument.Description != "" {
			ui.Text(c, argument.Description).FontSize(12).TextColor(c.Theme().TextMuted)
		}
		if argument.MaxValues == nil || argument.MaxValues != nil && *argument.MaxValues > 1 {
			ui.Text(c, locales.CliFormValuesHelp()).FontSize(11).TextColor(c.Theme().TextMuted)
		}
	})
}

func (a *App) genericCLIRepeatableArgument(c *ui.Context, argument cliadapter.Argument, label string) {
	p := &a.genericCLI
	values := p.repeat[argument.ID]
	for index := range values {
		value := values[index]
		ui.Row(c).Gap(6).Children(func() {
			input := ui.TextInput(c, &value).Label(label).Placeholder(strings.Join(argument.ValueNames, "/"))
			if input.Changed() {
				p.repeat[argument.ID][index] = value
				p.values[argument.ID] = append([]string(nil), p.repeat[argument.ID]...)
			}
			ui.Button(c, "−").Label(locales.CliFormRemoveValue()).Disabled(len(values) == 1 && argument.Required).OnClick(func() {
				p.repeat[argument.ID] = append(p.repeat[argument.ID][:index], p.repeat[argument.ID][index+1:]...)
				p.values[argument.ID] = append([]string(nil), p.repeat[argument.ID]...)
			})
		})
	}
	ui.Button(c, locales.CliFormAddValue()).OnClick(func() {
		if argument.MaxValues == nil || len(p.repeat[argument.ID]) < *argument.MaxValues || strings.EqualFold(argument.Action, "append") {
			p.repeat[argument.ID] = append(p.repeat[argument.ID], "")
			p.values[argument.ID] = append([]string(nil), p.repeat[argument.ID]...)
		}
	})
}

func (a *App) genericCLIReview(c *ui.Context) {
	p := &a.genericCLI
	ui.Column(c).Gap(10).Children(func() {
		ui.Text(c, locales.CliFormFrozen()).Bold()
		ui.Text(c, strings.Join(p.argv, " ")).FontSize(12).TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
			ui.Button(c, locales.Cancel()).OnClick(func() { p.review = false; p.argv = nil })
			ui.PrimaryButton(c, locales.CliFormStart()).Disabled((p.requiresProject && a.project() == nil) || a.genericCLIBlocked()).OnClick(a.startGenericCLI)
		})
	})
}
