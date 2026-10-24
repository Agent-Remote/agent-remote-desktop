package app_test

import (
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func capabilityTester(t *testing.T, language string) (*app.App, *ui.Tester, string) {
	t.Helper()
	binary := testutil.InstallControlHelper(t)
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	i18n.SetLocaleFunc(func() string { return language })
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
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	tt := ui.NewTester(a.View, 900, 600)
	tt.SetLocale(language)
	return a, tt, log
}

func TestCapabilitiesPageIsPassiveAndFiltersCommandPlatformDeclarations(t *testing.T) {
	_, tt, log := capabilityTester(t, "en")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if tt.HasText(locales.CapabilitiesState("observed_allowed")) {
		t.Fatal("capabilities falsely proved authorization")
	}
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("device install")
	// Text input is delivered before the next render on slower CI workers.
	tt.Frame()
	if !tt.HasText("agent-remote device install") || tt.HasText("agent-remote device revoke") {
		t.Fatal("command filter lost exact path")
	}
	if !tt.HasText(locales.CapabilitiesState("unverified")) {
		t.Fatal("command coverage did not expose the current permission state")
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "capabilities\ncontext\nschema:agent-remote\n" {
		t.Fatalf("passive page performed unexpected actions: %s", data)
	}
}

func TestCapabilitiesRoutesCredentialCommandsToSecurePage(t *testing.T) {
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
	tt.Type("credentials create")
	if !tt.HasText("agent-remote credentials create") {
		t.Fatal("credential command was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CredentialsTitle())
}

func TestCapabilitiesRoutesInitToCompositionWizard(t *testing.T) {
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
	tt.Type("agent-remote init")
	if !tt.HasText("agent-remote init") {
		t.Fatal("init command was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.InitWizardTitle()) {
		t.Fatal("init command bypassed the composition wizard")
	}
}

func TestCapabilitiesCommandCardShowsUnknownPermissionMapping(t *testing.T) {
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
	tt.Type("init")
	// TextInput updates are rendered on the next tester frame; advance once
	// before asserting the filtered card instead of racing the UI event loop.
	tt.Frame()
	if !tt.HasText("agent-remote init") {
		t.Fatal("unmapped command was not discoverable")
	}
	if !tt.HasText(locales.CapabilitiesState("unknown")) {
		t.Fatal("unmapped command did not expose its unknown permission state")
	}
}

func TestCapabilitiesNodeInstallShowsMissingTerminalWorkflow(t *testing.T) {
	// nodes.list is intentionally still available in this fixture. Removing
	// only the native install capability exercises the mixed-version boundary:
	// target discovery cannot make the reviewed install terminal executable.
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "node-terminal-unsupported")
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
	tt.Type("node install")
	tt.Frame()
	if !tt.HasText("agent-remote node install") {
		t.Fatal("node install was not discoverable")
	}
	if !tt.HasText(locales.CapabilitiesState("cli_unsupported")) {
		t.Fatal("missing node install terminal capability was not shown")
	}
	if tt.HasText(locales.CapabilitiesState("unverified")) {
		t.Fatal("node install was shown as unverified despite missing terminal workflow")
	}
}

func TestCapabilitiesRoutesSensitiveLoginCommandToSecurePage(t *testing.T) {
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
	tt.Type("login")
	if !tt.HasText("agent-remote login") {
		t.Fatal("sensitive login command was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.SignInRemote())
	if !tt.HasText(locales.SignInRemote()) {
		t.Fatal("sensitive login command did not route to the secure login page")
	}
	if tt.HasText(locales.CliFormSecureWorkflow()) {
		t.Fatal("login remained on the generic argv form")
	}
}

func TestCapabilitiesPreserveDeniedPermissionWithoutRequeryingAndDiscardOtherContext(t *testing.T) {
	_, tt, log := capabilityTester(t, "en")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "permission")
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("denied"))
	if !tt.HasText(locales.CapabilitiesState("observed_allowed")) {
		t.Fatal("independent sessions success lost")
	}
	data, _ := os.ReadFile(log)
	if strings.Count(string(data), "accounts.list\n") != 1 || strings.Count(string(data), "sessions.list\n") != 1 {
		t.Fatalf("capabilities made authenticated probes: %s", data)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "unconfigured")
	if err := tt.Click(locales.RefreshRemote()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("setup_required"))
	if tt.HasText(locales.CapabilitiesState("denied")) || tt.HasText(locales.CapabilitiesState("observed_allowed")) {
		t.Fatal("reused observations from a different context")
	}
}

func TestCapabilitiesCancelAndCloseBoundWorkerLifetime(t *testing.T) {
	a, tt, _ := capabilityTester(t, "en")
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "slow-schema")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CancelRemoteQuery()); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("closing retained capability discovery")
	}
}

func TestCapabilitiesMinimumLocalizedLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			_, tt, _ := capabilityTester(t, language)
			if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
			for _, label := range []string{locales.CapabilitiesOperations(), locales.CapabilitiesCommands(), locales.RefreshRemote(), locales.ReturnWorkbench()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "capabilities-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					f.Close()
					t.Fatal(err)
				}
				f.Close()
			}
		})
	}
}
