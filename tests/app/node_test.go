package app_test

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestNodeInstallUsesStructuredNodePickerAndReview(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	terminalBinary := ""
	if runtime.GOOS == "windows" {
		terminalBinary = testutil.InstallTerminalFixture(t)
	}
	lookups := 0
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			lookups++
			// Capability and node discovery use the JSON control fixture. The
			// confirmed install is the first managed terminal launch; use a
			// native ConPTY fixture there so Windows does not run this test
			// executable as an Agent and retain its copied image during cleanup.
			if terminalBinary != "" && lookups > 2 {
				return terminalBinary, nil
			}
			return binary, nil
		}
		return "", errors.New("test Agent unavailable")
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for !tt.HasText(locales.CapabilitiesState("unverified")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("node install")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for !tt.HasText(locales.NodeSummary("Studio Node", "ready")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.NodeSummary("Studio Node", "ready")) {
		t.Fatalf("node picker did not load: %v", tt.Texts())
	}
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("node install fell through to generic CLI form")
	}
	if err := tt.Click(locales.NodeSelect()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for !tt.HasText(locales.NodeReview()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if err := tt.Click(locales.NodeReview()); err != nil {
		t.Fatalf("review node: %v; texts=%v", err, tt.Texts())
	}
	if !tt.HasText(locales.NodeArgv("node install --node 11111111-1111-4111-8111-111111111111 --yes")) {
		t.Fatalf("frozen node argv missing before confirm: %v", tt.Texts())
	}
	if err := tt.Click(locales.NodeConfirm()); err != nil {
		t.Fatalf("click confirm: %v; texts=%v", err, tt.Texts())
	}
}
