package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func accountsFixture(method, id string, params json.RawMessage, home, mode string) any {
	if mode == "account-signal-wait" {
		accountDefaultSignalFixture()
	}
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if path := os.Getenv("DESKTOP_TEST_ACCOUNT_RECEIPT"); path != "" {
		if os.WriteFile(path, params, 0600) != nil {
			os.Exit(5)
		}
	}
	waitControlBarrier("DESKTOP_TEST_ACCOUNT_RELEASE", "DESKTOP_TEST_ACCOUNT_STARTED")
	if mode == "account-slow" {
		time.Sleep(time.Minute)
	}
	accountID := "account-1"
	if method != "accounts.create" {
		accountID, _ = input["account_id"].(string)
	}
	context := input["expected_context"]
	account := map[string]any{"id": accountID, "tool_type": "claude", "display_name": "Studio", "status": "active", "region_code": "JP", "timezone": "Asia/Tokyo", "locale": "en_US.UTF-8", "preferred_node_tags": []string{"gpu", "east"}, "affinity_node_id": "node-1", "runtime_backend": "native", "created_at": "2026-10-18T00:00:00Z", "updated_at": "2026-10-18T01:00:00Z"}
	binding := map[string]any{"tool_account_id": accountID, "status": "binding_waiting_user_login", "node_id": "node-1", "binding_session_id": "bind-session-1", "tmux_session_name": "bind_account-1", "account_remote_path": "/accounts/account-1", "task_id": "account:verify:task-1", "error_code": nil}
	if strings.HasPrefix(mode, "account-terminal-") {
		binding["binding_session_id"] = "bind-111111111111-aabbccddeeff"
	}
	if mode == "account-terminal-expired" {
		binding["status"] = "expired"
	}
	if mode == "account-terminal-disabled" {
		account["status"], binding["status"] = "disabled", "disabled"
	}
	var result map[string]any
	if method == "accounts.status" {
		result = map[string]any{"context": context, "account": account, "binding": binding}
	} else {
		if input["confirmed"] != true {
			os.Exit(4)
		}
		result = map[string]any{"operation_id": id, "context": context, "accepted": true, "account": nil, "binding": nil}
		switch method {
		case "accounts.create":
			for source, target := range map[string]string{"tool": "tool_type", "name": "display_name", "region": "region_code", "timezone": "timezone", "locale": "locale", "tags": "preferred_node_tags"} {
				account[target] = input[source]
			}
			account["status"] = "binding_requested"
			result["account"] = account
		case "accounts.disable":
			account["status"] = "disabled"
			result["account"] = account
		case "accounts.bind":
			binding["status"] = "binding_session_starting"
			result["binding"] = binding
		case "accounts.verify":
			binding["status"] = "binding_verifying"
			result["binding"] = binding
		}
	}
	switch mode {
	case "account-missing":
		delete(result, "binding")
	case "account-bad-context":
		context.(map[string]any)["server_url"] = "https://wrong.invalid"
	case "account-null-field":
		account["preferred_node_tags"] = nil
		binding["status"] = nil
	case "account-raw-command":
		binding["connect_command"] = "private-command-must-not-render"
		account["verifier"] = "private-verifier"
	case "account-unknown-state":
		account["status"] = "future"
		binding["status"] = "future"
	case "account-wrong-id":
		account["id"] = "wrong-id"
		binding["tool_account_id"] = "wrong-id"
	case "account-no-task":
		binding["task_id"] = nil
	case "account-no-accepted":
		delete(result, "accepted")
	case "account-false-accepted":
		result["accepted"] = false
	case "account-wrong-operation":
		result["operation_id"] = "wrong"
	case "account-wrong-create-fields":
		account["timezone"] = "Etc/UTC"
	case "account-mixed":
		result["account"], result["binding"] = account, binding
	case "account-disabled":
		if method == "accounts.status" {
			account["status"], binding["status"] = "disabled", "disabled"
		}
	case "account-binding-error":
		binding["error_code"] = "binding_failed"
	case "account-missing-context-field":
		delete(context.(map[string]any), "active_device_id")
	case "account-null-context":
		result["context"] = nil
	}
	if (mode == "account-rejected" || mode == "account-unknown" || mode == "account-context-changed" || mode == "account-denied" || mode == "account-not-found") && method != "accounts.status" || mode == "account-status-invalid" && method == "accounts.status" {
		code := map[string]string{"account-rejected": "account_rejected", "account-unknown": "account_outcome_unknown", "account-context-changed": "configuration_changed", "account-denied": "permission_denied", "account-not-found": "account_not_found", "account-status-invalid": "account_response_invalid"}[mode]
		json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": id, "type": "error", "phase": "failed", "error": map[string]any{"code": code, "retryable": false}})
		os.Exit(1)
	}
	if mode == "account-duplicate" {
		raw, _ := json.Marshal(result)
		output := strings.TrimSuffix(string(raw), "}") + `,"context":` + string(mustFixtureJSON(context)) + `}`
		fmt.Fprintf(os.Stdout, `{"protocol_version":1,"request_id":%q,"type":"result","phase":"completed","result":%s}`+"\n", id, output)
		os.Exit(0)
	}
	return result
}
func mustFixtureJSON(value any) []byte {
	raw, err := json.Marshal(value)
	if err != nil {
		os.Exit(6)
	}
	return raw
}
