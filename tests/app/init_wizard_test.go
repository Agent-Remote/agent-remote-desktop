package app_test

import (
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

func TestInitWizardRunsOrderedSafeReadinessFlow(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 850)
	tt.SetLocale("en")
	if err := openPage(tt, locales.InitWizardTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.LoginServer()); err != nil {
		t.Fatal(err)
	}
	tt.Type("https://test.invalid")
	if err := tt.Click(locales.LoginUsername()); err != nil {
		t.Fatal(err)
	}
	tt.Type("desktop-user")
	if err := tt.Click(locales.LoginPassword()); err != nil {
		t.Fatal(err)
	}
	tt.Type("private-login-password")
	if err := tt.Click(locales.ReviewLogin()); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("private-login-password") {
		t.Fatal("wizard review exposed password")
	}
	if err := tt.Click(locales.InitWizardConfirm()); err != nil {
		t.Fatal(err)
	}
	waitInitText(t, tt, locales.InitWizardComplete())
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	sequence := string(calls)
	for _, method := range []string{"setup.local\n", "context\n", "login.password\n", "diagnostics.inspect\n", "accounts.list\n", "sessions.list\n"} {
		if !strings.Contains(sequence, method) {
			t.Fatalf("ordered flow omitted %s: %q", method, sequence)
		}
	}
	if strings.Index(sequence, "setup.local\n") > strings.Index(sequence, "login.password\n") || strings.Index(sequence, "login.password\n") > strings.Index(sequence, "diagnostics.inspect\n") {
		t.Fatalf("mutation/readiness order was unsafe: %q", sequence)
	}
}

func TestInitWizardRequiresSeparateWireGuardPlanConfirmation(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	home := t.TempDir()
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 850)
	tt.SetLocale("en")
	if err := openPage(tt, locales.InitWizardTitle()); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ label, value string }{{locales.LoginServer(), "https://test.invalid"}, {locales.LoginUsername(), "desktop-user"}, {locales.LoginPassword(), "private-login-password"}} {
		if err := tt.Click(input.label); err != nil {
			t.Fatal(err)
		}
		tt.Type(input.value)
	}
	if err := tt.Click(locales.InitWizardConfigureWireGuard()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ReviewLogin()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.InitWizardConfirm()); err != nil {
		t.Fatal(err)
	}
	waitInitText(t, tt, locales.InitWizardWireGuardReview())
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "wireguard.config.apply") {
		t.Fatal("wireguard was written before its second confirmation")
	}
	if err := tt.Click(locales.InitWizardConfirmWireGuard()); err != nil {
		t.Fatal(err)
	}
	waitInitText(t, tt, locales.InitWizardComplete())
	calls, _ = os.ReadFile(log)
	sequence := string(calls)
	if !strings.Contains(sequence, "wireguard.config.plan\n") || !strings.Contains(sequence, "wireguard.config.apply\n") {
		t.Fatalf("wireguard plan/apply missing: %q", sequence)
	}
	if strings.Index(sequence, "wireguard.config.plan\n") > strings.Index(sequence, "wireguard.config.apply\n") {
		t.Fatalf("wireguard apply preceded plan: %q", sequence)
	}
}

func TestInitWizardDoesNotAuthenticateAfterIncompleteLocalSetup(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "setup-partial")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 850)
	tt.SetLocale("en")
	if err := openPage(tt, locales.InitWizardTitle()); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ label, value string }{{locales.LoginServer(), "https://test.invalid"}, {locales.LoginUsername(), "desktop-user"}, {locales.LoginPassword(), "private-login-password"}} {
		if err := tt.Click(input.label); err != nil {
			t.Fatal(err)
		}
		tt.Type(input.value)
	}
	if err := tt.Click(locales.ReviewLogin()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.InitWizardConfirm()); err != nil {
		t.Fatal(err)
	}
	waitInitText(t, tt, locales.LocalSetupIncomplete())
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "login.password") {
		t.Fatalf("login ran after incomplete setup: %q", calls)
	}
}

func waitInitText(t *testing.T, tt *ui.Tester, text string) {
	t.Helper()
	for i := 0; i < 1500; i++ {
		if tt.HasText(text) {
			return
		}
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %q; texts=%v", text, tt.Texts())
}
