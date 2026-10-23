package testutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func wireguardFixture(method, id string, params json.RawMessage, home, mode string) any {
	if method == "wireguard.config.plan" {
		var input struct {
			Home   string  `json:"expected_home"`
			Output *string `json:"output"`
		}
		if json.Unmarshal(params, &input) != nil {
			os.Exit(4)
		}
		captureWireGuard("DESKTOP_TEST_WIREGUARD_PLAN", params)
		markWireGuard("DESKTOP_TEST_WIREGUARD_PLAN_STARTED")
		waitControlBarrier("DESKTOP_TEST_WIREGUARD_PLAN_RELEASE", "")
		if mode == "wg-plan-slow" {
			time.Sleep(time.Minute)
		}
		if mode == "wg-plan-invalid" {
			wireguardFixtureError(id, "output_invalid")
		}
		path, anchor, missing := filepath.Join(home, "wireguard", "agent-remote.conf"), home, []string{"wireguard"}
		if input.Output != nil {
			path = *input.Output
			anchor = filepath.Dir(path)
			missing = []string{}
		}
		action := "create"
		var identity any
		if mode == "wg-replace" {
			action, identity = "replace", strings.Repeat("b", 64)
			anchor, missing = filepath.Dir(path), []string{}
		}
		result := map[string]any{"plan_version": 1, "home": home, "context": map[string]any{"server_url": "https://test.invalid", "active_device_id": "device-1"}, "output": map[string]any{
			"path": path, "action": action, "anchor_path": anchor, "anchor_identity": strings.Repeat("a", 64), "missing_directories": missing, "target_identity": identity,
		}}
		return patchWireGuardFixture(result, "DESKTOP_TEST_WIREGUARD_PLAN_PATCH")
	}
	var input struct {
		Confirmed bool           `json:"confirmed"`
		Plan      map[string]any `json:"expected_plan"`
	}
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if !input.Confirmed {
		wireguardFixtureError(id, "confirmation_required")
	}
	captureWireGuard("DESKTOP_TEST_WIREGUARD_APPLY", params)
	markWireGuard("DESKTOP_TEST_WIREGUARD_APPLY_STARTED")
	waitControlBarrier("DESKTOP_TEST_WIREGUARD_APPLY_RELEASE", "")
	if mode == "wg-apply-slow" {
		time.Sleep(time.Minute)
	}
	if mode == "wg-apply-output-changed" {
		wireguardFixtureError(id, "output_changed")
	}
	if mode == "wg-apply-unknown" {
		wireguardFixtureError(id, "wireguard_outcome_unknown")
	}
	output, ok := input.Plan["output"].(map[string]any)
	if !ok {
		os.Exit(4)
	}
	result := map[string]any{"operation_id": id, "home": input.Plan["home"], "context": input.Plan["context"], "output_path": output["path"], "complete": true, "key_state": "reused", "enrollment": "confirmed", "publication": "published", "code": "none", "peer_count": 2, "peer_id": "peer-test", "peer_version": 1}
	switch mode {
	case "wg-partial":
		result["complete"], result["publication"], result["code"], result["peer_count"] = false, "not_attempted", "wireguard_config_invalid", nil
	case "wg-enrollment-unknown":
		result["complete"], result["enrollment"], result["publication"], result["code"], result["peer_count"] = false, "unknown", "not_attempted", "wireguard_enrollment_unknown", nil
	case "wg-key-store-incomplete":
		result["complete"], result["key_state"], result["enrollment"], result["publication"], result["code"], result["peer_count"] = false, "unknown", "not_attempted", "not_attempted", "wireguard_key_store_incomplete", nil
	case "wg-publication-incomplete":
		result["complete"], result["code"] = false, "wireguard_publication_incomplete"
	case "wg-malformed":
		delete(result, "complete")
	case "wg-wrong-operation":
		result["operation_id"] = "wrong"
	}
	return patchWireGuardFixture(result, "DESKTOP_TEST_WIREGUARD_APPLY_PATCH")
}

// Protocol fault injection stays in the test executable. Production code never
// reads these variables or accepts replacement server receipts.
func patchWireGuardFixture(result map[string]any, name string) map[string]any {
	data := os.Getenv(name)
	if path := os.Getenv(name + "_FILE"); data == "" && path != "" {
		contents, err := os.ReadFile(path)
		if err != nil {
			os.Exit(6)
		}
		data = string(contents)
	}
	if data != "" {
		var patch struct {
			Set    map[string]any `json:"set"`
			Remove []string       `json:"remove"`
		}
		decoder := json.NewDecoder(strings.NewReader(data))
		decoder.UseNumber()
		if decoder.Decode(&patch) != nil {
			os.Exit(6)
		}
		for key, value := range patch.Set {
			result[key] = value
		}
		for _, key := range patch.Remove {
			delete(result, key)
		}
	}
	return result
}

func wireguardFixtureError(id, code string) {
	json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": id, "type": "error", "phase": "failed", "error": map[string]any{"code": code, "retryable": false}})
	os.Exit(1)
}

func captureWireGuard(name string, data []byte) {
	if path := os.Getenv(name); path != "" {
		if os.WriteFile(path, data, 0600) != nil {
			os.Exit(5)
		}
	}
}

func markWireGuard(name string) { captureWireGuard(name, nil) }
