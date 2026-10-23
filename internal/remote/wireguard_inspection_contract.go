package remote

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func wireGuardObject(raw json.RawMessage, fields ...string) (map[string]json.RawMessage, bool) {
	var object map[string]json.RawMessage
	if json.Unmarshal(raw, &object) != nil || len(object) != len(fields) {
		return nil, false
	}
	for _, name := range fields {
		if len(object[name]) == 0 {
			return nil, false
		}
	}
	return object, true
}

func wireGuardRequiredNumbers(object map[string]json.RawMessage, fields ...string) bool {
	for _, name := range fields {
		if len(object[name]) == 0 || bytes.Equal(bytes.TrimSpace(object[name]), []byte("null")) {
			return false
		}
	}
	return true
}

func validWireGuardObservation(home, platform, elevation string) bool {
	return validWireGuardPath(home) && platform == cliadapter.PlatformName(runtime.GOOS) &&
		(elevation == "elevated" || elevation == "standard" || elevation == "unknown")
}

func validWireGuardTool(tool WireGuardTool, name string, raw json.RawMessage) bool {
	if _, ok := wireGuardObject(raw, "name", "state", "path"); !ok || tool.Name != name {
		return false
	}
	if tool.State == "missing" {
		return tool.Path == nil
	}
	return (tool.State == "available" || tool.State == "not_executable") && tool.Path != nil && validWireGuardPath(*tool.Path)
}

func decodeWireGuardCheck(raw json.RawMessage) (WireGuardCheckReport, error) {
	var report WireGuardCheckReport
	object, ok := wireGuardObject(raw, "home", "platform", "elevation", "config", "tools")
	if !ok || decodeWireGuardJSON(raw, &report) != nil || !validWireGuardObservation(report.Home, report.Platform, report.Elevation) {
		return report, fault.CLIProtocol
	}
	var tools []json.RawMessage
	if json.Unmarshal(object["tools"], &tools) != nil || len(tools) != 2 || len(report.Tools) != 2 {
		return report, fault.CLIProtocol
	}
	tunnel := "wg-quick"
	if report.Platform == "windows" {
		tunnel = "wireguard"
	}
	if !validWireGuardTool(report.Tools[0], "wg", tools[0]) || !validWireGuardTool(report.Tools[1], tunnel, tools[1]) || !validWireGuardConfigInspection(report.Config, object["config"]) {
		return report, fault.CLIProtocol
	}
	return report, nil
}

func validWireGuardConfigInspection(config WireGuardConfigInspection, raw json.RawMessage) bool {
	if _, ok := wireGuardObject(raw, "path", "state", "addresses", "dns", "mtu", "peer_count", "unsupported_directives"); !ok || !validWireGuardPath(config.Path) || config.Addresses == nil || config.DNS == nil || config.UnsupportedDirectives == nil {
		return false
	}
	if config.State != "ready" && config.State != "unsupported" {
		switch config.State {
		case "missing", "unreadable", "unsafe_path", "insecure_permissions", "invalid":
			return len(config.Addresses) == 0 && len(config.DNS) == 0 && config.MTU == nil && config.PeerCount == nil && len(config.UnsupportedDirectives) == 0
		default:
			return false
		}
	}
	if config.PeerCount == nil || *config.PeerCount > 1024 || len(config.Addresses) == 0 || len(config.Addresses) > 64 || len(config.DNS) > 16 || (config.MTU != nil && *config.MTU < 576) || len(config.UnsupportedDirectives) > 6 {
		return false
	}
	for _, address := range config.Addresses {
		if !validWireGuardAddress(address, false) {
			return false
		}
	}
	for _, address := range config.DNS {
		ip, err := netip.ParseAddr(address)
		if err != nil || ip.Zone() != "" {
			return false
		}
	}
	seen := make(map[string]bool)
	for _, directive := range config.UnsupportedDirectives {
		if seen[directive] {
			return false
		}
		seen[directive] = true
		switch directive {
		case "PreUp", "PostUp", "PreDown", "PostDown", "SaveConfig", "Table":
		default:
			return false
		}
	}
	return (config.State == "ready") == (len(config.UnsupportedDirectives) == 0)
}

func decodeWireGuardStatus(raw json.RawMessage) (WireGuardStatusReport, error) {
	var report WireGuardStatusReport
	object, ok := wireGuardObject(raw, "home", "platform", "elevation", "tool", "state", "interfaces")
	if !ok || decodeWireGuardJSON(raw, &report) != nil || !validWireGuardObservation(report.Home, report.Platform, report.Elevation) || !validWireGuardTool(report.Tool, "wg", object["tool"]) || report.Interfaces == nil {
		return report, fault.CLIProtocol
	}
	expectedTool := "available"
	switch report.State {
	case "tool_missing":
		expectedTool = "missing"
	case "tool_not_executable":
		expectedTool = "not_executable"
	case "observed", "query_failed", "timed_out", "output_too_large", "invalid_output":
	default:
		return report, fault.CLIProtocol
	}
	if report.Tool.State != expectedTool || (report.State != "observed" && len(report.Interfaces) != 0) || !validWireGuardInterfaces(report.Interfaces, object["interfaces"]) {
		return report, fault.CLIProtocol
	}
	return report, nil
}

func validWireGuardInterfaces(interfaces []WireGuardInterface, raw json.RawMessage) bool {
	var objects []json.RawMessage
	if json.Unmarshal(raw, &objects) != nil || len(interfaces) > 64 || len(objects) != len(interfaces) {
		return false
	}
	names := make(map[string]bool)
	peers, addresses := 0, 0
	for index, iface := range interfaces {
		object, ok := wireGuardObject(objects[index], "name", "public_key", "listen_port", "fwmark", "peers")
		if !ok || !wireGuardRequiredNumbers(object, "listen_port", "fwmark") || iface.Name == "" || len(iface.Name) > 255 || strings.IndexFunc(iface.Name, unicode.IsControl) >= 0 || names[iface.Name] || (iface.PublicKey != nil && !validWireGuardPublicKey(*iface.PublicKey)) || iface.Peers == nil {
			return false
		}
		names[iface.Name] = true
		var rawPeers []json.RawMessage
		if json.Unmarshal(object["peers"], &rawPeers) != nil || len(rawPeers) != len(iface.Peers) {
			return false
		}
		keys := make(map[string]bool)
		for i, peer := range iface.Peers {
			if !validWireGuardPeer(peer, rawPeers[i]) || keys[peer.PublicKey] {
				return false
			}
			keys[peer.PublicKey] = true
			peers++
			addresses += len(peer.AllowedIPs)
			if peers > 1024 || addresses > 4096 {
				return false
			}
		}
	}
	return true
}

func validWireGuardPeer(peer WireGuardPeer, raw json.RawMessage) bool {
	object, ok := wireGuardObject(raw, "public_key", "endpoint", "allowed_ips", "latest_handshake", "received_bytes", "sent_bytes", "persistent_keepalive")
	if !ok || !wireGuardRequiredNumbers(object, "latest_handshake", "received_bytes", "sent_bytes", "persistent_keepalive") || !validWireGuardPublicKey(peer.PublicKey) || (peer.Endpoint != nil && !validWireGuardEndpoint(*peer.Endpoint)) || peer.AllowedIPs == nil || len(peer.AllowedIPs) > 256 || peer.LatestHandshake > 253402300799 {
		return false
	}
	for _, address := range peer.AllowedIPs {
		if !validWireGuardAddress(address, true) {
			return false
		}
	}
	return true
}

func validWireGuardAddress(address string, prefixOnly bool) bool {
	if prefix, err := netip.ParsePrefix(address); err == nil {
		return prefix.Addr().Zone() == ""
	}
	if prefixOnly {
		return false
	}
	ip, err := netip.ParseAddr(address)
	return err == nil && ip.Zone() == ""
}

func validWireGuardPublicKey(key string) bool {
	decoded, err := base64.StdEncoding.DecodeString(key)
	return err == nil && len(decoded) == 32 && base64.StdEncoding.EncodeToString(decoded) == key && !bytes.Equal(decoded, make([]byte, 32))
}

func validWireGuardEndpoint(endpoint string) bool {
	if len(endpoint) > 320 {
		return false
	}
	host, port, err := net.SplitHostPort(endpoint)
	if err != nil || host == "" {
		return false
	}
	value, err := strconv.ParseUint(port, 10, 16)
	if err != nil || value == 0 {
		return false
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		zone := ip.Zone()
		if zone == "" {
			return true
		}
		if !ip.Is6() || len(zone) > 64 {
			return false
		}
		for _, c := range zone {
			if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '_' && c != '.' && c != '-' {
				return false
			}
		}
		return true
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, c := range label {
			if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && c != '-' {
				return false
			}
		}
	}
	return true
}
