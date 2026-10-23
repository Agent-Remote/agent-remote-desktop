package app_test

import (
	"encoding/json"
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

func accountsTester(t *testing.T, language string) (*ui.Tester, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
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
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.RemoteAccounts()); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return tt, log
}
func openAccountDetails(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.AccountDetailsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountExactID("account-1"))
}
func fillAccountCreate(t *testing.T, tt *ui.Tester) {
	t.Helper()
	for _, field := range []struct{ label, value string }{{locales.AccountFieldTool(), "claude"}, {locales.AccountFieldName(), "Tokyo Studio"}, {locales.AccountFieldRegion(), "JP"}, {locales.AccountFieldTimezone(), "Asia/Tokyo"}, {locales.AccountFieldLocale(), "ja_JP.UTF-8"}, {locales.AccountFieldTags(), `["gpu","east"]`}} {
		if err := tt.Click(field.label); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(field.value)
	}
}
func TestAccountsCreateRequiresExactReviewAndPreservesAllCLIFields(t *testing.T) {
	tt, log := accountsTester(t, "en")
	if err := tt.Click(locales.AccountActionTitle("create")); err != nil {
		t.Fatal(err)
	}
	fillAccountCreate(t, tt)
	if err := tt.Click(locales.AccountReviewCreate()); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{locales.AccountFieldValue(locales.AccountFieldName(), "Tokyo Studio"), locales.AccountFieldValue(locales.AccountFieldTimezone(), "Asia/Tokyo"), "https://test.invalid", "device-1", locales.AccountActionHelp("create")} {
		if !tt.HasText(want) {
			t.Fatalf("missing review %s; text=%v", want, tt.Texts())
		}
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.create") {
		t.Fatal("creation before confirmation")
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts.create") {
		t.Fatal("review cancel created account")
	}
	if err := tt.Click(locales.AccountReviewCreate()); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "request")
	t.Setenv("DESKTOP_TEST_ACCOUNT_RECEIPT", capture)
	if err := tt.Click(locales.AccountConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountAccepted("create"))
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var input map[string]any
	if json.Unmarshal(data, &input) != nil {
		t.Fatal("request")
	}
	for key, want := range map[string]string{"tool": "claude", "name": "Tokyo Studio", "region": "JP", "timezone": "Asia/Tokyo", "locale": "ja_JP.UTF-8"} {
		if input[key] != want {
			t.Fatalf("lost %s: %s", key, data)
		}
	}
	tags, ok := input["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "gpu" || tags[1] != "east" || input["confirmed"] != true {
		t.Fatalf("tags/confirmation: %s", data)
	}
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AccountAccepted("create")) {
		t.Fatal("navigation lost receipt")
	}
	if calls, _ := os.ReadFile(log); strings.Count(string(calls), "accounts.create") != 1 {
		t.Fatalf("creation replayed: %s", calls)
	}
}
func TestAccountsActionsConfirmExactScopeAndNeverPollMutation(t *testing.T) {
	for _, action := range []string{"bind", "verify", "disable"} {
		t.Run(action, func(t *testing.T) {
			tt, log := accountsTester(t, "en")
			openAccountDetails(t, tt)
			if err := tt.Click(locales.AccountActionTitle(action)); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.AccountExactID("account-1")) || !tt.HasText("https://test.invalid") || !tt.HasText("device-1") || !tt.HasText(locales.AccountActionHelp(action)) {
				t.Fatal("scope/help missing")
			}
			if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "accounts."+action) {
				t.Fatalf("mutated without confirmation: %s", calls)
			}
			if err := tt.Click(locales.AccountConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.AccountAccepted(action))
			if action != "disable" && !tt.HasText(locales.AccountTask("account:verify:task-1")) {
				t.Fatal("accepted task missing")
			}
			if tt.HasText(locales.AccountConfirm()) {
				t.Fatal("blind retry remained")
			}
			if err := tt.Click(locales.AccountRefreshStatus()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.AccountState("active"))
			if !tt.HasText(locales.AccountAccepted(action)) {
				t.Fatal("status refresh discarded mutation receipt")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts."+action) != 1 {
				t.Fatalf("refresh replayed mutation: %s", calls)
			}
		})
	}
}
func TestAccountsMutationErrorsPreserveKnownAndUnknownResults(t *testing.T) {
	for mode, want := range map[string]string{"account-rejected": locales.AccountRejected(), "account-unknown": locales.AccountOutcomeUnknown(), "account-no-accepted": locales.AccountOutcomeUnknown(), "account-context-changed": locales.RemoteContextChanged(), "account-raw-command": locales.AccountOutcomeUnknown()} {
		t.Run(mode, func(t *testing.T) {
			tt, log := accountsTester(t, "en")
			openAccountDetails(t, tt)
			if err := tt.Click(locales.AccountActionTitle("bind")); err != nil {
				t.Fatal(err)
			}
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			if err := tt.Click(locales.AccountConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, want)
			if tt.HasText(locales.AccountConfirm()) || strings.Contains(strings.Join(tt.Texts(), "\n"), "private-") {
				t.Fatal("unsafe retry/raw diagnostic")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts.bind") != 1 {
				t.Fatalf("retry: %s", calls)
			}
		})
	}
}
func TestAccountsAuthenticationMutationInvalidatesReview(t *testing.T) {
	tt, log := accountsTester(t, "en")
	openAccountDetails(t, tt)
	if err := tt.Click(locales.AccountActionTitle("bind")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SignOutRemote()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ConfirmLogout()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LogoutLocalComplete())
	// Navigating through the list must perform a fresh observation before actions.
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "account-1")
	openAccountDetails(t, tt)
	if tt.HasText(locales.AccountConfirm()) {
		t.Fatal("stale review survived auth mutation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.bind") {
		t.Fatalf("stale review submitted: %s", calls)
	}
}
func TestAccountsLocalizedMinimumReviewLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		for _, action := range []string{"create", "disable"} {
			t.Run(language+"/"+action, func(t *testing.T) {
				tt, _ := accountsTester(t, language)
				if action == "create" {
					if err := tt.Click(locales.AccountActionTitle(action)); err != nil {
						t.Fatal(err)
					}
					fillAccountCreate(t, tt)
					if err := tt.Click(locales.AccountReviewCreate()); err != nil {
						t.Fatal(err)
					}
				} else {
					openAccountDetails(t, tt)
					if err := tt.Click(locales.AccountActionTitle(action)); err != nil {
						t.Fatal(err)
					}
				}
				for _, label := range []string{locales.AccountConfirm(), locales.Cancel(), locales.ReturnWorkbench()} {
					r, ok := tt.Find(label)
					if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
						t.Errorf("%q outside window: %+v", label, r)
					}
				}
				if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
					if err := os.MkdirAll(output, 0755); err != nil {
						t.Fatal(err)
					}
					f, err := os.Create(filepath.Join(output, "accounts-"+action+"-"+language+".png"))
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
}

func TestAccountsNewDetailsIgnoreCachedRemoteContext(t *testing.T) {
	tt, log := accountsTester(t, "en")
	t.Setenv("DESKTOP_TEST_CONTEXT_SERVER", "https://current.invalid")
	openAccountDetails(t, tt)
	if !tt.HasText("https://current.invalid") || tt.HasText("https://test.invalid") {
		t.Fatal("new detail used cached remote context")
	}
	calls, _ := os.ReadFile(log)
	if !strings.Contains(string(calls), "context\n") {
		t.Fatalf("new detail did not query context: %s", calls)
	}
}

func TestAccountsCrossPageResultInvalidatesOldAccountList(t *testing.T) {
	tt, log := accountsTester(t, "en")
	openAccountDetails(t, tt)
	barrier := t.TempDir()
	started, release := filepath.Join(barrier, "started"), filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_ACCOUNT_STARTED", started)
	t.Setenv("DESKTOP_TEST_ACCOUNT_RELEASE", release)
	if err := tt.Click(locales.AccountActionTitle("disable")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountConfirm()); err != nil {
		t.Fatal(err)
	}
	waitAccountStarted(t, started)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountDetailsTitle())
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountListStale())
	if tt.HasText(locales.AccountDetailsTitle()) {
		t.Fatal("stale account rows remained actionable")
	}
	if err := tt.Click(locales.AccountOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AccountAccepted("disable")) {
		t.Fatal("cross-page mutation receipt lost")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.disable") != 1 {
		t.Fatalf("mutation repeated: %s", calls)
	}
}
func TestAccountsCancelAfterNavigationPreservesUnknownOutcome(t *testing.T) {
	tt, log := accountsTester(t, "en")
	openAccountDetails(t, tt)
	barrier := t.TempDir()
	started, release := filepath.Join(barrier, "started"), filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_ACCOUNT_STARTED", started)
	t.Setenv("DESKTOP_TEST_ACCOUNT_RELEASE", release)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "account-slow")
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountActionTitle("verify")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountConfirm()); err != nil {
		t.Fatal(err)
	}
	waitAccountStarted(t, started)
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountStopWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountOutcomeUnknown())
	if tt.HasText(locales.AccountConfirm()) || tt.HasText(locales.RemoteQueryCanceled()) {
		t.Fatal("write cancellation presented as safe retry or canceled read")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.verify") != 1 {
		t.Fatalf("mutation retried: %s", calls)
	}
}
func waitAccountStarted(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(path); err == nil {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("account helper did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestAccountsAuthenticationInvalidatesRefreshedStatusButPreservesReceipt(t *testing.T) {
	tt, log := accountsTester(t, "en")
	openAccountDetails(t, tt)
	if err := tt.Click(locales.AccountActionTitle("bind")); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountAccepted("bind"))
	if err := tt.Click(locales.AccountRefreshStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountState("active"))
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.SignOutRemote()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ConfirmLogout()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LogoutLocalComplete())
	if err := tt.Click(locales.AccountOperationStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AccountAccepted("bind")) || !tt.HasText(locales.RemoteContextChanged()) || tt.HasText(locales.AccountState("active")) || tt.HasText(locales.AccountBindingConnect()) {
		t.Fatal("authentication change either lost receipt or retained actionable status")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.bind") != 1 {
		t.Fatalf("auth change replayed task: %s", calls)
	}
}
