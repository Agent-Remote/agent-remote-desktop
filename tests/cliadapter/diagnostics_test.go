package cliadapter_test

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

func TestDiagnosticsUsesTypedQueriesAndVersionChecksAreExplicit(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	for _, versions := range []bool{false, true} {
		report, err := remote.InspectDiagnostics(context.Background(), client, true, versions, true)
		if err != nil {
			t.Fatal(err)
		}
		if !report.Online || report.VerifyVersions != versions || report.Home != client.Home || report.ManifestSource != "default" || report.Checks[0].Status != "missing" || len(report.Dependencies) != 1 {
			t.Fatalf("invalid report: %+v", report)
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "capabilities\ndiagnostics.inspect\ncapabilities\ndiagnostics.inspect\n" {
		t.Fatalf("unexpected query/retry: %s", data)
	}
}

func TestDiagnosticsWithRealRustCLILeavesMissingHomeUninitialized(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY to verify the real Rust control protocol")
	}
	client := cliadapter.Client{Binary: binary, Home: filepath.Join(t.TempDir(), "absent")}
	report, err := remote.InspectDiagnostics(context.Background(), client, false, false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Dependencies) == 0 || report.ManifestSource != "default" {
		t.Fatalf("missing release defaults: %+v", report)
	}
	for _, check := range report.Checks {
		if check.ID == "credential" && check.Status != "not_checked" {
			t.Fatal("passive checks claimed sign-in authority")
		}
	}
	dependencies, err := remote.InspectDependencies(context.Background(), client, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(dependencies.Dependencies) != len(report.Dependencies) {
		t.Fatal("dependency-only report differs")
	}
	if _, err := os.Stat(client.Home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read-only diagnostics initialized state: %v", err)
	}
}

func TestDiagnosticsRejectsUntrustedOrIncompleteReports(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"diagnostics-wrong-home", "diagnostics-missing-checks", "diagnostics-bad-status", "diagnostics-online-mismatch"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			report, err := remote.InspectDiagnostics(context.Background(), client, false, false, false)
			if !errors.Is(err, fault.CLIProtocol) || report.Home != "" {
				t.Fatalf("accepted invalid diagnostic receipt: %+v %v", report, err)
			}
		})
	}
}

func TestDiagnosticOperationsRejectIncompatibleCapabilitiesBeforeInspection(t *testing.T) {
	for _, mode := range []string{"other-platform", "missing-platform", "duplicate-capability", "invalid-effect"} {
		for _, method := range []string{"diagnostics.inspect", "dependencies.inspect"} {
			t.Run(mode+"/"+method, func(t *testing.T) {
				client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
				log := filepath.Join(t.TempDir(), "calls")
				t.Setenv("DESKTOP_TEST_CLI_LOG", log)
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				var err error
				if method == "diagnostics.inspect" {
					_, err = remote.InspectDiagnostics(context.Background(), client, true, true, true)
				} else {
					_, err = remote.InspectDependencies(context.Background(), client, true)
				}
				if !errors.Is(err, fault.CLIProtocol) {
					t.Fatalf("incompatible capabilities accepted: %v", err)
				}
				calls, err := os.ReadFile(log)
				if err != nil || string(calls) != "capabilities\n" {
					t.Fatalf("incompatible CLI reached inspection: %q (%v)", calls, err)
				}
			})
		}
	}
}
