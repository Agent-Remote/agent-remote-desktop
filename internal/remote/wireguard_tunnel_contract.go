package remote

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func decodeWireGuardTunnelPlan(raw json.RawMessage) (WireGuardTunnelPlan, error) {
	var plan WireGuardTunnelPlan
	object, ok := wireGuardObject(raw, "version", "home", "platform", "action", "config", "binding", "tools", "target", "issues")
	if !ok || decodeWireGuardJSON(raw, &plan) != nil || plan.Version != 1 || !validWireGuardObservation(plan.Home, plan.Platform, "unknown") || (plan.Action != "up" && plan.Action != "down") || !validWireGuardConfigInspection(plan.Config, object["config"]) {
		return plan, fault.CLIProtocol
	}
	parsed := plan.Config.State == "ready" || plan.Config.State == "unsupported"
	if parsed != (plan.Binding != nil) {
		return plan, fault.CLIProtocol
	}
	if binding := plan.Binding; binding != nil {
		if _, ok := wireGuardObject(object["binding"], "file_identity", "content_sha256", "public_key"); !ok || !validWireGuardIdentity(binding.FileIdentity) || !validWireGuardIdentity(binding.ContentSHA256) || !validWireGuardPublicKey(binding.PublicKey) {
			return plan, fault.CLIProtocol
		}
	}
	var rawTools []json.RawMessage
	if json.Unmarshal(object["tools"], &rawTools) != nil || len(plan.Tools) != 2 || len(rawTools) != 2 {
		return plan, fault.CLIProtocol
	}
	names := []string{"wg", "wg-quick"}
	if plan.Platform == "windows" {
		names[1] = "wireguard"
	}
	for i, tool := range plan.Tools {
		if !validWireGuardTunnelTool(tool, names[i], rawTools[i]) {
			return plan, fault.CLIProtocol
		}
	}
	if !validWireGuardTarget(plan.Target, plan.Platform, plan.Config.Path, object["target"]) || plan.Issues == nil || !wireGuardTunnelIssuesMatch(plan) {
		return plan, fault.CLIProtocol
	}
	return plan, nil
}

func wireGuardTunnelIssuesMatch(plan WireGuardTunnelPlan) bool {
	expected := wireGuardTunnelIssues(plan)
	if slices.Equal(plan.Issues, expected) {
		return true
	}
	// Older CLI builds predate the protected execution broker marker. Keep
	// decoding those read-only plans compatible while accepting the current
	// explicit blocker when the CLI reports it.
	return slices.Equal(plan.Issues, append(append([]string(nil), expected...), "execution_bridge_required"))
}

func validWireGuardTunnelTool(tool WireGuardTunnelTool, name string, raw json.RawMessage) bool {
	if _, ok := wireGuardObject(raw, "name", "state", "path", "identity", "sha256"); !ok || tool.Name != name || (tool.Identity == nil) != (tool.SHA256 == nil) {
		return false
	}
	if tool.State == "missing" {
		return tool.Path == nil && tool.Identity == nil
	}
	if tool.Path == nil || !validWireGuardPath(*tool.Path) {
		return false
	}
	if tool.State == "not_executable" {
		return tool.Identity == nil
	}
	return tool.State == "available" && (tool.Identity == nil || (validWireGuardIdentity(*tool.Identity) && validWireGuardIdentity(*tool.SHA256)))
}

func wireGuardLogicalName(path, platform string) string {
	name := filepath.Base(path)
	// Rust Path::file_stem preserves dotfiles without a second dot.
	if dot := strings.LastIndexByte(name, '.'); dot > 0 {
		name = name[:dot]
	}
	limit := 15
	if platform == "windows" {
		limit = 32
	}
	if name == "" || len(name) > limit || name == "." || name == ".." {
		return ""
	}
	for _, c := range name {
		if !(c >= 'a' && c <= 'z') && !(c >= 'A' && c <= 'Z') && !(c >= '0' && c <= '9') && !strings.ContainsRune("_=+.-", c) {
			return ""
		}
	}
	if platform == "windows" {
		if strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || wireGuardWindowsReservedName(name) {
			return ""
		}
		if dot := strings.LastIndexByte(name, '.'); dot > 0 && wireGuardWindowsReservedName(name[:dot]) {
			return ""
		}
	}
	return name
}

func wireGuardWindowsReservedName(name string) bool {
	name = strings.ToUpper(name)
	if name == "CON" || name == "PRN" || name == "AUX" || name == "NUL" {
		return true
	}
	return len(name) == 4 && (name[:3] == "COM" || name[:3] == "LPT") && name[3] >= '1' && name[3] <= '9'
}

func validWireGuardTarget(target WireGuardTargetObservation, platform, config string, raw json.RawMessage) bool {
	if _, ok := wireGuardObject(raw, "logical_name", "state", "reason", "status_state", "identity"); !ok || target.LogicalName != wireGuardLogicalName(config, platform) {
		return false
	}
	switch target.StatusState {
	case "not_queried", "observed", "tool_missing", "tool_not_executable", "query_failed", "timed_out", "output_too_large", "invalid_output":
	default:
		return false
	}
	if target.Reason != nil {
		switch *target.Reason {
		case "invalid_name", "native_query_failed", "mapping_missing", "mapping_unsafe", "mapping_stale", "interface_not_wireguard", "service_invalid", "target_changed", "status_unavailable", "target_not_in_status":
		default:
			return false
		}
	}
	if target.LogicalName == "" {
		return target.State == "unknown" && target.Reason != nil && *target.Reason == "invalid_name" && target.StatusState == "not_queried" && target.Identity == nil
	}
	switch target.State {
	case "absent":
		return platform != "macos" && target.Reason == nil && target.Identity == nil && target.StatusState == "not_queried"
	case "unknown":
		if target.Reason == nil || target.Identity != nil {
			return false
		}
		if *target.Reason == "target_changed" {
			return true
		}
		if target.StatusState != "not_queried" {
			return false
		}
		return *target.Reason == "native_query_failed" || (platform == "macos" && slices.Contains([]string{"mapping_missing", "mapping_unsafe", "mapping_stale"}, *target.Reason))
	case "conflict":
		return target.Identity == nil && target.StatusState == "not_queried" && target.Reason != nil && ((platform == "linux" && *target.Reason == "interface_not_wireguard") || (platform == "windows" && *target.Reason == "service_invalid"))
	case "present":
		if target.Reason != nil && *target.Reason != "status_unavailable" && *target.Reason != "target_not_in_status" {
			return false
		}
	default:
		return false
	}
	if target.StatusState == "not_queried" || target.Identity == nil || !validWireGuardTargetIdentity(*target.Identity, platform, target.LogicalName) {
		return false
	}
	key := target.Identity.PublicKey()
	if key != nil && (!validWireGuardPublicKey(*key) || target.StatusState != "observed" || target.Reason != nil) {
		return false
	}
	if target.StatusState != "observed" {
		return key == nil && target.Reason != nil && *target.Reason == "status_unavailable"
	}
	return target.Reason == nil || *target.Reason == "target_not_in_status"
}

func validWireGuardTargetIdentity(identity WireGuardTargetIdentity, platform, logicalName string) bool {
	switch platform {
	case "linux":
		target := identity.Linux
		return identity.Kind == "linux_interface" && target != nil && target.Name == logicalName && target.Index != 0 && target.Index <= 1<<31-1 && validWireGuardIdentity(target.Namespace)
	case "macos":
		target := identity.MacOS
		if identity.Kind != "macos_interface" || target == nil || target.Index == 0 || !validWireGuardIdentity(target.MappingIdentity) || !validWireGuardIdentity(target.SocketIdentity) || !strings.HasPrefix(target.Name, "utun") || len(target.Name) < 5 || len(target.Name) > 15 {
			return false
		}
		number, err := strconv.ParseUint(target.Name[4:], 10, 32)
		return err == nil && target.Name == "utun"+strconv.FormatUint(number, 10)
	case "windows":
		target := identity.Windows
		return identity.Kind == "windows_service" && target != nil && target.ServiceName == "WireGuardTunnel$"+logicalName && validWireGuardPath(target.BinaryPath) && validWireGuardPath(target.ConfigPath) && (target.ConfigIdentity == nil || validWireGuardIdentity(*target.ConfigIdentity)) && target.ServiceState >= 1 && target.ServiceState <= 7 && target.StartType <= 4 && target.Account != "" && len(target.Account) <= 512 && strings.IndexFunc(target.Account, unicode.IsControl) < 0
	default:
		return false
	}
}

func wireGuardTunnelIssues(plan WireGuardTunnelPlan) []string {
	issues := make([]string, 0, 12)
	if plan.Config.State != "ready" && plan.Config.State != "unsupported" {
		issues = append(issues, "config_not_ready")
	}
	if plan.Config.State == "unsupported" {
		issues = append(issues, "config_unsupported")
	}
	if slices.ContainsFunc(plan.Tools, func(tool WireGuardTunnelTool) bool { return tool.State != "available" }) {
		issues = append(issues, "tool_unavailable")
	}
	if slices.ContainsFunc(plan.Tools, func(tool WireGuardTunnelTool) bool { return tool.State == "available" && tool.Identity == nil }) {
		issues = append(issues, "tool_snapshot_unavailable")
	}
	switch plan.Target.State {
	case "unknown":
		issues = append(issues, "target_unknown")
	case "conflict":
		issues = append(issues, "target_conflict")
	case "present":
		if plan.Action == "up" {
			issues = append(issues, "target_already_present")
		}
	case "absent":
		if plan.Action == "down" {
			issues = append(issues, "target_absent")
		}
	}
	if plan.Action == "down" && plan.Target.State == "present" {
		key := plan.Target.Identity.PublicKey()
		if plan.Binding != nil && key != nil && *key != plan.Binding.PublicKey {
			issues = append(issues, "config_target_mismatch")
		}
		if key == nil {
			issues = append(issues, "target_key_unobserved")
		}
		if service := plan.Target.Identity.Windows; service != nil && (plan.Binding == nil || service.ConfigIdentity == nil || *service.ConfigIdentity != plan.Binding.FileIdentity) {
			issues = append(issues, "service_config_mismatch")
		}
	}
	return issues
}
