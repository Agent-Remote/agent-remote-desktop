package app_test

import (
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestOperationCenterKeepsSafeCrossPageFailureProjection(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "login-unknown")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.SignInRemote()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.LoginUsername()); err != nil {
		t.Fatal(err)
	}
	tt.Type("desktop-user")
	if err := tt.Click(locales.LoginPassword()); err != nil {
		t.Fatal(err)
	}
	tt.Type("secret-that-must-not-cross-the-center")
	if err := tt.Click(locales.ReviewLogin()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ConfirmLogin()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LoginOutcomeUnknown())
	if err := openPage(tt, locales.OperationsSidebar()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !tt.HasText(locales.OperationsTitle()) && time.Now().Before(deadline) {
		tt.Frame()
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText(locales.OperationLogin()) || !tt.HasText(locales.OperationFailed()) {
		t.Fatalf("operation center did not project login failure: %v", tt.Texts())
	}
	if strings.Contains(strings.Join(tt.Texts(), "\n"), "secret-that-must-not-cross-the-center") {
		t.Fatal("operation center exposed login input")
	}
	if err := tt.Click(locales.OperationsOpen()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.SignInRemote()) {
		t.Fatal("operation center did not return to owning page")
	}
}

func TestOperationCenterEmptyStateAndLocalizedLayout(t *testing.T) {
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
			if err := openPage(tt, locales.OperationsSidebar()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.OperationsEmpty()) {
				t.Fatalf("empty state missing: %v", tt.Texts())
			}
			for _, label := range []string{locales.OperationsTitle(), locales.OperationsHelp(), locales.ReturnWorkbench()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q is outside window: %+v", label, r)
				}
			}
		})
	}
}
