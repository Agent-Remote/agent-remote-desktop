package app_test

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestMain(m *testing.M) {
	if testutil.ControlHelper() {
		return
	}
	os.Exit(m.Run())
}

func waitRemoteText(t *testing.T, tester *ui.Tester, text string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for !tester.HasText(text) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tester.Frame()
	}
	if !tester.HasText(text) {
		t.Fatalf("remote page did not show %q", text)
	}
}

func TestRemotePageQueriesFiltersAndTranslatesPermissionErrors(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "permission")
	lang := "en"
	i18n.SetLocaleFunc(func() string { return lang })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click("Remote management"); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if !tt.HasText("Select a local project before connecting a remote terminal.") {
		t.Fatal("missing project prerequisite")
	}
	if err := tt.Click("Filter by name, ID, path or status"); err != nil {
		t.Fatal(err)
	}
	tt.Type("beta")
	if tt.HasText("11111111-1111-4111-8111-111111111111") || !tt.HasText("22222222-2222-4222-8222-222222222222") {
		t.Fatal("filter did not narrow sessions")
	}
	if err := tt.Click("Tool accounts"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("The server denied access to this operation.") {
		t.Fatal("permission error shown as empty accounts")
	}
	lang = "zh-Hans"
	tt.Frame()
	if !tt.HasText("服务器拒绝了此操作的访问权限。") {
		t.Fatal("existing remote error did not retranslate")
	}
}

func TestRemoteQueryCanBeCanceledWithoutBlockingAppClose(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "slow")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click("Remote management"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Cancel query"); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Query canceled. No remote session was stopped.")
	if err := tt.Click("Refresh"); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("application close retained pending query")
	}
}

func TestRemoteAttachConfirmationAndMinimumLocalizedLayout(t *testing.T) {
	t.Setenv("DESKTOP_TEST_REMOTE_DEVICE", testutil.RemoteAttachDeviceID)
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			i18n.SetLocaleFunc(func() string { return language })
			t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
			home := t.TempDir()
			state := testutil.SavedWorkspace(t)
			testutil.StoreWorkspace(t, home, state)
			a, err := app.New(home)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			tt := ui.NewTester(a.View, 900, 600)
			tt.SetLocale(language)
			if err := tt.Click(locales.RemoteManagement()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
			for _, label := range []string{locales.RefreshRemote(), locales.RemoteSessions(), locales.RemoteAccounts(), locales.FilterRemote(), locales.ConnectRemoteSession()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "remote-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					f.Close()
					t.Fatal(err)
				}
				f.Close()
			}
			if err := tt.Click(locales.ConnectRemoteSession()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.RemoteAttachConfirmation()) || !tt.HasText(state.Projects[0].Path) {
				t.Fatal("attachment consent lost exact local context")
			}
			tt.Key(ui.Cmd|ui.Alt, ui.KeyRight)
			if !tt.HasText(state.Projects[0].Path) {
				t.Fatal("background shortcut changed attachment context")
			}
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
			after := testutil.LoadWorkspace(t, home)
			if len(after.Sessions) != len(state.Sessions) {
				t.Fatal("canceling attachment created a session")
			}
		})
	}
}
