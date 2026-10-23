package cliadapter_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
)

// TestRealSchemaCommandsHaveDesktopCoverage audits the release binaries when
// DESKTOP_CLI_BINARY/DESKTOP_FCLAUDE_BINARY are supplied.  Command discovery
// alone is not GUI coverage: every executable leaf must either have a
// reviewed capability mapping, a dedicated terminal contract, or an explicit
// page route whose workflow owns its capability checks.  The test is skipped
// in ordinary unit runs because the Rust binaries are external build inputs.
func TestRealSchemaCommandsHaveDesktopCoverage(t *testing.T) {
	tests := []struct {
		executable string
		env        string
	}{
		{executable: "agent-remote", env: "DESKTOP_CLI_BINARY"},
		{executable: "fclaude", env: "DESKTOP_FCLAUDE_BINARY"},
	}
	for _, tc := range tests {
		t.Run(tc.executable, func(t *testing.T) {
			binary := strings.TrimSpace(os.Getenv(tc.env))
			if binary == "" {
				t.Skip("set " + tc.env + " for the real CLI schema coverage audit")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			schema, err := cliadapter.Discover(ctx, binary, tc.executable)
			if err != nil {
				t.Fatal(err)
			}
			for _, command := range schema.Commands {
				// Parent groups are not invocations unless the CLI documents a
				// default action.  Both fclaude's root and forward's parent are
				// executable defaults and are covered explicitly below.
				if command.HasSubcommands && !actionableParent(tc.executable, command.Path) {
					continue
				}
				if _, _, ok := cliadapter.KnownCapability(tc.executable, command.Path); ok {
					continue
				}
				if _, ok := cliadapter.Dedicated(tc.executable, command.Path); ok {
					continue
				}
				if explicitPageRoute(tc.executable, command.Path) {
					continue
				}
				t.Errorf("schema command has no desktop coverage route: %s %s", tc.executable, strings.Join(command.Path, " "))
			}
			for _, command := range schema.Commands {
				if command.HasSubcommands && !actionableParent(tc.executable, command.Path) {
					continue
				}
				workflow, hasWorkflow := cliadapter.Dedicated(tc.executable, command.Path)
				values, _ := cliadapter.InitialDedicatedFormValues(tc.executable, command)
				for _, argument := range command.Arguments {
					if cliadapter.IsDesktopManagedArgument(argument) || cliadapter.IsBooleanArgument(argument) || len(values[argument.ID]) > 0 {
						continue
					}
					if len(argument.Choices) > 0 {
						values[argument.ID] = []string{argument.Choices[0]}
						continue
					}
					if argument.Required || argument.MinValues > 0 {
						values[argument.ID] = []string{"coverage-fixture"}
					}
				}
				// Dedicated exact attach workflows require identity flags even
				// though the CLI keeps them optional for legacy invocations. Seed
				// only those reviewed flags in this synthetic coverage fixture;
				// production forms still obtain their values from the selected
				// context-bound page.
				if hasWorkflow {
					for _, required := range workflow.RequiredFlags {
						for _, argument := range command.Arguments {
							if "--"+argument.Long != required && "-"+argument.Short != required {
								continue
							}
							if len(values[argument.ID]) == 0 {
								values[argument.ID] = []string{"coverage-fixture"}
							}
						}
					}
				}
				if _, err := cliadapter.BuildArgv(command, values); err != nil {
					t.Errorf("schema arguments cannot be represented by generic form for %s %s: %v", tc.executable, strings.Join(command.Path, " "), err)
					continue
				}
				if hasWorkflow {
					argv, err := cliadapter.BuildArgv(command, values)
					if err != nil {
						t.Errorf("dedicated fixture cannot build argv for %s %s: %v", tc.executable, strings.Join(command.Path, " "), err)
						continue
					}
					if err := cliadapter.ValidateDedicatedArgv(workflow, argv, true); err != nil {
						t.Errorf("dedicated fixture cannot pass reviewed argv for %s %s: %v", tc.executable, strings.Join(command.Path, " "), err)
					}
				}
			}
		})
	}
}

func actionableParent(executable string, path []string) bool {
	if executable == "fclaude" && len(path) == 0 {
		return true
	}
	return len(path) == 1 && path[0] == "forward" && (executable == "agent-remote" || executable == "fclaude")
}

func explicitPageRoute(executable string, path []string) bool {
	if executable != "agent-remote" || len(path) != 1 {
		return false
	}
	// init is a composed setup wizard and login is a credential-safe page;
	// neither is represented by one desktop-control capability or terminal
	// operation, so their routes are intentionally owned by internal/app.
	return path[0] == "init" || path[0] == "login"
}
