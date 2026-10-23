package app_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestCapabilitiesDepsStatusUsesInspectThenExplicitRepair(t *testing.T) {
	_, tt, log := capabilityTester(t, "en")
	if err := tt.Click(locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("deps status")
	if !tt.HasText("agent-remote deps status") {
		t.Fatal("deps status command was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CommandDiagnosticsMethod("deps.inspect"))
	waitRemoteText(t, tt, locales.DiagnosticsDependencies())
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("deps status fell through to generic argv form")
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "deps.repair\n") {
		t.Fatalf("opening deps status implicitly repaired dependencies: %s", calls)
	}
	if err := tt.Click(locales.CommandDiagnosticsRepairDependencies()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CommandDiagnosticsRepairReview("agent-remote deps status --fix")) {
		t.Fatalf("dependency repair did not open an explicit review: %v", tt.Texts())
	}
	calls, err = os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "deps.repair\n") {
		t.Fatalf("opening dependency repair review submitted before confirmation: %s", calls)
	}
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CommandDiagnosticsMethod("deps.inspect"))
	if tt.HasText(locales.CommandDiagnosticsRepairReview("agent-remote deps status --fix")) {
		t.Fatal("canceling dependency repair left the review open")
	}
	calls, err = os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(calls), "deps.repair\n") {
		t.Fatalf("canceling dependency repair submitted the operation: %s", calls)
	}
	if err := tt.Click(locales.CommandDiagnosticsRepairDependencies()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CommandDiagnosticsRepairReview("agent-remote deps status --fix")) {
		t.Fatal("dependency repair review did not reopen")
	}
	if err := tt.Click(locales.CommandDiagnosticsRepairConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CommandDiagnosticsMethod("deps.repair"))
	waitRemoteText(t, tt, locales.LocalSetupStatus("ready"))
	calls, err = os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(calls), "deps.repair\n") {
		t.Fatalf("explicit dependency repair did not call deps.repair: %s", calls)
	}
}

func TestStructuredCommandDiagnosticsPagesUseControlMethods(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", errors.New("test component not installed")
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := tt.Click(locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if err := tt.Click(locales.CommandDiagnosticsStatus()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsCheckLabel("server"))
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("status structured workflow fell through to generic terminal form")
	}
	if err := tt.Click(locales.ReturnDiagnostics()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CommandDiagnosticsDoctor()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsChecks())
	if err := tt.Click(locales.ReturnDiagnostics()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CommandDiagnosticsSSH()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CommandDiagnosticsSSHVersion("OpenSSH_test"))
}

func TestStructuredDependenciesPageUsesDepsInspectMethod(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", errors.New("test component not installed")
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := tt.Click(locales.DiagnosticsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsSuggestion("repair_installation"))
	if err := tt.Click(locales.DependenciesTitle()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CommandDiagnosticsDependencies()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.DiagnosticsDependencies())
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("deps structured workflow fell through to generic terminal form")
	}
}
