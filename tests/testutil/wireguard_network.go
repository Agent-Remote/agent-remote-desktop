package testutil

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func wireguardNetworkFixture(method, id string, params json.RawMessage, home, mode string) any {
	kind := strings.ToUpper(strings.TrimPrefix(method, "wireguard."))
	prefix := "DESKTOP_TEST_WIREGUARD_" + strings.ReplaceAll(kind, ".", "_")
	captureWireGuard(prefix, params)
	waitControlBarrier(prefix+"_RELEASE", prefix+"_STARTED")
	if mode == "wg-network-slow" {
		time.Sleep(time.Minute)
	}
	if mode == "wg-network-error" {
		wireguardFixtureError(id, "configuration_unavailable")
	}
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	tool := map[string]any{"name": "wg", "state": "available", "path": filepath.Join(home, "bin", "wg")}
	result := map[string]any{"home": home, "platform": platform, "elevation": "standard"}
	if method == "wireguard.bridge.status" {
		return patchWireGuardFixture(map[string]any{
			"version": 1, "home": home, "platform": platform, "state": "unsupported",
			"installation": "not_installed", "approval": "not_requested",
			"protocol": "unsupported", "authentication": "unavailable", "execution": "unavailable",
		}, prefix+"_PATCH")
	}
	if method == "wireguard.check" {
		var input struct {
			Home   string  `json:"expected_home"`
			Config *string `json:"config"`
		}
		if json.Unmarshal(params, &input) != nil {
			os.Exit(4)
		}
		path := filepath.Join(home, "wireguard", "agent-remote.conf")
		if input.Config != nil {
			path = *input.Config
		}
		config := map[string]any{"path": path, "state": "ready", "addresses": []string{"10.77.0.2/32"}, "dns": []string{"1.1.1.1"}, "mtu": 1000, "peer_count": 2, "unsupported_directives": []string{}}
		if mode == "wg-network-unsupported-config" {
			config["state"], config["unsupported_directives"] = "unsupported", []string{"PostUp", "SaveConfig"}
		}
		if mode == "wg-network-missing-config" {
			config["state"], config["addresses"], config["dns"], config["mtu"], config["peer_count"] = "missing", []string{}, []string{}, nil, nil
		}
		tunnel := "wg-quick"
		if platform == "windows" {
			tunnel = "wireguard"
		}
		result["config"], result["tools"] = config, []any{tool, map[string]any{"name": tunnel, "state": "available", "path": filepath.Join(home, "bin", tunnel)}}
	} else {
		peers := []any{}
		count := 2
		if mode == "wg-network-many" {
			count = 25
		}
		for index := 0; index < count; index++ {
			key := make([]byte, 32)
			key[0] = byte(index + 1)
			peers = append(peers, map[string]any{"public_key": base64.StdEncoding.EncodeToString(key), "endpoint": fmt.Sprintf("127.0.0.1:%d", 51820+index), "allowed_ips": []string{fmt.Sprintf("10.77.0.%d/32", index+1)}, "latest_handshake": uint64(index) * 1700000000, "received_bytes": uint64(index) * 1024, "sent_bytes": uint64(index) * 2048, "persistent_keepalive": 25})
		}
		if mode == "wg-network-format" {
			peers[0].(map[string]any)["latest_handshake"] = uint64(1700000000)
			peers[0].(map[string]any)["received_bytes"] = ^uint64(0)
		}
		result["tool"], result["state"], result["interfaces"] = tool, "observed", []any{map[string]any{"name": "wg-desktop", "public_key": nil, "listen_port": 51820, "fwmark": 0, "peers": peers}}
		if mode == "wg-network-empty" {
			result["interfaces"] = []any{}
		}
		if mode == "wg-network-failed" {
			result["state"], result["interfaces"] = "query_failed", []any{}
		}
		if mode == "wg-network-many" {
			interfaces := result["interfaces"].([]any)
			for index := 1; index < 5; index++ {
				interfaces = append(interfaces, map[string]any{"name": fmt.Sprintf("wg-extra-%d", index), "public_key": nil, "listen_port": 51820, "fwmark": 0, "peers": []any{}})
			}
			result["interfaces"] = interfaces
		}
	}
	return patchWireGuardFixture(result, prefix+"_PATCH")
}
