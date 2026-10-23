package testutil

import (
	"encoding/json"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

const remoteDeleteRequestID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

func remoteDeleteFixture(raw json.RawMessage, home, mode string, query bool) map[string]any {
	var input struct {
		RequestID       string                        `json:"request_id"`
		ToolType        string                        `json:"tool_type"`
		SessionIDs      []string                      `json:"session_ids"`
		Selection       remote.SessionDeleteSelection `json:"selection"`
		SelectionDigest string                        `json:"selection_digest"`
		Intent          string                        `json:"intent_fingerprint"`
		Expected        map[string]any                `json:"expected_context"`
	}
	_ = json.Unmarshal(raw, &input)
	if query {
		input.RequestID = remoteDeleteRequestID
	}
	if input.ToolType == "" {
		input.ToolType = "claude"
	}
	if len(input.SessionIDs) == 0 {
		input.SessionIDs = []string{"11111111-1111-4111-8111-111111111111"}
	}
	if len(input.Selection.Statuses) == 0 {
		input.Selection = remote.SessionDeleteSelection{ToolType: input.ToolType, Statuses: []string{"failed", "interrupted", "stopped"}}
	}
	if input.SelectionDigest == "" {
		input.SelectionDigest = remote.SessionDeleteSelectionDigest(input.ToolType, input.SessionIDs, input.Selection)
	}
	if input.Intent == "" {
		input.Intent = remote.SessionDeleteIntentFingerprint(input.ToolType, input.SessionIDs, input.Selection)
	}
	server, device := "https://test.invalid", "device-1"
	if input.Expected != nil {
		if value, ok := input.Expected["server_url"].(string); ok {
			server = value
		}
		if value, ok := input.Expected["active_device_id"].(string); ok {
			device = value
		}
	}
	phase, status, deleted := "deleting", "accepted", false
	if query || mode == "remote-delete-settled" {
		phase, status, deleted = "deleted", "succeeded", true
	}
	if mode == "remote-delete-blocked" {
		phase, status = "blocked", "failed"
	}
	if mode == "remote-delete-unknown" && !query {
		phase, status = "unknown", "unknown"
	}
	contexts := make([]any, 0, len(input.SessionIDs))
	statuses := map[string]any{}
	for index, id := range input.SessionIDs {
		workspace := "66666666-6666-4666-8666-666666666666"
		if index > 0 {
			workspace = "77777777-7777-4777-8777-777777777777"
		}
		contexts = append(contexts, map[string]any{"session_id": id, "tool_account_id": "44444444-4444-4444-8444-444444444444", "workspace_id": workspace, "project_key": "project-" + id[:4], "node_id": "55555555-5555-4555-8555-555555555555", "runtime_backend": "native", "runtime_resource_id": nil})
		statuses[id] = map[bool]string{true: "deleted", false: "stopped"}[deleted]
	}
	deletedIDs := []string{}
	if deleted {
		deletedIDs = append(deletedIDs, input.SessionIDs...)
	}
	return map[string]any{
		"schema_version": 1, "request_id": remoteDeleteRequestID, "phase": phase,
		"context":   map[string]any{"home": home, "server_url": server, "active_device_id": device},
		"tool_type": input.ToolType, "session_ids": input.SessionIDs, "selection": input.Selection,
		"selection_digest": input.SelectionDigest, "intent_fingerprint": input.Intent, "reason": "user_requested",
		"delete_task_id": "delete_tool_sessions", "delete_task_record_id": "88888888-8888-4888-8888-888888888888", "task_status": status,
		"deleted_session_ids": deletedIDs, "session_statuses": statuses, "session_contexts": contexts,
		"error_code": map[bool]any{true: nil, false: nil}[deleted], "result_status": status, "session_deleted": deleted,
		"collection_mismatch": false, "retention_blockers": []string{},
	}
}
