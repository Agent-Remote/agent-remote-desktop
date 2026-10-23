package app_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

const bindingAccountID = "11111111-1111-4111-8111-111111111111"
const bindingSessionID = "bind-111111111111-aabbccddeeff"

func bindingAccountsTester(t *testing.T, mode, language string, project bool) (*ui.Tester, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home, projectPath := t.TempDir(), t.TempDir()
	if project {
		state := storage.State{Preferences: workspace.Defaults(), Projects: []workspace.Project{{ID: "binding-project", Name: "Binding project", Path: projectPath}}}
		state.Preferences.SelectedProject = "binding-project"
		testutil.StoreWorkspace(t, home, state)
	}
	a, err := app.New(home)
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
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.RemoteAccounts()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountDetailsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountExactID(bindingAccountID))
	return tt, projectPath
}
func saveAccountScreenshot(t *testing.T, tt *ui.Tester, name string) {
	t.Helper()
	output := os.Getenv("DESKTOP_RENDER_DIR")
	if output == "" {
		return
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(output, name+".png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatal(err)
	}
}
func TestAccountsBindingReviewMinimumLayoutAndExactScope(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, project := bindingAccountsTester(t, "account-terminal-ready", language, true)
			saveAccountScreenshot(t, tt, "accounts-status-"+language)
			tt.Scroll(650, 350, 0, 10000)
			saveAccountScreenshot(t, tt, "accounts-status-binding-"+language)
			if err := tt.Click(locales.AccountBindingConnect()); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{locales.AccountExactID(bindingAccountID), locales.AccountBindingSession(bindingSessionID), "https://test.invalid", "device-1", project, locales.AccountBindingConnectHelp()} {
				if !tt.HasText(want) {
					t.Fatalf("confirmation missing %q", want)
				}
			}
			for _, label := range []string{locales.AccountBindingConnect(), locales.Cancel(), locales.ReturnWorkbench()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Fatalf("binding control %q outside window: %+v", label, r)
				}
			}
			saveAccountScreenshot(t, tt, "accounts-binding-review-"+language)
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(locales.AccountBindingConnectHelp()) {
				t.Fatal("canceled terminal review remained")
			}
		})
	}
}
func TestAccountsBindingRequiresTerminalCapabilityAndCurrentSession(t *testing.T) {
	for _, mode := range []string{"account-terminal-unsupported", "account-terminal-expired", "account-terminal-disabled"} {
		t.Run(mode, func(t *testing.T) {
			tt, _ := bindingAccountsTester(t, mode, "en", true)
			tt.Scroll(650, 350, 0, 10000)
			if tt.HasText(locales.AccountBindingConnect()) {
				t.Fatal("unsupported/stale/disabled binding offered terminal")
			}
		})
	}
	t.Run("project-required", func(t *testing.T) {
		tt, _ := bindingAccountsTester(t, "account-terminal-ready", "en", false)
		tt.Scroll(650, 350, 0, 10000)
		if !tt.HasText(locales.RemoteProjectRequired()) {
			t.Fatal("project requirement omitted")
		}
		_ = tt.Click(locales.AccountBindingConnect())
		if tt.HasText(locales.AccountBindingConnectHelp()) {
			t.Fatal("terminal review opened without project")
		}
	})
}

func TestAccountsDisabledAccountRequiresExplicitRebind(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := bindingAccountsTester(t, "account-terminal-disabled", language, true)
			log := filepath.Join(t.TempDir(), "calls")
			capture := filepath.Join(t.TempDir(), "request")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			t.Setenv("DESKTOP_TEST_ACCOUNT_RECEIPT", capture)
			if !tt.HasText(locales.AccountState("disabled")) {
				t.Fatal("disabled state missing")
			}
			for _, action := range []string{"verify", "disable"} {
				_ = tt.Click(locales.AccountActionTitle(action))
				if tt.HasText(locales.AccountConfirm()) {
					t.Fatalf("disabled account allowed %s", action)
				}
			}
			tt.Scroll(650, 350, 0, 10000)
			if tt.HasText(locales.AccountBindingConnect()) {
				t.Fatal("disabled account offered terminal")
			}
			tt.Scroll(650, 350, 0, -10000)
			if err := tt.Click(locales.AccountActionTitle("bind")); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{locales.AccountExactID(bindingAccountID), "https://test.invalid", "device-1", locales.AccountRebindDisabledHelp(), locales.AccountActionHelp("bind")} {
				if !tt.HasText(want) {
					t.Fatalf("rebind review missing %q", want)
				}
			}
			saveAccountScreenshot(t, tt, "accounts-disabled-rebind-"+language)
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.bind") {
				t.Fatalf("cancel submitted bind: %s", calls)
			}
			if tt.HasText(locales.AccountRebindDisabledHelp()) {
				t.Fatal("canceled rebind review remains")
			}
			tt.Scroll(650, 350, 0, -10000)
			if err := tt.Click(locales.AccountActionTitle("bind")); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.AccountConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.AccountAccepted("bind"))
			if !tt.HasText(locales.AccountBindingStatus(locales.AccountState("binding_session_starting"))) {
				t.Fatal("accepted rebind lifecycle missing")
			}
			if tt.HasText(locales.AccountBindingConnect()) {
				t.Fatal("accepted task offered terminal without refreshed status")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts.bind") != 1 {
				t.Fatalf("unexpected bind count: %s", calls)
			}
			data, err := os.ReadFile(capture)
			if err != nil {
				t.Fatal(err)
			}
			var input struct {
				Confirmed bool   `json:"confirmed"`
				AccountID string `json:"account_id"`
				Expected  struct {
					Server string `json:"server_url"`
					Device string `json:"active_device_id"`
				} `json:"expected_context"`
			}
			if err := json.Unmarshal(data, &input); err != nil {
				t.Fatal(err)
			}
			if !input.Confirmed || input.AccountID != bindingAccountID || input.Expected.Server != "https://test.invalid" || input.Expected.Device != "device-1" {
				t.Fatalf("incorrect exact rebind request: %s", data)
			}
		})
	}
}
