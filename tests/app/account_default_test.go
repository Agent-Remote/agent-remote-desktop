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

func accountDefaultTester(t *testing.T, mode, language string) (*ui.Tester, string) {
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
	if err := tt.Click(locales.AccountDefaultTitle()); err != nil {
		t.Fatal(err)
	}
	want := locales.AccountDefaultCurrent("previous-account")
	if mode == "unconfigured" {
		want = locales.AccountDefaultCurrent(locales.AccountDefaultNone())
	}
	if mode == "default-unsupported" {
		want = locales.AccountDefaultUnsupported()
	}
	waitRemoteText(t, tt, want)
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	return tt, log
}
func TestAccountDefaultReviewAndExactConfirmation(t *testing.T) {
	for _, clearDefault := range []bool{false, true} {
		t.Run(map[bool]string{true: "clear", false: "set"}[clearDefault], func(t *testing.T) {
			tt, log := accountDefaultTester(t, "", "en")
			label := locales.AccountDefaultSelect()
			if clearDefault {
				label = locales.AccountDefaultClear()
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			for _, want := range []string{locales.AccountDefaultPrevious("previous-account"), "https://test.invalid", "device-1", locales.AccountDefaultToolValue("claude")} {
				if !tt.HasText(want) {
					t.Fatalf("missing review %s", want)
				}
			}
			if data, _ := os.ReadFile(log); len(data) != 0 {
				t.Fatalf("review mutated: %s", data)
			}
			if err := tt.Click(locales.Cancel()); err != nil {
				t.Fatal(err)
			}
			if data, _ := os.ReadFile(log); len(data) != 0 {
				t.Fatal("cancel mutated")
			}
			if err := tt.Click(label); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "receipt")
			t.Setenv("DESKTOP_TEST_DEFAULT_RECEIPT", path)
			if err := tt.Click(locales.AccountDefaultConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, locales.AccountDefaultSaved())
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var input map[string]any
			if json.Unmarshal(data, &input) != nil {
				t.Fatal("input JSON")
			}
			if input["confirmed"] != true || input["expected_account_id"] != "previous-account" || input["tool"] != "claude" {
				t.Fatalf("wrong review: %s", data)
			}
			if !clearDefault && input["account_id"] != "account-1" {
				t.Fatalf("not full listed ID: %s", data)
			}
			if clearDefault {
				if _, ok := input["account_id"]; ok {
					t.Fatal("clear included set account")
				}
			}
			if err := tt.Click(locales.ReturnWorkbench()); err != nil {
				t.Fatal(err)
			}
			if err := openPage(tt, locales.AccountDefaultStatus()); err != nil {
				t.Fatal(err)
			}
			if !tt.HasText(locales.AccountDefaultSaved()) {
				t.Fatal("navigation lost result")
			}
			calls, _ := os.ReadFile(log)
			method := "accounts.default.set"
			if clearDefault {
				method = "accounts.default.clear"
			}
			if strings.Count(string(calls), method) != 1 {
				t.Fatalf("wrong writes: %s", calls)
			}
		})
	}
}
func TestAccountDefaultOfflineClearAndOldCLI(t *testing.T) {
	t.Run("offline", func(t *testing.T) {
		tt, _ := accountDefaultTester(t, "unconfigured", "en")
		if tt.HasText(locales.AccountDefaultSelect()) {
			t.Fatal("offline set offered")
		}
		if err := tt.Click(locales.AccountDefaultClear()); err != nil {
			t.Fatal(err)
		}
		if err := tt.Click(locales.AccountDefaultConfirm()); err != nil {
			t.Fatal(err)
		}
		waitRemoteText(t, tt, locales.AccountDefaultUnchanged())
	})
	t.Run("old-cli", func(t *testing.T) {
		tt, log := accountDefaultTester(t, "default-unsupported", "en")
		if tt.HasText(locales.AccountDefaultClear()) || tt.HasText(locales.AccountDefaultSelect()) {
			t.Fatal("old CLI offered mutation")
		}
		if data, _ := os.ReadFile(log); len(data) != 0 {
			t.Fatalf("old CLI calls %s", data)
		}
	})
}
func TestAccountDefaultFailureDoesNotOfferBlindRetry(t *testing.T) {
	for mode, want := range map[string]string{"default-missing-changed": locales.AccountDefaultUnknown(), "default-stale": locales.AccountDefaultChanged(), "default-context-changed": locales.RemoteContextChanged(), "default-denied": locales.RemotePermissionDenied()} {
		t.Run(mode, func(t *testing.T) {
			tt, log := accountDefaultTester(t, mode, "en")
			if err := tt.Click(locales.AccountDefaultSelect()); err != nil {
				t.Fatal(err)
			}
			if err := tt.Click(locales.AccountDefaultConfirm()); err != nil {
				t.Fatal(err)
			}
			waitRemoteText(t, tt, want)
			if tt.HasText(locales.AccountDefaultConfirm()) {
				t.Fatal("blind retry offered")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts.default.set") != 1 {
				t.Fatalf("retried: %s", calls)
			}
		})
	}
}
func TestAccountDefaultLocalizedReviewLayout(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans", "en-XA"} {
		t.Run(language, func(t *testing.T) {
			tt, _ := accountDefaultTester(t, "", language)
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "account-default-list-"+language+".png"))
				if err != nil {
					t.Fatal(err)
				}
				if err := png.Encode(f, tt.Image()); err != nil {
					t.Fatal(err)
				}
				f.Close()
			}
			if err := tt.Click(locales.AccountDefaultSelect()); err != nil {
				t.Fatal(err)
			}
			for _, label := range []string{locales.AccountDefaultConfirm(), locales.Cancel(), locales.ReturnWorkbench()} {
				r, ok := tt.Find(label)
				if !ok || r.X < 0 || r.Y < 0 || r.X+r.W > 901 || r.Y+r.H > 601 {
					t.Errorf("%q outside window: %+v", label, r)
				}
			}
			if output := os.Getenv("DESKTOP_RENDER_DIR"); output != "" {
				if err := os.MkdirAll(output, 0755); err != nil {
					t.Fatal(err)
				}
				f, err := os.Create(filepath.Join(output, "account-default-"+language+".png"))
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

func TestAccountDefaultCancelAfterNavigationPreservesUnknownOutcome(t *testing.T) {
	tt, log := accountDefaultTester(t, "default-slow", "en")
	started := filepath.Join(t.TempDir(), "started")
	t.Setenv("DESKTOP_TEST_DEFAULT_STARTED", started)
	// An existing release lets the helper record the started marker and then hold.
	release := filepath.Join(t.TempDir(), "release")
	if err := os.WriteFile(release, []byte("go"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DESKTOP_TEST_DEFAULT_RELEASE", release)
	if err := tt.Click(locales.AccountDefaultSelect()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountDefaultConfirm()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mutation did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if err := tt.Click(locales.ReturnWorkbench()); err != nil {
		t.Fatal(err)
	}
	if err := openPage(tt, locales.AccountDefaultStatus()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.AccountDefaultWaiting()) {
		t.Fatal("navigation lost in-flight mutation")
	}
	if err := tt.Click(locales.AccountDefaultStopWaiting()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountDefaultUnknown())
	if tt.HasText(locales.AccountDefaultConfirm()) || tt.HasText(locales.RemoteQueryCanceled()) {
		t.Fatal("canceled write offered as retryable read")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "accounts.default.set") != 1 {
		t.Fatalf("mutation retried: %s", calls)
	}
}

func TestAccountDefaultReviewInvalidatedByAuthenticationChange(t *testing.T) {
	tt, log := accountDefaultTester(t, "", "en")
	if err := tt.Click(locales.AccountDefaultSelect()); err != nil {
		t.Fatal(err)
	}
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "11111111-1111-4111-8111-111111111111")
	if err := tt.Click(locales.SignOutRemote()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.ConfirmLogout()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.LogoutLocalComplete())
	if err := openPage(tt, locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountDefaultTitle()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.RemoteContextChanged()) || tt.HasText(locales.AccountDefaultConfirm()) {
		t.Fatal("stale account review survived auth mutation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Contains(string(calls), "accounts.default.set") {
		t.Fatalf("stale review wrote: %s", calls)
	}
}

func TestAccountDefaultRemotePermissionFailurePreservesLocalClear(t *testing.T) {
	tt, _ := accountDefaultTester(t, "permission", "en")
	if !tt.HasText(locales.RemotePermissionDenied()) || tt.HasText(locales.AccountDefaultSelect()) {
		t.Fatal("remote permission failure mislabeled or account selection offered")
	}
	if err := tt.Click(locales.AccountDefaultClear()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.AccountDefaultConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.AccountDefaultSaved())
}
