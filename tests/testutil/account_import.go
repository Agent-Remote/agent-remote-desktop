package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const ImportRequestID = "22222222-2222-4222-8222-222222222222"

func importContext(home string) map[string]any {
	return map[string]any{"home": home, "server_url": "https://test.invalid", "active_device_id": "device-1"}
}
func importDiscovery(home string, selection any) map[string]any {
	exclude := []string{"~/.claude.json", "~/.claude/cache", "~/.claude/logs", "~/.claude/transcripts"}
	include := []string{"~/.claude/rules", "~/.claude/settings.json"}
	if values, ok := selection.(map[string]any); ok {
		if values["exclude_skills"] == true {
			exclude = append(exclude, "~/.claude/skills")
		}
		if values["include_resume_history"] == true {
			include = append(include, "~/.claude/projects")
		}
	}
	return map[string]any{"source_home": filepath.Join(home, "source"), "selection": selection, "include": include, "exclude": exclude}
}

func importPlan(home, mode string, input map[string]any) map[string]any {
	plan := importDiscovery(home, input["selection"])
	plan["context"], plan["account_id"] = input["expected_context"], input["account_id"]
	count := 2
	if mode == "import-paged" {
		count = 10
	}
	if mode == "import-large" {
		count = 500
	}
	if mode == "import-empty" {
		count = 0
	}
	files := []any{}
	for i := 0; i < count; i++ {
		files = append(files, map[string]any{"path": fmt.Sprintf("~/.claude/rules/rule-%04d.md", i), "size": 16, "sha256": mutagenDigest("a"), "mode": 384})
	}
	plan["manifest"], plan["total_bytes"] = files, count*16
	plan["digest"], plan["source_fingerprint"], plan["intent_fingerprint"] = mutagenDigest("b"), mutagenDigest("c"), mutagenDigest("d")
	switch mode {
	case "import-missing":
		delete(plan, "total_bytes")
	case "import-raw":
		plan["content"] = "private contents"
	case "import-bad-size":
		plan["total_bytes"] = 999
	case "import-duplicate":
		files[1] = files[0]
	case "import-path-escape":
		files[0].(map[string]any)["path"] = "~/.claude/../private"
	case "import-bad-mode":
		files[0].(map[string]any)["mode"] = 511
	case "import-bad-hash":
		files[0].(map[string]any)["sha256"] = "bad"
	case "import-null-selection":
		plan["selection"] = nil
	case "import-wrong-account":
		plan["account_id"] = "other-account"
	}
	return plan
}
func importReference(home, key string) map[string]any {
	return map[string]any{"context": importContext(home), "account_id": "account-1", "request_id": key, "digest": mutagenDigest("b"), "intent_fingerprint": mutagenDigest("d"), "task_id": nil, "phase": "pending"}
}
func importFailure(id, code string) {
	json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": id, "type": "error", "phase": "failed", "error": map[string]any{"code": code, "retryable": false}})
	os.Exit(1)
}
func accountImportFixture(method, id string, params json.RawMessage, home, mode string) any {
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if capture := os.Getenv("DESKTOP_TEST_IMPORT_REQUEST"); capture != "" {
		_ = os.WriteFile(capture, params, 0600)
	}
	selection := input["selection"]
	switch method {
	case "accounts.import.discover":
		return importDiscovery(home, selection)
	case "accounts.import.prepare":
		return importPlan(home, mode, input)
	case "accounts.import.validate":
		value := importDiscovery(home, selection)
		value["operation_id"], value["context"], value["account_id"], value["audit_written"] = id, input["expected_context"], input["account_id"], true
		value["accepted"], value["rejected"], value["warning_codes"] = value["include"], []string{}, []string{}
		if mode == "import-reject-validation" {
			value["accepted"], value["rejected"] = []string{"~/.claude/settings.json"}, []string{"~/.claude/rules"}
		}
		if mode == "import-bad-warning" {
			value["warning_codes"] = []string{"raw private server error"}
		}
		return value
	case "accounts.import.pending":
		if os.Getenv("DESKTOP_TEST_IMPORT_PENDING_ERROR") != "" {
			importFailure(id, "account_import_local_unavailable")
		}
		refs := []any{}
		if path := os.Getenv("DESKTOP_TEST_IMPORT_JOURNAL"); path != "" {
			if data, err := os.ReadFile(path); err == nil {
				var ref map[string]any
				_ = json.Unmarshal(data, &ref)
				refs = append(refs, ref)
			}
		} else if mode == "import-saved" {
			refs = append(refs, importReference(home, ImportRequestID))
		}
		return map[string]any{"home": home, "items": refs}
	case "accounts.import.forget":
		if mode == "import-not-settled" {
			importFailure(id, "account_import_not_settled")
		}
		if path := os.Getenv("DESKTOP_TEST_IMPORT_JOURNAL"); path != "" {
			_ = os.Remove(path)
		}
		return map[string]any{"operation_id": id, "context": input["expected_context"], "account_id": input["account_id"], "request_id": input["request_id"], "digest": input["expected_digest"], "task_id": input["expected_task_id"], "forgotten": true}
	}
	ref := importReference(home, input["request_id"].(string))
	ref["context"], ref["account_id"] = input["expected_context"], input["account_id"]
	if method == "accounts.import.apply" {
		if mode == "import-changed" {
			importFailure(id, "account_import_source_changed")
		}
		if code := os.Getenv("DESKTOP_TEST_IMPORT_APPLY_ERROR"); code != "" {
			importFailure(id, code)
		}
		if mode == "import-skills-managed" {
			ref["phase"] = "rejected"
		}
		if path := os.Getenv("DESKTOP_TEST_IMPORT_JOURNAL"); path != "" {
			data, _ := json.Marshal(ref)
			_ = os.WriteFile(path, data, 0600)
		}
		waitControlBarrier("DESKTOP_TEST_IMPORT_RELEASE", "DESKTOP_TEST_IMPORT_STARTED")
		if mode == "import-skills-managed" {
			importFailure(id, "account_import_skills_managed")
		}
		if mode == "import-unknown" {
			importFailure(id, "account_import_outcome_unknown")
		}
		if mode == "import-changed-retained" {
			importFailure(id, "account_import_source_changed")
		}
	}
	if method == "accounts.import.status" && mode == "import-status-unknown" {
		importFailure(id, "account_import_status_unknown")
	}
	ref["task_id"], ref["accepted"], ref["file_count"], ref["warning_codes"] = "import_tool_account_config:account-1:33333333-3333-4333-8333-333333333333", true, 2, []string{}
	if mode == "import-paged" {
		ref["file_count"] = 10
	}
	if mode == "import-large" {
		ref["file_count"] = 500
	}
	if method == "accounts.import.apply" {
		ref["operation_id"], ref["phase"] = id, "accepted"
	} else {
		ref["phase"] = "succeeded"
	}
	switch mode {
	case "import-receipt-wrong-key":
		ref["request_id"] = ImportRequestID
	case "import-receipt-wrong-digest":
		ref["digest"] = mutagenDigest("f")
	case "import-receipt-wrong-intent":
		ref["intent_fingerprint"] = mutagenDigest("f")
	case "import-receipt-wrong-count":
		ref["file_count"] = 99
	case "import-receipt-no-task":
		ref["task_id"] = nil
	case "import-receipt-bad-phase":
		ref["phase"] = "made_up"
	case "import-receipt-bad-opid":
		ref["operation_id"] = "other"
	case "import-bad-warning":
		ref["warning_codes"] = []string{"private diagnostic"}
	}
	return ref
}
