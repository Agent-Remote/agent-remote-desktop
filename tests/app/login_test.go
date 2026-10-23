package app_test

import (
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func loginTester(t *testing.T, mode string) (*ui.Tester, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := a.Close(); err != nil {
			t.Error(err)
		}
	})
	tt := ui.NewTester(a.View, 1100, 850)
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.SignInRemote()); err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ label, value string }{{locales.LoginUsername(), "desktop-user"}, {locales.LoginPassword(), "private-login-password"}, {locales.LoginTOTP(), "654321"}} {
		if err := tt.Click(input.label); err != nil {
			t.Fatal(err)
		}
		tt.Type(input.value)
	}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return tt, log
}

func TestLoginReviewSubmitsOnlyAfterConfirmationAndNeverDisplaysSecrets(t *testing.T) {
	tt, log := loginTester(t, "login-success")
	if text := strings.Join(tt.Texts(), "\n"); strings.Contains(text, "private-login-password") || strings.Contains(text, "654321") {
		t.Fatal("credential fields exposed secret text")
	}
	receipt := filepath.Join(t.TempDir(), "receipt")
	t.Setenv("DESKTOP_TEST_LOGIN_RECEIPT", receipt)
	if err := tt.Click(locales.ReviewLogin()); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(log); strings.Contains(string(data), "login.password") {
		t.Fatal("submitted before confirmation")
	}
	if !tt.HasText("https://test.invalid") || !tt.HasText("desktop-user") || !tt.HasText(locales.LoginDeviceRegistrationHelp()) {
		t.Fatal("review omitted destination or registration effect")
	}
	if text := strings.Join(tt.Texts(), "\n"); strings.Contains(text, "private-login-password") || strings.Contains(text, "654321") {
		t.Fatal("review exposed a secret")
	}
	if err := tt.Click(locales.ConfirmLogin()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LoginCompleted())
	data, err := os.ReadFile(receipt)
	if err != nil {
		t.Fatal(err)
	}
	var value map[string]any
	if err = json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	if value["password_received"] != true || value["totp_received"] != true {
		t.Fatal("secret stdin payload missing")
	}
	if strings.Contains(string(data), "private-login-password") || strings.Contains(string(data), "654321") {
		t.Fatal("secret leaked into safe receipt")
	}
	expected := value["expected"].(map[string]any)
	if expected["server_url"] != "https://test.invalid" || expected["active_device_id"] != "device-1" {
		t.Fatal("lost confirmed CLI context")
	}
	args := value["args"].([]any)
	if len(args) != 1 || args[0] != "--desktop-control" {
		t.Fatal("login details entered command arguments")
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "login.password\n" {
		t.Fatalf("unexpected login replay: %q", calls)
	}
}

func TestLoginCancelOrUnknownResultNeverReportsRollbackOrRetries(t *testing.T) {
	for _, mode := range []string{"slow-login", "login-unknown"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := loginTester(t, mode)
			if err := tt.Click(locales.ReviewLogin()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.ConfirmLogin()); err != nil {
				t.Fatal(err)
			}
			if mode == "slow-login" {
				if err := tt.Click(locales.StopLoginWaiting()); err != nil {
					t.Fatal(err)
				}
			}
			waitRemoteText(t, tt, locales.LoginOutcomeUnknown())
			if tt.HasText(locales.LoginCompleted()) || tt.HasText(locales.RemoteQueryCanceled()) || tt.HasText(locales.ConfirmLogin()) {
				t.Fatal("unknown mutation result shown as success or safe retry")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "login.password") > 1 {
				t.Fatal("automatically retried login")
			}
			if err := tt.Click(locales.ReturnRemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			if err := tt.Click(locales.SignInRemote()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.LoginUserOnly()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.LoginUsername()); err != nil {
				t.Fatal(err)
			}
			tt.Type("desktop-user")
			if err := tt.Click(locales.ReviewLogin()); err != nil {
				t.Fatal(err)
			}
			if tt.HasText(locales.LoginReviewHelp()) {
				t.Fatal("old password remained usable for a new sign-in")
			}
			if tt.HasText("private-login-password") || tt.HasText("654321") {
				t.Fatal("retained old credential input")
			}
		})
	}
}

func TestLoginMinimumLocalizedLayout(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return language })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			a, err := app.New(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			tt := ui.NewTester(a.View, 900, 600)
			tt.SetLocale(language)
			if err := openPage(tt, locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			if err := tt.Click(locales.SignInRemote()); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{locales.LoginServer(), locales.LoginUsername(), locales.LoginPassword(), locales.ReviewLogin(), locales.Cancel()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q is outside the window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "login-"+language+".png"))
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
