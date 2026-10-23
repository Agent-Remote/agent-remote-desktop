package remote_test

import (
	"context"
	"errors"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestLoadNodesReturnsBoundedInventory(t *testing.T) {
	home := t.TempDir()
	result, err := remote.LoadNodes(context.Background(), cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home})
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.Operation != remote.NodesListMethod || len(result.Items) != 1 || result.Items[0].ID == "" {
		t.Fatalf("unexpected nodes: %+v", result)
	}
}

func TestLoadNodesFailsClosedOnContractDrift(t *testing.T) {
	for _, scenario := range []string{"nodes-unknown-field", "nodes-context-changed", "nodes-duplicate"} {
		t.Run(scenario, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", scenario)
			_, err := remote.LoadNodes(context.Background(), cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()})
			if !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("scenario %s returned %v", scenario, err)
			}
		})
	}
}

func TestNodesListRequiresUserCredentialContract(t *testing.T) {
	caps := cliadapter.Capabilities{Version: cliadapter.ProtocolVersion, CLIVersion: "test", Platform: cliadapter.PlatformName(runtime.GOOS), Operations: []cliadapter.Capability{{Method: remote.NodesListMethod, Effect: "read", Credential: "user"}}}
	if !remote.NodesListSupported(caps) {
		t.Fatal("user credential must satisfy nodes administrator contract")
	}
	caps.Operations[0].Credential = "device"
	if remote.NodesListSupported(caps) {
		t.Fatal("device credential must not satisfy nodes administrator contract")
	}
}
