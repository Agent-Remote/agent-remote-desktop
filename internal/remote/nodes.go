package remote

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const NodesListMethod = "nodes.list"

// NodeOverview is the context-bound managed Node inventory used by the node
// install page. It contains only public routing metadata.
type NodeOverview struct {
	SchemaVersion int                      `json:"schema_version"`
	Operation     string                   `json:"operation"`
	Context       cliadapter.ConfigContext `json:"context"`
	Items         []cliadapter.NodeRecord  `json:"items"`
}

func NodesListSupported(c cliadapter.Capabilities) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method == NodesListMethod {
			return op.Effect == "read" && op.Credential == "user" && op.Confirmation == ""
		}
	}
	return false
}

func LoadNodes(ctx context.Context, client cliadapter.Client) (NodeOverview, error) {
	var result NodeOverview
	if client.Binary == "" || client.Home == "" {
		return result, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return result, err
	}
	if !NodesListSupported(caps) {
		return result, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var expected cliadapter.ConfigContext
	if err := client.Call(ctx, "context", struct{}{}, &expected); err != nil {
		return result, err
	}
	if !validNodesContext(expected, client.Home) {
		return result, fault.CLIProtocol
	}
	var raw json.RawMessage
	if err := client.Call(ctx, NodesListMethod, struct{}{}, &raw); err != nil {
		return result, err
	}
	if err := decodeStrict(raw, &result); err != nil || result.SchemaVersion != 1 || result.Operation != NodesListMethod || !sameNodesContext(result.Context, expected) || !validNodes(result) {
		return NodeOverview{}, fault.CLIProtocol
	}
	return result, nil
}

func validNodesContext(value cliadapter.ConfigContext, home string) bool {
	return filepath.Clean(value.Home) == filepath.Clean(home) && value.ServerURL != nil && strings.TrimSpace(*value.ServerURL) != "" && (value.DeviceID == nil || validID(*value.DeviceID))
}

func sameNodesContext(a, b cliadapter.ConfigContext) bool {
	if !validNodesContext(a, b.Home) || !validNodesContext(b, b.Home) || *a.ServerURL != *b.ServerURL {
		return false
	}
	if a.DeviceID == nil || b.DeviceID == nil {
		return a.DeviceID == nil && b.DeviceID == nil
	}
	return *a.DeviceID == *b.DeviceID
}

func validNodes(value NodeOverview) bool {
	if len(value.Items) > 256 || value.Items == nil {
		return false
	}
	seen := make(map[string]struct{}, len(value.Items))
	for _, node := range value.Items {
		if !validID(node.ID) || node.Name == "" || !validText(node.Name, 256) || !validText(node.Status, 64) {
			return false
		}
		if _, ok := seen[node.ID]; ok {
			return false
		}
		seen[node.ID] = struct{}{}
		if !optionalText(node.SSHHost, 512) || !optionalText(node.SSHUser, 128) || !optionalText(node.Version, 128) {
			return false
		}
		if node.SSHPort != nil && *node.SSHPort == 0 {
			return false
		}
	}
	return true
}
