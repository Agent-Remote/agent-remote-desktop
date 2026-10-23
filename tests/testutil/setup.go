package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// setupFixture never creates CLI state; its writes are explicit test barriers
// and captured request parameters in temporary directories supplied by tests.
func setupFixture(id string, params json.RawMessage, home, mode string) any {
	var input struct {
		Confirmed    bool   `json:"confirmed"`
		ExpectedHome string `json:"expected_home"`
	}
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if mode == "setup-configuration-unavailable" {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": id, "type": "error", "phase": "failed", "error": map[string]any{"code": "configuration_unavailable", "retryable": false}})
		os.Exit(1)
	}
	if !input.Confirmed {
		json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": id, "type": "error", "phase": "failed", "error": map[string]any{"code": "confirmation_required", "retryable": false}})
		os.Exit(1)
	}
	if path := os.Getenv("DESKTOP_TEST_SETUP_RECEIPT"); path != "" {
		if os.WriteFile(path, params, 0600) != nil {
			os.Exit(5)
		}
	}
	if path := os.Getenv("DESKTOP_TEST_SETUP_STARTED"); path != "" {
		if os.WriteFile(path, nil, 0600) != nil {
			os.Exit(7)
		}
	}
	waitControlBarrier("DESKTOP_TEST_SETUP_RELEASE", "")
	if mode == "slow-setup" {
		time.Sleep(time.Minute)
	}
	steps := []map[string]any{}
	for _, step := range []struct{ id, path string }{
		{"home", home}, {"bin", filepath.Join(home, "bin")}, {"dependencies", filepath.Join(home, "dependencies")},
		{"secrets", filepath.Join(home, "secrets")}, {"wireguard", filepath.Join(home, "wireguard")}, {"ssh", filepath.Join(home, "ssh")},
		{"database", filepath.Join(home, "state.sqlite3")}, {"manifest", filepath.Join(home, "dependencies", "manifest.json")},
	} {
		steps = append(steps, map[string]any{"id": step.id, "path": step.path, "status": "applied", "code": "none"})
	}
	result := map[string]any{"operation_id": id, "home": home, "complete": true, "steps": steps}
	switch mode {
	case "setup-partial":
		steps[6]["status"], steps[6]["code"] = "failed", "unsupported_state_version"
		result["complete"] = false
	case "setup-malformed", "setup-missing-complete":
		delete(result, "complete")
	case "setup-wrong-operation":
		result["operation_id"] = "wrong"
	case "setup-wrong-home":
		result["home"] = filepath.Join(home, "other")
	case "setup-wrong-path":
		steps[6]["path"] = filepath.Join(home, "other.sqlite3")
	case "setup-inconsistent-complete":
		result["complete"] = false
	case "setup-duplicate-step":
		steps[7] = steps[6]
	case "setup-unknown-code":
		steps[7]["code"] = "arbitrary"
	case "setup-unknown-status":
		steps[7]["status"] = "arbitrary"
	case "setup-truncated-steps":
		result["steps"] = steps[:7]
	}
	return result
}
