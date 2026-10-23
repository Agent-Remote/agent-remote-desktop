package testutil

import (
	"encoding/json"
	"os"
	"strings"
)

func syncFixture(method, id string, params json.RawMessage, mode string) any {
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if path := os.Getenv("DESKTOP_TEST_SYNC_REQUEST"); path != "" {
		if os.WriteFile(path, params, 0600) != nil {
			os.Exit(4)
		}
	}
	waitControlBarrier("DESKTOP_TEST_SYNC_RELEASE", "DESKTOP_TEST_SYNC_STARTED")
	var target map[string]any
	if method == "sync.status" {
		target = map[string]any{"local_path": input["local_path"], "project_key": "project-key", "workspace_id": "workspace-1", "sync_session_id": "sync-1", "fingerprint": "sha256:" + strings.Repeat("a", 64)}
	} else {
		target = input["expected_target"].(map[string]any)
	}
	detail := map[string]any{"id": "sync-1", "workspace_id": "workspace-1", "node_id": "node-1", "local_path": target["local_path"], "remote_path": "/workspace/one", "status": "active", "conflict_status": "none", "mutagen_session_id": "ar-sync-1", "remote_endpoint": "node:/workspace/one", "sync_mode": "two_way", "sync_git": true, "exclude": []string{"node_modules", ".DS_Store"}}
	mutagen := map[string]any{"state": "available", "conflicts": "none_observed", "paused": false, "status": "watching", "conflict_count": 0}
	result := map[string]any{"context": input["expected_context"], "target": target, "sync": detail, "mutagen": mutagen}
	if method != "sync.status" {
		result["operation_id"], result["method"], result["dry_run"] = id, method, input["dry_run"]
		result["result_target"], result["outcome"], result["error_code"] = target, "completed", nil
		localState := "completed"
		if input["dry_run"] == true {
			localState = "skipped"
		}
		result["stages"] = []any{map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "mutagen", "state": localState}, map[string]any{"name": "persist", "state": "completed"}}
	}
	if method == "sync.ensure" {
		if target["sync_session_id"] == nil {
			created := map[string]any{}
			for key, value := range target {
				created[key] = value
			}
			created["workspace_id"], created["sync_session_id"], created["fingerprint"] = "workspace-1", "sync-1", "sha256:"+strings.Repeat("a", 64)
			result["result_target"] = created
		}
		result["stages"] = append([]any{map[string]any{"name": "workspace", "state": "completed"}}, result["stages"].([]any)...)
	}
	switch mode {
	case "sync-missing":
		delete(result, "mutagen")
	case "sync-null":
		result["target"] = nil
	case "sync-unknown-field":
		detail["secret"] = "private-never-render"
	case "sync-wrong-path":
		target["local_path"] = "/wrong/path"
	case "sync-wrong-context":
		input["expected_context"].(map[string]any)["server_url"] = "https://wrong.invalid"
	case "sync-bad-fingerprint":
		target["fingerprint"] = "sha256:bad"
	case "sync-contradictory-state":
		mutagen["state"], mutagen["conflicts"] = "unavailable", "none_observed"
	case "sync-unknown-zero":
		mutagen["conflicts"] = "unknown"
	case "sync-paused-zero":
		mutagen["paused"] = true
		mutagen["conflicts"] = "unknown"
	case "sync-high-conflicts":
		mutagen["conflicts"] = "detected"
		mutagen["conflict_count"] = uint64(18446744073709551615)
	case "sync-conflict":
		detail["status"] = "conflicted"
		mutagen["conflicts"] = "detected"
		mutagen["conflict_count"] = 1
		detail["conflict_status"] = "has_conflicts"
	case "sync-no-session":
		mutagen["paused"], mutagen["status"], mutagen["conflict_count"] = nil, nil, nil
		result["sync"] = nil
		target["sync_session_id"], target["fingerprint"], target["workspace_id"] = nil, nil, nil
		mutagen["state"], mutagen["conflicts"] = "unavailable", "unknown"
	case "sync-partial":
		result["outcome"], result["error_code"] = "partial", "sync_local_failed"
		result["stages"] = []any{map[string]any{"name": "server", "state": "completed"}, map[string]any{"name": "mutagen", "state": "failed"}}
	case "sync-unknown":
		result["outcome"], result["error_code"] = "unknown", "sync_outcome_unknown"
		result["stages"] = []any{map[string]any{"name": "server", "state": "unknown"}}
	case "sync-wrong-method":
		result["method"] = "sync.future"
	case "sync-wrong-id":
		result["operation_id"] = "wrong"
	case "sync-wrong-dry-run":
		result["dry_run"] = true
	case "sync-empty-stages":
		result["stages"] = nil
	case "sync-bad-stage":
		result["stages"] = []any{map[string]any{"name": "future", "state": "completed"}}
	case "sync-completed-failed":
		result["stages"] = []any{map[string]any{"name": "server", "state": "failed"}}
	}
	return result
}
