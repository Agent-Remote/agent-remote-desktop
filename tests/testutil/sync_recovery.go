package testutil

import (
	"encoding/json"
	"os"
	"strings"
)

func syncRecoveryFixture(method, id string, params json.RawMessage, mode string) any {
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if path := os.Getenv("DESKTOP_TEST_SYNC_RECOVERY_REQUEST"); path != "" && method == "sync.recovery.apply" {
		if os.WriteFile(path, params, 0600) != nil {
			os.Exit(4)
		}
	}
	if method == "sync.recovery.apply" {
		waitControlBarrier("DESKTOP_TEST_SYNC_RECOVERY_RELEASE", "DESKTOP_TEST_SYNC_RECOVERY_STARTED")
	}
	var plan map[string]any
	if method == "sync.recovery.preview" {
		namespace := "sha256:" + strings.Repeat("b", 64)
		configuration := "sha256:" + strings.Repeat("c", 64)
		mappingFingerprint := "sha256:" + strings.Repeat("d", 64)
		target := map[string]any{"local_path": input["local_path"], "project_key": "project-key", "workspace_id": "workspace-1", "sync_session_id": "sync-1", "fingerprint": nil}
		workspace := map[string]any{"id": "workspace-1", "server_url": "https://test.invalid", "project_key": "project-key", "local_path": input["local_path"], "display_name": "Old project", "remote_path": "/workspace/one"}
		session := map[string]any{"id": "sync-1", "server_url": "https://test.invalid", "workspace_id": "workspace-1", "node_id": "node-1", "status": "active", "conflict_status": "none", "mutagen_session_id": "ar-sync-1", "remote_endpoint": "node:/workspace/one"}
		mutagenTarget := map[string]any{"identifier": "sync_exact_old", "name": "ar-sync-1", "local_path": input["local_path"], "remote_path": "/workspace/one", "endpoint": "node:/workspace/one", "configuration_fingerprint": configuration}
		mutagen := map[string]any{"state": "available", "recoverable": true, "error_code": nil, "namespace_fingerprint": namespace, "target": mutagenTarget}
		plan = map[string]any{"context": input["expected_context"], "target": target, "mapping": map[string]any{"workspace": workspace, "sync": session}, "mapping_fingerprint": mappingFingerprint, "server_state": "sync_not_found", "mutagen": mutagen, "recoverable": true, "error_code": nil}
		if mode == "recovery-workspace-missing" {
			plan["server_state"] = "workspace_not_found"
		}
		if mode == "recovery-blocked" {
			plan["recoverable"], plan["error_code"] = false, "sync_mutagen_target_changed"
			mutagen["state"], mutagen["recoverable"], mutagen["error_code"], mutagen["target"] = "blocked", false, "sync_mutagen_target_changed", nil
		}
		if mode == "recovery-absent" {
			mutagen["state"], mutagen["target"] = "session_missing", nil
		}
	} else {
		plan = input["expected_plan"].(map[string]any)
	}
	mutagen := plan["mutagen"].(map[string]any)
	var result map[string]any
	if method == "sync.recovery.preview" {
		result = plan
	} else {
		target := map[string]any{}
		for key, value := range plan["target"].(map[string]any) {
			target[key] = value
		}
		target["sync_session_id"] = nil
		if plan["server_state"] == "workspace_not_found" {
			target["workspace_id"] = nil
		}
		observation := map[string]any{"state": "session_missing", "recoverable": true, "error_code": nil, "namespace_fingerprint": mutagen["namespace_fingerprint"], "target": nil}
		result = map[string]any{"operation_id": id, "context": input["expected_context"], "method": method, "plan": plan, "result_target": target, "outcome": "completed", "stages": []any{map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "mutagen", "state": "completed"}, map[string]any{"name": "persist", "state": "completed"}}, "mutagen": observation, "error_code": nil}
		if mode == "recovery-partial" || mode == "recovery-unknown" {
			result["result_target"] = plan["target"]
			result["outcome"] = strings.TrimPrefix(mode, "recovery-")
			state, code := "failed", "sync_target_changed"
			if mode == "recovery-unknown" {
				state, code = "unknown", "sync_mutagen_timeout"
			}
			result["error_code"] = code
			result["stages"] = []any{map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "mutagen", "state": state}}
			result["mutagen"] = nil
		}
	}
	switch mode {
	case "recovery-bad-scope":
		plan["server_state"] = "permission_denied"
	case "recovery-missing-field":
		delete(plan, "mapping_fingerprint")
	case "recovery-unknown-field":
		plan["secret"] = "do-not-render"
	case "recovery-bad-digest":
		plan["mapping_fingerprint"] = "sha256:bad"
	case "recovery-bad-target":
		plan["target"].(map[string]any)["local_path"] = "/wrong/path"
	case "recovery-bad-context":
		plan["context"].(map[string]any)["server_url"] = "https://wrong.invalid"
	case "recovery-null-namespace":
		mutagen["namespace_fingerprint"] = nil
	case "recovery-contradictory":
		mutagen["recoverable"] = false
	case "recovery-wrong-endpoint":
		mutagen["target"].(map[string]any)["endpoint"] = "other:/workspace/one"
	case "recovery-wrong-identifier":
		mutagen["target"].(map[string]any)["identifier"] = "sync_replacement"
	case "recovery-wrong-result":
		result["result_target"].(map[string]any)["workspace_id"] = nil
	case "recovery-bad-stage":
		result["stages"] = []any{map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "persist", "state": "completed"}}
	case "recovery-wrong-namespace":
		result["mutagen"].(map[string]any)["namespace_fingerprint"] = "sha256:" + strings.Repeat("e", 64)
	case "recovery-no-observation":
		result["mutagen"] = nil
	case "recovery-still-available":
		result["mutagen"] = plan["mutagen"]
	case "recovery-persist-partial", "recovery-persist-unknown":
		result["outcome"], result["error_code"], result["result_target"] = "partial", "state_unavailable", plan["target"]
		result["stages"].([]any)[2].(map[string]any)["state"] = "failed"
		if mode == "recovery-persist-unknown" {
			result["outcome"] = "unknown"
			result["stages"].([]any)[2].(map[string]any)["state"] = "unknown"
		}
	case "recovery-partial-false-observation":
		result["outcome"], result["error_code"], result["result_target"] = "partial", "sync_mutagen_unavailable", plan["target"]
		result["stages"] = []any{map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "mutagen", "state": "failed"}}
	case "recovery-wrong-operation":
		result["operation_id"] = "wrong"
	}
	return result
}
