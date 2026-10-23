package testutil

import (
	"encoding/json"
	"os"
	"runtime"
)

// commandDiagnosticsFixture mirrors only the versioned desktop-control shape;
// it never emits human CLI output. Tests can opt into malformed responses with
// DESKTOP_TEST_COMMAND_DIAGNOSTICS_MODE to verify the desktop stays fail-closed.
func commandDiagnosticsFixture(method string, params json.RawMessage, home, mode string) (any, bool) {
	if method != "status.inspect" && method != "doctor.inspect" && method != "doctor.repair" && method != "deps.inspect" && method != "deps.repair" && method != "ssh.inspect" {
		return nil, false
	}
	platform := runtime.GOOS
	if platform == "darwin" {
		platform = "macos"
	}
	if method == "ssh.inspect" {
		var options struct {
			SessionID *string `json:"session_id"`
		}
		_ = json.Unmarshal(params, &options)
		result := map[string]any{
			"schema_version": 1, "operation": method, "home": home, "platform": platform,
			"ssh":           map[string]any{"status": "available", "version": "OpenSSH_test"},
			"authorization": map[string]any{"status": "not_requested"},
		}
		if options.SessionID != nil {
			result["authorization"] = map[string]any{
				"status": "succeeded", "session_id": *options.SessionID,
				"node_id":               "22222222-2222-4222-8222-222222222222",
				"authorization_task_id": "33333333-3333-4333-8333-333333333333",
				"expires_in":            uint64(120),
			}
		}
		if mode == "command-diagnostics-ssh-malformed" && options.SessionID != nil {
			result["authorization"].(map[string]any)["node_id"] = "node\nprivate"
		}
		if mode == "command-diagnostics-unknown-field" {
			result["unexpected"] = true
		}
		if mode == "command-diagnostics-wrong-home" {
			result["home"] = home + "-wrong"
		}
		return result, true
	}
	var options struct {
		Online         bool `json:"online"`
		DeepCheck      bool `json:"deep_check"`
		VerifyVersions bool `json:"verify_versions"`
	}
	if json.Unmarshal(params, &options) != nil {
		return nil, false
	}
	if method == "doctor.inspect" || method == "doctor.repair" {
		// Doctor defaults are true in the CLI contract when omitted. Desktop
		// callers send explicit values, but keep the fixture faithful for direct
		// contract tests.
		if len(params) == 0 {
			options.Online, options.DeepCheck, options.VerifyVersions = true, true, true
		}
	}
	result := map[string]any{
		"schema_version": 1, "operation": method, "home": home, "platform": platform,
		"online": options.Online, "verify_versions": options.VerifyVersions, "deep_check": options.DeepCheck,
		"manifest_source": "default",
		"checks":          []any{map[string]any{"id": "manifest", "status": "missing", "suggestion": "repair_installation"}},
		"dependencies": []any{map[string]any{
			"name": "mutagen", "path": home + "/bin/mutagen", "status": "missing",
			"required_version": "managed-by-agent-remote-release", "license": "MIT",
			"license_notice": "THIRD_PARTY_NOTICES.md", "version_status": "not_checked",
		}},
	}
	switch method {
	case "status.inspect":
		result["remote"] = map[string]any{
			"server": map[string]any{"status": "not_checked"},
			"device": map[string]any{"status": "not_checked"},
		}
	case "doctor.repair":
		result["setup"] = map[string]any{
			"home": home, "complete": true,
			"steps": []any{map[string]any{"id": "home", "path": home, "status": "ready", "code": "none"}},
		}
		if mode == "command-diagnostics-setup-wrong-home" {
			result["setup"].(map[string]any)["home"] = home + "-wrong"
		}
	case "deps.repair":
		result["repair"] = map[string]any{"complete": true}
	}
	if mode == "command-diagnostics-unknown-field" {
		result["unexpected"] = true
	}
	if mode == "command-diagnostics-wrong-home" {
		result["home"] = home + "-wrong"
	}
	return result, true
}

func commandDiagnosticsMode() string { return os.Getenv("DESKTOP_TEST_COMMAND_DIAGNOSTICS_MODE") }
