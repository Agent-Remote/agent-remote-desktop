package testutil

import (
	"encoding/json"
	"os"
	"strings"
)

const RuntimeAccountID = "11111111-1111-4111-8111-111111111111"
const RuntimeActorID = "22222222-2222-4222-8222-222222222222"
const RuntimeOwnerID = "33333333-3333-4333-8333-333333333333"
const RuntimeOriginalTask = "migrate_tool_account_runtime:" + RuntimeAccountID + ":44444444-4444-4444-8444-444444444444"
const RuntimeRequestID = "55555555-5555-4555-8555-555555555555"

func RuntimeContext(home string) map[string]any {
	return map[string]any{"home": home, "server_url": "https://test.invalid", "actor_user_id": RuntimeActorID}
}
func RuntimeOriginal() map[string]any {
	return map[string]any{"task_id": RuntimeOriginalTask, "task_record_id": "66666666-6666-4666-8666-666666666666", "user_id": RuntimeOwnerID, "tool_account_id": RuntimeAccountID, "node_id": "77777777-7777-4777-8777-777777777777", "tool_type": "claude", "source_runtime_backend": "native", "target_runtime_backend": "docker_sandbox"}
}
func RuntimeBinding(key, action string) map[string]any {
	o := RuntimeOriginal()
	b := map[string]any{"version": 1, "task_id": "recover_tool_account_runtime:" + RuntimeAccountID + ":" + key, "task_record_id": "88888888-8888-4888-8888-888888888888", "original_task_id": o["task_id"], "original_task_record_id": o["task_record_id"], "user_id": o["user_id"], "tool_account_id": o["tool_account_id"], "node_id": o["node_id"], "tool_type": o["tool_type"], "source_runtime_backend": o["source_runtime_backend"], "target_runtime_backend": o["target_runtime_backend"]}
	if action != "verify_target" {
		b["action"] = action
		b["version"] = 2
		if action == "repair_source" {
			b["version"] = 3
		}
	}
	return b
}
func runtimeRecoveryFixture(method, id string, params json.RawMessage, home, mode string) any {
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if mode == "runtime-denied" && method != "accounts.runtime_recovery.authority" && method != "accounts.runtime_recovery.pending" {
		importFailure(id, "permission_denied")
	}
	if mode == "runtime-expired" && method == "accounts.runtime_recovery.authority" {
		importFailure(id, "authentication_required")
	}
	if mode == "runtime-error-raw" {
		importFailure(id, "private diagnostic must not be displayed")
	}
	journal := os.Getenv("DESKTOP_TEST_RUNTIME_JOURNAL")
	read := func() map[string]any {
		var ref map[string]any
		if raw, err := os.ReadFile(journal); err == nil {
			_ = json.Unmarshal(raw, &ref)
		}
		return ref
	}
	write := func(ref map[string]any) {
		if journal != "" {
			raw, _ := json.Marshal(ref)
			_ = os.WriteFile(journal, raw, 0600)
		}
	}
	switch method {
	case "accounts.runtime_recovery.authority":
		return map[string]any{"context": RuntimeContext(home), "admin": mode != "runtime-denied"}
	case "accounts.runtime_recovery.inspect":
		plan := map[string]any{"context": input["expected_context"], "account_id": input["account_id"], "original": RuntimeOriginal(), "action": input["action"], "original_status": "failed", "account_status": "migrating", "digest": mutagenDigest("a")}
		if mode == "runtime-original-extra" {
			plan["original"].(map[string]any)["payload"] = "private"
		}
		if mode == "runtime-original-null" {
			plan["original"].(map[string]any)["user_id"] = nil
		}
		if mode == "runtime-eligible" {
			plan["eligible"] = true
		}
		return plan
	case "accounts.runtime_recovery.pending":
		items := []any{}
		if ref := read(); ref != nil {
			items = append(items, ref)
		}
		result := map[string]any{"home": home, "items": items}
		if mode == "runtime-pending-wrong-home" {
			result["home"] = home + "/other"
		}
		return result
	case "accounts.runtime_recovery.forget":
		if mode == "runtime-not-terminal" {
			importFailure(id, "runtime_recovery_not_terminal")
		}
		_ = os.Remove(journal)
		return map[string]any{"operation_id": id, "context": input["expected_context"], "account_id": input["account_id"], "request_id": input["request_id"], "digest": input["expected_digest"], "forgotten": true}
	}
	key := input["request_id"].(string)
	action := "verify_target"
	digest := input["expected_digest"]
	if method == "accounts.runtime_recovery.apply" {
		action = input["action"].(string)
		if mode == "runtime-pre-submit" {
			importFailure(id, "configuration_changed")
		}
		ref := map[string]any{"context": input["expected_context"], "account_id": input["account_id"], "request_id": key, "digest": digest, "original": input["original"], "action": action, "binding": nil, "phase": "unknown", "rejection_code": nil}
		write(ref)
		waitControlBarrier("DESKTOP_TEST_RUNTIME_RELEASE", "DESKTOP_TEST_RUNTIME_STARTED")
		if mode == "runtime-unknown" {
			importFailure(id, "runtime_recovery_outcome_unknown")
		}
		if mode == "runtime-rejected" {
			ref["phase"], ref["rejection_code"] = "rejected", "runtime_recovery_rejected"
			write(ref)
			importFailure(id, "runtime_recovery_rejected")
		}
		ref["phase"], ref["binding"] = "accepted", RuntimeBinding(key, action)
		write(ref)
	} else {
		if mode == "runtime-status-unknown" {
			importFailure(id, "runtime_recovery_status_unknown")
		}
		if ref := read(); ref != nil {
			action = ref["action"].(string)
		} else if strings.HasPrefix(mode, "runtime-manual-") {
			action = strings.TrimPrefix(mode, "runtime-manual-")
		}
	}
	binding := RuntimeBinding(key, action)
	status := "succeeded"
	if method == "accounts.runtime_recovery.apply" {
		status = "pending"
	}
	result := map[string]any{"context": input["expected_context"], "account_id": input["account_id"], "request_id": key, "digest": digest, "recovery": map[string]any{"binding": binding, "status": status}, "target_completion_confirmed": status == "succeeded" && action == "verify_target", "source_restoration_confirmed": status == "succeeded" && action != "verify_target"}
	if method == "accounts.runtime_recovery.apply" {
		result["operation_id"] = id
	}
	switch mode {
	case "runtime-v1-null-action":
		binding["action"] = nil
	case "runtime-wrong-owner":
		binding["user_id"] = RuntimeActorID
	case "runtime-wrong-key":
		binding["task_id"] = "recover_tool_account_runtime:" + RuntimeAccountID + ":" + RuntimeActorID
	case "runtime-both-confirmed":
		result["target_completion_confirmed"], result["source_restoration_confirmed"] = true, true
	case "runtime-no-confirmation":
		delete(result, "target_completion_confirmed")
	case "runtime-wrong-digest":
		result["digest"] = mutagenDigest("f")
	case "runtime-wrong-opid":
		result["operation_id"] = "other"
	case "runtime-wrong-version":
		binding["version"] = 3
	}
	return result
}
