package remote

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type WireGuardCheckInput struct {
	ExpectedHome string  `json:"expected_home"`
	Config       *string `json:"config"`
}

type WireGuardTool struct {
	Name  string  `json:"name"`
	State string  `json:"state"`
	Path  *string `json:"path"`
}

type WireGuardConfigInspection struct {
	Path                  string   `json:"path"`
	State                 string   `json:"state"`
	Addresses             []string `json:"addresses"`
	DNS                   []string `json:"dns"`
	MTU                   *uint16  `json:"mtu"`
	PeerCount             *uint64  `json:"peer_count"`
	UnsupportedDirectives []string `json:"unsupported_directives"`
}

type WireGuardCheckReport struct {
	Home      string                    `json:"home"`
	Platform  string                    `json:"platform"`
	Elevation string                    `json:"elevation"`
	Config    WireGuardConfigInspection `json:"config"`
	Tools     []WireGuardTool           `json:"tools"`
}

type WireGuardPeer struct {
	PublicKey           string   `json:"public_key"`
	Endpoint            *string  `json:"endpoint"`
	AllowedIPs          []string `json:"allowed_ips"`
	LatestHandshake     uint64   `json:"latest_handshake"`
	ReceivedBytes       uint64   `json:"received_bytes"`
	SentBytes           uint64   `json:"sent_bytes"`
	PersistentKeepalive uint16   `json:"persistent_keepalive"`
}

type WireGuardInterface struct {
	Name       string          `json:"name"`
	PublicKey  *string         `json:"public_key"`
	ListenPort uint16          `json:"listen_port"`
	FWMark     uint32          `json:"fwmark"`
	Peers      []WireGuardPeer `json:"peers"`
}

type WireGuardStatusReport struct {
	Home       string               `json:"home"`
	Platform   string               `json:"platform"`
	Elevation  string               `json:"elevation"`
	Tool       WireGuardTool        `json:"tool"`
	State      string               `json:"state"`
	Interfaces []WireGuardInterface `json:"interfaces"`
}

// CheckWireGuard asks the CLI to inspect one configuration. Only its public
// summary crosses this boundary; the desktop never opens the configuration.
func CheckWireGuard(ctx context.Context, client cliadapter.Client, input WireGuardCheckInput) (WireGuardCheckReport, error) {
	if input.Config != nil {
		config := *input.Config
		input.Config = &config
	}
	if !validWireGuardPath(input.ExpectedHome) || input.ExpectedHome != client.Home {
		return WireGuardCheckReport{}, fault.CLIContextChanged
	}
	path := filepath.Join(input.ExpectedHome, "wireguard", "agent-remote.conf")
	if input.Config != nil {
		if !validWireGuardPath(*input.Config) {
			return WireGuardCheckReport{}, fault.WireGuardOutputPath
		}
		path = *input.Config
	}
	if err := checkWireGuardReadCapability(ctx, client, "wireguard.check"); err != nil {
		return WireGuardCheckReport{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.check", input, &raw); err != nil {
		return WireGuardCheckReport{}, err
	}
	report, err := decodeWireGuardCheck(raw)
	if err != nil || report.Home != input.ExpectedHome || report.Config.Path != path {
		return WireGuardCheckReport{}, fault.CLIProtocol
	}
	return report, nil
}

// InspectWireGuardStatus is an explicit, bounded observation. It never invokes
// an authorization prompt or retries a failed tool query.
func InspectWireGuardStatus(ctx context.Context, client cliadapter.Client) (WireGuardStatusReport, error) {
	if !validWireGuardPath(client.Home) {
		return WireGuardStatusReport{}, fault.CLIContextChanged
	}
	if err := checkWireGuardReadCapability(ctx, client, "wireguard.status"); err != nil {
		return WireGuardStatusReport{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "wireguard.status", struct {
		Home string `json:"expected_home"`
	}{client.Home}, &raw); err != nil {
		return WireGuardStatusReport{}, err
	}
	report, err := decodeWireGuardStatus(raw)
	if err != nil || report.Home != client.Home {
		return WireGuardStatusReport{}, fault.CLIProtocol
	}
	return report, nil
}

func checkWireGuardReadCapability(ctx context.Context, client cliadapter.Client, method string) error {
	var capabilities cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &capabilities); err != nil {
		return err
	}
	if capabilities.Validate() != nil || !capabilities.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	for _, operation := range capabilities.Operations {
		if operation.Method == method && operation.Effect == "read" && operation.Credential == "none" {
			return nil
		}
	}
	return &cliadapter.OperationError{Code: "unsupported_method"}
}
