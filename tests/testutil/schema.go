package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func schemaHelper() bool {
	if len(os.Args) != 2 || os.Args[1] != "--desktop-schema" {
		return false
	}
	mode := os.Getenv("DESKTOP_TEST_CLI_SCENARIO")
	if mode == "slow-schema" {
		time.Sleep(time.Minute)
		return true
	}
	if mode == "oversized-schema" {
		fmt.Print(strings.Repeat("x", 5*1024*1024))
		return true
	}
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if path := os.Getenv("DESKTOP_TEST_CLI_LOG"); path != "" {
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			os.Exit(3)
		}
		fmt.Fprintln(f, "schema:"+name)
		f.Close()
	}
	commands := []any{
		map[string]any{"path": []string{}, "has_subcommands": true, "arguments": []any{}},
		map[string]any{"path": []string{"init"}, "arguments": []any{}},
		map[string]any{"path": []string{"attach"}, "arguments": []any{}},
		// `status` is intentionally left without a dedicated desktop mapping;
		// capability coverage must still surface it with the unknown permission
		// state instead of silently dropping an otherwise valid CLI command.
		map[string]any{"path": []string{"status"}, "arguments": []any{}},
		map[string]any{"path": []string{"doctor"}, "arguments": []any{
			map[string]any{"id": "fix", "long": "fix", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		map[string]any{"path": []string{"device", "install"}, "platforms": []string{"macos"}, "arguments": []any{
			map[string]any{"id": "source", "long": "source", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"APP_OR_ZIP"}},
		}},
		map[string]any{"path": []string{"device", "uninstall"}, "platforms": []string{"macos"}, "arguments": []any{
			map[string]any{"id": "yes", "long": "yes", "short": "y", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		map[string]any{"path": []string{"device", "status"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"device", "launch"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"device", "diagnose"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"device", "revoke"}, "arguments": []any{}},
		map[string]any{"path": []string{"credentials", "create"}, "arguments": []any{
			map[string]any{"id": "name", "long": "name", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"NAME"}},
		}},
		map[string]any{"path": []string{"credentials", "bind"}, "arguments": []any{
			map[string]any{"id": "account", "long": "account", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"ACCOUNT_ID"}},
		}},
		map[string]any{"path": []string{"credentials", "unbind"}, "arguments": []any{
			map[string]any{"id": "account", "long": "account", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"ACCOUNT_ID"}},
		}},
		map[string]any{"path": []string{"credentials", "list"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "list"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "create"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "bind"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "attach"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "import-config"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "verify"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "status"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "disable"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "recover-runtime"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "recovery-status"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "default", "set"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "default", "get"}, "arguments": []any{}},
		map[string]any{"path": []string{"account", "default", "clear"}, "arguments": []any{}},
		map[string]any{"path": []string{"deps", "mutagen-environment"}, "arguments": []any{}},
		map[string]any{"path": []string{"deps", "status"}, "arguments": []any{
			map[string]any{"id": "fix", "long": "fix", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		map[string]any{"path": []string{"ssh", "check"}, "arguments": []any{
			map[string]any{"id": "session_id", "long": "session-id", "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"ID"}},
		}},
		map[string]any{"path": []string{"sync", "ensure"}, "arguments": []any{}},
		map[string]any{"path": []string{"sync", "status"}, "arguments": []any{}},
		map[string]any{"path": []string{"sync", "pause"}, "arguments": []any{}},
		map[string]any{"path": []string{"sync", "resume"}, "arguments": []any{}},
		map[string]any{"path": []string{"sync", "resolve"}, "arguments": []any{}},
		map[string]any{"path": []string{"sync", "reset"}, "arguments": []any{}},
		map[string]any{"path": []string{"wireguard", "config"}, "platforms": []string{"linux", "macos", "windows"}, "arguments": []any{}},
		map[string]any{"path": []string{"wireguard", "check"}, "platforms": []string{"linux", "macos", "windows"}, "arguments": []any{}},
		map[string]any{"path": []string{"wireguard", "status"}, "platforms": []string{"linux", "macos", "windows"}, "arguments": []any{}},
		map[string]any{"path": []string{"wireguard", "down"}, "platforms": []string{"linux", "macos", "windows"}, "arguments": []any{}},
		map[string]any{"path": []string{"device", "rotate-token"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "list"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "info"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "status"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "check"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "state", "list"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "state", "conflicts"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "state", "info"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "state", "diff"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "state", "export"}, "arguments": []any{
			map[string]any{"id": "output", "long": "output", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"OUTPUT"}, "value_hint": "AnyPath"},
		}},
		map[string]any{"path": []string{"skill", "add"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "update"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "enable"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "disable"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "pin"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "unpin"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "inherit"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "remove"}, "arguments": []any{}},
		map[string]any{"path": []string{"skill", "rollback"}, "arguments": []any{}},
		map[string]any{"path": []string{"node", "install"}, "arguments": []any{
			map[string]any{"id": "node", "long": "node", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"NODE_REFERENCE"}},
			map[string]any{"id": "enable_ego_browser", "long": "enable-ego-browser", "action": "SetTrue", "min_values": 0, "max_values": 0},
			map[string]any{"id": "yes", "long": "yes", "short": "y", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		map[string]any{"path": []string{"login"}, "arguments": []any{
			map[string]any{"id": "password", "long": "password", "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"PASSWORD"}},
			map[string]any{"id": "totp_code", "long": "totp-code", "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"CODE"}},
		}},
		map[string]any{"path": []string{"logout"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "status"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "list"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "requests"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "cancel-request"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "delete-device"}, "platforms": []string{"macos"}, "arguments": []any{
			map[string]any{"id": "id", "positional_index": 1, "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"ID"}},
			map[string]any{"id": "yes", "long": "yes", "short": "y", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		map[string]any{"path": []string{"ego-browser", "delete-binding"}, "platforms": []string{"macos"}, "arguments": []any{
			map[string]any{"id": "id", "positional_index": 1, "required": true, "action": "Set", "min_values": 1, "max_values": 1, "value_names": []string{"ID"}},
			map[string]any{"id": "yes", "long": "yes", "short": "y", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		map[string]any{"path": []string{"ego-browser", "connect"}, "platforms": []string{"macos"}, "arguments": []any{
			map[string]any{"id": "tool_session", "positional_index": 1, "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"TOOL_SESSION"}},
			map[string]any{"id": "yes", "long": "yes", "short": "y", "action": "SetTrue", "min_values": 0, "max_values": 0},
		}},
		// These aliases are backed by the structured ego-browser binding and
		// claim contracts. Keep them in the fixture so capability-page route
		// tests exercise the same command set as the released schema.
		map[string]any{"path": []string{"ego-browser", "claim"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "pause"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "resume"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "stop"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"ego-browser", "revoke"}, "platforms": []string{"macos"}, "arguments": []any{}},
		map[string]any{"path": []string{"wireguard", "up"}, "platforms": []string{"linux", "macos", "windows"}, "arguments": []any{}},
		// `forward` has optional list/stop subcommands but also performs the
		// create operation directly. Keep it in the fixture so capability-page
		// tests cover the parent operation as well as its children.
		map[string]any{"path": []string{"forward"}, "has_subcommands": true, "arguments": []any{
			map[string]any{"id": "remote_port", "positional_index": 1, "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"REMOTE_PORT"}},
			map[string]any{"id": "session", "long": "session", "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"SESSION"}},
			map[string]any{"id": "local_port", "long": "local-port", "action": "Set", "defaults": []string{"same"}, "min_values": 1, "max_values": 1, "value_names": []string{"PORT"}},
			map[string]any{"id": "open", "long": "open", "action": "SetTrue", "min_values": 0, "max_values": 0},
			map[string]any{"id": "ttl_seconds", "long": "ttl-seconds", "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"SECONDS"}},
		}},
	}
	if name == "fclaude" {
		commands = []any{
			map[string]any{"path": []string{}, "has_subcommands": true, "arguments": []any{
				map[string]any{"id": "claude_args", "positional_index": 1, "action": "Append", "min_values": 1, "value_names": []string{"CLAUDE_ARG"}},
			}},
			map[string]any{"path": []string{"run"}, "arguments": []any{
				map[string]any{"id": "claude_args", "positional_index": 1, "action": "Append", "min_values": 0, "value_names": []string{"CLAUDE_ARG"}},
			}},
			map[string]any{"path": []string{"new"}, "arguments": []any{}},
			map[string]any{"path": []string{"list"}, "arguments": []any{}},
			map[string]any{"path": []string{"attach"}, "arguments": []any{}},
			// Keep the lifecycle and query leaves in the fixture.  They are
			// present in the released fclaude schema and route to the same
			// context-bound remote session page as attach/new.
			map[string]any{"path": []string{"stop"}, "arguments": []any{}},
			map[string]any{"path": []string{"stop-status"}, "arguments": []any{}},
			map[string]any{"path": []string{"delete"}, "arguments": []any{}},
			map[string]any{"path": []string{"delete-status"}, "arguments": []any{}},
			map[string]any{"path": []string{"forward"}, "has_subcommands": true, "arguments": []any{
				map[string]any{"id": "remote_port", "positional_index": 1, "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"REMOTE_PORT"}},
				map[string]any{"id": "session", "long": "session", "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"SESSION"}},
				map[string]any{"id": "local_port", "long": "local-port", "action": "Set", "defaults": []string{"same"}, "min_values": 1, "max_values": 1, "value_names": []string{"PORT"}},
				map[string]any{"id": "open", "long": "open", "action": "SetTrue", "min_values": 0, "max_values": 0},
				map[string]any{"id": "ttl_seconds", "long": "ttl-seconds", "action": "Set", "min_values": 0, "max_values": 1, "value_names": []string{"SECONDS"}},
			}},
			map[string]any{"path": []string{"forward", "list"}, "arguments": []any{}},
			map[string]any{"path": []string{"forward", "stop"}, "arguments": []any{}},
		}
	}
	if mode == "legacy-schema" {
		for _, command := range commands {
			delete(command.(map[string]any), "platforms")
		}
	}
	version := "test"
	if mode == "schema-version-changed" {
		version = "different"
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"schema_version": 1, "executable": name, "cli_version": version, "commands": commands})
	os.Exit(0)
	return true
}
