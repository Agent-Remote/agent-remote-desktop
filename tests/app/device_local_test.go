package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestDevicePageShowsStructuredLocalStatus(t *testing.T) {
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
	tt.Type("device status")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.DeviceLocalTitle()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.DeviceLocalTitle()) {
		t.Fatalf("local Device card did not load: %v", tt.Texts())
	}
	if !tt.HasText(locales.DeviceLocalInstallation("installed")) || !tt.HasText(locales.DeviceLocalChecks("valid", "valid", "valid", "not_required")) {
		t.Fatalf("local Device status missing: %v", tt.Texts())
	}
	if !tt.HasText(locales.DeviceLocalPlanTitle("diagnose")) || !tt.HasText(locales.DeviceLocalPlanStage("inspect_bundle", "planned")) {
		t.Fatalf("local Device lifecycle plan missing: %v", tt.Texts())
	}
	if !tt.HasText(locales.CliFormReview() + " " + locales.DeviceAction("diagnose")) {
		t.Fatalf("device diagnose review action missing: %v", tt.Texts())
	}
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("device status fell through to generic terminal form")
	}
	if err := tt.Click(locales.CliFormReview() + " " + locales.DeviceAction("diagnose")); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CliFormTitle()) || !tt.HasText(locales.CliFormCommand()+": device diagnose") {
		t.Fatalf("device diagnose did not open the reviewed schema form: %v", tt.Texts())
	}
}
