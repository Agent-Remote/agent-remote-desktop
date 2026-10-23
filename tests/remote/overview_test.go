package remote_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestLoadRejectsInventoryFromDifferentHomeScope(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "context-home-changed")

	overview, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if overview.AccountsError != fault.CLIContextChanged {
		t.Fatalf("accounts error = %v, want context changed", overview.AccountsError)
	}
	if overview.SessionsError != fault.CLIContextChanged {
		t.Fatalf("sessions error = %v, want context changed", overview.SessionsError)
	}
	if overview.Accounts != nil || overview.Sessions != nil {
		t.Fatalf("cross-home inventory was accepted: accounts=%v sessions=%v", overview.Accounts, overview.Sessions)
	}
}

func TestLoadRejectsMalformedPublicInventoryFields(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	for _, scenario := range []string{"remote-account-malformed", "remote-session-malformed"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", scenario)
			overview, err := remote.Load(context.Background(), client)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "remote-account-malformed" {
				if overview.AccountsError != fault.CLIProtocol || overview.Accounts != nil {
					t.Fatalf("malformed account inventory accepted: items=%v err=%v", overview.Accounts, overview.AccountsError)
				}
			} else if overview.SessionsError != fault.CLIProtocol || overview.Sessions != nil {
				t.Fatalf("malformed session inventory accepted: items=%v err=%v", overview.Sessions, overview.SessionsError)
			}
		})
	}
}

func TestLoadWithSessionFilterPassesServerSelectorAndKeepsDefaultAll(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	paramsLog := filepath.Join(t.TempDir(), "session-params")
	t.Setenv("DESKTOP_TEST_CLI_PARAMS_LOG", paramsLog)

	filtered, err := remote.LoadWithSessionFilter(context.Background(), client, remote.SessionListFilter{
		ToolType: "claude", Statuses: []string{"stopped", "running"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.SessionsError != nil || len(filtered.Sessions) != 1 || filtered.Sessions[0].Status != "running" {
		t.Fatalf("server selector was not applied: sessions=%+v err=%v", filtered.Sessions, filtered.SessionsError)
	}
	data, err := os.ReadFile(paramsLog)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != `{"tool_type":"claude","statuses":["running","stopped"]}
` {
		t.Fatalf("unexpected sessions.list params: %q", data)
	}

	defaultAll, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if defaultAll.SessionsError != nil || len(defaultAll.Sessions) != 2 {
		t.Fatalf("default empty selector did not mean all: sessions=%+v err=%v", defaultAll.Sessions, defaultAll.SessionsError)
	}
	data, err = os.ReadFile(paramsLog)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "{\"tool_type\":\"claude\",\"statuses\":[\"running\",\"stopped\"]}\n{}\n" {
		t.Fatalf("default selector was not empty: %q", data)
	}
}

func TestLoadWithSessionFilterRejectsUnsafeOrAmbiguousSelectorsBeforeCLI(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	paramsLog := filepath.Join(t.TempDir(), "session-params")
	t.Setenv("DESKTOP_TEST_CLI_PARAMS_LOG", paramsLog)
	for name, filter := range map[string]remote.SessionListFilter{
		"control in tool":   {ToolType: "claude\nprivate"},
		"control in status": {Statuses: []string{"running\x00"}},
		"duplicate status":  {Statuses: []string{"running", "running"}},
		"too many statuses": {Statuses: make([]string, 17)},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := remote.LoadWithSessionFilter(context.Background(), client, filter); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("error = %v, want CLI protocol rejection", err)
			}
		})
	}
	if data, err := os.ReadFile(paramsLog); err == nil && len(data) != 0 {
		t.Fatalf("unsafe selectors started CLI query: %q", data)
	}
}
