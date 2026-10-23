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

func logoutTester(t *testing.T, mode, language string) (*ui.Tester, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return language })
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
	tt := ui.NewTester(a.View, 900, 600)
	tt.SetLocale(language)
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.SignOutRemote()); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return tt, log
}

func TestLogoutReviewsExactTargetAndSupportsLocalOnly(t *testing.T) {
	for _, revoke := range []bool{true, false} {
		t.Run(map[bool]string{true: "revoke", false: "local"}[revoke], func(t *testing.T) {
			tt, log := logoutTester(t, "", "en")
			if !tt.HasText("https://test.invalid") || !tt.HasText("device-1") || !tt.HasText(locales.LogoutReviewHelp()) {
				t.Fatal("logout review omitted scope")
			}
			if data, _ := os.ReadFile(log); len(data) != 0 {
				t.Fatal("operation submitted before confirmation")
			}
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			if data, _ := os.ReadFile(log); strings.Contains(string(data), "logout") {
				t.Fatal("cancel mutated credentials")
			}
			if err := tt.Click(locales.SignOutRemote()); err != nil {
				t.Fatal(err)
			}
			if !revoke {
				if err := tt.Click(locales.LogoutRevokeRemote()); err != nil {
					t.Fatal(err)
				}
			}
			path := filepath.Join(t.TempDir(), "receipt")
			t.Setenv("DESKTOP_TEST_LOGOUT_RECEIPT", path)
			if err := tt.Click(locales.ConfirmLogout()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.LogoutLocalComplete())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var input struct {
				Confirmed bool           `json:"confirmed"`
				Revoke    bool           `json:"revoke_remote"`
				Expected  map[string]any `json:"expected_context"`
			}
			if err := json.Unmarshal(data, &input); err != nil {
				t.Fatal(err)
			}
			if !input.Confirmed || input.Revoke != revoke || input.Expected["server_url"] != "https://test.invalid" || input.Expected["active_device_id"] != "device-1" {
				t.Fatalf("incorrect confirmation scope: %s", data)
			}
			state := locales.LogoutRevocationConfirmed()
			if !revoke {
				state = locales.LogoutRevocationSkipped()
			}
			if !tt.HasText(locales.LogoutResultItem(locales.LogoutUserRevocation(), state)) {
				t.Fatal("remote revocation choice lost")
			}
			calls, _ := os.ReadFile(log)
			if string(calls) != "logout\n" {
				t.Fatalf("unexpected side effects: %q", calls)
			}
		})
	}
}

func TestLogoutUnknownAndPartialResultsDoNotOfferBlindRetry(t *testing.T) {
	for _, mode := range []string{"slow-logout", "logout-malformed", "logout-wrong-operation", "logout-partial", "logout-local-incomplete"} {
		t.Run(mode, func(t *testing.T) {
			tt, log := logoutTester(t, mode, "en")
			if err := tt.Click(locales.ConfirmLogout()); err != nil {
				t.Fatal(err)
			}
			if mode == "slow-logout" {
				if err := tt.Click(locales.StopLogoutWaiting()); err != nil {
					t.Fatal(err)
				}
			}
			want := locales.LogoutOutcomeUnknown()
			if mode == "logout-partial" {
				want = locales.LogoutResultItem(locales.LogoutUserRevocation(), locales.LogoutRevocationUnconfirmed())
			}
			if mode == "logout-local-incomplete" {
				want = locales.LogoutLocalIncomplete()
			}
			waitRemoteText(t, tt, want)
			if tt.HasText(locales.ConfirmLogout()) || tt.HasText(locales.RemoteQueryCanceled()) {
				t.Fatal("unconfirmed logout offered as reversible or safe retry")
			}
			if mode != "logout-partial" && tt.HasText(locales.LogoutLocalComplete()) {
				t.Fatal("unknown result reported success")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "logout") > 1 {
				t.Fatal("automatically retried logout")
			}
		})
	}
}

func TestLogoutMinimumLocalizedLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := logoutTester(t, "", language)
			for _, label := range []string{locales.SignOutRemote(), locales.LogoutRevokeRemote(), locales.ConfirmLogout(), locales.Cancel()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "logout-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
