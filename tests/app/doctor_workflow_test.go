package app_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/locales"
)

func TestCapabilitiesRoutesDoctorToReviewedWorkflow(t *testing.T) {
	_, tt, _ := capabilityTester(t, "en")
	if err := openPage(tt, locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("agent-remote doctor")
	if !tt.HasText("agent-remote doctor") {
		t.Fatal("doctor command was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CliDoctorHelp()) || !tt.HasText(locales.CommandDiagnosticsMethod("doctor.inspect")) {
		t.Fatal("doctor did not use the structured workflow")
	}
	waitRemoteText(t, tt, locales.DiagnosticsChecks())
	if err := tt.Click(locales.CommandDiagnosticsRepairDoctor()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CommandDiagnosticsRepairReview("agent-remote doctor --fix")) {
		t.Fatalf("doctor repair did not open an explicit review: %v", tt.Texts())
	}
	if err := tt.Click(locales.CommandDiagnosticsRepairConfirm()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CommandDiagnosticsMethod("doctor.repair"))
	if !tt.HasText(locales.CommandDiagnosticsMethod("doctor.repair")) {
		t.Fatalf("doctor repair workflow was not rendered: %v", tt.Texts())
	}
}
