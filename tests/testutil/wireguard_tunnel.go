package testutil

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func wireguardTunnelFixture(id string, params json.RawMessage, home, mode string) any {
	const prefix = "DESKTOP_TEST_WIREGUARD_TUNNEL_PLAN"
	captureWireGuard(prefix, params)
	waitControlBarrier(prefix+"_RELEASE", prefix+"_STARTED")
	if mode == "wg-tunnel-error" {
		wireguardFixtureError(id, "configuration_unavailable")
	}
	var input remote.WireGuardTunnelPlanInput
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	path := filepath.Join(home, "wireguard", "agent-remote.conf")
	if input.Config != nil {
		path = *input.Config
	}
	logical := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	platform, tunnel := runtime.GOOS, "wg-quick"
	if platform == "darwin" {
		platform = "macos"
	}
	if platform == "windows" {
		tunnel = "wireguard"
	}
	public := make([]byte, 32)
	public[0] = 1
	key := base64.StdEncoding.EncodeToString(public)
	fileIdentity, contentHash := strings.Repeat("a", 64), strings.Repeat("b", 64)
	mtu, peerCount := uint16(1000), uint64(1)
	plan := remote.WireGuardTunnelPlan{
		Version: 1, Home: home, Platform: platform, Action: input.Action,
		Config:  remote.WireGuardConfigInspection{Path: path, State: "ready", Addresses: []string{"10.77.0.2/32"}, DNS: []string{}, MTU: &mtu, PeerCount: &peerCount, UnsupportedDirectives: []string{}},
		Binding: &remote.WireGuardTunnelBinding{FileIdentity: fileIdentity, ContentSHA256: contentHash, PublicKey: key},
		Tools:   []remote.WireGuardTunnelTool{},
		Target:  remote.WireGuardTargetObservation{LogicalName: logical, State: "absent", StatusState: "not_queried"},
		Issues:  []string{},
	}
	for _, name := range []string{"wg", tunnel} {
		toolPath, identity, digest := filepath.Join(home, "bin", name), strings.Repeat("c", 64), strings.Repeat("d", 64)
		plan.Tools = append(plan.Tools, remote.WireGuardTunnelTool{Name: name, State: "available", Path: &toolPath, Identity: &identity, SHA256: &digest})
	}
	if platform == "macos" {
		reason := "mapping_missing"
		plan.Target.State, plan.Target.Reason = "unknown", &reason
		plan.Issues = []string{"target_unknown"}
	}
	if input.Action == "down" {
		plan.Target.State, plan.Target.StatusState, plan.Target.Reason = "present", "observed", nil
		plan.Issues = []string{}
		identity := &remote.WireGuardTargetIdentity{}
		switch platform {
		case "linux":
			identity.Kind, identity.Linux = "linux_interface", &remote.WireGuardLinuxTarget{Name: logical, Index: 12, Namespace: strings.Repeat("e", 64), PublicKey: &key}
		case "macos":
			identity.Kind, identity.MacOS = "macos_interface", &remote.WireGuardMacOSTarget{Name: "utun7", Index: 12, MappingIdentity: strings.Repeat("e", 64), SocketIdentity: strings.Repeat("f", 64), PublicKey: &key}
		case "windows":
			identity.Kind, identity.Windows = "windows_service", &remote.WireGuardWindowsTarget{ServiceName: "WireGuardTunnel$" + logical, BinaryPath: filepath.Join(home, "bin", "wireguard.exe"), ConfigPath: path, ConfigIdentity: &fileIdentity, ServiceState: 4, StartType: 2, Account: "LocalSystem", PublicKey: &key}
		}
		plan.Target.Identity = identity
	}
	if mode == "wg-tunnel-unknown" {
		reason := "native_query_failed"
		plan.Target = remote.WireGuardTargetObservation{LogicalName: logical, State: "unknown", Reason: &reason, StatusState: "not_queried"}
		plan.Issues = []string{"target_unknown"}
	}
	data, err := json.Marshal(plan)
	if err != nil {
		os.Exit(5)
	}
	var result map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if decoder.Decode(&result) != nil {
		os.Exit(5)
	}
	return patchWireGuardFixture(result, prefix+"_PATCH")
}
