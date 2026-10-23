package cliadapter

import (
	"errors"
	"runtime"
	"strings"
)

// DedicatedWorkflow describes CLI command families that need a reviewed
// desktop entry point. They still run in the native managed terminal, while
// the desktop applies a stricter capability and confirmation gate than the
// generic schema form.
//
// Method is deliberately the exact command path. It is used as the terminal
// capability key and is never sent as an executable command or interpreted by
// a shell.
type DedicatedWorkflow struct {
	Executable   string
	Path         []string
	Method       string
	Effect       string
	Credential   string
	Confirmation string
	Platforms    []string
	// RequiredFlags lists option names that must carry a value in the frozen
	// argv. It is used for exact remote attachment, where the CLI schema keeps
	// identity expectations optional for legacy compatibility but desktop must
	// never launch the unbound form.
	RequiredFlags        []string
	RequiresConfirmation bool
	ForceYes             bool
}

var (
	ErrDedicatedWorkflowUnknown      = errors.New("dedicated workflow is unknown")
	ErrDedicatedWorkflowUnconfirmed  = errors.New("dedicated workflow requires exact confirmation")
	ErrDedicatedWorkflowArgvMismatch = errors.New("dedicated workflow argv does not match the reviewed command")
	ErrDedicatedWorkflowMissingYes   = errors.New("dedicated workflow mutation requires --yes")
	ErrDedicatedWorkflowUnsafePath   = errors.New("dedicated workflow output path is unsafe")
)

// Dedicated returns the reviewed workflow metadata for a command path. The
// returned path is copied so callers cannot mutate the contract after review.
func Dedicated(executable string, path []string) (DedicatedWorkflow, bool) {
	copyPath := append([]string(nil), path...)
	method := executable + " " + strings.Join(copyPath, " ")
	w := DedicatedWorkflow{
		Executable:           executable,
		Path:                 copyPath,
		Method:               method,
		Effect:               "write",
		Credential:           "device",
		Confirmation:         "exact_request",
		RequiresConfirmation: true,
	}
	if executable == "fclaude" {
		// Invoking fclaude without a subcommand is the documented default
		// run/resume operation. The schema marks its root as a command group
		// because it also exposes lifecycle subcommands, but the root still
		// accepts Claude passthrough args and must remain reachable from the
		// desktop command catalog.
		if len(path) == 0 {
			w.Method = "fclaude run"
			w.Credential = "stored"
			return w, true
		}
		// fclaude run/new pass arguments through to Claude Code and can create
		// or resume a remote session. Keep those invocations in the native
		// managed terminal behind an exact argv review; the CLI remains the
		// authority for account selection, server authorization, and session
		// lifecycle semantics. Listing is read-only but still benefits from a
		// declared terminal capability so the desktop does not present an
		// unverified generic command as a complete GUI workflow.
		if len(path) != 1 {
			return DedicatedWorkflow{}, false
		}
		switch path[0] {
		case "run", "new":
			w.Credential = "stored"
			return w, true
		case "list":
			w.Effect = "read"
			w.Credential = "stored"
			w.Confirmation = ""
			w.RequiresConfirmation = false
			return w, true
		case "attach":
			// The CLI retains a legacy short-ID attach form. Desktop uses the
			// exact sessions.attach contract and therefore requires both
			// reviewed identity expectations in the immutable argv.
			w.Method = "sessions.attach"
			w.Credential = "device"
			w.RequiredFlags = []string{"--expected-server", "--expected-device"}
			return w, true
		default:
			return DedicatedWorkflow{}, false
		}
	}
	if executable != "agent-remote" || len(path) == 0 {
		return DedicatedWorkflow{}, false
	}
	if len(path) == 1 {
		if path[0] == "attach" {
			// The legacy agent-remote attach catalog node is routed to the
			// context-bound remote session picker. The picker launches the exact
			// fclaude attach form, so expose the same sessions.attach capability
			// gate on this legacy entry instead of presenting unknown permission.
			w.Method = "sessions.attach"
			w.Credential = "device"
			return w, true
		}
		if path[0] != "doctor" {
			return DedicatedWorkflow{}, false
		}
		// `doctor` is a dual-mode command: the default invocation observes
		// local installation state, while `--fix` may create or refresh local
		// metadata. Keep both modes behind one exact reviewed terminal
		// capability so the desktop never guesses whether a repair is needed
		// and never launches an unreviewed mutation.
		w.Effect = "write"
		w.Credential = "none"
		return w, true
	}
	switch {
	case path[0] == "ssh" && len(path) == 2 && path[1] == "check":
		// SSH checks are read-only local transport diagnostics. Keep the
		// optional session authorization inspection in the managed terminal so
		// the complete reviewed argv (including an exact session reference) is
		// frozen before execution and its output stays in the terminal.
		if len(path) != 2 {
			return DedicatedWorkflow{}, false
		}
		w.Effect = "read"
		w.Credential = "none"
		w.Confirmation = ""
		w.RequiresConfirmation = false
		return w, true
	case path[0] == "device":
		// The local Device app is implemented only on macOS. Keep these
		// operations in the managed terminal so the schema-derived argv is
		// reviewed and frozen before local installation or launch work begins.
		if len(path) != 2 {
			return DedicatedWorkflow{}, false
		}
		w.Platforms = []string{"macos"}
		w.Credential = "none"
		switch path[1] {
		case "status", "diagnose":
			w.Effect = "read"
			w.Confirmation = ""
			w.RequiresConfirmation = false
		case "install", "launch":
			// Install and launch have no --yes flag in the CLI schema. The
			// explicit desktop review is the confirmation boundary.
		case "uninstall":
			w.ForceYes = true
		default:
			return DedicatedWorkflow{}, false
		}
		return w, true
	case path[0] == "account" && len(path) == 2 && path[1] == "attach":
		// Account attach is a binding connection, not a normal remote session
		// attach. The account page builds the exact account UUID, binding
		// generation and expected server/device argv from the refreshed status;
		// bind the catalog entry to that terminal capability so an older CLI
		// cannot be presented as available merely because the account page is
		// reachable.
		w.Method = "accounts.binding.attach"
		w.Credential = "device"
		w.RequiredFlags = []string{"--binding-session", "--expected-server", "--expected-device"}
		return w, true
	case path[0] == "node" && len(path) == 2 && path[1] == "install":
		// Node installation itself is a controlled terminal operation. The
		// target Node is remote and may be Linux even when the client is not.
		w.ForceYes = true
	case path[0] == "skill" && len(path) >= 2 && path[1] == "state":
		// State repair and cleanup are local mutations with a deliberately
		// non-interactive CLI flag. Keep these paths behind the same reviewed
		// terminal capability as other destructive maintenance commands so a
		// generic schema form cannot leave an interactive prompt running in a
		// managed terminal or silently skip the confirmation boundary.
		if len(path) != 3 {
			return DedicatedWorkflow{}, false
		}
		switch path[2] {
		case "prune", "reset", "migrate", "resolve", "restore":
		case "list", "conflicts", "info", "diff", "export":
			// State inspection is still a reviewed terminal workflow. It keeps
			// the exact command and paging cursor in the managed PTY while
			// allowing the operation to be observed from the desktop center.
			w.Effect = "read"
			w.Credential = "user"
			w.Confirmation = ""
			w.RequiresConfirmation = false
			return w, true
		default:
			return DedicatedWorkflow{}, false
		}
		w.Credential = "stored"
		w.ForceYes = true
	case path[0] == "skill":
		// Skill source and library mutations need the CLI's user-session
		// credential and a non-interactive confirmation. Keep the complete
		// command in the managed terminal, but require the same reviewed
		// capability boundary as state repair so generic forms cannot launch
		// an unreviewed install/update/remove/rollback.
		if len(path) != 2 {
			return DedicatedWorkflow{}, false
		}
		switch path[1] {
		case "add", "update", "enable", "disable", "pin", "unpin", "inherit", "remove", "rollback":
			w.Credential = "user"
			w.ForceYes = true
		case "check", "info", "status", "list":
			w.Effect = "read"
			w.Credential = "user"
			w.Confirmation = ""
			w.RequiresConfirmation = false
		case "retry":
			w.Credential = "user"
			w.ForceYes = true
		default:
			return DedicatedWorkflow{}, false
		}
	case path[0] == "ego-browser":
		// The catalog also contains the `ego-browser` command group itself.
		// Dedicated is called from more than one discovery surface, so keep the
		// guard here instead of relying on HasSubcommands from one schema shape.
		if len(path) != 2 {
			return DedicatedWorkflow{}, false
		}
		// The Bridge and Device Client are distributed as macOS-only community
		// components. Some read-only commands can parse on other hosts, but
		// they cannot reach a supported local bridge there; expose that fact at
		// the desktop gate instead of presenting an undeclared command as
		// executable on Windows/Linux.
		w.Platforms = []string{"macos"}
		w.Credential = "stored"
		switch path[1] {
		case "status", "list", "requests":
			w.Effect = "read"
			w.Credential = "stored"
			w.Confirmation = ""
			w.RequiresConfirmation = false
		case "upgrade":
			w.Platforms = []string{"macos"}
			w.ForceYes = true
		case "setup", "connect", "repair", "remove", "forget-this-mac", "forget", "re-enroll", "device-rotate", "switch-server", "claim", "pause", "resume", "stop", "revoke", "delete-device", "device-delete", "delete-binding", "binding-delete", "cancel-request":
			w.ForceYes = true
		case "register":
			// Register has no --yes flag in the CLI schema. The desktop's
			// exact argv review is the confirmation boundary for this mutation;
			// requiring a flag that the CLI cannot accept would make the command
			// impossible to launch.
		default:
			return DedicatedWorkflow{}, false
		}
	default:
		return DedicatedWorkflow{}, false
	}
	return w, true
}

// InitialDedicatedFormValues applies the non-interactive confirmation default
// for a reviewed mutation. It is kept in the adapter so every UI surface uses
// the same frozen input rule instead of deciding independently which flag is
// safe to inject.
func InitialDedicatedFormValues(executable string, command Command) (FormValues, bool) {
	values := InitialFormValues(command)
	workflow, ok := Dedicated(executable, command.Path)
	if !ok || !workflow.ForceYes {
		return values, ok
	}
	for _, argument := range command.Arguments {
		if argument.ID == "yes" || argument.Long == "yes" {
			values[argument.ID] = []string{"true"}
			break
		}
	}
	return values, true
}

// DedicatedCapabilityState applies the desktop terminal capability contract.
// A declared terminal operation proves that this CLI build implements the
// workflow; it does not claim that server authorization has been observed.
// Mutating workflows therefore return unverified and rely on the CLI's own
// credential/permission checks at execution time.
func DedicatedCapabilityState(w DedicatedWorkflow, capabilities Capabilities, protocolError error) string {
	if protocolError != nil || capabilities.Validate() != nil {
		return "unavailable"
	}
	if len(w.Platforms) > 0 {
		host := PlatformName(runtime.GOOS)
		found := false
		for _, platform := range w.Platforms {
			if PlatformName(platform) == host {
				found = true
				break
			}
		}
		if !found {
			return "platform_unsupported"
		}
	}
	if !capabilities.SupportsTerminalContract(w.Method, w.Effect, w.Credential, w.Confirmation) {
		return "cli_unsupported"
	}
	if w.RequiresConfirmation {
		return "unverified"
	}
	return "ready"
}

// ValidateDedicatedArgv verifies the immutable command that the UI is about
// to hand to the managed terminal. The caller must pass confirmed=true only
// after the exact argv review button has been activated.
func ValidateDedicatedArgv(w DedicatedWorkflow, argv []string, confirmed bool) error {
	if w.Method == "" {
		return ErrDedicatedWorkflowUnknown
	}
	if !confirmed && w.RequiresConfirmation {
		return ErrDedicatedWorkflowUnconfirmed
	}
	if len(argv) < len(w.Path) {
		return ErrDedicatedWorkflowArgvMismatch
	}
	for i, part := range w.Path {
		if argv[i] != part || strings.ContainsRune(argv[i], 0) {
			return ErrDedicatedWorkflowArgvMismatch
		}
	}
	for _, value := range argv {
		if strings.ContainsRune(value, 0) {
			return ErrDedicatedWorkflowArgvMismatch
		}
	}
	if w.ForceYes && dedicatedRequiresYes(w, argv) && !containsYesFlag(argv[len(w.Path):]) {
		return ErrDedicatedWorkflowMissingYes
	}
	if err := validateRequiredFlags(w.RequiredFlags, argv[len(w.Path):]); err != nil {
		return err
	}
	if w.Method == "agent-remote skill state export" {
		if err := validateSkillExportOutput(argv[len(w.Path):]); err != nil {
			return err
		}
	}
	return nil
}

// validateSkillExportOutput protects the reviewed terminal boundary before a
// user supplied destination reaches the CLI. The CLI remains responsible for
// checking that the destination is absent or empty, is a real directory, and
// has a writable parent. Desktop only rejects values that could escape the
// user-visible path review through NUL bytes or parent traversal. Both slash
// styles are checked so the same frozen argv is safe on every supported host.
func validateSkillExportOutput(args []string) error {
	found := false
	var value string
	for i, arg := range args {
		if arg != "--output" {
			continue
		}
		if found {
			return ErrDedicatedWorkflowUnsafePath
		}
		found = true
		if i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") {
			return ErrDedicatedWorkflowUnsafePath
		}
		value = args[i+1]
	}
	if !found {
		// BuildArgv enforces the required --output argument. This guard keeps
		// the direct validation helper fail closed for hand-written argv.
		return ErrDedicatedWorkflowUnsafePath
	}
	if strings.ContainsRune(value, 0) {
		return ErrDedicatedWorkflowUnsafePath
	}
	for _, component := range strings.FieldsFunc(value, func(r rune) bool { return r == '/' || r == '\\' }) {
		if component == ".." {
			return ErrDedicatedWorkflowUnsafePath
		}
	}
	return nil
}

func validateRequiredFlags(required []string, args []string) error {
	for _, flag := range required {
		found := false
		for i, arg := range args {
			if arg != flag {
				continue
			}
			if found || i+1 >= len(args) || args[i+1] == "" || strings.HasPrefix(args[i+1], "-") || strings.ContainsRune(args[i+1], 0) {
				return ErrDedicatedWorkflowArgvMismatch
			}
			found = true
		}
		if !found {
			return ErrDedicatedWorkflowArgvMismatch
		}
	}
	return nil
}

// skill add --list is an explicitly read-only discovery mode. Clap rejects
// --yes together with --list, so the reviewed workflow keeps the same exact
// terminal boundary while allowing the user to clear the injected mutation
// confirmation before submitting that mode.
func dedicatedRequiresYes(w DedicatedWorkflow, argv []string) bool {
	if w.Method == "agent-remote skill add" {
		for _, arg := range argv[len(w.Path):] {
			if arg == "--list" {
				return false
			}
		}
	}
	return w.ForceYes
}

func containsYesFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--yes" || arg == "-y" {
			return true
		}
	}
	return false
}
