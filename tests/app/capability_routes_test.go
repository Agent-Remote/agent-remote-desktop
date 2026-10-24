package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// Test the command catalog as a user would use it. A command that already has
// a context-aware desktop page must never fall through to the generic argv
// form; this also protects the async remote inventory hand-off.
func TestCapabilitiesRoutesCommandFamiliesToDedicatedPages(t *testing.T) {
	tests := []struct {
		name          string
		filter        string
		display       string
		want          string
		expectGeneric bool
	}{
		{name: "accounts", filter: "account list", want: locales.RemoteManagement()},
		{name: "account binding attach", filter: "account attach", want: locales.RemoteManagement()},
		{name: "legacy remote attach", filter: "attach", want: locales.RemoteManagement()},
		{name: "logout", filter: "logout", want: locales.SignOutRemote()},
		{name: "credentials", filter: "credentials list", want: locales.CredentialsTitle()},
		{name: "mutagen", filter: "deps mutagen-environment", want: locales.MutagenEnvironmentTitle()},
		{name: "local status", filter: "agent-remote status", display: "agent-remote status", want: locales.CommandDiagnosticsMethod("status.inspect")},
		{name: "doctor structured", filter: "agent-remote doctor", display: "agent-remote doctor", want: locales.CommandDiagnosticsMethod("doctor.inspect")},
		{name: "ssh check structured", filter: "agent-remote ssh check", display: "agent-remote ssh check", want: locales.CommandDiagnosticsMethod("ssh.inspect")},
		{name: "dependency status", filter: "deps status", want: locales.CommandDiagnosticsMethod("deps.inspect")},
		{name: "sync", filter: "sync status", want: locales.SyncTitle()},
		{name: "wireguard config", filter: "wireguard config", want: locales.WireGuardConfigTitle()},
		{name: "wireguard network", filter: "wireguard status", want: locales.WireGuardNetworkTitle()},
		{name: "device status", filter: "device status", want: locales.DeviceTitle()},
		{name: "device diagnose", filter: "device diagnose", want: locales.DeviceTitle()},
		{name: "ego browser status", filter: "ego-browser status", want: locales.EgoBrowserTitle()},
		{name: "ego browser list", filter: "ego-browser list", want: locales.EgoBrowserTitle()},
		{name: "ego browser requests", filter: "ego-browser requests", want: locales.EgoBrowserTitle()},
		{name: "ego browser cancel request", filter: "ego-browser cancel-request", want: locales.EgoBrowserTitle()},
		{name: "ego browser connect", filter: "ego-browser connect", want: locales.EgoBrowserTitle()},
		{name: "ego browser claim", filter: "ego-browser claim", want: locales.EgoBrowserTitle()},
		{name: "ego browser pause", filter: "ego-browser pause", want: locales.EgoBrowserTitle()},
		{name: "ego browser resume", filter: "ego-browser resume", want: locales.EgoBrowserTitle()},
		{name: "ego browser stop", filter: "ego-browser stop", want: locales.EgoBrowserTitle()},
		{name: "ego browser revoke", filter: "ego-browser revoke", want: locales.EgoBrowserTitle()},
		{name: "ego browser delete device", filter: "ego-browser delete-device", want: locales.EgoBrowserTitle()},
		{name: "ego browser delete binding", filter: "ego-browser delete-binding", want: locales.EgoBrowserTitle()},
		{name: "device", filter: "device rotate-token", want: locales.DeviceTitle()},
		{name: "skills", filter: "skill list", want: locales.SkillsTitle()},
		{name: "skill info", filter: "skill info", want: locales.SkillsTitle()},
		{name: "skill state info", filter: "skill state info", want: locales.SkillsTitle()},
		{name: "skill state diff", filter: "skill state diff", want: locales.SkillsTitle()},
		{name: "skill add reviewed", filter: "skill add", want: locales.CliFormTitle(), expectGeneric: true},
		{name: "skill state export reviewed", filter: "skill state export", want: locales.CliFormTitle(), expectGeneric: true},
		{name: "runtime recovery", filter: "account recovery-status", want: locales.RuntimeRecoveryTitle()},
		{name: "account default", filter: "account default get", want: locales.AccountDefaultTitle()},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			binary := testutil.InstallControlHelper(t)
			i18n.SetLocaleFunc(func() string { return "en" })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
				if name == "agent-remote" {
					return binary, nil
				}
				return "", errors.New("test component not installed")
			}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = a.Close() })
			tt := ui.NewTester(a.View, 1100, 820)
			tt.SetLocale("en")
			if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
			if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
				t.Fatal(err)
			}
			tt.Type(tc.filter)
			// Text input is delivered before the next render on slower CI workers.
			// Advance once before inspecting the filtered command card.
			tt.Frame()
			display := tc.display
			if display == "" {
				display = "agent-remote " + tc.filter
			}
			if !tt.HasText(display) {
				t.Fatalf("command %q was not discoverable", tc.filter)
			}
			if err := tt.Click(locales.CliFormOpen()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, tc.want)
			if !tc.expectGeneric && tt.HasText(locales.CliFormTitle()) {
				t.Fatalf("command %q fell through to generic argv form", tc.filter)
			}
			if tc.filter == "skill state export" && !tt.HasText("--output · "+locales.CliFormRequired()) {
				t.Fatal("skill state export review did not expose its required output destination")
			}
		})
	}
}

func TestCapabilitiesRoutesFclaudeAttachToRemoteSessionInventory(t *testing.T) {
	agentBinary := testutil.InstallControlHelper(t)
	fclaudeBinary := filepath.Join(t.TempDir(), "fclaude")
	if err := os.Symlink(agentBinary, fclaudeBinary); err != nil {
		t.Fatal(err)
	}
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		switch name {
		case "agent-remote":
			return agentBinary, nil
		case "fclaude":
			return fclaudeBinary, nil
		default:
			return "", errors.New("test component not installed")
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("fclaude attach")
	if !tt.HasText("fclaude attach") {
		t.Fatal("fclaude attach was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpenRemote()); err != nil {
		t.Fatal(err)
	}
	// The sidebar keeps the Remote management label visible on every page;
	// wait for the remote context itself to prove the pending route completed.
	waitRemoteText(t, tt, "https://test.invalid")
	if !tt.HasText(locales.RemoteSessions()) {
		t.Fatal("fclaude attach did not open the remote session inventory")
	}
}

func TestCapabilitiesExposeForwardParentOperation(t *testing.T) {
	_, tt, _ := capabilityTester(t, "en")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("agent-remote forward")
	if !tt.HasText("agent-remote forward") {
		t.Fatal("forward parent operation was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpenPortForward()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.PortForwardTitle())
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("forward parent operation fell through to generic argv form")
	}
}

func TestCapabilitiesRoutesFClaudeDefaultInvocationToRemoteSessions(t *testing.T) {
	agentBinary := testutil.InstallControlHelper(t)
	fclaudeBinary := filepath.Join(t.TempDir(), "fclaude")
	if err := os.Symlink(agentBinary, fclaudeBinary); err != nil {
		t.Fatal(err)
	}
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		switch name {
		case "agent-remote":
			return agentBinary, nil
		case "fclaude":
			return fclaudeBinary, nil
		default:
			return "", errors.New("test component not installed")
		}
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("fclaude")
	if !tt.HasText("fclaude") {
		t.Fatal("fclaude default invocation was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpenRemote()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if !tt.HasText(locales.RemoteSessions()) {
		t.Fatal("fclaude default invocation did not open the remote session inventory")
	}
}

// The released fclaude schema also exposes request/status leaves for session
// lifecycle and forwarding. Keep each of those leaves on its context-bound
// page so a schema refresh cannot silently regress them to an unbound argv
// form.
func TestCapabilitiesRoutesAllFClaudeLifecycleAndForwardLeaves(t *testing.T) {
	tests := []struct {
		name       string
		filter     string
		open       string
		wantRemote bool
	}{
		{name: "stop", filter: "fclaude stop", open: locales.CliFormOpenRemote(), wantRemote: true},
		{name: "stop status", filter: "fclaude stop-status", open: locales.CliFormOpenRemote(), wantRemote: true},
		{name: "delete", filter: "fclaude delete", open: locales.CliFormOpenRemote(), wantRemote: true},
		{name: "delete status", filter: "fclaude delete-status", open: locales.CliFormOpenRemote(), wantRemote: true},
		{name: "forward list", filter: "fclaude forward list", open: locales.CliFormOpenPortForward()},
		{name: "forward stop", filter: "fclaude forward stop", open: locales.CliFormOpenPortForward()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			agentBinary := testutil.InstallControlHelper(t)
			fclaudeBinary := filepath.Join(t.TempDir(), "fclaude")
			if err := os.Symlink(agentBinary, fclaudeBinary); err != nil {
				t.Fatal(err)
			}
			i18n.SetLocaleFunc(func() string { return "en" })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
				switch name {
				case "agent-remote":
					return agentBinary, nil
				case "fclaude":
					return fclaudeBinary, nil
				default:
					return "", errors.New("test component not installed")
				}
			}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = a.Close() })
			tt := ui.NewTester(a.View, 1100, 820)
			tt.SetLocale("en")
			if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
			if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
				t.Fatal(err)
			}
			tt.Type(tc.filter)
			if !tt.HasText(tc.filter) {
				t.Fatalf("command %q was not discoverable", tc.filter)
			}
			if err := tt.Click(tc.open); err != nil {
				t.Fatal(err)
			}
			if tc.wantRemote {
				waitRemoteText(t, tt, "https://test.invalid")
				if !tt.HasText(locales.RemoteSessions()) {
					t.Fatalf("command %q did not open the remote session page", tc.filter)
				}
			} else if !tt.HasText(locales.PortForwardTitle()) {
				t.Fatalf("command %q did not open the port forwarding page", tc.filter)
			}
			if tt.HasText(locales.CliFormTitle()) {
				t.Fatalf("command %q fell through to generic argv form", tc.filter)
			}
		})
	}
}
