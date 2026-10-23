package cliadapter

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// FormValues is the editable, string-only representation of one command
// form. Values are kept as slices because Clap's Append and repeated options
// are part of the schema contract. The map is deliberately independent from
// the UI so a submit can take a complete immutable snapshot before starting
// a terminal process.
type FormValues map[string][]string

var (
	ErrFormArgumentUnknown = errors.New("unknown command argument")
	ErrFormArgumentMissing = errors.New("required command argument missing")
	ErrFormArgumentCount   = errors.New("invalid command argument count")
	ErrFormArgumentChoice  = errors.New("command argument choice is invalid")
	ErrFormArgumentFlag    = errors.New("command argument flag is invalid")
	// ErrFormArgumentValue is returned before a reviewed argv reaches an
	// exec boundary. In particular, NUL cannot be represented by an OS
	// process argument; accepting it into the frozen review would defer a
	// deterministic validation failure until process launch.
	ErrFormArgumentValue   = errors.New("command argument value is invalid")
	ErrFormArgumentManaged = errors.New("desktop-managed command argument cannot be overridden")
)

// CloneFormValues returns a deep copy suitable for freezing at submit time.
func CloneFormValues(values FormValues) FormValues {
	clone := make(FormValues, len(values))
	for key, items := range values {
		clone[key] = append([]string(nil), items...)
	}
	return clone
}

// InitialFormValues applies schema defaults without mutating a schema or the
// caller's state. A boolean flag's default is represented by "true" or
// "false" so the same snapshot code handles checkboxes and text fields.
func InitialFormValues(command Command) FormValues {
	values := make(FormValues, len(command.Arguments))
	for _, argument := range command.Arguments {
		if len(argument.Defaults) != 0 {
			values[argument.ID] = append([]string(nil), argument.Defaults...)
			continue
		}
		if IsBooleanArgument(argument) {
			if action := strings.ToLower(argument.Action); action == "setfalse" || action == "storefalse" {
				// Clap's *False actions default to true and emit their flag only
				// when the user opts into false.
				values[argument.ID] = []string{"true"}
			} else {
				values[argument.ID] = []string{"false"}
			}
		}
	}
	return values
}

// IsBooleanArgument identifies Clap's flag actions emitted by the schema.
// Unknown actions remain value controls; this keeps the adapter forward
// compatible with newer Clap releases instead of silently dropping input.
func IsBooleanArgument(argument Argument) bool {
	switch strings.ToLower(argument.Action) {
	case "settrue", "setfalse", "storetrue", "storefalse", "count":
		return true
	default:
		return argument.MinValues == 0 && argument.MaxValues != nil && *argument.MaxValues == 0
	}
}

// IsDesktopManagedArgument identifies schema fields whose value is owned by
// the desktop process rather than by an individual command form. The CLI's
// global --home flag is intentionally excluded from generic argv: launch
// requests already bind AGENT_REMOTE_HOME to the selected configuration
// environment, and allowing a second home in argv would let a form escape
// the capability/context checks performed for that environment.
func IsDesktopManagedArgument(argument Argument) bool {
	return strings.EqualFold(argument.ID, "home") || strings.EqualFold(argument.Long, "home")
}

func hasNonEmptyValue(items []string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) != "" {
			return true
		}
	}
	return false
}

func validArgumentValue(value string) bool {
	return !strings.ContainsRune(value, 0)
}

// IsSensitiveArgument conservatively identifies schema fields that can carry
// credentials or one-time authorization material. Generic forms must not put
// these values in argv or show them in a frozen-command review; dedicated
// workflows use the CLI's controlled input channel instead.
func IsSensitiveArgument(argument Argument) bool {
	fields := []string{strings.ToLower(argument.ID), strings.ToLower(argument.Long), strings.ToLower(argument.ValueHint)}
	for _, field := range fields {
		field = strings.ReplaceAll(field, "-", "_")
		for _, token := range []string{"password", "token", "secret", "private_key", "totp", "device_code"} {
			if strings.Contains(field, token) {
				return true
			}
		}
	}
	return false
}

// CommandHasSensitiveArguments reports whether a command needs a dedicated
// credential-safe workflow instead of a generic argv form.
func CommandHasSensitiveArguments(command Command) bool {
	// Developer credential commands are always routed through their reviewed
	// workflow, even when a future schema happens to omit a secret-looking
	// argument. This keeps the generic argv path from becoming a credential API.
	if len(command.Path) >= 2 && command.Path[0] == "credentials" {
		switch command.Path[1] {
		case "create", "bind", "unbind":
			return true
		}
	}
	for _, argument := range command.Arguments {
		if IsSensitiveArgument(argument) {
			return true
		}
	}
	return false
}

// BuildArgv validates values against the exact discovered command and emits a
// deterministic argv. It never invokes a shell and never retries a command.
// The returned slice is safe to retain as the frozen submit snapshot.
func BuildArgv(command Command, input FormValues) ([]string, error) {
	values := CloneFormValues(input)
	known := make(map[string]Argument, len(command.Arguments))
	for _, argument := range command.Arguments {
		if argument.ID == "" || known[argument.ID].ID != "" {
			return nil, fmt.Errorf("%w: %q", ErrFormArgumentUnknown, argument.ID)
		}
		known[argument.ID] = argument
	}
	for key := range values {
		if _, ok := known[key]; !ok {
			return nil, fmt.Errorf("%w: %q", ErrFormArgumentUnknown, key)
		}
	}

	type positional struct {
		index int
		value string
	}
	var positionals []positional
	argv := append([]string(nil), command.Path...)
	for _, argument := range command.Arguments {
		items := append([]string(nil), values[argument.ID]...)
		if len(items) == 0 {
			items = append(items, argument.Defaults...)
		}
		if IsDesktopManagedArgument(argument) {
			if hasNonEmptyValue(items) {
				return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentManaged)
			}
			// The selected desktop home is supplied through the launch
			// environment. Never emit a duplicate --home argument.
			continue
		}
		if IsBooleanArgument(argument) {
			count, emit, err := booleanCount(argument, items)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", argument.ID, err)
			}
			for i := 0; i < count; i++ {
				argv = append(argv, argumentFlag(argument))
			}
			if emit && argumentFlag(argument) == "" {
				return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentFlag)
			}
			continue
		}
		// Text controls can leave an empty row when a user clears an optional
		// field or an optional repeatable value.  An empty argv value is almost
		// never what a schema form means and, for flags, would silently turn a
		// cleared control into `--flag ""`.  Drop those rows before enforcing
		// cardinality; required/minimum-valued arguments then fail closed below.
		if !argument.Required && argument.MinValues == 0 {
			compact := items[:0]
			for _, item := range items {
				if item != "" {
					compact = append(compact, item)
				}
			}
			items = compact
		}
		if argument.Required && len(items) == 0 {
			return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentMissing)
		}
		min := argument.MinValues
		if argument.Required && min == 0 {
			min = 1
		}
		if len(items) < min || strings.ToLower(argument.Action) != "append" && argument.MaxValues != nil && len(items) > *argument.MaxValues {
			return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentCount)
		}
		for _, item := range items {
			if !validArgumentValue(item) {
				return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentValue)
			}
			if item == "" && (argument.Required || min > 0) {
				return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentMissing)
			}
			if len(argument.Choices) != 0 && !contains(argument.Choices, item) {
				return nil, fmt.Errorf("%s: %w: %q", argument.ID, ErrFormArgumentChoice, item)
			}
			if argument.Position != nil {
				positionals = append(positionals, positional{index: *argument.Position, value: item})
			}
		}
		if argument.Position == nil && len(items) != 0 {
			flag := argumentFlag(argument)
			if flag == "" {
				return nil, fmt.Errorf("%s: %w", argument.ID, ErrFormArgumentFlag)
			}
			if strings.ToLower(argument.Action) == "append" {
				for _, item := range items {
					argv = append(argv, flag, item)
				}
			} else {
				argv = append(argv, flag)
				argv = append(argv, items...)
			}
		}
	}
	// Position indexes are relative to the command's positional arguments. A
	// stable sort also preserves repeated values with the same index.
	for i := 1; i < len(positionals); i++ {
		for j := i; j > 0 && positionals[j].index < positionals[j-1].index; j-- {
			positionals[j], positionals[j-1] = positionals[j-1], positionals[j]
		}
	}
	for _, item := range positionals {
		argv = append(argv, item.value)
	}
	return argv, nil
}

func booleanCount(argument Argument, values []string) (int, bool, error) {
	action := strings.ToLower(argument.Action)
	if action == "count" {
		if len(values) == 0 {
			return 0, false, nil
		}
		if len(values) != 1 {
			return 0, false, ErrFormArgumentCount
		}
		count, err := strconv.Atoi(values[0])
		if err != nil || count < 0 {
			return 0, false, ErrFormArgumentFlag
		}
		return count, count > 0, nil
	}
	if len(values) == 0 {
		return 0, false, nil
	}
	if len(values) != 1 {
		return 0, false, ErrFormArgumentCount
	}
	value, err := strconv.ParseBool(values[0])
	if err != nil {
		return 0, false, ErrFormArgumentFlag
	}
	if action == "setfalse" || action == "storefalse" {
		// Clap's SetFalse/StoreFalse flags represent a value whose default is
		// true. Passing the flag opts into false, so the UI value and emitted
		// flag presence are intentionally inverted.
		return boolInt(!value), !value, nil
	}
	return boolInt(value), value, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func argumentFlag(argument Argument) string {
	if argument.Long != "" {
		return "--" + argument.Long
	}
	if argument.Short != "" {
		return "-" + argument.Short
	}
	return ""
}

func contains(items []string, wanted string) bool {
	for _, item := range items {
		if item == wanted {
			return true
		}
	}
	return false
}

// CommandPath is the stable display and key form used by the generic page.
func CommandPath(command Command) string { return strings.Join(command.Path, " ") }

// PlatformState is the command-specific client-host declaration. An absent
// declaration is intentionally "undeclared" and remains executable.
func PlatformState(command Command, host string) string {
	if len(command.Platforms) == 0 {
		return "undeclared"
	}
	host = PlatformName(host)
	for _, platform := range command.Platforms {
		if PlatformName(platform) == host {
			return "supported"
		}
	}
	return "unsupported"
}

// KnownCapability returns the narrow set of command-to-control capability
// mappings that are authoritative today. Unknown syntax is still available as
// a native terminal command and is never treated as authorized by omission.
func KnownCapability(executable string, path []string) (method, effect string, ok bool) {
	if executable == "fclaude" && len(path) == 0 {
		// The default invocation opens the same inventory as `run`: it may
		// resume an existing workspace session and offers the structured create
		// action separately.
		return "sessions.list", "read", true
	}
	key := executable + " " + strings.Join(path, " ")
	switch key {
	case "agent-remote status":
		return "status.inspect", "read", true
	case "agent-remote doctor":
		return "doctor.inspect", "read", true
	case "agent-remote deps status":
		return "deps.inspect", "read", true
	case "agent-remote ssh check":
		return "ssh.inspect", "read", true
	case "agent-remote deps mutagen-environment":
		return "mutagen.environment.preview", "read", true
	case "agent-remote sync ensure":
		return "sync.ensure", "write", true
	case "agent-remote sync status":
		return "sync.status", "read", true
	case "agent-remote sync pause":
		return "sync.pause", "write", true
	case "agent-remote sync resume":
		return "sync.resume", "write", true
	case "agent-remote sync resolve":
		return "sync.resolve", "write", true
	case "agent-remote sync reset":
		return "sync.reset", "write", true
	case "agent-remote wireguard check":
		return "wireguard.check", "read", true
	case "agent-remote wireguard status":
		return "wireguard.status", "read", true
	case "agent-remote wireguard config":
		return "wireguard.config.plan", "read", true
	case "agent-remote wireguard up", "agent-remote wireguard down":
		return "wireguard.tunnel.apply", "write", true
	case "agent-remote account list":
		return "accounts.list", "read", true
	case "agent-remote account create":
		return "accounts.create", "write", true
	case "agent-remote account bind":
		return "accounts.bind", "write", true
	case "agent-remote account import-config":
		// The command contains discover/prepare/validate/apply/status/forget
		// modes. The GUI import workflow starts with the read-only prepare
		// capability and performs each later mutation with its own exact gate.
		return "accounts.import.prepare", "read", true
	case "agent-remote account verify":
		return "accounts.verify", "write", true
	case "agent-remote account status":
		return "accounts.status", "read", true
	case "agent-remote account disable":
		return "accounts.disable", "write", true
	case "agent-remote account default set":
		return "accounts.default.set", "write", true
	case "agent-remote account default get":
		return "accounts.default.get", "read", true
	case "agent-remote account default clear":
		return "accounts.default.clear", "write", true
	case "agent-remote account recover-runtime":
		return "accounts.runtime_recovery.authority", "read", true
	case "agent-remote account recovery-status":
		return "accounts.runtime_recovery.status", "read", true
	case "agent-remote credentials list":
		return "credentials.list", "read", true
	case "agent-remote credentials create":
		return "credentials.create", "write", true
	case "agent-remote credentials bind":
		return "credentials.bind", "write", true
	case "agent-remote credentials unbind":
		return "credentials.unbind", "write", true
	case "agent-remote device revoke":
		return "device.revoke", "write", true
	case "agent-remote device rotate-token":
		return "device.rotate_token", "write", true
	case "agent-remote device status":
		return "device.status", "read", true
	case "agent-remote node install":
		return "nodes.list", "read", true
	case "agent-remote ego-browser status":
		return "ego_browser.status", "read", true
	case "agent-remote ego-browser list":
		return "ego_browser.candidates", "read", true
	case "agent-remote ego-browser connect", "agent-remote ego-browser claim":
		return "ego_browser.claim.control", "write", true
	case "agent-remote ego-browser requests":
		return "ego_browser.requests", "read", true
	case "agent-remote ego-browser cancel-request":
		return "ego_browser.request.cancel", "write", true
	case "agent-remote ego-browser pause", "agent-remote ego-browser resume", "agent-remote ego-browser stop", "agent-remote ego-browser revoke":
		return "ego_browser.binding.control", "write", true
	case "agent-remote ego-browser delete-device", "agent-remote ego-browser device-delete", "agent-remote ego-browser delete-binding", "agent-remote ego-browser binding-delete":
		return "ego_browser.delete.control", "write", true
	case "agent-remote skill list":
		return "skills.list", "read", true
	case "agent-remote skill info":
		return "skills.info", "read", true
	case "agent-remote skill status", "agent-remote skill check":
		return "skills.list", "read", true
	case "agent-remote skill state list":
		return "skills.state.list", "read", true
	case "agent-remote skill state conflicts":
		return "skills.state.conflicts", "read", true
	case "agent-remote skill state info":
		return "skills.state.info", "read", true
	case "agent-remote skill state diff":
		return "skills.state.diff", "read", true
	case "agent-remote skill add", "agent-remote skill update", "agent-remote skill enable", "agent-remote skill disable", "agent-remote skill pin", "agent-remote skill unpin", "agent-remote skill inherit", "agent-remote skill remove", "agent-remote skill rollback":
		return "skills.change", "write", true
	case "agent-remote skill retry":
		return "skills.retry.plan", "read", true
	case "agent-remote logout":
		return "logout", "write", true
	case "fclaude list":
		return "sessions.list", "read", true
	case "fclaude run":
		return "sessions.list", "read", true
	case "fclaude new":
		return "sessions.create", "write", true
	case "fclaude attach", "agent-remote attach":
		// Both attach spellings open the context-bound session inventory
		// before launching the exact sessions.attach terminal workflow. The
		// inventory permission is therefore part of the command-card gate;
		// the Dedicated workflow still checks the separate terminal method.
		return "sessions.list", "read", true
	// The CLI lifecycle and forwarding commands are syntax-level aliases for
	// the structured remote workflows. Keep their capability gate tied to the
	// same operation used by the dedicated Remote and Network pages so the
	// command catalog cannot suggest that a generic argv form is a complete
	// implementation.
	case "fclaude stop":
		return "sessions.stop", "write", true
	case "fclaude stop-status":
		// New CLIs advertise sessions.stop.request; the remote adapter also
		// accepts the explicit sessions.stop.status spelling for compatibility.
		return "sessions.stop.request", "read", true
	case "fclaude delete":
		return "sessions.delete", "write", true
	case "fclaude delete-status":
		return "sessions.delete.request", "read", true
	case "agent-remote forward", "fclaude forward":
		return "port_forwards.create", "write", true
	case "agent-remote forward list", "fclaude forward list":
		return "port_forwards.list", "read", true
	case "agent-remote forward stop", "fclaude forward stop":
		return "port_forwards.stop", "write", true
	default:
		return "", "", false
	}
}

// CapabilityState applies a known capability gate without turning a missing
// mapping into a false authorization claim. The protocol error is supplied by
// the caller because discovery may be unavailable while a schema remains
// cached on screen.
func CapabilityState(executable string, path []string, capabilities Capabilities, protocolError error) string {
	method, effect, ok := KnownCapability(executable, path)
	if !ok {
		return "unknown"
	}
	if protocolError != nil || capabilities.Validate() != nil {
		return "unavailable"
	}
	if !capabilities.MatchesHostPlatform() {
		return "platform_unsupported"
	}
	if !capabilities.SupportsEffect(method, effect) {
		return "unsupported"
	}
	return "available"
}
