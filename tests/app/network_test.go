package app_test

import (
	"encoding/json"
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

func networkTester(t *testing.T, mode, language string) (*ui.Tester, *app.App, string, string) {
	t.Helper()
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
	i18n.SetLocaleFunc(func() string { return language })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	home, log := t.TempDir(), filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
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
	openNetworkSearch(t, tt)
	return tt, a, home, log
}

func openNetworkSearch(t *testing.T, tt *ui.Tester) {
	t.Helper()
	if err := tt.Click(locales.SearchWorkbench()); err != nil {
		t.Fatal(err)
	}
	tt.Type(locales.WireGuardNetworkTitle())
	if err := tt.Click(locales.WireGuardNetworkTitle()); err != nil {
		t.Fatal(err)
	}
}

func TestNetworkNavigationIsPassiveAndQueriesAreIndependent(t *testing.T) {
	tt, _, home, log := networkTester(t, "unconfigured", "en")
	if calls, _ := os.ReadFile(log); len(calls) != 0 {
		t.Fatalf("navigation invoked CLI: %s", calls)
	}
	for _, label := range []string{locales.WireGuardStatusSummary(), locales.WireGuardCheckSummary()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	if calls, _ := os.ReadFile(log); len(calls) != 0 {
		t.Fatalf("section selection invoked CLI: %s", calls)
	}
	capture := filepath.Join(t.TempDir(), "check")
	t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK", capture)
	if err := tt.Click(locales.WireGuardCheckConfig()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardConfigState("ready"))
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var input struct {
		Home   string  `json:"expected_home"`
		Config *string `json:"config"`
	}
	if json.Unmarshal(data, &input) != nil || input.Home != home || input.Config != nil {
		t.Fatalf("wrong default config request: %s", data)
	}
	if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "wg-desktop")
	if err := tt.Click(locales.Settings()); err != nil {
		t.Fatal(err)
	}
	openNetworkSearch(t, tt)
	if !tt.HasText("wg-desktop") {
		t.Fatal("navigation discarded status")
	}
	if err := tt.Click(locales.WireGuardCheckSummary()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.WireGuardConfigState("ready")) {
		t.Fatal("status query discarded config observation")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "wireguard.check\n") != 1 || strings.Count(string(calls), "wireguard.status\n") != 1 || strings.Contains(string(calls), "context\n") || strings.Contains(string(calls), "login") {
		t.Fatalf("queries required remote context, repeated or mixed operations: %s", calls)
	}
}

func TestNetworkCustomConfigurationAndEditingInvalidatesObservation(t *testing.T) {
	tt, _, _, log := networkTester(t, "", "en")
	for _, label := range []string{locales.WireGuardCustomConfig(), locales.WireGuardConfigPath()} {
		if err := tt.Click(label); err != nil {
			t.Fatal(err)
		}
	}
	tt.Type("relative.conf")
	if err := tt.Click(locales.WireGuardCheckConfig()); err != nil {
		t.Fatal(err)
	}
	if calls, _ := os.ReadFile(log); strings.Contains(string(calls), "wireguard.check") {
		t.Fatal("relative config dispatched")
	}
	// Select the input again: the disabled query button may have changed focus.
	if err := tt.Click(locales.WireGuardConfigPath()); err != nil {
		t.Fatal(err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	path := filepath.Join(t.TempDir(), "custom network.conf")
	tt.Type(path)
	capture := filepath.Join(t.TempDir(), "check")
	t.Setenv("DESKTOP_TEST_WIREGUARD_CHECK", capture)
	if err := tt.Click(locales.WireGuardCheckConfig()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.WireGuardConfigState("ready"))
	data, _ := os.ReadFile(capture)
	var input map[string]any
	if json.Unmarshal(data, &input) != nil || input["config"] != path {
		t.Fatalf("custom config lost: %s", data)
	}
	if err := tt.Click(locales.WireGuardDefaultConfig()); err != nil {
		t.Fatal(err)
	}
	if tt.HasText(locales.WireGuardConfigState("ready")) || !tt.HasText(locales.WireGuardObservationStale()) {
		t.Fatal("changed config kept current observation")
	}
}

func TestNetworkFailuresNeverBecomeEmptySuccess(t *testing.T) {
	for _, mode := range []string{"wg-network-failed", "wg-network-error", "wg-network-unsupported", "wg-network-empty"} {
		t.Run(mode, func(t *testing.T) {
			tt, _, _, log := networkTester(t, mode, "en")
			if err := tt.Click(locales.WireGuardReadStatus()); err != nil {
				t.Fatal(err)
			}
			want := locales.WireGuardStatusState("query_failed")
			switch mode {
			case "wg-network-error":
				want = locales.RemoteConfigurationUnavailable()
			case "wg-network-unsupported":
				want = locales.WireGuardNetworkUnsupported()
			case "wg-network-empty":
				want = locales.WireGuardNoInterfaces()
			}
			waitRemoteText(t, tt, want)
			if mode != "wg-network-empty" && tt.HasText(locales.WireGuardNoInterfaces()) {
				t.Fatal("failed query displayed empty success")
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "wireguard.status\n") > 1 {
				t.Fatal("query retried automatically")
			}
		})
	}
}
