package testutil

import (
	"encoding/json"
	"os"
)

const remoteStopRequestID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"

func remoteStopFixture(raw json.RawMessage, home, mode string, query bool) map[string]any {
	var input struct {
		RequestID       string         `json:"request_id"`
		SessionID       string         `json:"session_id"`
		ExpectedNodeID  string         `json:"expected_node_id"`
		ExpectedBackend string         `json:"expected_runtime_backend"`
		Expected        map[string]any `json:"expected_context"`
	}
	_ = json.Unmarshal(raw, &input)
	if query {
		input.RequestID = remoteStopRequestID
	}
	if input.SessionID == "" {
		input.SessionID = "11111111-1111-4111-8111-111111111111"
	}
	if input.ExpectedNodeID == "" {
		input.ExpectedNodeID = "55555555-5555-4555-8555-555555555555"
	}
	if input.ExpectedBackend == "" {
		input.ExpectedBackend = "native"
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
	if value := os.Getenv("DESKTOP_TEST_REMOTE_DEVICE"); value != "" {
		device = value
	}
	phase, process, save := "stopping", false, "pending"
	resultStatus := "accepted"
	errorCode := any(nil)
	if query || mode == "remote-stop-settled" {
		phase, process, save, resultStatus = "settled", true, "saved", "succeeded"
	}
	if mode == "remote-stop-failed" {
		phase, process, save, resultStatus = "unresolved", false, "failed", "failed"
		code := "SESSION_STOP_UNRESOLVED"
		errorCode = code
	}
	if mode == "remote-stop-unknown" && !query {
		return map[string]any{
			"schema_version": 1, "request_id": remoteStopRequestID, "phase": "unknown",
			"session_id": input.SessionID, "node_id": input.ExpectedNodeID, "runtime_backend": input.ExpectedBackend,
			"session_deleted": false, "error_code": "SESSION_STOP_SUBMISSION_UNKNOWN",
			"context": map[string]any{"home": home, "server_url": server, "active_device_id": device},
		}
	}
	return map[string]any{
		"schema_version": 1, "request_id": remoteStopRequestID, "phase": phase,
		"session_id": input.SessionID, "node_id": input.ExpectedNodeID, "runtime_backend": input.ExpectedBackend,
		"runtime_resource_id": "runtime-1", "stop_task_id": "stop_tool_session:" + input.SessionID,
		"stop_task_record_id": "66666666-6666-4666-8666-666666666666", "stop_task_status": resultStatus,
		"session_status":                  map[string]any{"stopping": "stopping", "settled": "stopped", "unresolved": "stopping"}[phase],
		"skill_finalization_operation_id": "77777777-7777-4777-8777-777777777777", "session_deleted": false,
		"observed_at": "2026-10-19T00:00:00Z", "error_code": errorCode,
		"intent_fingerprint": "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"reason":             "user_requested", "result_status": resultStatus, "process_stopped": process,
		"save_status": save, "context": map[string]any{"home": home, "server_url": server, "active_device_id": device},
	}
}
