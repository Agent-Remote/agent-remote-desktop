package remote_test

import (
	"context"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestCommandDiagnosticsContractsProjectStructuredResults(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}

	status, err := remote.InspectStatus(context.Background(), client, false)
	if err != nil || status.Operation != "status.inspect" || status.Remote == nil {
		t.Fatalf("status = %+v, err = %v", status, err)
	}
	doctor, err := remote.InspectDoctor(context.Background(), client, remote.CommandDiagnosticOptions{Online: true, DeepCheck: true, VerifyVersions: true})
	if err != nil || doctor.Operation != "doctor.inspect" || doctor.Setup != nil {
		t.Fatalf("doctor = %+v, err = %v", doctor, err)
	}
	deps, err := remote.InspectDependenciesCommand(context.Background(), client, true)
	if err != nil || deps.Operation != "deps.inspect" || deps.Dependencies == nil {
		t.Fatalf("deps = %+v, err = %v", deps, err)
	}
	repaired, err := remote.RepairDoctor(context.Background(), client, remote.CommandRepairOptions{CommandDiagnosticOptions: remote.CommandDiagnosticOptions{Online: false, DeepCheck: false, VerifyVersions: false}, Confirmed: true, ExpectedHome: home})
	if err != nil || repaired.Setup == nil || !repaired.Setup.Complete {
		t.Fatalf("doctor repair = %+v, err = %v", repaired, err)
	}
	depsRepaired, err := remote.RepairDependenciesCommand(context.Background(), client, remote.CommandRepairOptions{CommandDiagnosticOptions: remote.CommandDiagnosticOptions{VerifyVersions: false}, Confirmed: true, ExpectedHome: home})
	if err != nil || depsRepaired.Repair == nil || !depsRepaired.Repair.Complete {
		t.Fatalf("deps repair = %+v, err = %v", depsRepaired, err)
	}
	ssh, err := remote.InspectSSH(context.Background(), client, nil, nil, nil)
	if err != nil || ssh.Operation != "ssh.inspect" || ssh.Authorization.Status != "not_requested" {
		t.Fatalf("ssh = %+v, err = %v", ssh, err)
	}
}

func TestCommandDiagnosticsRejectsCrossHomeAndUnknownFields(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_COMMAND_DIAGNOSTICS_MODE", "command-diagnostics-wrong-home")
	_, err := remote.InspectStatus(context.Background(), client, false)
	if err != fault.CLIContextChanged {
		t.Fatalf("wrong home error = %v, want %v", err, fault.CLIContextChanged)
	}
	t.Setenv("DESKTOP_TEST_COMMAND_DIAGNOSTICS_MODE", "command-diagnostics-unknown-field")
	_, err = remote.InspectStatus(context.Background(), client, false)
	if err != fault.CLIProtocol {
		t.Fatalf("unknown field error = %v, want %v", err, fault.CLIProtocol)
	}
}

func TestCommandDiagnosticsPreflightsRepairAndSSHContext(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	if _, err := remote.RepairDoctor(context.Background(), client, remote.CommandRepairOptions{ExpectedHome: home}); err != fault.LocalSetupReview {
		t.Fatalf("unconfirmed repair error = %v, want %v", err, fault.LocalSetupReview)
	}
	if _, err := remote.RepairDoctor(context.Background(), client, remote.CommandRepairOptions{Confirmed: true, ExpectedHome: home + "-other"}); err != fault.CLIContextChanged {
		t.Fatalf("cross-home repair error = %v, want %v", err, fault.CLIContextChanged)
	}
	if _, err := remote.InspectSSH(context.Background(), client, strptr("11111111-1111-4111-8111-111111111111"), nil, nil); err != fault.CLIProtocol {
		t.Fatalf("partial SSH context error = %v, want %v", err, fault.CLIProtocol)
	}
	nilSession := strptr("00000000-0000-0000-0000-000000000000")
	server := strptr("https://test.invalid")
	device := strptr("11111111-1111-4111-8111-111111111111")
	if _, err := remote.InspectSSH(context.Background(), client, nilSession, server, device); err != fault.CLIProtocol {
		t.Fatalf("nil SSH session UUID was accepted: %v", err)
	}
}

func TestCommandDiagnosticsRejectsRepairReceiptFromAnotherHome(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_COMMAND_DIAGNOSTICS_MODE", "command-diagnostics-setup-wrong-home")
	_, err := remote.RepairDoctor(context.Background(), client, remote.CommandRepairOptions{CommandDiagnosticOptions: remote.CommandDiagnosticOptions{}, Confirmed: true, ExpectedHome: home})
	if err != fault.CLIProtocol {
		t.Fatalf("wrong setup receipt error = %v, want %v", err, fault.CLIProtocol)
	}
}

func TestCommandDiagnosticsRejectsMalformedSSHAuthorizationProjection(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	session := "11111111-1111-4111-8111-111111111111"
	server := "https://test.invalid"
	device := "22222222-2222-4222-8222-222222222222"
	t.Setenv("DESKTOP_TEST_COMMAND_DIAGNOSTICS_MODE", "command-diagnostics-ssh-malformed")
	if _, err := remote.InspectSSH(context.Background(), client, &session, &server, &device); err != fault.CLIProtocol {
		t.Fatalf("malformed SSH authorization was accepted: %v", err)
	}
}
