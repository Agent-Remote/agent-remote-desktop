package app_test

import (
	"encoding/json"
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

func TestBrowserLoginRequiresConfirmationAndExplicitBrowserOpen(t *testing.T) {
	tt, log := loginTester(t, "device-wait")
	approved := filepath.Join(t.TempDir(), "approved")
	input := filepath.Join(t.TempDir(), "input")
	t.Setenv("DESKTOP_TEST_DEVICE_CODE_APPROVED", approved)
	t.Setenv("DESKTOP_TEST_DEVICE_CODE_INPUT", input)
	if err := tt.Click(locales.LoginWithBrowser()); err != nil {
		t.Fatal(err)
	}
	for _, text := range tt.Texts() {
		if text == locales.LoginPassword() {
			t.Fatal("browser login still requests password")
		}
	}
	if err := tt.Click(locales.ReviewLogin()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "login.device_code") {
		t.Fatal("submitted before confirmation")
	}
	if err := tt.Click(locales.ConfirmLogin()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "ABCD-1234")
	if len(tt.OpenedURLs()) != 0 {
		t.Fatal("opened browser without explicit click")
	}
	if err := tt.Click(locales.LoginCopyCode()); err != nil {
		t.Fatal(err)
	}
	if tt.Clipboard() != "ABCD-1234" {
		t.Fatal("did not copy user code")
	}
	tt.FailOpenURL(errors.New("test browser failure"))
	if err := tt.Click(locales.LoginOpenBrowser()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.LoginBrowserOpenFailed()) {
		t.Fatal("browser failure had no localized recovery")
	}
	tt.FailOpenURL(nil)
	if err := tt.Click(locales.LoginOpenBrowser()); err != nil {
		t.Fatal(err)
	}
	if urls := tt.OpenedURLs(); len(urls) != 2 || urls[0] != "https://auth.test.invalid/cli?code=ABCD-1234" {
		t.Fatalf("wrong destination: %v", urls)
	}
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err = json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"password", "username", "totp_code", "device_code"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("browser flow sent %s", key)
		}
	}
	if payload["confirmed"] != true || payload["server_url"] != "https://test.invalid" {
		t.Fatal("lost exact confirmation")
	}
	if err = os.WriteFile(approved, nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LoginCompleted())
	if tt.HasText("ABCD-1234") {
		t.Fatal("authorization code retained after completion")
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "login.device_code\n" {
		t.Fatalf("unexpected login replay: %q", calls)
	}
}

func TestBrowserLoginCancelShowsUnknownOutcome(t *testing.T) {
	tt, _ := loginTester(t, "device-wait")
	for _, label := range []string{locales.LoginWithBrowser(), locales.ReviewLogin(), locales.ConfirmLogin()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	waitRemoteText(t, tt, "ABCD-1234")
	if err := tt.Click(locales.StopLoginWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LoginOutcomeUnknown())
	if tt.HasText("ABCD-1234") || tt.HasText(locales.LoginCompleted()) {
		t.Fatal("canceled flow retained approval or claimed success")
	}
}

func TestBrowserLoginMinimumLocalizedLayout(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "device-wait")
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
			if err = tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			for _, label := range []string{locales.SignInRemote(), locales.LoginWithBrowser(), locales.ReviewLogin(), locales.ConfirmLogin()} {
				if err = tt.Click(label); err != nil {
					t.Fatal(err)
				}
			}
			waitRemoteText(t, tt, "ABCD-1234")
			for _, label := range []string{locales.LoginAwaitingApproval(), "ABCD-1234", locales.LoginCopyCode(), locales.LoginOpenBrowser(), locales.StopLoginWaiting()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err = os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "device-code-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err = png.Encode(f, tt.Image()); err != nil {
					f.Close()
					t.Fatal(err)
				}
				f.Close()
			}
		})
	}
}

func TestBrowserApprovalExpiresWhileWindowIsNotRendering(t *testing.T) {
	tt, _ := loginTester(t, "device-wait-short")
	progress := filepath.Join(t.TempDir(), "progress-sent")
	t.Setenv("DESKTOP_TEST_PROGRESS_SENT", progress)
	for _, label := range []string{locales.LoginWithBrowser(), locales.ReviewLogin(), locales.ConfirmLogin()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(progress); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("CLI did not publish approval")
		}
		time.Sleep(10 * time.Millisecond)
	}
	// No frames are rendered while the window is hidden. Receiving its event
	// later must not restart the authorization code's lifetime.
	time.Sleep(1500 * time.Millisecond)
	tt.Frame()
	if !tt.HasText(locales.LoginCodeElapsed()) {
		t.Fatal("rendering restarted the code lifetime")
	}
	for _, label := range []string{locales.LoginCopyCode(), locales.LoginOpenBrowser()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if len(tt.OpenedURLs()) != 0 || tt.Clipboard() != "" {
		t.Fatal("expired code remained actionable")
	}
	if err := tt.Click(locales.StopLoginWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LoginOutcomeUnknown())
}
