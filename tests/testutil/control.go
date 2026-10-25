package testutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// ControlHelper runs in a test subprocess disguised as the CLI, never an Agent.
func ControlHelper() bool {
	if schemaHelper() {
		return true
	}
	if len(os.Args) != 2 || os.Args[1] != "--desktop-control" {
		return false
	}
	mode := os.Getenv("DESKTOP_TEST_CLI_SCENARIO")
	if mode == "device-hold-stdout" {
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if mode == "slow" {
		time.Sleep(time.Minute)
		return true
	}
	var request struct {
		ID           string          `json:"request_id"`
		Method       string          `json:"method"`
		StreamEvents bool            `json:"stream_events"`
		Params       json.RawMessage `json:"params"`
	}
	if json.NewDecoder(os.Stdin).Decode(&request) != nil {
		os.Exit(2)
	}
	if mode == "slow-login" && request.Method == "login.password" {
		time.Sleep(time.Minute)
		return true
	}
	home := os.Getenv("AGENT_REMOTE_HOME")
	if path := os.Getenv("DESKTOP_TEST_CLI_LOG"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(3)
		}
		fmt.Fprintln(f, request.Method)
		f.Close()
	}
	if path := os.Getenv("DESKTOP_TEST_CLI_PARAMS_LOG"); path != "" && request.Method == "sessions.list" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(3)
		}
		fmt.Fprintln(f, string(request.Params))
		f.Close()
	}
	if mode == "slow-logout" && request.Method == "logout" {
		time.Sleep(time.Minute)
		return true
	}
	if request.Method == "capabilities" {
		waitControlBarrier("DESKTOP_TEST_CAPABILITIES_RELEASE", "DESKTOP_TEST_CAPABILITIES_STARTED")
	}
	if request.Method == "diagnostics.inspect" {
		waitControlBarrier("DESKTOP_TEST_DIAGNOSTICS_RELEASE", "DESKTOP_TEST_DIAGNOSTICS_STARTED")
	}
	if request.Method == "sessions.list" {
		waitControlBarrier("DESKTOP_TEST_REMOTE_RELEASE", "DESKTOP_TEST_REMOTE_STARTED")
	}
	if request.Method == "logout" {
		waitControlBarrier("DESKTOP_TEST_LOGOUT_RELEASE", "")
	}
	var result any
	switch request.Method {
	case "capabilities":
		platform := runtime.GOOS
		if platform == "darwin" {
			platform = "macos"
		}
		result = map[string]any{"protocol_version": 1, "cli_version": "test", "platform": platform, "operations": []any{
			map[string]any{"method": "accounts.runtime_recovery.authority", "effect": "read", "credential": "user"},
			map[string]any{"method": "accounts.runtime_recovery.inspect", "effect": "read", "credential": "user"},
			map[string]any{"method": "accounts.runtime_recovery.status", "effect": "read", "credential": "user"},
			map[string]any{"method": "accounts.runtime_recovery.pending", "effect": "read", "credential": "none"},
			map[string]any{"method": "accounts.runtime_recovery.apply", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.runtime_recovery.forget", "effect": "write", "credential": "none", "confirmation": "exact_request"},
			map[string]any{"method": "context", "effect": "read"},
			map[string]any{"method": "accounts.import.discover", "effect": "read", "credential": "none"},
			map[string]any{"method": "accounts.import.prepare", "effect": "read", "credential": "none"},
			map[string]any{"method": "accounts.import.pending", "effect": "read", "credential": "none"},
			map[string]any{"method": "accounts.import.validate", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.import.apply", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.import.status", "effect": "read", "credential": "device"},
			map[string]any{"method": "accounts.import.forget", "effect": "write", "credential": "none", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.list", "effect": "read", "credential": "device"},
			map[string]any{"method": "credentials.list", "effect": "read", "credential": "device"},
			map[string]any{"method": "credentials.create", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "credentials.bind", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "credentials.unbind", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "skills.list", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.info", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.state.list", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.state.conflicts", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.state.info", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.state.diff", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.change", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "skills.retry.plan", "effect": "read", "credential": "user"},
			map[string]any{"method": "skills.retry.control", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "mutagen.environment.preview", "effect": "read", "credential": "none"},
			map[string]any{"method": "mutagen.environment.apply", "effect": "write", "credential": "none", "confirmation": "exact_request"},
			map[string]any{"method": "sync.status", "effect": "read", "credential": "device"},
			map[string]any{"method": "sync.recovery.preview", "effect": "read", "credential": "device"},
			map[string]any{"method": "sync.recovery.apply", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sync.ensure", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sync.pause", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sync.resume", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sync.resolve", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sync.reset", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.create", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.status", "effect": "read", "credential": "device"},
			map[string]any{"method": "accounts.bind", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.verify", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.disable", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.default.get", "effect": "read", "credential": "none"},
			map[string]any{"method": "accounts.default.set", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "accounts.default.clear", "effect": "write", "credential": "none", "confirmation": "exact_request"},
			map[string]any{"method": "device.status", "effect": "read", "credential": "device"},
			map[string]any{"method": "device.local.status", "effect": "read", "credential": "none"},
			map[string]any{"method": "device.local.plan", "effect": "read", "credential": "none"},
			map[string]any{"method": "device.revoke", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "device.rotate_token", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "ego_browser.status", "effect": "read", "credential": "user"},
			map[string]any{"method": "ego_browser.candidates", "effect": "read", "credential": "user"},
			map[string]any{"method": "ego_browser.claim.plan", "effect": "read", "credential": "user"},
			map[string]any{"method": "ego_browser.claim.control", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "ego_browser.requests", "effect": "read", "credential": "user"},
			map[string]any{"method": "ego_browser.request.cancel", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "ego_browser.binding.plan", "effect": "read", "credential": "user"},
			map[string]any{"method": "ego_browser.binding.control", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "ego_browser.delete.plan", "effect": "read", "credential": "user"},
			map[string]any{"method": "ego_browser.delete.control", "effect": "write", "credential": "user", "confirmation": "exact_request"},
			map[string]any{"method": "nodes.list", "effect": "read", "credential": "user"},
			map[string]any{"method": "sessions.list", "effect": "read"},
			map[string]any{"method": "sessions.stop", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sessions.stop.request", "effect": "read", "credential": "device"},
			map[string]any{"method": "sessions.stop.status", "effect": "read", "credential": "device"},
			map[string]any{"method": "sessions.delete", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "sessions.delete.request", "effect": "read", "credential": "device"},
			map[string]any{"method": "sessions.delete.status", "effect": "read", "credential": "device"},
			map[string]any{"method": "login.password", "effect": "write"},
			map[string]any{"method": "logout", "effect": "write"},
			map[string]any{"method": "login.device_code", "effect": "write"},
			map[string]any{"method": "diagnostics.inspect", "effect": "read"},
			map[string]any{"method": "dependencies.inspect", "effect": "read"},
			map[string]any{"method": "status.inspect", "effect": "read"},
			map[string]any{"method": "doctor.inspect", "effect": "read"},
			map[string]any{"method": "doctor.repair", "effect": "write", "credential": "none", "confirmation": "exact_request"},
			map[string]any{"method": "deps.inspect", "effect": "read"},
			map[string]any{"method": "deps.repair", "effect": "write", "credential": "none", "confirmation": "exact_request"},
			map[string]any{"method": "ssh.inspect", "effect": "read"},
			map[string]any{"method": "setup.local", "effect": "write"},
			map[string]any{"method": "wireguard.config.plan", "effect": "read", "credential": "none"},
			map[string]any{"method": "wireguard.config.apply", "effect": "write", "credential": "device", "confirmation": "exact_request"},
			map[string]any{"method": "wireguard.check", "effect": "read", "credential": "none"},
			map[string]any{"method": "wireguard.status", "effect": "read", "credential": "none"},
			map[string]any{"method": "wireguard.bridge.status", "effect": "read", "credential": "none"},
			map[string]any{"method": "wireguard.tunnel.plan", "effect": "read", "credential": "none"},
			map[string]any{"method": "wireguard.tunnel.apply", "effect": "write", "credential": "none", "confirmation": "exact_request"},
		}}
		caps := result.(map[string]any)
		caps["terminal_operations"] = []any{
			map[string]any{"method": "accounts.binding.attach", "protocol_version": 1},
			map[string]any{"method": "sessions.attach", "protocol_version": 1},
			map[string]any{"method": "fclaude run", "protocol_version": 1},
			map[string]any{"method": "fclaude new", "protocol_version": 1},
			map[string]any{"method": "fclaude list", "protocol_version": 1},
			map[string]any{"method": "agent-remote doctor", "protocol_version": 1},
			map[string]any{"method": "agent-remote ssh check", "protocol_version": 1},
			map[string]any{"method": "agent-remote node install", "protocol_version": 1},
			map[string]any{"method": "agent-remote device install", "protocol_version": 1},
			map[string]any{"method": "agent-remote device uninstall", "protocol_version": 1},
			map[string]any{"method": "agent-remote device status", "protocol_version": 1},
			map[string]any{"method": "agent-remote device launch", "protocol_version": 1},
			map[string]any{"method": "agent-remote device diagnose", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state prune", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state reset", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state migrate", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state resolve", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state restore", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state list", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state conflicts", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state info", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state diff", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill state export", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill add", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill update", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill enable", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill disable", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill pin", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill unpin", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill inherit", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill remove", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill rollback", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill check", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill info", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill status", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill list", "protocol_version": 1},
			map[string]any{"method": "agent-remote skill retry", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser setup", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser connect", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser repair", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser upgrade", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser remove", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser status", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser list", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser requests", "protocol_version": 1},
			// Matches agent-remote-cli src/desktop_control/queries.rs.
			map[string]any{"method": "agent-remote ego-browser cancel-request", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser forget-this-mac", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser forget", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser re-enroll", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser device-rotate", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser switch-server", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser register", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser claim", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser pause", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser resume", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser stop", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser revoke", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser delete-device", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser device-delete", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser delete-binding", "protocol_version": 1},
			map[string]any{"method": "agent-remote ego-browser binding-delete", "protocol_version": 1},
		}
		if mode == "node-terminal-unsupported" {
			// Keep the read-only nodes.list operation available while removing
			// the separate managed-terminal install capability. This models a
			// mixed-version CLI where target discovery is present but the
			// destructive install workflow is not, so the command card must
			// expose cli_unsupported rather than imply an install is ready.
			filtered := make([]any, 0, len(caps["terminal_operations"].([]any)))
			for _, item := range caps["terminal_operations"].([]any) {
				method, _ := item.(map[string]any)["method"].(string)
				if method != "agent-remote node install" {
					filtered = append(filtered, item)
				}
			}
			caps["terminal_operations"] = filtered
		}
		// The transfer declaration is intentionally opt-in in fixtures. Older
		// control responses remain text-only, while this scenario exercises the
		// exact v1 file/image bridge shown by the remote confirmation UI.
		if mode == "remote-attach-transfer" {
			caps["attachment_transfer"] = map[string]any{
				"supported": true, "protocol_version": 1,
				"max_bytes": 256 * 1024 * 1024, "max_entries": 32,
			}
		}
		if mode == "remote-attach-unsupported" {
			caps["terminal_operations"] = []any{}
		}
		if mode == "remote-attach-future" {
			caps["terminal_operations"] = []any{map[string]any{"method": "sessions.attach", "protocol_version": 2}}
		}
		if mode == "account-terminal-unsupported" {
			delete(caps, "terminal_operations")
		}
		switch mode {
		case "account-unsupported":
			ops := []any{}
			for _, op := range caps["operations"].([]any) {
				name := op.(map[string]any)["method"].(string)
				if name != "accounts.create" && name != "accounts.status" && name != "accounts.bind" && name != "accounts.verify" && name != "accounts.disable" {
					ops = append(ops, op)
				}
			}
			caps["operations"] = ops
		case "default-unsupported":
			ops := []any{}
			for _, op := range caps["operations"].([]any) {
				if !strings.HasPrefix(op.(map[string]any)["method"].(string), "accounts.default.") {
					ops = append(ops, op)
				}
			}
			caps["operations"] = ops
		case "credentials-unsupported":
			ops := []any{}
			for _, op := range caps["operations"].([]any) {
				if op.(map[string]any)["method"] != "credentials.list" {
					ops = append(ops, op)
				}
			}
			caps["operations"] = ops
		case "skills-unsupported":
			ops := []any{}
			for _, op := range caps["operations"].([]any) {
				if op.(map[string]any)["method"] != "skills.list" && op.(map[string]any)["method"] != "skills.info" && op.(map[string]any)["method"] != "skills.state.list" && op.(map[string]any)["method"] != "skills.state.conflicts" && op.(map[string]any)["method"] != "skills.state.info" && op.(map[string]any)["method"] != "skills.state.diff" && op.(map[string]any)["method"] != "skills.change" && op.(map[string]any)["method"] != "skills.retry.plan" && op.(map[string]any)["method"] != "skills.retry.control" {
					ops = append(ops, op)
				}
			}
			caps["operations"] = ops
		case "other-platform":
			caps["platform"] = "windows"
			if runtime.GOOS == "windows" {
				caps["platform"] = "linux"
			}
		case "missing-platform":
			delete(caps, "platform")
		case "duplicate-capability":
			operations := caps["operations"].([]any)
			caps["operations"] = append(operations, operations[0])
		case "invalid-effect":
			caps["operations"].([]any)[0].(map[string]any)["effect"] = "unknown"
		case "setup-unsupported", "setup-readonly":
			operations := []any{}
			for _, operation := range caps["operations"].([]any) {
				entry := operation.(map[string]any)
				if entry["method"] == "setup.local" {
					if mode == "setup-unsupported" {
						continue
					}
					entry["effect"] = "read"
				}
				operations = append(operations, operation)
			}
			caps["operations"] = operations
		case "wg-unsupported", "wg-readonly":
			operations := []any{}
			for _, operation := range caps["operations"].([]any) {
				entry := operation.(map[string]any)
				if strings.HasPrefix(entry["method"].(string), "wireguard.config.") {
					if mode == "wg-unsupported" {
						continue
					}
					if entry["method"] == "wireguard.config.apply" {
						entry["effect"] = "read"
					}
				}
				operations = append(operations, operation)
			}
			caps["operations"] = operations
		case "read-only-capabilities":
			operations := []any{}
			for _, operation := range caps["operations"].([]any) {
				if operation.(map[string]any)["effect"] == "read" {
					operations = append(operations, operation)
				}
			}
			caps["operations"] = operations
		case "wg-network-unsupported", "wg-network-write", "wg-network-credential":
			operations := []any{}
			for _, operation := range caps["operations"].([]any) {
				entry := operation.(map[string]any)
				if entry["method"] == "wireguard.check" || entry["method"] == "wireguard.status" {
					if mode == "wg-network-unsupported" {
						continue
					}
					if mode == "wg-network-write" {
						entry["effect"] = "write"
					}
					if mode == "wg-network-credential" {
						entry["credential"] = "device"
					}
				}
				operations = append(operations, operation)
			}
			caps["operations"] = operations
		case "wg-tunnel-unsupported", "wg-tunnel-write", "wg-tunnel-credential":
			operations := []any{}
			for _, operation := range caps["operations"].([]any) {
				entry := operation.(map[string]any)
				if entry["method"] == "wireguard.tunnel.plan" || entry["method"] == "wireguard.tunnel.apply" {
					if mode == "wg-tunnel-unsupported" {
						continue
					}
					if mode == "wg-tunnel-write" {
						entry["effect"] = "write"
					}
					if mode == "wg-tunnel-credential" {
						entry["credential"] = "device"
					}
				}
				operations = append(operations, operation)
			}
			caps["operations"] = operations
		case "remote-create":
			caps["operations"] = append(caps["operations"].([]any),
				map[string]any{"method": "sessions.create", "effect": "write", "credential": "device", "confirmation": "exact_request"},
				map[string]any{"method": "sessions.create.request", "effect": "read", "credential": "device"},
			)
		}
	case "context":
		result = map[string]any{"home": home, "server_url": "https://test.invalid", "active_device_id": "device-1"}
		if strings.HasPrefix(mode, "runtime-") {
			result.(map[string]any)["active_device_id"] = nil
		}
		if mode == "unconfigured" {
			result = map[string]any{"home": home}
		}
		if server := os.Getenv("DESKTOP_TEST_CONTEXT_SERVER"); server != "" {
			result.(map[string]any)["server_url"] = server
		}
	case "accounts.list":
		result = map[string]any{"items": []any{map[string]any{"id": "account-1", "tool_type": "claude", "display_name": "Studio", "status": "ready", "region_code": "JP", "runtime_backend": "native"}}}
		if strings.HasPrefix(mode, "account-terminal-") {
			result.(map[string]any)["items"].([]any)[0].(map[string]any)["id"] = "11111111-1111-4111-8111-111111111111"
		}
		if mode == "remote-account-malformed" {
			result.(map[string]any)["items"].([]any)[0].(map[string]any)["display_name"] = "Studio\nprivate"
		}
	case "credentials.list":
		result = map[string]any{"items": []any{
			map[string]any{"id": "credential-1", "display_name": "Studio workstation", "status": "ready", "github_cli_mode": "remote_login", "ssh_mode": "agent_forwarding", "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-02-03T04:05:06Z"},
		}}
	case "nodes.list":
		result = map[string]any{
			"schema_version": 1, "operation": "nodes.list",
			"context": map[string]any{"home": home, "server_url": "https://test.invalid", "active_device_id": "device-1"},
			"items": []any{map[string]any{
				"id": "11111111-1111-4111-8111-111111111111", "name": "Studio Node", "status": "ready",
				"ssh_host": "node.test.invalid", "ssh_port": 22, "ssh_user": "agent-remote", "version": "1.0.0",
				"ego_browser_enabled": false, "configured_enabled": true, "effective_enabled": true,
				"node_execution_allowed": true, "enrollment_admission": true, "execution_admission": true,
			}},
		}
		if mode == "nodes-unknown-field" {
			result.(map[string]any)["unexpected"] = true
		}
		if mode == "nodes-context-changed" {
			result.(map[string]any)["context"].(map[string]any)["active_device_id"] = "other-device"
		}
		if mode == "nodes-duplicate" {
			result.(map[string]any)["items"] = append(result.(map[string]any)["items"].([]any), result.(map[string]any)["items"].([]any)[0])
		}
	case "skills.list":
		result = map[string]any{
			"generation": 7,
			"items": []any{map[string]any{
				"id": "11111111-1111-4111-8111-111111111111", "name": "Review helper", "removed": false,
				"default_enabled": true, "default_revision_id": "revision-1",
				"effective":         map[string]any{"enabled": true, "revision_id": "revision-1", "eligible": true, "included": true, "exclusion_reason": nil},
				"project_discovery": "available", "model_loaded": true, "revision_count": 2,
			}},
			"local_items": []any{map[string]any{
				"id": "local-1", "name": "Account helper", "account_id": "account-1", "status": "ready",
				"enabled": true, "revision_id": "local-revision-1", "model_loaded": false,
			}},
		}
	case "skills.state.conflicts":
		result = map[string]any{
			"selector": map[string]any{
				"account_id": "11111111-1111-4111-8111-111111111111",
				"scope":      "account-directory",
				"skill":      nil,
			},
			"publications": map[string]any{
				"items": []any{map[string]any{
					"id": "33333333-3333-4333-8333-333333333333", "attempt": 2,
					"status": "conflicted", "reason": "concurrent_publication",
				}}, "next_cursor": nil,
			},
			"migrations": map[string]any{
				"items": []any{map[string]any{
					"id": "44444444-4444-4444-8444-444444444444", "skill_id": "11111111-1111-4111-8111-111111111111",
					"name": "Review helper", "mode": "forward", "status": "conflicted",
					"installation_epoch": 3, "target_epoch": 4, "directory_epoch": 5,
					"created_at": "2026-10-19T10:00:00Z",
				}}, "next_cursor": nil,
			},
		}
	case "skills.info":
		var input struct {
			Skill     string  `json:"skill"`
			Tool      *string `json:"tool"`
			AccountID *string `json:"account_id"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.Skill == "" {
			os.Exit(4)
		}
		selector := map[string]any{"skill": input.Skill, "tool": input.Tool, "account_id": input.AccountID}
		result = map[string]any{
			"selector": selector,
			"item": map[string]any{
				"kind": "library", "id": "11111111-1111-4111-8111-111111111111", "name": "Review helper",
				"removed": false, "epoch": 3, "default_enabled": true, "default_revision_id": "revision-1",
				"revision_count": 2, "revisions": []any{
					map[string]any{"id": "revision-1", "number": 1, "retained": true},
					map[string]any{"id": "revision-2", "number": 2, "retained": true},
				},
				"effective":         map[string]any{"enabled": true, "revision_id": "revision-1", "eligible": true, "included": true, "exclusion_reason": nil},
				"project_discovery": "available", "model_loaded": true,
			},
		}
	case "skills.state.list":
		result = map[string]any{
			"selector": map[string]any{
				"account_id": "11111111-1111-4111-8111-111111111111",
				"scope":      "account-directory",
				"skill":      nil,
			},
			"checkpoints": map[string]any{
				"items": []any{map[string]any{
					"id": "55555555-5555-4555-8555-555555555555", "account_id": "11111111-1111-4111-8111-111111111111", "scope": "account-directory",
					"state_id": nil, "skill_id": nil, "origin": nil, "revision_id": nil,
					"installation_epoch": nil, "state_epoch": nil, "directory_epoch": 3, "current_state_epoch": nil, "current_directory_epoch": 4,
					"parent_id": nil, "content_digest": strings.Repeat("a", 64), "retained": true, "is_head": true,
					"invalid_skill_format": false, "source_session_reference_id": nil, "finalization_id": nil, "finalization_status": nil,
					"storage_location": "server", "created_at": "2026-10-19T10:00:00Z",
				}}, "next_cursor": nil,
			},
			"pending": map[string]any{
				"items": []any{map[string]any{
					"id": "66666666-6666-4666-8666-666666666666", "snapshot_id": "77777777-7777-4777-8777-777777777777", "session_reference_id": "88888888-8888-4888-8888-888888888888", "node_id": "99999999-9999-4999-8999-999999999999", "incoming_digest": strings.Repeat("b", 64), "status": "upload_pending", "storage_location": "source_node", "exportable_from_server": false,
				}}, "next_cursor": nil,
			},
		}
	case "skills.state.info":
		var input struct {
			CheckpointID string `json:"checkpoint_id"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.CheckpointID == "" {
			os.Exit(4)
		}
		result = map[string]any{
			"status": "ready",
			"checkpoint": map[string]any{
				"id": input.CheckpointID, "account_id": "11111111-1111-4111-8111-111111111111", "scope": "account-directory",
				"state_id": nil, "skill_id": nil, "origin": nil, "revision_id": nil,
				"installation_epoch": nil, "state_epoch": nil, "directory_epoch": 3, "current_state_epoch": nil, "current_directory_epoch": 4,
				"parent_id": nil, "content_digest": strings.Repeat("a", 64), "retained": true, "is_head": true,
				"invalid_skill_format": false, "source_session_reference_id": nil, "finalization_id": nil, "finalization_status": nil,
				"storage_location": "server", "created_at": "2026-10-19T10:00:00Z",
			},
		}
	case "skills.state.diff":
		var input struct {
			AccountID    *string `json:"account_id"`
			Scope        *string `json:"scope"`
			Skill        *string `json:"skill"`
			CheckpointID *string `json:"checkpoint_id"`
			Limit        uint16  `json:"limit"`
			Cursor       *string `json:"cursor"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.Limit == 0 {
			os.Exit(4)
		}
		modeName := "current"
		if input.CheckpointID != nil {
			modeName = "checkpoint"
		}
		result = map[string]any{
			"mode": modeName, "checkpoint_id": "55555555-5555-4555-8555-555555555555",
			"base_kind": "package_revision", "base_reference_id": "revision-1",
			"base_tree_digest": strings.Repeat("a", 64), "current_tree_digest": strings.Repeat("b", 64),
			"items": []any{map[string]any{
				"path":    "README.md",
				"base":    map[string]any{"path": "README.md", "kind": "file", "mode": uint32(420), "size": uint64(12), "sha256": strings.Repeat("c", 64), "content_kind": "text"},
				"current": map[string]any{"path": "README.md", "kind": "file", "mode": uint32(420), "size": uint64(18), "sha256": strings.Repeat("d", 64), "content_kind": "text"},
			}}, "next_cursor": nil,
		}
	case "skills.change":
		var input struct {
			Confirmed          bool           `json:"confirmed"`
			Expected           map[string]any `json:"expected_context"`
			ExpectedGeneration int64          `json:"expected_generation"`
			Skill              string         `json:"skill"`
			Action             string         `json:"action"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.ExpectedGeneration != 7 || input.Skill == "" || (input.Action != "enable" && input.Action != "disable") {
			os.Exit(4)
		}
		result = map[string]any{
			"operation_id": request.ID,
			"context":      map[string]any{"home": home, "server_url": input.Expected["server_url"], "active_device_id": input.Expected["active_device_id"]},
			"accepted":     true, "action": input.Action, "skill": input.Skill,
			"generation": int64(8), "changed": true, "status": "committed",
		}
	case "skills.retry.plan":
		var input struct {
			Expected    map[string]any `json:"expected_context"`
			OperationID string         `json:"operation_id"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.OperationID == "" {
			os.Exit(4)
		}
		result = map[string]any{
			"schema_version": 1, "operation": "skills.retry.plan",
			"context":      map[string]any{"home": home, "server_url": input.Expected["server_url"], "active_device_id": input.Expected["active_device_id"]},
			"operation_id": input.OperationID, "status": "failed", "retryable": true,
			"generation": int64(7), "target_count": 1,
			"targets":          []any{map[string]any{"account_id": "11111111-1111-4111-8111-111111111111", "attempt_id": "22222222-2222-4222-8222-222222222222", "attempt_number": uint64(1)}},
			"plan_fingerprint": strings.Repeat("a", 64), "requires_confirmation": true,
		}
	case "skills.retry.control":
		var input struct {
			Confirmed   bool           `json:"confirmed"`
			Expected    map[string]any `json:"expected_context"`
			OperationID string         `json:"operation_id"`
			ExpectedGen int64          `json:"expected_generation"`
			Fingerprint string         `json:"expected_plan_fingerprint"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.ExpectedGen != 7 || input.OperationID == "" || input.Fingerprint == "" {
			os.Exit(4)
		}
		attempt := "33333333-3333-4333-8333-333333333333"
		retryable := true
		result = map[string]any{
			"schema_version": 1, "operation": "skills.retry.control", "operation_id": request.ID, "source_operation_id": input.OperationID, "request_id": request.ID,
			"context": map[string]any{"home": home, "server_url": input.Expected["server_url"], "active_device_id": input.Expected["active_device_id"]},
			"status":  "accepted", "accepted": true, "generation": int64(7), "target_count": 1, "plan_fingerprint": input.Fingerprint,
			"targets": []any{map[string]any{"account_id": "11111111-1111-4111-8111-111111111111", "attempt_id": &attempt, "attempt_number": ptrUint64(2), "readiness": "pending", "retryable": &retryable}},
		}
	case "credentials.create", "credentials.bind", "credentials.unbind":
		var input struct {
			Confirmed   bool           `json:"confirmed"`
			Expected    map[string]any `json:"expected_context"`
			DisplayName string         `json:"display_name"`
			GHMode      string         `json:"gh_mode"`
			SSHMode     string         `json:"ssh_mode"`
			AccountID   string         `json:"account_id"`
			ProfileID   string         `json:"profile_id"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed {
			os.Exit(4)
		}
		if path := os.Getenv("DESKTOP_TEST_CREDENTIAL_RECEIPT"); path != "" {
			if os.WriteFile(path, request.Params, 0600) != nil {
				os.Exit(5)
			}
		}
		profile := map[string]any{"id": "credential-1", "display_name": input.DisplayName, "status": "active", "github_cli_mode": input.GHMode, "ssh_mode": input.SSHMode, "created_at": "2026-01-02T03:04:05Z", "updated_at": "2026-02-03T04:05:06Z"}
		if profile["display_name"] == "" {
			profile["display_name"] = "Studio workstation"
		}
		if profile["github_cli_mode"] == "" {
			profile["github_cli_mode"] = "remote_login"
		}
		if profile["ssh_mode"] == "" {
			profile["ssh_mode"] = "agent_forwarding"
		}
		result = map[string]any{"operation_id": request.ID, "context": map[string]any{"home": home, "server_url": input.Expected["server_url"], "active_device_id": input.Expected["active_device_id"]}, "accepted": true, "action": strings.TrimPrefix(request.Method, "credentials."), "account_id": nil, "profile_id": nil, "profile": nil}
		if request.Method == "credentials.create" {
			result.(map[string]any)["profile"] = profile
		} else {
			result.(map[string]any)["account_id"] = input.AccountID
		}
		if request.Method == "credentials.bind" {
			result.(map[string]any)["profile"] = profile
			result.(map[string]any)["profile_id"] = input.ProfileID
		}
	case "sessions.list":
		items := []any{
			map[string]any{"id": "11111111-1111-4111-8111-111111111111", "tool_type": "claude", "status": "running", "workspace_local_path": "/project/alpha", "runtime_backend": "native"},
			map[string]any{"id": "22222222-2222-4222-8222-222222222222", "tool_type": "claude", "status": "interrupted", "workspace_local_path": "/project/beta", "runtime_backend": "native"},
		}
		var filter struct {
			ToolType string   `json:"tool_type"`
			Statuses []string `json:"statuses"`
		}
		if len(request.Params) > 0 && json.Unmarshal(request.Params, &filter) != nil {
			os.Exit(4)
		}
		if filter.ToolType != "" || len(filter.Statuses) > 0 {
			allowed := make(map[string]bool, len(filter.Statuses))
			for _, status := range filter.Statuses {
				allowed[status] = true
			}
			filtered := items[:0]
			for _, raw := range items {
				item := raw.(map[string]any)
				if filter.ToolType != "" && item["tool_type"] != filter.ToolType {
					continue
				}
				if len(allowed) > 0 && !allowed[item["status"].(string)] {
					continue
				}
				filtered = append(filtered, raw)
			}
			items = filtered
		}
		result = map[string]any{"items": items}
		remoteSessionFixture(result.(map[string]any), mode)
	case "sessions.create":
		var input struct {
			Expected  map[string]any `json:"expected_context"`
			Tool      string         `json:"tool_type"`
			Account   string         `json:"tool_account_id"`
			Workspace string         `json:"workspace_id"`
			Project   string         `json:"project_key"`
			Digest    string         `json:"argv_digest"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.Tool == "" || input.Account == "" || input.Workspace == "" || input.Project == "" || input.Digest == "" {
			os.Exit(4)
		}
		requestID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
		sessionID := "44444444-4444-4444-8444-444444444444"
		result = map[string]any{
			"schema_version": 1, "request_id": requestID, "tool_type": input.Tool,
			"tool_account_id": input.Account, "workspace_id": input.Workspace, "project_key": input.Project,
			"argv_digest": input.Digest, "phase": "accepted", "node_id": "33333333-3333-4333-8333-333333333333",
			"runtime_backend": "native", "session_id": sessionID, "task_id": "create_tool_session:" + sessionID,
			"task_record_id": "55555555-5555-4555-8555-555555555555", "context": input.Expected,
		}
	case "sessions.create.request":
		var input struct {
			RequestID string         `json:"request_id"`
			Expected  map[string]any `json:"expected_context"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.RequestID == "" {
			os.Exit(4)
		}
		result = map[string]any{
			"schema_version": 1, "request_id": input.RequestID, "tool_type": "claude",
			"tool_account_id": "11111111-1111-4111-8111-111111111111",
			"workspace_id":    "22222222-2222-4222-8222-222222222222", "project_key": "project",
			"argv_digest": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			"phase":       "accepted", "node_id": "33333333-3333-4333-8333-333333333333", "runtime_backend": "native",
			"session_id": "44444444-4444-4444-8444-444444444444", "task_id": "create_tool_session:44444444-4444-4444-8444-444444444444",
			"task_record_id": "55555555-5555-4555-8555-555555555555", "context": input.Expected,
		}
	case "sessions.stop":
		result = remoteStopFixture(request.Params, home, mode, false)
	case "sessions.stop.request", "sessions.stop.status":
		result = remoteStopFixture(request.Params, home, mode, true)
	case "sessions.delete":
		result = remoteDeleteFixture(request.Params, home, mode, false)
	case "sessions.delete.request", "sessions.delete.status":
		result = remoteDeleteFixture(request.Params, home, mode, true)
	case "accounts.import.discover", "accounts.import.prepare", "accounts.import.validate", "accounts.import.apply", "accounts.import.status", "accounts.import.pending", "accounts.import.forget":
		result = accountImportFixture(request.Method, request.ID, request.Params, home, mode)
	case "accounts.runtime_recovery.authority", "accounts.runtime_recovery.inspect", "accounts.runtime_recovery.apply", "accounts.runtime_recovery.status", "accounts.runtime_recovery.pending", "accounts.runtime_recovery.forget":
		result = runtimeRecoveryFixture(request.Method, request.ID, request.Params, home, mode)
	case "mutagen.environment.preview", "mutagen.environment.apply":
		result = mutagenEnvironmentFixture(request.Method, request.ID, request.Params, home, mode)
	case "sync.recovery.preview", "sync.recovery.apply":
		result = syncRecoveryFixture(request.Method, request.ID, request.Params, mode)
	case "sync.status", "sync.ensure", "sync.pause", "sync.resume", "sync.resolve", "sync.reset":
		result = syncFixture(request.Method, request.ID, request.Params, mode)
	case "accounts.create", "accounts.status", "accounts.bind", "accounts.verify", "accounts.disable":
		result = accountsFixture(request.Method, request.ID, request.Params, home, mode)
	case "accounts.default.get", "accounts.default.set", "accounts.default.clear":
		result = accountDefaultFixture(request.Method, request.ID, request.Params, home, mode)
	case "ego_browser.status":
		deviceStatus := "active"
		bindingStatus := "connected"
		if mode == "ego-browser-delete-terminal" {
			deviceStatus = "revoked"
			bindingStatus = "stopped"
		}
		result = map[string]any{
			"schema_version": 1, "operation": "ego_browser.status", "context": map[string]any{"home": home, "server_url": "https://test.invalid", "active_device_id": "device-1"},
			"local":    map[string]any{"installation": "installed", "store": "present", "admission": "unknown"},
			"policy":   map[string]any{"enabled": true, "enrollment": "allowed", "execution": "allowed", "protocol": "v1"},
			"devices":  []any{map[string]any{"id": "browser-device-1", "generation": 3, "device_generation": 3, "status": deviceStatus, "release_profile": "community", "bridge_version": "1.2.3", "runtime_version": "1.2.3", "skill_version": "1.2.3"}},
			"bindings": []any{map[string]any{"id": "binding-1", "device_id": "browser-device-1", "tool_session_id": "session-1", "node_id": "node-1", "status": bindingStatus, "relay_binding_kind": "direct", "authorization_mode": "full", "release_profile": "community", "bridge_protocol_version": "v1", "lease_health": "healthy", "generation": 2, "binding_generation": 2, "connected_at": "2026-10-21T00:00:00Z", "lease_until": "2026-10-21T01:00:00Z", "stop_reason": nil}},
		}
		if mode == "ego-browser-unknown-field" {
			result.(map[string]any)["unexpected"] = true
		}
	case "ego_browser.candidates":
		var input struct {
			Expected map[string]any `json:"expected_context"`
		}
		if json.Unmarshal(request.Params, &input) != nil {
			os.Exit(4)
		}
		result = map[string]any{
			"schema_version": 1, "operation": "ego_browser.candidates", "context": input.Expected,
			"items": []any{map[string]any{"tool_session_id": "session-1", "tool_type": "claude", "tool_account_id": "account-1", "workspace_id": "workspace-1", "project_key": "project-1", "display_name": "Claude Studio", "status": "ready", "node_id": "node-1", "runtime_backend": "native", "current_ego_browser_device_id": nil, "current_ego_browser_device_name": nil, "binding_id": nil, "controllable": true}},
		}
	case "ego_browser.claim.plan":
		var input struct {
			Expected    map[string]any `json:"expected_context"`
			Session     string         `json:"tool_session_id"`
			Fingerprint string         `json:"candidate_fingerprint"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.Session == "" || input.Fingerprint == "" {
			os.Exit(4)
		}
		candidate := map[string]any{"tool_session_id": input.Session, "tool_type": "claude", "tool_account_id": "account-1", "workspace_id": "workspace-1", "project_key": "project-1", "display_name": "Claude Studio", "status": "ready", "node_id": "node-1", "runtime_backend": "native", "current_ego_browser_device_id": nil, "current_ego_browser_device_name": nil, "binding_id": nil, "controllable": true}
		result = map[string]any{"schema_version": 1, "operation": "ego_browser.claim.plan", "context": input.Expected, "candidate": candidate, "candidate_fingerprint": input.Fingerprint, "local_admission": "ready", "requires_confirmation": true, "allowed": true}
	case "ego_browser.claim.control":
		var input struct {
			Confirmed   bool           `json:"confirmed"`
			Expected    map[string]any `json:"expected_context"`
			Session     string         `json:"tool_session_id"`
			Fingerprint string         `json:"candidate_fingerprint"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.Session == "" || input.Fingerprint == "" {
			os.Exit(4)
		}
		candidate := map[string]any{"tool_session_id": input.Session, "tool_type": "claude", "tool_account_id": "account-1", "workspace_id": "workspace-1", "project_key": "project-1", "display_name": "Claude Studio", "status": "ready", "node_id": "node-1", "runtime_backend": "native", "current_ego_browser_device_id": nil, "current_ego_browser_device_name": nil, "binding_id": nil, "controllable": true}
		result = map[string]any{"operation_id": request.ID, "schema_version": 1, "operation": "ego_browser.claim.control", "context": input.Expected, "candidate": candidate, "candidate_fingerprint": input.Fingerprint, "status": "claim_submitted", "local_admission": "open"}
	case "ego_browser.requests":
		var input struct {
			Expected  map[string]any `json:"expected_context"`
			BindingID string         `json:"binding_id"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.BindingID == "" {
			os.Exit(4)
		}
		result = map[string]any{
			"schema_version": 1, "operation": "ego_browser.requests", "context": input.Expected,
			"binding_id": input.BindingID, "binding_generation": 2,
			"items": []any{map[string]any{"id": "request-ledger-1", "binding_id": input.BindingID, "generation": 2, "binding_generation": 2, "request_id": "request-1", "sequence": 5, "message_type": "evaluate", "payload_bytes": 42, "status": "active", "created_at": "2026-10-21T00:00:00Z"}},
		}
		if mode == "ego-browser-request-unknown-field" {
			result.(map[string]any)["unexpected"] = true
		}
	case "ego_browser.request.cancel":
		var input struct {
			Expected          map[string]any `json:"expected_context"`
			BindingID         string         `json:"binding_id"`
			RequestID         string         `json:"request_id"`
			BindingGeneration uint64         `json:"binding_generation"`
			Generation        uint64         `json:"generation"`
			Sequence          uint64         `json:"sequence"`
			Confirmed         bool           `json:"confirmed"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.BindingID == "" || input.RequestID == "" {
			os.Exit(4)
		}
		result = map[string]any{
			"operation_id": request.ID, "schema_version": 1, "operation": "ego_browser.request.cancel", "context": input.Expected,
			"binding_id": input.BindingID, "request_id": input.RequestID, "status": "cancel_requested",
			"item": map[string]any{"id": input.RequestID, "binding_id": input.BindingID, "generation": input.Generation, "binding_generation": input.BindingGeneration, "request_id": "request-1", "sequence": input.Sequence, "message_type": "evaluate", "payload_bytes": 42, "status": "cancel_requested", "created_at": "2026-10-21T00:00:00Z"},
		}
	case "ego_browser.binding.plan":
		var input struct {
			Expected          map[string]any `json:"expected_context"`
			BindingID         string         `json:"binding_id"`
			BindingGeneration uint64         `json:"binding_generation"`
			Generation        uint64         `json:"generation"`
			Action            string         `json:"action"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.BindingID == "" || input.BindingGeneration == 0 || input.Generation == 0 || input.Action == "" {
			os.Exit(4)
		}
		result = map[string]any{
			"schema_version": 1, "operation": "ego_browser.binding.plan", "context": input.Expected,
			"binding": map[string]any{"id": input.BindingID, "device_id": "browser-device-1", "tool_session_id": "session-1", "node_id": "node-1", "status": "connected", "lease_health": "healthy", "generation": input.Generation, "binding_generation": input.BindingGeneration, "lease_until": "2026-10-21T01:00:00Z", "stop_reason": nil},
			"action":  input.Action, "allowed": input.Action != "resume", "requires_confirmation": true, "generation": input.Generation, "binding_generation": input.BindingGeneration,
		}
	case "ego_browser.binding.control":
		var input struct {
			Confirmed         bool           `json:"confirmed"`
			Expected          map[string]any `json:"expected_context"`
			BindingID         string         `json:"binding_id"`
			BindingGeneration uint64         `json:"binding_generation"`
			Generation        uint64         `json:"generation"`
			Action            string         `json:"action"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.BindingID == "" || input.BindingGeneration == 0 || input.Generation == 0 || input.Action == "" {
			os.Exit(4)
		}
		status := "paused"
		if input.Action == "stop" {
			status = "stopped"
		}
		if input.Action == "revoke" {
			status = "revoked"
		}
		if input.Action == "resume" {
			status = "connected"
		}
		result = map[string]any{
			"operation_id": request.ID, "schema_version": 1, "operation": "ego_browser.binding.control", "context": input.Expected,
			"binding": map[string]any{"id": input.BindingID, "device_id": "browser-device-1", "tool_session_id": "session-1", "node_id": "node-1", "status": status, "lease_health": "healthy", "generation": input.Generation + 1, "binding_generation": input.BindingGeneration + 1, "lease_until": nil, "stop_reason": nil},
			"action":  input.Action, "status": status, "generation": input.Generation + 1, "binding_generation": input.BindingGeneration + 1,
		}
	case "ego_browser.delete.plan":
		var input struct {
			Expected          map[string]any `json:"expected_context"`
			Target            string         `json:"target"`
			ID                string         `json:"id"`
			Generation        uint64         `json:"generation"`
			BindingGeneration uint64         `json:"binding_generation"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.ID == "" || input.Generation == 0 || (input.Target == "binding" && input.BindingGeneration == 0) || (input.Target == "device" && input.BindingGeneration != 0) {
			os.Exit(4)
		}
		status := "revoked"
		allowed := input.Target == "device" && input.Generation == 3
		if input.Target == "binding" {
			status = "stopped"
			allowed = input.Generation == 2 && input.BindingGeneration == 2
		}
		result = map[string]any{
			"schema_version": 1, "operation": "ego_browser.delete.plan", "context": input.Expected,
			"target": input.Target, "id": input.ID, "status": status, "allowed": allowed,
			"requires_confirmation": true, "generation": input.Generation, "binding_generation": input.BindingGeneration,
		}
		if mode == "ego-browser-delete-unknown-field" {
			result.(map[string]any)["unexpected"] = true
		}
	case "ego_browser.delete.control":
		var input struct {
			Confirmed         bool           `json:"confirmed"`
			Expected          map[string]any `json:"expected_context"`
			Target            string         `json:"target"`
			ID                string         `json:"id"`
			Generation        uint64         `json:"generation"`
			BindingGeneration uint64         `json:"binding_generation"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.ID == "" || input.Generation == 0 {
			os.Exit(4)
		}
		result = map[string]any{
			"operation_id": request.ID, "schema_version": 1, "operation": "ego_browser.delete.control", "context": input.Expected,
			"target": input.Target, "id": input.ID, "status": "deleted", "generation": input.Generation, "binding_generation": input.BindingGeneration, "local_cleanup_complete": true,
		}
		if mode == "ego-browser-delete-malformed-receipt" {
			result.(map[string]any)["status"] = "unknown"
		}
	case "device.status":
		var input struct {
			Expected map[string]any `json:"expected_context"`
		}
		if json.Unmarshal(request.Params, &input) != nil {
			os.Exit(4)
		}
		result = map[string]any{
			"context": input.Expected,
			"device":  map[string]any{"id": "device-1", "name": "Studio Device", "platform": "macos", "status": "active", "last_seen_at": "2026-10-21T00:00:00Z", "created_at": "2026-01-01T00:00:00Z"},
		}
	case "device.local.status":
		var input struct {
			ExpectedHome string `json:"expected_home"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.ExpectedHome != home {
			os.Exit(4)
		}
		version := "1.0.0"
		result = map[string]any{
			"schema_version": 1, "operation": "device.local.status", "home": home, "platform": "macos",
			"app": map[string]any{
				"installation": "installed", "path": filepath.Join(home, "Applications", "Agent Remote Device.app"), "version": version,
				"bundle_identifier": "valid", "code_signature": "valid", "signing_identity": "valid",
				"gatekeeper": "not_required", "network_broker_xpc": "present", "gui_executor_xpc": "present", "running": true,
			},
		}
	case "device.local.plan":
		var input struct {
			ExpectedHome string  `json:"expected_home"`
			Action       string  `json:"action"`
			Source       *string `json:"source"`
		}
		if json.Unmarshal(request.Params, &input) != nil || input.ExpectedHome != home {
			os.Exit(4)
		}
		if input.Action != "install" && input.Action != "uninstall" && input.Action != "launch" && input.Action != "diagnose" {
			os.Exit(4)
		}
		stages := map[string][]any{
			"install":   {map[string]any{"id": "validate_source", "effect": "read", "state": "planned"}, map[string]any{"id": "stage_bundle", "effect": "write", "state": "planned"}, map[string]any{"id": "replace_destination", "effect": "write", "state": "planned"}, map[string]any{"id": "verify_installation", "effect": "read", "state": "planned"}, map[string]any{"id": "cleanup_staging", "effect": "write", "state": "planned"}},
			"uninstall": {map[string]any{"id": "stop_process", "effect": "write", "state": "planned"}, map[string]any{"id": "remove_bundle", "effect": "write", "state": "planned"}, map[string]any{"id": "clear_local_data", "effect": "write", "state": "planned"}, map[string]any{"id": "verify_removal", "effect": "read", "state": "planned"}},
			"launch":    {map[string]any{"id": "verify_installation", "effect": "read", "state": "planned"}, map[string]any{"id": "verify_credential", "effect": "read", "state": "planned"}, map[string]any{"id": "launch_process", "effect": "write", "state": "planned"}},
			"diagnose":  {map[string]any{"id": "inspect_bundle", "effect": "read", "state": "planned"}, map[string]any{"id": "inspect_platform", "effect": "read", "state": "planned"}, map[string]any{"id": "inspect_runtime", "effect": "read", "state": "planned"}},
		}
		result = map[string]any{
			"schema_version": 1, "operation": "device.local.plan", "home": home, "platform": "macos", "action": input.Action,
			"terminal_method": "agent-remote device " + input.Action, "requires_confirmation": input.Action != "diagnose", "destructive": input.Action == "install" || input.Action == "uninstall", "resume_policy": "inspect_before_retry", "stages": stages[input.Action],
		}
		if input.Source != nil {
			result.(map[string]any)["source"] = *input.Source
		}
	case "wireguard.config.plan", "wireguard.config.apply":
		result = wireguardFixture(request.Method, request.ID, request.Params, home, mode)
	case "wireguard.check", "wireguard.status", "wireguard.bridge.status":
		result = wireguardNetworkFixture(request.Method, request.ID, request.Params, home, mode)
	case "wireguard.tunnel.plan":
		result = wireguardTunnelFixture(request.ID, request.Params, home, mode)
	case "wireguard.tunnel.apply":
		var input struct {
			Confirmed    bool           `json:"confirmed"`
			ExpectedHome string         `json:"expected_home"`
			Action       string         `json:"action"`
			Config       *string        `json:"config"`
			ExpectedPlan map[string]any `json:"expected_plan"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed || input.ExpectedHome != home || input.ExpectedPlan == nil {
			wireguardFixtureError(request.ID, "invalid_params")
		}
		captureWireGuard("DESKTOP_TEST_WIREGUARD_TUNNEL_APPLY", request.Params)
		markWireGuard("DESKTOP_TEST_WIREGUARD_TUNNEL_APPLY_STARTED")
		waitControlBarrier("DESKTOP_TEST_WIREGUARD_TUNNEL_APPLY_RELEASE", "")
		if mode == "wg-tunnel-apply-slow" {
			time.Sleep(time.Minute)
		}
		if mode == "wg-tunnel-apply-unknown" {
			wireguardFixtureError(request.ID, "wireguard_outcome_unknown")
		}
		if mode == "wg-tunnel-apply-stale" {
			wireguardFixtureError(request.ID, "wireguard_plan_stale")
		}
		path := filepath.Join(home, "wireguard", "agent-remote.conf")
		if input.Config != nil {
			path = *input.Config
		}
		target := "present"
		if input.Action == "down" {
			target = "absent"
		}
		// The protected broker returns its own digest-bound operation identity;
		// it is intentionally distinct from the outer desktop request ID.
		result = map[string]any{"operation_id": strings.Repeat("a", 64), "home": home, "action": input.Action, "config": path, "state": "observed", "target_state": target}
	case "status.inspect", "doctor.inspect", "doctor.repair", "deps.inspect", "deps.repair", "ssh.inspect":
		result, _ = commandDiagnosticsFixture(request.Method, request.Params, home, commandDiagnosticsMode())
	case "setup.local":
		result = setupFixture(request.ID, request.Params, home, mode)
	case "logout":
		var input struct {
			Confirmed bool           `json:"confirmed"`
			Revoke    bool           `json:"revoke_remote"`
			Expected  map[string]any `json:"expected_context"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed {
			os.Exit(4)
		}
		if path := os.Getenv("DESKTOP_TEST_LOGOUT_RECEIPT"); path != "" {
			if os.WriteFile(path, request.Params, 0600) != nil {
				os.Exit(5)
			}
		}
		state := "confirmed"
		if !input.Revoke {
			state = "not_requested"
		} else if mode == "logout-partial" {
			state = "unconfirmed"
		}
		result = map[string]any{"operation_id": request.ID, "server_url": input.Expected["server_url"], "active_device_id": input.Expected["active_device_id"],
			"user": map[string]any{"local_removed": true, "remote": state}, "device": map[string]any{"local_removed": true, "remote": state},
			"broker_removed": true, "refresh_metadata_removed": true, "config_cleared": true, "local_complete": true}
		if mode == "logout-malformed" {
			delete(result.(map[string]any), "local_complete")
		}
		if mode == "logout-local-incomplete" {
			receipt := result.(map[string]any)
			receipt["device"].(map[string]any)["local_removed"] = false
			receipt["config_cleared"], receipt["local_complete"] = false, false
		}
		if mode == "logout-wrong-operation" {
			result.(map[string]any)["operation_id"] = "wrong"
		}
	case "login.device_code":
		deviceCodeFixture(request.ID, request.Params, mode)
		return true
	case "login.password":
		var input struct {
			Server    string         `json:"server_url"`
			Username  string         `json:"username"`
			Password  string         `json:"password"`
			TOTP      *string        `json:"totp_code"`
			Confirmed bool           `json:"confirmed"`
			Expected  map[string]any `json:"expected_context"`
		}
		if json.Unmarshal(request.Params, &input) != nil || !input.Confirmed {
			os.Exit(4)
		}
		if path := os.Getenv("DESKTOP_TEST_LOGIN_RECEIPT"); path != "" {
			safe := map[string]any{"server": input.Server, "username": input.Username, "expected": input.Expected, "password_received": input.Password == "private-login-password", "totp_received": input.TOTP != nil && *input.TOTP == "654321", "args": os.Args[1:]}
			data, _ := json.Marshal(safe)
			if os.WriteFile(path, data, 0600) != nil {
				os.Exit(5)
			}
		}
		result = map[string]any{"operation_id": request.ID, "server_url": strings.TrimRight(strings.TrimSpace(input.Server), "/"), "registered_device_id": "device-1", "credential_backend": "file", "expires_in": 3600}
	default:
		result, _ = diagnosticsFixture(request.Method, request.Params, home, mode)
	}
	if mode == "missing-items" && request.Method == "sessions.list" {
		result = map[string]any{}
	}
	if mode == "skills-conflicts-unknown" && request.Method == "skills.state.conflicts" {
		result.(map[string]any)["private_saved_input"] = "must-stay-in-cli"
	}
	if mode == "skills-info-unknown" && request.Method == "skills.info" {
		result.(map[string]any)["unexpected_private_field"] = "must-stay-in-cli"
	}
	if mode == "skills-diff-unknown" && request.Method == "skills.state.diff" {
		result.(map[string]any)["unexpected_private_field"] = "must-stay-in-cli"
	}
	if request.Method == "accounts.list" || request.Method == "sessions.list" || request.Method == "credentials.list" || request.Method == "skills.list" || request.Method == "skills.info" || request.Method == "skills.state.list" || request.Method == "skills.state.conflicts" || request.Method == "skills.state.info" || request.Method == "skills.state.diff" {
		server := "https://test.invalid"
		if mode == "context-changed" {
			server = "https://different.invalid"
		}
		responseHome := home
		if mode == "context-home-changed" {
			responseHome = filepath.Join(home, "other-home")
		}
		result.(map[string]any)["context"] = map[string]any{"home": responseHome, "server_url": server, "active_device_id": "device-1"}
	}
	if request.Method == "context" {
		applyRemoteContextFixture(result.(map[string]any))
	}
	if request.Method == "accounts.list" || request.Method == "sessions.list" || request.Method == "credentials.list" || request.Method == "skills.list" || request.Method == "skills.info" || request.Method == "skills.state.list" || request.Method == "skills.state.conflicts" || request.Method == "skills.state.info" || request.Method == "skills.state.diff" {
		applyRemoteContextFixture(result.(map[string]any)["context"].(map[string]any))
	}
	if request.StreamEvents {
		phase := "started"
		progress := map[string]any{"operation": request.Method, "stage": "started"}
		switch mode {
		case "progress-invalid-phase":
			phase = "unknown"
		case "progress-nonobject":
			progress = nil
		case "progress-too-many":
			for i := 0; i < 129; i++ {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": request.ID, "type": "progress", "phase": phase, "progress": map[string]any{"index": i}})
			}
		}
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": request.ID, "type": "progress", "phase": phase, "progress": progress})
	}
	value := map[string]any{"protocol_version": 1, "request_id": request.ID, "type": "result", "phase": "completed", "result": result}
	code := 0
	if request.Method == "mutagen.environment.apply" && mode == "environment-signal-unknown" {
		delete(value, "result")
		value["type"], value["phase"] = "error", "failed"
		value["error"] = map[string]any{"code": "sync_mutagen_environment_outcome_unknown", "retryable": false}
		code = 1
	}
	if (request.Method == "sync.status" && mode == "recovery-stale") || (request.Method == "sync.recovery.preview" && (mode == "recovery-denied" || mode == "recovery-unavailable" || mode == "recovery-not-required")) {
		failure := "sync_not_found"
		switch mode {
		case "recovery-denied":
			failure = "permission_denied"
		case "recovery-unavailable":
			failure = "server_unavailable"
		case "recovery-not-required":
			failure = "sync_recovery_not_required"
		}
		delete(value, "result")
		value["type"], value["phase"] = "error", "failed"
		value["error"] = map[string]any{"code": failure, "retryable": false}
		code = 1
	}
	switch mode {
	case "setup-capability-unsupported-version":
		if request.Method == "capabilities" {
			delete(value, "result")
			value["type"], value["phase"] = "error", "failed"
			value["error"] = map[string]any{"code": "unsupported_version", "retryable": false}
			code = 1
		}
	case "login-unknown":
		if request.Method == "login.password" {
			delete(value, "result")
			value["type"], value["phase"] = "error", "failed"
			value["error"] = map[string]any{"code": "registration_outcome_unknown", "retryable": false}
			code = 1
		}
	case "wrong-id":
		value["request_id"] = "unrelated"
	case "wrong-version":
		value["protocol_version"] = 9
	case "wrong-phase":
		value["phase"] = "pending"
	case "null-result":
		value["result"] = nil
	case "result-failed-exit":
		code = 1
	case "permission":
		if request.Method == "accounts.list" {
			delete(value, "result")
			value["type"], value["phase"] = "error", "failed"
			value["error"] = map[string]any{"code": "permission_denied", "retryable": false, "http_status": 403}
			code = 1
		}
	case "raw-error":
		fmt.Fprintln(os.Stderr, "private diagnostic should never reach the UI")
		os.Exit(1)
	case "oversized":
		value["result"] = strings.Repeat("x", 5*1024*1024)
	}
	json.NewEncoder(os.Stdout).Encode(value)
	// Tests that exercise desktop shutdown can wait until the control helper
	// has emitted a response without advancing the UI frame. This makes the
	// worker-result drain deterministic while keeping the marker content free
	// of request parameters or receipt data.
	if path := os.Getenv("DESKTOP_TEST_CLI_RESPONSE_FILE"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(6)
		}
		_, _ = fmt.Fprintln(f, request.Method)
		_ = f.Close()
	}
	if mode == "trailing" {
		fmt.Fprintln(os.Stdout, `{"unexpected":"event"}`)
	}
	os.Exit(code)
	return true
}

func InstallControlHelper(t *testing.T) string {
	return InstallControlHelperAs(t, "agent-remote")
}

// InstallControlHelperAs installs this test binary under a caller supplied
// executable name. The schema fixture derives its advertised executable from
// argv[0], so fclaude tests must execute a real fclaude-named image on
// Windows; a symlink without .exe is not a runnable CreateProcess target.
func InstallControlHelperAs(t *testing.T, name string) string {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		if filepath.Ext(name) == "" {
			name += ".exe"
		}
	}
	target := filepath.Join(dir, name)
	// Reuse the executable already loaded by this test process. Fresh copies
	// can stall in macOS dyld before Go or the fixture starts. The helper is
	// read-only; each test still owns its CLI path and environment. Windows
	// keeps a copy because creating symlinks may require additional privileges.
	if runtime.GOOS != "windows" {
		if err := os.Symlink(executable, target); err != nil {
			t.Fatalf("link test executable: %v", err)
		}
		t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
		return target
	}
	source, err := os.Open(executable)
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	dest, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, copyErr := io.Copy(dest, source)
	closeErr := dest.Close()
	if copyErr != nil || closeErr != nil {
		t.Fatalf("copy test executable: %v %v", copyErr, closeErr)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	return target
}

func ptrUint64(value uint64) *uint64 { return &value }
