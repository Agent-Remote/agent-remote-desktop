package cliadapter_test

import (
	"errors"
	"reflect"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

func TestBuildArgvFreezesSchemaOrderAndRepeatsValues(t *testing.T) {
	pos := 1
	command := cliadapter.Command{
		Path: []string{"skill", "add"},
		Arguments: []cliadapter.Argument{
			{ID: "force", Long: "force", Action: "SetTrue", MinValues: 0, MaxValues: intPtr(0)},
			{ID: "name", Position: &pos, Required: true, MinValues: 1, MaxValues: intPtr(1)},
			{ID: "tag", Long: "tag", Action: "Append", MinValues: 0, MaxValues: intPtr(1)},
		},
	}
	values := cliadapter.FormValues{"force": {"true"}, "name": {"demo"}, "tag": {"one", "two"}}
	argv, err := cliadapter.BuildArgv(command, values)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"skill", "add", "--force", "--tag", "one", "--tag", "two", "demo"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %#v, want %#v", argv, want)
	}
	values["name"][0] = "changed-after-submit"
	if argv[len(argv)-1] != "demo" {
		t.Fatalf("argv was not frozen: %#v", argv)
	}
}

func TestBuildArgvGroupsMultiValueSetOptions(t *testing.T) {
	command := cliadapter.Command{Path: []string{"run"}, Arguments: []cliadapter.Argument{{ID: "pair", Long: "pair", Action: "Set", MinValues: 2, MaxValues: intPtr(2)}}}
	argv, err := cliadapter.BuildArgv(command, cliadapter.FormValues{"pair": {"left", "right"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"run", "--pair", "left", "right"}
	if !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %#v, want %#v", argv, want)
	}
}

func TestBuildArgvValidatesRequiredChoicesAndUnknownKeys(t *testing.T) {
	command := cliadapter.Command{Path: []string{"run"}, Arguments: []cliadapter.Argument{{ID: "mode", Long: "mode", Required: true, MinValues: 1, MaxValues: intPtr(1), Choices: []string{"safe", "fast"}}}}
	for name, test := range map[string]struct {
		values cliadapter.FormValues
		target error
	}{
		"missing": {values: cliadapter.FormValues{}, target: cliadapter.ErrFormArgumentMissing},
		"choice":  {values: cliadapter.FormValues{"mode": {"unsafe"}}, target: cliadapter.ErrFormArgumentChoice},
		"unknown": {values: cliadapter.FormValues{"other": {"x"}}, target: cliadapter.ErrFormArgumentUnknown},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := cliadapter.BuildArgv(command, test.values)
			if !errors.Is(err, test.target) {
				t.Fatalf("error = %v, want %v", err, test.target)
			}
		})
	}
}

func TestBuildArgvDropsClearedOptionalValuesAndRejectsBlankMinimum(t *testing.T) {
	command := cliadapter.Command{Path: []string{"run"}, Arguments: []cliadapter.Argument{
		{ID: "optional", Long: "optional", Action: "Set", MinValues: 0, MaxValues: intPtr(1)},
		{ID: "required", Long: "required", Action: "Set", MinValues: 1, MaxValues: intPtr(1)},
	}}
	argv, err := cliadapter.BuildArgv(command, cliadapter.FormValues{"optional": {""}, "required": {"value"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"run", "--required", "value"}; !reflect.DeepEqual(argv, want) {
		t.Fatalf("argv = %#v, want %#v", argv, want)
	}
	if _, err := cliadapter.BuildArgv(command, cliadapter.FormValues{"required": {""}}); !errors.Is(err, cliadapter.ErrFormArgumentMissing) {
		t.Fatalf("blank required value error = %v, want missing", err)
	}
}

func TestBuildArgvRejectsNULBeforeReview(t *testing.T) {
	command := cliadapter.Command{Path: []string{"status"}, Arguments: []cliadapter.Argument{
		{ID: "filter", Long: "filter", MinValues: 1, MaxValues: intPtr(1)},
	}}
	argv, err := cliadapter.BuildArgv(command, cliadapter.FormValues{"filter": {"safe\x00value"}})
	if !errors.Is(err, cliadapter.ErrFormArgumentValue) {
		t.Fatalf("NUL value error = %v, want ErrFormArgumentValue", err)
	}
	if argv != nil {
		t.Fatalf("NUL value produced argv: %#v", argv)
	}
}

func TestBuildArgvInvertsSetFalseFlags(t *testing.T) {
	command := cliadapter.Command{Path: []string{"logout"}, Arguments: []cliadapter.Argument{{ID: "revoke", Long: "no-revoke-remote", Action: "SetFalse", Defaults: []string{"true"}, MinValues: 0, MaxValues: intPtr(0)}}}
	for name, values := range map[string]cliadapter.FormValues{
		"default true omits flag": {"revoke": {"true"}},
		"false emits flag":        {"revoke": {"false"}},
	} {
		t.Run(name, func(t *testing.T) {
			argv, err := cliadapter.BuildArgv(command, values)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"logout"}
			if name == "false emits flag" {
				want = append(want, "--no-revoke-remote")
			}
			if !reflect.DeepEqual(argv, want) {
				t.Fatalf("argv = %#v, want %#v", argv, want)
			}
		})
	}
}

func TestBuildArgvRejectsDesktopHomeOverride(t *testing.T) {
	command := cliadapter.Command{Path: []string{"status"}, Arguments: []cliadapter.Argument{
		{ID: "home", Long: "home", Global: true, Action: "Set", MinValues: 1, MaxValues: intPtr(1)},
		{ID: "json", Long: "json", Action: "SetTrue", MinValues: 0, MaxValues: intPtr(0)},
	}}
	if !cliadapter.IsDesktopManagedArgument(command.Arguments[0]) {
		t.Fatal("global home was not classified as desktop-managed")
	}
	if argv, err := cliadapter.BuildArgv(command, cliadapter.FormValues{"home": {"/tmp/other-home"}}); !errors.Is(err, cliadapter.ErrFormArgumentManaged) || argv != nil {
		t.Fatalf("home override result = %#v, %v; want managed-argument rejection", argv, err)
	}
	argv, err := cliadapter.BuildArgv(command, cliadapter.FormValues{"home": {""}, "json": {"true"}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(argv, []string{"status", "--json"}) {
		t.Fatalf("empty managed home argv = %#v", argv)
	}
}

func TestInitialFormValuesRespectFalseFlagDefault(t *testing.T) {
	command := cliadapter.Command{Arguments: []cliadapter.Argument{{ID: "keep", Long: "no-keep", Action: "SetFalse", MinValues: 0, MaxValues: intPtr(0)}}}
	values := cliadapter.InitialFormValues(command)
	if !reflect.DeepEqual(values, cliadapter.FormValues{"keep": {"true"}}) {
		t.Fatalf("defaults = %#v", values)
	}
}

func TestGenericCommandGatesPlatformAndKnownCapability(t *testing.T) {
	command := cliadapter.Command{Path: []string{"wireguard", "up"}, Platforms: []string{"linux", "macos", "windows"}}
	if got := cliadapter.PlatformState(command, "freebsd"); got != "unsupported" {
		t.Fatalf("freebsd state = %q", got)
	}
	if got := cliadapter.PlatformState(cliadapter.Command{Path: []string{"status"}}, "linux"); got != "undeclared" {
		t.Fatalf("undeclared state = %q", got)
	}
	capabilities := cliadapter.Capabilities{Version: 1, CLIVersion: "x", Platform: cliadapter.PlatformName(runtime.GOOS), Operations: []cliadapter.Capability{{Method: "wireguard.status", Effect: "read"}}}
	if got := cliadapter.CapabilityState("agent-remote", []string{"wireguard", "status"}, capabilities, nil); got != "available" {
		t.Fatalf("capability state = %q", got)
	}
	if got := cliadapter.CapabilityState("agent-remote", []string{"wireguard", "status"}, capabilities, errors.New("discovery failed")); got != "unavailable" {
		t.Fatalf("error capability state = %q", got)
	}
	if got := cliadapter.CapabilityState("fclaude", []string{"new"}, capabilities, nil); got != "unsupported" {
		t.Fatalf("fclaude new capability state = %q", got)
	}
	method, effect, ok := cliadapter.KnownCapability("agent-remote", []string{"credentials", "list"})
	if !ok || method != "credentials.list" || effect != "read" {
		t.Fatalf("credentials list mapping = (%q, %q, %v)", method, effect, ok)
	}
	for _, test := range []struct {
		path   []string
		method string
		effect string
	}{
		{[]string{"deps", "mutagen-environment"}, "mutagen.environment.preview", "read"},
		{[]string{"sync", "ensure"}, "sync.ensure", "write"},
		{[]string{"sync", "status"}, "sync.status", "read"},
		{[]string{"sync", "pause"}, "sync.pause", "write"},
		{[]string{"sync", "resume"}, "sync.resume", "write"},
		{[]string{"sync", "resolve"}, "sync.resolve", "write"},
		{[]string{"sync", "reset"}, "sync.reset", "write"},
		{[]string{"wireguard", "config"}, "wireguard.config.plan", "read"},
		{[]string{"wireguard", "up"}, "wireguard.tunnel.apply", "write"},
		{[]string{"account", "create"}, "accounts.create", "write"},
		{[]string{"account", "import-config"}, "accounts.import.prepare", "read"},
		{[]string{"account", "default", "clear"}, "accounts.default.clear", "write"},
		{[]string{"account", "recover-runtime"}, "accounts.runtime_recovery.authority", "read"},
		{[]string{"account", "recovery-status"}, "accounts.runtime_recovery.status", "read"},
		{[]string{"credentials", "bind"}, "credentials.bind", "write"},
		{[]string{"device", "status"}, "device.status", "read"},
		{[]string{"device", "rotate-token"}, "device.rotate_token", "write"},
		{[]string{"node", "install"}, "nodes.list", "read"},
		{[]string{"attach"}, "sessions.list", "read"},
		{[]string{"skill", "list"}, "skills.list", "read"},
		{[]string{"skill", "info"}, "skills.info", "read"},
		{[]string{"skill", "state", "list"}, "skills.state.list", "read"},
		{[]string{"skill", "state", "conflicts"}, "skills.state.conflicts", "read"},
		{[]string{"skill", "state", "info"}, "skills.state.info", "read"},
		{[]string{"skill", "state", "diff"}, "skills.state.diff", "read"},
		{[]string{"skill", "add"}, "skills.change", "write"},
		{[]string{"skill", "retry"}, "skills.retry.plan", "read"},
		{[]string{"ego-browser", "list"}, "ego_browser.candidates", "read"},
		{[]string{"ego-browser", "connect"}, "ego_browser.claim.control", "write"},
		{[]string{"ego-browser", "requests"}, "ego_browser.requests", "read"},
		{[]string{"ego-browser", "cancel-request"}, "ego_browser.request.cancel", "write"},
		{[]string{"ego-browser", "claim"}, "ego_browser.claim.control", "write"},
		{[]string{"ego-browser", "pause"}, "ego_browser.binding.control", "write"},
		{[]string{"ego-browser", "delete-device"}, "ego_browser.delete.control", "write"},
		{[]string{"logout"}, "logout", "write"},
	} {
		method, effect, ok := cliadapter.KnownCapability("agent-remote", test.path)
		if !ok || method != test.method || effect != test.effect {
			t.Errorf("%v mapping = (%q, %q, %v), want (%q, %q, true)", test.path, method, effect, ok, test.method, test.effect)
		}
	}
}

func TestFClaudeLifecycleCommandsUseDedicatedRemoteCapabilities(t *testing.T) {
	tests := []struct {
		path   []string
		method string
		effect string
	}{
		{[]string{}, "sessions.list", "read"},
		{[]string{"run"}, "sessions.list", "read"},
		{[]string{"new"}, "sessions.create", "write"},
		{[]string{"list"}, "sessions.list", "read"},
		{[]string{"attach"}, "sessions.list", "read"},
		{[]string{"stop"}, "sessions.stop", "write"},
		{[]string{"stop-status"}, "sessions.stop.request", "read"},
		{[]string{"delete"}, "sessions.delete", "write"},
		{[]string{"delete-status"}, "sessions.delete.request", "read"},
		{[]string{"forward"}, "port_forwards.create", "write"},
		{[]string{"forward", "list"}, "port_forwards.list", "read"},
		{[]string{"forward", "stop"}, "port_forwards.stop", "write"},
	}
	for _, test := range tests {
		method, effect, ok := cliadapter.KnownCapability("fclaude", test.path)
		if !ok || method != test.method || effect != test.effect {
			t.Errorf("fclaude %v mapping = (%q, %q, %v), want (%q, %q, true)", test.path, method, effect, ok, test.method, test.effect)
		}
	}
}

func TestAgentRemoteForwardCommandsUseSharedPortForwardCapabilities(t *testing.T) {
	tests := []struct {
		path   []string
		method string
		effect string
	}{
		{[]string{"forward"}, "port_forwards.create", "write"},
		{[]string{"forward", "list"}, "port_forwards.list", "read"},
		{[]string{"forward", "stop"}, "port_forwards.stop", "write"},
	}
	for _, test := range tests {
		method, effect, ok := cliadapter.KnownCapability("agent-remote", test.path)
		if !ok || method != test.method || effect != test.effect {
			t.Errorf("agent-remote %v mapping = (%q, %q, %v), want (%q, %q, true)", test.path, method, effect, ok, test.method, test.effect)
		}
	}
}

func TestSensitiveArgumentsRequireDedicatedWorkflow(t *testing.T) {
	if !cliadapter.IsSensitiveArgument(cliadapter.Argument{ID: "totp_code"}) {
		t.Fatal("totp code was not classified as sensitive")
	}
	if !cliadapter.IsSensitiveArgument(cliadapter.Argument{Long: "wireguard-private-key"}) {
		t.Fatal("private key was not classified as sensitive")
	}
	if cliadapter.IsSensitiveArgument(cliadapter.Argument{Long: "account-id"}) {
		t.Fatal("ordinary account id was classified as sensitive")
	}
	command := cliadapter.Command{Arguments: []cliadapter.Argument{{ID: "password"}}}
	if !cliadapter.CommandHasSensitiveArguments(command) {
		t.Fatal("sensitive command was not classified")
	}
	for _, action := range []string{"create", "bind", "unbind"} {
		if !cliadapter.CommandHasSensitiveArguments(cliadapter.Command{Path: []string{"credentials", action}}) {
			t.Fatalf("credentials %s was exposed to generic argv workflow", action)
		}
	}
}

func intPtr(value int) *int { return &value }
