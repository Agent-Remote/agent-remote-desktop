package app_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// Utility commands such as status are allowed before a workspace is selected.
// Status uses the structured control page and therefore does not create a
// terminal utility project or route through generic argv review.
func TestStructuredStatusRunsWithoutWorkspace(t *testing.T) {
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
	tt.Type("agent-remote status")
	tt.Frame()
	if !tt.HasText("agent-remote status") {
		t.Fatal("status command was not discoverable")
	}
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CommandDiagnosticsMethod("status.inspect"))
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("status fell through to generic argv review")
	}
}

// Device lifecycle commands are local/home-scoped workflows. They still run
// in the managed terminal, but selecting a workspace must not be a prerequisite
// for reviewing or starting them from the command catalog.
func TestDeviceTerminalWorkflowRunsWithoutWorkspace(t *testing.T) {
	t.Setenv("DESKTOP_TERMINAL_TESTS", "1")
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
	tt.Type("device install")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CliFormTitle())
	if err := tt.Click("--source · " + locales.CliFormRequired()); err != nil {
		t.Fatal(err)
	}
	tt.Type("/tmp/Agent-Remote-Device.zip")
	if err := tt.Click(locales.CliFormReview()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CliFormStart()) {
		t.Fatalf("device install review did not expose start action: %v", tt.Texts())
	}
	if err := tt.Click(locales.CliFormStart()); err != nil {
		t.Fatal(err)
	}
	// startGenericCLI clears the form before launching the managed terminal.
	// If device were incorrectly classified as project-scoped, the start
	// button would remain on this review with a project-required error.
	deadline := time.Now().Add(5 * time.Second)
	for tt.HasText(locales.CliFormTitle()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if tt.HasText(locales.CliFormTitle()) || tt.HasText(locales.CliFormProjectRequired()) {
		t.Fatalf("device workflow still required a project: %v", tt.Texts())
	}
}

func TestSkillExportReviewRejectsParentTraversalOutput(t *testing.T) {
	_, tt, _ := capabilityTester(t, "en")
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
	tt.Type("skill state export")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CliFormTitle())
	outputLabel := "--output · " + locales.CliFormRequired()
	if !tt.HasText(outputLabel) {
		t.Fatalf("export review omitted required output field: %v", tt.Texts())
	}
	if err := tt.Click(outputLabel); err != nil {
		t.Fatal(err)
	}
	tt.Type("./exports/../secrets")
	if err := tt.Click(locales.CliFormReview()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CliDedicatedUnsafePath()) {
		t.Fatalf("unsafe output path was not rejected: %v", tt.Texts())
	}
}

// fclaude run/new are context-aware remote workflows. The command catalog
// routes them to the exact remote session inventory/create review instead of
// manufacturing a generic argv form. Native fclaude passthrough remains
// available from the workbench terminal entry point.
func TestFClaudeRunAndNewUseRemoteSessionWorkflows(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "remote-create")
	agentBinary := testutil.InstallControlHelper(t)
	fclaudeBinary := filepath.Join(t.TempDir(), "fclaude")
	if err := os.Symlink(agentBinary, fclaudeBinary); err != nil {
		t.Fatal(err)
	}
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })

	for _, command := range []string{"run", "new", "list"} {
		t.Run(command, func(t *testing.T) {
			home, projectPath := t.TempDir(), t.TempDir()
			state := storage.State{Preferences: workspace.Defaults(), Projects: []workspace.Project{{ID: "project", Name: "Project", Path: projectPath}}}
			state.Preferences.SelectedProject = "project"
			testutil.StoreWorkspace(t, home, state)
			a, err := app.NewWithOptions(home, app.Options{FindExecutable: func(name string) (string, error) {
				switch name {
				case "agent-remote":
					return agentBinary, nil
				case "fclaude":
					return fclaudeBinary, nil
				default:
					return "", errors.New("test component not installed")
				}
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
			tt.Type("fclaude " + command)
			if !tt.HasText("fclaude " + command) {
				t.Fatal("fclaude command was not discoverable")
			}
			open := locales.CliFormOpenRemote()
			if err := tt.Click(open); err != nil {
				t.Fatal(err)
			}
			if command == "new" {
				// The create route opens a modal over the remote page, so the
				// modal's title is the stable visible hand-off marker.
				waitRemoteText(t, tt, locales.CreateRemoteSession())
				if !tt.HasText(locales.SessionCreateTool()) {
					t.Fatal("fclaude new did not open the structured create review")
				}
			} else {
				waitRemoteText(t, tt, "https://test.invalid")
				if !tt.HasText(locales.RemoteSessions()) {
					t.Fatal("fclaude command did not open the remote session inventory")
				}
			}
		})
	}
}
