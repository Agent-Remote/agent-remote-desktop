package cliadapter_test

import (
	"errors"
	"reflect"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func terminalCaps(methods ...string) cliadapter.Capabilities {
	items := make([]cliadapter.TerminalCapability, 0, len(methods))
	for _, method := range methods {
		items = append(items, cliadapter.TerminalCapability{Method: method, Version: 1})
	}
	return cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: runtime.GOOS, TerminalOperations: items,
		Operations: []cliadapter.Capability{{Method: "capabilities", Effect: "read"}}}
}

func TestDedicatedWorkflowCoversNodeAndEgoBrowserMutations(t *testing.T) {
	if _, ok := cliadapter.Dedicated("ego-browser", []string{"ego-browser"}); ok {
		t.Fatal("ego-browser command group must not be treated as a leaf workflow")
	}
	for _, path := range [][]string{{"ssh"}, {"node"}, {"skill", "state"}, {"skill"}} {
		if _, ok := cliadapter.Dedicated("agent-remote", path); ok {
			t.Fatalf("incomplete command group %v was treated as a leaf workflow", path)
		}
	}
	for _, test := range []struct {
		path  []string
		argv  []string
		write bool
	}{
		{path: []string{"node", "install"}, argv: []string{"node", "install", "--node", "node-id", "--yes"}, write: true},
		{path: []string{"ego-browser", "connect"}, argv: []string{"ego-browser", "connect", "session-id", "--yes"}, write: true},
		{path: []string{"ego-browser", "status"}, argv: []string{"ego-browser", "status"}, write: false},
	} {
		workflow, ok := cliadapter.Dedicated("agent-remote", test.path)
		if !ok {
			t.Fatalf("workflow %v was not discovered", test.path)
		}
		if got := workflow.Effect == "write"; got != test.write {
			t.Fatalf("workflow %v effect write=%v, want %v", test.path, got, test.write)
		}
		if err := cliadapter.ValidateDedicatedArgv(workflow, test.argv, true); err != nil {
			t.Fatalf("workflow %v argv rejected: %v", test.path, err)
		}
		if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), nil); got == "cli_unsupported" {
			t.Fatalf("workflow %v was unsupported despite terminal declaration", test.path)
		}
	}
	for _, path := range [][]string{
		{"ego-browser", "setup"}, {"ego-browser", "status"},
		{"ego-browser", "list"}, {"ego-browser", "requests"},
		{"ego-browser", "pause"}, {"ego-browser", "delete-binding"},
	} {
		workflow, ok := cliadapter.Dedicated("agent-remote", path)
		if !ok {
			t.Fatalf("workflow %v was not discovered", path)
		}
		if !reflect.DeepEqual(workflow.Platforms, []string{"macos"}) {
			t.Fatalf("workflow %v platforms = %#v, want macos", path, workflow.Platforms)
		}
		foreign := workflow
		if runtime.GOOS == "windows" {
			foreign.Platforms = []string{"linux"}
		} else {
			foreign.Platforms = []string{"windows"}
		}
		if got := cliadapter.DedicatedCapabilityState(foreign, terminalCaps(workflow.Method), nil); got != "platform_unsupported" {
			t.Fatalf("workflow %v foreign platform state = %q", path, got)
		}
	}
}

func TestEgoBrowserRegisterUsesReviewWithoutSyntheticYes(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"ego-browser", "register"})
	if !ok {
		t.Fatal("ego-browser register workflow was not discovered")
	}
	if workflow.ForceYes || !workflow.RequiresConfirmation || workflow.Method != "agent-remote ego-browser register" {
		t.Fatalf("unexpected register workflow: %+v", workflow)
	}
	argv := []string{"ego-browser", "register", "--server-url", "https://example.invalid", "--signer-certificate-sha256", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"}
	if err := cliadapter.ValidateDedicatedArgv(workflow, argv, true); err != nil {
		t.Fatalf("reviewed register argv rejected: %v", err)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, argv, false); !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnconfirmed) {
		t.Fatalf("unconfirmed register argv error = %v", err)
	}
}

func TestDedicatedWorkflowCoversReadOnlySSHCheck(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"ssh", "check"})
	if !ok {
		t.Fatal("ssh check workflow was not discovered")
	}
	if workflow.Method != "agent-remote ssh check" || workflow.Effect != "read" || workflow.Credential != "none" || workflow.RequiresConfirmation || workflow.ForceYes {
		t.Fatalf("unexpected ssh check workflow: %+v", workflow)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"ssh", "check", "--session-id", "11111111-1111-4111-8111-111111111111"}, false); err != nil {
		t.Fatalf("ssh check argv rejected: %v", err)
	}
	if _, ok := cliadapter.Dedicated("agent-remote", []string{"ssh", "check", "extra"}); ok {
		t.Fatal("non-canonical ssh check path was accepted")
	}
}

func TestDedicatedWorkflowCoversAccountBindingAttach(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"account", "attach"})
	if !ok {
		t.Fatal("account attach workflow was not discovered")
	}
	if workflow.Method != "accounts.binding.attach" || workflow.Effect != "write" || workflow.Credential != "device" || workflow.Confirmation != "exact_request" || !workflow.RequiresConfirmation || workflow.ForceYes || !reflect.DeepEqual(workflow.RequiredFlags, []string{"--binding-session", "--expected-server", "--expected-device"}) {
		t.Fatalf("unexpected account attach workflow: %+v", workflow)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"account", "attach", "account-id", "--binding-session", "binding-id", "--expected-server", "https://example.invalid", "--expected-device", "device-id"}, true); err != nil {
		t.Fatalf("reviewed account attach argv rejected: %v", err)
	}
	for _, invalid := range [][]string{
		{"account", "attach", "account-id", "--binding-session", "binding-id"},
		{"account", "attach", "account-id", "--binding-session", "binding-id", "--expected-server", "https://example.invalid", "--expected-device"},
	} {
		if err := cliadapter.ValidateDedicatedArgv(workflow, invalid, true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowArgvMismatch) {
			t.Fatalf("invalid account attach argv %v error = %v", invalid, err)
		}
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), nil); got != "unverified" {
		t.Fatalf("account attach capability state = %q, want unverified", got)
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(), nil); got != "cli_unsupported" {
		t.Fatalf("account attach without terminal declaration = %q, want cli_unsupported", got)
	}
	if _, ok := cliadapter.Dedicated("agent-remote", []string{"account", "attach", "extra"}); ok {
		t.Fatal("non-canonical account attach path was accepted")
	}
}

func TestDedicatedWorkflowCoversLegacyAgentRemoteAttachCapability(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"attach"})
	if !ok {
		t.Fatal("legacy agent-remote attach workflow was not discovered")
	}
	if workflow.Method != "sessions.attach" || workflow.Effect != "write" || workflow.Credential != "device" || workflow.Confirmation != "exact_request" || !workflow.RequiresConfirmation {
		t.Fatalf("unexpected legacy attach workflow: %+v", workflow)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"attach", "session-id", "--print-only"}, true); err != nil {
		t.Fatalf("legacy attach argv rejected: %v", err)
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), nil); got != "unverified" {
		t.Fatalf("legacy attach capability state = %q, want unverified", got)
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(), nil); got != "cli_unsupported" {
		t.Fatalf("legacy attach without terminal declaration = %q, want cli_unsupported", got)
	}
}

func TestDedicatedWorkflowCoversFClaudeRunNewAndList(t *testing.T) {
	for _, test := range []struct {
		path                 []string
		argv                 []string
		effect               string
		requiresConfirmation bool
	}{
		{path: []string{"run"}, argv: []string{"run", "--model", "sonnet"}, effect: "write", requiresConfirmation: true},
		{path: []string{"new"}, argv: []string{"new", "--", "--model", "opus"}, effect: "write", requiresConfirmation: true},
		{path: []string{"list"}, argv: []string{"list", "--running", "--no-trunc"}, effect: "read", requiresConfirmation: false},
	} {
		t.Run(test.path[0], func(t *testing.T) {
			workflow, ok := cliadapter.Dedicated("fclaude", test.path)
			if !ok {
				t.Fatalf("workflow %v was not discovered", test.path)
			}
			if workflow.Method != "fclaude "+test.path[0] || workflow.Effect != test.effect || workflow.Credential != "stored" || workflow.RequiresConfirmation != test.requiresConfirmation || workflow.ForceYes {
				t.Fatalf("unexpected workflow: %+v", workflow)
			}
			if err := cliadapter.ValidateDedicatedArgv(workflow, test.argv, test.requiresConfirmation); err != nil {
				t.Fatalf("reviewed argv rejected: %v", err)
			}
			if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), nil); test.requiresConfirmation && got != "unverified" || !test.requiresConfirmation && got != "ready" {
				t.Fatalf("capability state = %q", got)
			}
		})
	}
	if _, ok := cliadapter.Dedicated("fclaude", []string{"run", "extra"}); ok {
		t.Fatal("non-canonical fclaude path was accepted")
	}
}

func TestDedicatedWorkflowCoversFClaudeDefaultInvocation(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("fclaude", nil)
	if !ok {
		t.Fatal("fclaude default invocation was not discovered")
	}
	if workflow.Method != "fclaude run" || workflow.Effect != "write" || workflow.Credential != "stored" || !workflow.RequiresConfirmation {
		t.Fatalf("unexpected default workflow: %+v", workflow)
	}
	for _, argv := range [][]string{{}, {"--account-id", "account-1", "--", "--model", "sonnet"}} {
		if err := cliadapter.ValidateDedicatedArgv(workflow, argv, true); err != nil {
			t.Fatalf("default argv %v rejected: %v", argv, err)
		}
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, nil, false); !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnconfirmed) {
		t.Fatalf("unconfirmed default argv error = %v", err)
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), nil); got != "unverified" {
		t.Fatalf("default capability state = %q, want unverified", got)
	}
}

func TestDedicatedWorkflowCoversFClaudeExactAttach(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("fclaude", []string{"attach"})
	if !ok {
		t.Fatal("fclaude attach workflow was not discovered")
	}
	if workflow.Method != "sessions.attach" || workflow.Effect != "write" || workflow.Credential != "device" || !workflow.RequiresConfirmation || !reflect.DeepEqual(workflow.RequiredFlags, []string{"--expected-server", "--expected-device"}) {
		t.Fatalf("unexpected exact attach workflow: %+v", workflow)
	}
	argv := []string{"attach", "11111111-1111-4111-8111-111111111111", "--expected-server", "https://example.invalid", "--expected-device", "22222222-2222-4222-8222-222222222222"}
	if err := cliadapter.ValidateDedicatedArgv(workflow, argv, true); err != nil {
		t.Fatalf("exact attach argv rejected: %v", err)
	}
	for _, invalid := range [][]string{
		{"attach", "11111111-1111-4111-8111-111111111111"},
		{"attach", "11111111-1111-4111-8111-111111111111", "--expected-server", "https://example.invalid"},
		{"attach", "11111111-1111-4111-8111-111111111111", "--expected-server", "https://example.invalid", "--expected-device"},
		{"attach", "11111111-1111-4111-8111-111111111111", "--expected-server", "-bad", "--expected-device", "22222222-2222-4222-8222-222222222222"},
	} {
		if err := cliadapter.ValidateDedicatedArgv(workflow, invalid, true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowArgvMismatch) {
			t.Fatalf("invalid exact attach argv %v error = %v", invalid, err)
		}
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, argv, false); !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnconfirmed) {
		t.Fatalf("unconfirmed exact attach error = %v", err)
	}
}

func TestDedicatedWorkflowCoversDoctorDualMode(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"doctor"})
	if !ok {
		t.Fatal("doctor workflow was not discovered")
	}
	if workflow.Method != "agent-remote doctor" || workflow.Effect != "write" || workflow.Credential != "none" || !workflow.RequiresConfirmation || workflow.ForceYes {
		t.Fatalf("unexpected doctor workflow: %+v", workflow)
	}
	for _, argv := range [][]string{{"doctor"}, {"doctor", "--fix"}} {
		if err := cliadapter.ValidateDedicatedArgv(workflow, argv, true); err != nil {
			t.Fatalf("reviewed doctor argv %v rejected: %v", argv, err)
		}
		if err := cliadapter.ValidateDedicatedArgv(workflow, argv, false); !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnconfirmed) {
			t.Fatalf("unconfirmed doctor argv %v error = %v", argv, err)
		}
	}
	if _, ok := cliadapter.Dedicated("agent-remote", []string{"doctor", "extra"}); ok {
		t.Fatal("non-canonical doctor path was accepted")
	}
}

func TestDedicatedWorkflowCoversMacOSDeviceCommands(t *testing.T) {
	tests := []struct {
		path           []string
		argv           []string
		effect         string
		credential     string
		confirmed      bool
		forceYes       bool
		requiresReview bool
	}{
		{path: []string{"device", "install"}, argv: []string{"device", "install", "--source", "/tmp/Agent Remote Device.zip"}, effect: "write", credential: "none", confirmed: true, requiresReview: true},
		{path: []string{"device", "uninstall"}, argv: []string{"device", "uninstall", "--yes"}, effect: "write", credential: "none", confirmed: true, forceYes: true, requiresReview: true},
		{path: []string{"device", "status"}, argv: []string{"device", "status"}, effect: "read", credential: "none"},
		{path: []string{"device", "launch"}, argv: []string{"device", "launch"}, effect: "write", credential: "none", confirmed: true, requiresReview: true},
		{path: []string{"device", "diagnose"}, argv: []string{"device", "diagnose"}, effect: "read", credential: "none"},
	}
	for _, test := range tests {
		t.Run(test.path[1], func(t *testing.T) {
			workflow, ok := cliadapter.Dedicated("agent-remote", test.path)
			if !ok {
				t.Fatalf("workflow %v was not discovered", test.path)
			}
			if workflow.Effect != test.effect || workflow.Credential != test.credential || !reflect.DeepEqual(workflow.Platforms, []string{"macos"}) || workflow.ForceYes != test.forceYes || workflow.RequiresConfirmation != test.requiresReview {
				t.Fatalf("unexpected workflow: %+v", workflow)
			}
			if err := cliadapter.ValidateDedicatedArgv(workflow, test.argv, test.confirmed); err != nil {
				t.Fatalf("reviewed argv rejected: %v", err)
			}
			foreign := workflow
			if runtime.GOOS == "windows" {
				foreign.Platforms = []string{"linux"}
			} else {
				foreign.Platforms = []string{"windows"}
			}
			if got := cliadapter.DedicatedCapabilityState(foreign, terminalCaps(workflow.Method), nil); got != "platform_unsupported" {
				t.Fatalf("foreign platform state = %q", got)
			}
			if test.forceYes {
				if err := cliadapter.ValidateDedicatedArgv(workflow, test.argv[:2], true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowMissingYes) {
					t.Fatalf("missing --yes error = %v", err)
				}
			}
		})
	}
	if _, ok := cliadapter.Dedicated("agent-remote", []string{"device", "install", "extra"}); ok {
		t.Fatal("non-canonical device path was accepted")
	}
}

func TestDedicatedWorkflowCoversSkillStateMutations(t *testing.T) {
	for _, action := range []string{"prune", "reset", "migrate", "resolve", "restore"} {
		t.Run(action, func(t *testing.T) {
			path := []string{"skill", "state", action}
			workflow, ok := cliadapter.Dedicated("agent-remote", path)
			if !ok {
				t.Fatalf("missing dedicated workflow for %v", path)
			}
			if workflow.Method != "agent-remote skill state "+action || !workflow.ForceYes || !workflow.RequiresConfirmation {
				t.Fatalf("unexpected workflow: %+v", workflow)
			}
			values, ok := cliadapter.InitialDedicatedFormValues("agent-remote", cliadapter.Command{
				Path: path,
				Arguments: []cliadapter.Argument{{
					ID: "yes", Long: "yes", Action: "SetTrue", Defaults: []string{"false"}, MaxValues: intPtr(0),
				}},
			})
			if !ok || !reflect.DeepEqual(values, cliadapter.FormValues{"yes": {"true"}}) {
				t.Fatalf("dedicated form did not freeze --yes: %#v (ok=%v)", values, ok)
			}
			if err := cliadapter.ValidateDedicatedArgv(workflow, append(path, "--yes"), true); err != nil {
				t.Fatalf("reviewed argv rejected: %v", err)
			}
			if err := cliadapter.ValidateDedicatedArgv(workflow, path, true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowMissingYes) {
				t.Fatalf("missing --yes error = %v", err)
			}
		})
	}
	if _, ok := cliadapter.Dedicated("agent-remote", []string{"skill", "state", "reset", "unexpected"}); ok {
		t.Fatal("non-canonical skill state path was accepted")
	}
}

func TestTerminalCapabilityWorkflowMetadataBindsSkillStateReview(t *testing.T) {
	base := terminalCaps("agent-remote skill state resolve")
	base.TerminalOperations[0].Workflow = "skill_state_resolve"
	base.TerminalOperations[0].Effect = "write"
	base.TerminalOperations[0].Credential = "stored"
	base.TerminalOperations[0].Confirmation = "exact_request"
	if err := base.Validate(); err != nil {
		t.Fatalf("valid workflow metadata rejected: %v", err)
	}
	if !base.SupportsTerminalWorkflow("agent-remote skill state resolve", "skill_state_resolve") {
		t.Fatal("matching workflow metadata was not accepted")
	}
	if base.SupportsTerminalWorkflow("agent-remote skill state resolve", "skill_state_migrate") {
		t.Fatal("wrong workflow metadata was accepted")
	}
	base.TerminalOperations[0].Workflow = "SkillStateResolve"
	if base.Validate() == nil {
		t.Fatal("invalid workflow label was accepted")
	}
	base.TerminalOperations[0].Workflow = "skill_state_resolve"
	base.TerminalOperations[0].Credential = "token"
	if base.Validate() == nil {
		t.Fatal("invalid terminal credential was accepted")
	}
}

func TestDedicatedWorkflowCoversSkillLibraryMutations(t *testing.T) {
	for _, action := range []string{"add", "update", "enable", "disable", "pin", "unpin", "inherit", "remove", "rollback"} {
		t.Run(action, func(t *testing.T) {
			path := []string{"skill", action}
			workflow, ok := cliadapter.Dedicated("agent-remote", path)
			if !ok {
				t.Fatalf("missing dedicated workflow for %v", path)
			}
			if workflow.Method != "agent-remote skill "+action || workflow.Credential != "user" || !workflow.ForceYes || !workflow.RequiresConfirmation {
				t.Fatalf("unexpected workflow: %+v", workflow)
			}
			values, ok := cliadapter.InitialDedicatedFormValues("agent-remote", cliadapter.Command{
				Path: path,
				Arguments: []cliadapter.Argument{{
					ID: "yes", Long: "yes", Action: "SetTrue", Defaults: []string{"false"}, MaxValues: intPtr(0),
				}},
			})
			if !ok || !reflect.DeepEqual(values, cliadapter.FormValues{"yes": {"true"}}) {
				t.Fatalf("dedicated form did not freeze --yes: %#v (ok=%v)", values, ok)
			}
			if err := cliadapter.ValidateDedicatedArgv(workflow, append(path, "--yes"), true); err != nil {
				t.Fatalf("reviewed argv rejected: %v", err)
			}
			if err := cliadapter.ValidateDedicatedArgv(workflow, path, true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowMissingYes) {
				t.Fatalf("missing --yes error = %v", err)
			}
			withoutYes, ok := cliadapter.InitialDedicatedFormValues("agent-remote", cliadapter.Command{Path: path})
			if !ok {
				t.Fatal("dedicated workflow unexpectedly unavailable without schema arguments")
			}
			if _, found := withoutYes["yes"]; found {
				t.Fatalf("--yes was injected without a schema argument: %#v", withoutYes)
			}
		})
	}
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"skill", "add"})
	if !ok {
		t.Fatal("skill add workflow missing")
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"skill", "add", "--list"}, true); err != nil {
		t.Fatalf("read-only skill add discovery was rejected: %v", err)
	}
}

func TestDedicatedWorkflowCoversSkillQueriesAndRetry(t *testing.T) {
	for _, path := range [][]string{
		{"skill", "check"}, {"skill", "info"}, {"skill", "status"}, {"skill", "list"},
		{"skill", "state", "list"}, {"skill", "state", "conflicts"}, {"skill", "state", "info"},
		{"skill", "state", "diff"}, {"skill", "state", "export"},
	} {
		workflow, ok := cliadapter.Dedicated("agent-remote", path)
		if !ok {
			t.Fatalf("missing read-only workflow for %v", path)
		}
		if workflow.Effect != "read" || workflow.Credential != "user" || workflow.RequiresConfirmation || workflow.ForceYes {
			t.Fatalf("unexpected read-only workflow for %v: %+v", path, workflow)
		}
		if len(path) == 3 && path[2] == "export" {
			// Export has a required destination which is checked separately below.
			continue
		}
		if err := cliadapter.ValidateDedicatedArgv(workflow, path, false); err != nil {
			t.Fatalf("read-only argv rejected for %v: %v", path, err)
		}
	}
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"skill", "retry"})
	if !ok || workflow.Effect != "write" || workflow.Credential != "user" || !workflow.RequiresConfirmation || !workflow.ForceYes {
		t.Fatalf("unexpected retry workflow: %+v (ok=%v)", workflow, ok)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"skill", "retry", "11111111-1111-4111-8111-111111111111", "--yes"}, true); err != nil {
		t.Fatalf("retry argv rejected: %v", err)
	}
}

func TestSkillExportReviewRejectsUnsafeOutputPaths(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"skill", "state", "export"})
	if !ok {
		t.Fatal("skill state export workflow missing")
	}
	for _, test := range []struct {
		name string
		path string
	}{
		{name: "unix parent traversal", path: "./exports/../secrets"},
		{name: "windows parent traversal", path: `C:\exports\..\secrets`},
		{name: "empty", path: ""},
		{name: "nul", path: "./exports\x00checkpoint"},
	} {
		t.Run(test.name, func(t *testing.T) {
			argv := []string{"skill", "state", "export", "--output", test.path}
			err := cliadapter.ValidateDedicatedArgv(workflow, argv, false)
			if test.name == "nul" {
				if !errors.Is(err, cliadapter.ErrDedicatedWorkflowArgvMismatch) {
					t.Fatalf("NUL output path error = %v", err)
				}
				return
			}
			if !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnsafePath) {
				t.Fatalf("unsafe output path error = %v", err)
			}
		})
	}
	for _, path := range []string{"./exports/checkpoint", "/tmp/agent-remote-export", `C:\Users\rem\exports`} {
		argv := []string{"skill", "state", "export", "--output", path}
		if err := cliadapter.ValidateDedicatedArgv(workflow, argv, false); err != nil {
			t.Fatalf("safe output path %q rejected: %v", path, err)
		}
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"skill", "state", "export", "--output", "./one", "--output", "./two"}, false); !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnsafePath) {
		t.Fatalf("duplicate output flags were accepted: %v", err)
	}
}

func TestDedicatedWorkflowRequiresFrozenConfirmationAndNonInteractiveYes(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"ego-browser", "connect"})
	if !ok {
		t.Fatal("ego-browser connect workflow missing")
	}
	pos := 1
	values, ok := cliadapter.InitialDedicatedFormValues("agent-remote", cliadapter.Command{Path: []string{"ego-browser", "connect"}, Arguments: []cliadapter.Argument{
		{ID: "tool_session", Position: &pos, MinValues: 0, MaxValues: intPtr(1)},
		{ID: "yes", Long: "yes", Action: "SetTrue", MinValues: 0, MaxValues: intPtr(0)},
	}})
	if !ok || len(values["yes"]) != 1 || values["yes"][0] != "true" {
		t.Fatalf("dedicated form did not freeze --yes: %#v", values)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"ego-browser", "connect", "session-id", "--yes"}, false); !errors.Is(err, cliadapter.ErrDedicatedWorkflowUnconfirmed) {
		t.Fatalf("unconfirmed argv error = %v", err)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"ego-browser", "connect", "session-id"}, true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowMissingYes) {
		t.Fatalf("missing --yes error = %v", err)
	}
	if err := cliadapter.ValidateDedicatedArgv(workflow, []string{"ego-browser", "repair", "--yes"}, true); !errors.Is(err, cliadapter.ErrDedicatedWorkflowArgvMismatch) {
		t.Fatalf("mismatched argv error = %v", err)
	}
}

func TestDedicatedWorkflowHonorsPlatformAndCapabilityGates(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("agent-remote", []string{"ego-browser", "upgrade"})
	if !ok {
		t.Fatal("ego-browser upgrade workflow missing")
	}
	foreign := workflow
	if runtime.GOOS == "windows" {
		foreign.Platforms = []string{"linux"}
	} else {
		foreign.Platforms = []string{"windows"}
	}
	if got := cliadapter.DedicatedCapabilityState(foreign, terminalCaps(workflow.Method), nil); got != "platform_unsupported" {
		t.Fatalf("foreign platform state = %q", got)
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(), nil); got != "cli_unsupported" {
		t.Fatalf("missing terminal declaration state = %q", got)
	}
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), errors.New("discovery")); got != "unavailable" {
		t.Fatalf("protocol failure state = %q", got)
	}
}

func TestDedicatedCapabilityRejectsExplicitlyWeakerTerminalContract(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("fclaude", []string{"run"})
	if !ok {
		t.Fatal("fclaude run workflow missing")
	}
	cases := []struct {
		name   string
		mutate func(*cliadapter.TerminalCapability)
	}{
		{"read_effect", func(capability *cliadapter.TerminalCapability) { capability.Effect = "read" }},
		{"wrong_credential", func(capability *cliadapter.TerminalCapability) { capability.Credential = "none" }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			capabilities := terminalCaps(workflow.Method)
			capabilities.TerminalOperations[0].Effect = workflow.Effect
			capabilities.TerminalOperations[0].Credential = workflow.Credential
			capabilities.TerminalOperations[0].Confirmation = workflow.Confirmation
			test.mutate(&capabilities.TerminalOperations[0])
			if got := cliadapter.DedicatedCapabilityState(workflow, capabilities, nil); got != "cli_unsupported" {
				t.Fatalf("explicitly weaker contract state = %q, want cli_unsupported", got)
			}
		})
	}
	readWorkflow, ok := cliadapter.Dedicated("fclaude", []string{"list"})
	if !ok {
		t.Fatal("fclaude list workflow missing")
	}
	readCapabilities := terminalCaps(readWorkflow.Method)
	readCapabilities.TerminalOperations[0].Confirmation = "exact_request"
	if got := cliadapter.DedicatedCapabilityState(readWorkflow, readCapabilities, nil); got != "cli_unsupported" {
		t.Fatalf("read workflow with explicit confirmation state = %q, want cli_unsupported", got)
	}
}

func TestDedicatedCapabilityAcceptsLegacyOmittedTerminalMetadata(t *testing.T) {
	workflow, ok := cliadapter.Dedicated("fclaude", []string{"run"})
	if !ok {
		t.Fatal("fclaude run workflow missing")
	}
	// CLI v1 terminal declarations predate the optional metadata fields. An
	// omitted field remains compatible; an explicit weaker declaration is not.
	if got := cliadapter.DedicatedCapabilityState(workflow, terminalCaps(workflow.Method), nil); got != "unverified" {
		t.Fatalf("legacy metadata state = %q, want unverified", got)
	}
}
