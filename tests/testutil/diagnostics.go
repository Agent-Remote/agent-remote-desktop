package testutil

import (
	"encoding/json"
	"os"
	"runtime"
)

func diagnosticsFixture(method string, params json.RawMessage, home, mode string) (any, bool) {
	if method != "diagnostics.inspect" && method != "dependencies.inspect" {
		return nil, false
	}
	var options struct {
		Deep   bool `json:"deep_check"`
		Online bool `json:"online"`
		Verify bool `json:"verify_versions"`
	}
	if json.Unmarshal(params, &options) != nil {
		return nil, false
	}
	if path := os.Getenv("DESKTOP_TEST_DIAGNOSTICS_PARAMS"); path != "" {
		if os.WriteFile(path, params, 0600) != nil {
			os.Exit(6)
		}
	}
	result := map[string]any{"home": home, "platform": runtime.GOOS, "online": options.Online, "deep_check": options.Deep, "verify_versions": options.Verify, "manifest_source": "default", "checks": []any{map[string]any{"id": "manifest", "status": "missing", "suggestion": "repair_installation"}}, "dependencies": []any{map[string]any{"name": "mutagen", "path": home + "/bin/mutagen", "status": "missing", "required_version": "managed-by-agent-remote-release", "license": "MIT", "license_notice": "THIRD_PARTY_NOTICES.md", "version_status": "not_checked"}}}
	switch mode {
	case "diagnostics-wrong-home":
		result["home"] = home + "-wrong"
	case "diagnostics-missing-checks":
		delete(result, "checks")
	case "diagnostics-bad-status":
		result["checks"].([]any)[0].(map[string]any)["status"] = "authenticated"
	case "diagnostics-online-mismatch":
		result["online"] = !options.Online
	}
	return result, true
}
