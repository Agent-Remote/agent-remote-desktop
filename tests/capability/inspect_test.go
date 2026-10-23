package capability_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/capability"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestMain(m *testing.M) {
	if testutil.ControlHelper() {
		return
	}
	os.Exit(m.Run())
}

func TestInspectionIsPassiveAndPreservesUnknownPlatformAndMissingLauncher(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "legacy-schema")
	snapshot := capability.Inspect(context.Background(), cliadapter.Client{Binary: binary, Home: t.TempDir()}, "")
	if snapshot.ProtocolError != nil || snapshot.ContextError != nil || snapshot.Catalogs[0].Err != nil || snapshot.Catalogs[1].Err == nil {
		t.Fatalf("%+v", snapshot)
	}
	for _, command := range snapshot.Catalogs[0].Schema.Commands {
		if len(command.Platforms) != 0 {
			t.Fatal("legacy platform inferred")
		}
	}
	data, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "capabilities\ncontext\nschema:agent-remote\n" {
		t.Fatalf("inspection performed unexpected operation: %q", data)
	}
}

func TestInspectionRejectsProtocolAndVersionMismatch(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	for _, mode := range []string{"wrong-version", "wrong-id", "schema-version-changed", "missing-platform", "duplicate-capability", "invalid-effect"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			s := capability.Inspect(context.Background(), cliadapter.Client{Binary: binary, Home: t.TempDir()}, "")
			if s.ProtocolError == nil && s.Catalogs[0].Err == nil {
				t.Fatal("accepted incompatible CLI")
			}
		})
	}
}

func TestDiscoveryCancellationAndOutputBound(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	for _, mode := range []string{"slow-schema", "oversized-schema"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			start := time.Now()
			if _, err := cliadapter.Discover(ctx, binary, "agent-remote"); err == nil {
				t.Fatal("accepted unbounded schema")
			}
			if time.Since(start) > 4*time.Second {
				t.Fatal("discovery did not stop")
			}
		})
	}
}

func TestRealCLIPlatformSchemaIsPassive(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real CLI contract")
	}
	home := t.TempDir()
	t.Setenv("AGENT_REMOTE_HOME", home)
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	s := capability.Inspect(context.Background(), cliadapter.Client{Binary: binary, Home: home}, "")
	if s.ProtocolError != nil || s.ContextError != nil || s.Catalogs[0].Err != nil {
		t.Fatalf("%+v", s)
	}
	seen := false
	for _, command := range s.Catalogs[0].Schema.Commands {
		if strings.Join(command.Path, " ") == "device install" {
			seen = true
			if capability.PlatformState(command, "windows") != "unsupported" {
				t.Fatal("lost CLI platform restriction")
			}
		}
		if strings.Join(command.Path, " ") == "device revoke" && len(command.Platforms) != 0 {
			t.Fatal("inherited parent limitation")
		}
	}
	if !seen {
		t.Fatal("missing device command")
	}
	entries, err := os.ReadDir(home)
	if err != nil || len(entries) != 0 {
		t.Fatalf("passive inspection changed home: %v %v", entries, err)
	}
}
