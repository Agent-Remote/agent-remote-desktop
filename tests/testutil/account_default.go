package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func accountDefaultFixture(method, id string, params json.RawMessage, home, mode string) any {
	if mode == "default-signal-wait" {
		accountDefaultSignalFixture()
	}
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	context := map[string]any{"home": home, "server_url": "https://test.invalid", "active_device_id": "device-1"}
	if mode == "unconfigured" {
		context["server_url"], context["active_device_id"] = nil, nil
	}
	var current any = "previous-account"
	if mode == "default-none" || mode == "unconfigured" {
		current = nil
	}
	var result map[string]any
	if method == "accounts.default.get" {
		result = map[string]any{"context": context, "tool": input["tool"], "account_id": current}
	} else {
		if input["confirmed"] != true {
			os.Exit(4)
		}
		if path := os.Getenv("DESKTOP_TEST_DEFAULT_RECEIPT"); path != "" {
			if os.WriteFile(path, params, 0600) != nil {
				os.Exit(5)
			}
		}
		waitControlBarrier("DESKTOP_TEST_DEFAULT_RELEASE", "DESKTOP_TEST_DEFAULT_STARTED")
		if mode == "default-slow" {
			time.Sleep(time.Minute)
		}
		if mode == "default-stale" || mode == "default-context-changed" || mode == "default-denied" {
			code := "default_changed"
			if mode == "default-context-changed" {
				code = "configuration_changed"
			}
			if mode == "default-denied" {
				code = "permission_denied"
			}
			json.NewEncoder(os.Stdout).Encode(map[string]any{"protocol_version": 1, "request_id": id, "type": "error", "phase": "failed", "error": map[string]any{"code": code, "retryable": false}})
			os.Exit(1)
		}
		next := input["account_id"]
		if method == "accounts.default.clear" {
			next = nil
		}
		result = map[string]any{"operation_id": id, "context": input["expected_context"], "tool": input["tool"], "previous_account_id": input["expected_account_id"], "account_id": next, "changed": input["expected_account_id"] != next}
	}
	switch mode {
	case "default-missing":
		delete(result, "account_id")
	case "default-null-tool":
		result["tool"] = nil
	case "default-wrong-tool":
		result["tool"] = "unrelated"
	case "default-unknown-field":
		result["future"] = true
	case "default-wrong-home":
		result["context"].(map[string]any)["home"] = "/wrong"
	case "default-missing-context-field":
		delete(result["context"].(map[string]any), "server_url")
	case "default-null-context":
		result["context"] = nil
	case "default-missing-changed":
		if method != "accounts.default.get" {
			delete(result, "changed")
		}
	case "default-false-changed":
		if method != "accounts.default.get" {
			result["changed"] = false
		}
	case "default-wrong-operation":
		if method != "accounts.default.get" {
			result["operation_id"] = "wrong"
		}
	case "default-wrong-previous":
		if method != "accounts.default.get" {
			result["previous_account_id"] = "other"
		}
	case "default-wrong-result":
		if method != "accounts.default.get" {
			result["account_id"] = "other"
		}
	}
	if mode == "default-duplicate" {
		raw, _ := json.Marshal(result)
		output := strings.TrimSuffix(string(raw), "}") + `,"tool":"claude"}`
		fmt.Fprintf(os.Stdout, `{"protocol_version":1,"request_id":%q,"type":"result","phase":"completed","result":%s}`+"\n", id, output)
		os.Exit(0)
	}
	return result
}
