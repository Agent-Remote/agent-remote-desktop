package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func mutagenDigest(char string) string { return "sha256:" + strings.Repeat(char, 64) }
func mutagenEnvironmentPlan(home, mode string) map[string]any {
	binding := map[string]any{"home": home, "ssh_directory": filepath.Join(home, "ssh"), "fingerprint": mutagenDigest("a")}
	previous := map[string]any{"home": filepath.Join(home, "previous"), "ssh_directory": filepath.Join(home, "previous", "ssh"), "fingerprint": mutagenDigest("b")}
	tools := []any{}
	for _, name := range []string{"mutagen", "ssh", "scp"} {
		tools = append(tools, map[string]any{"name": name, "path": filepath.Join(home, "bin", name), "fingerprint": mutagenDigest("c")})
	}
	sessions := []any{}
	count := 2
	if mode == "environment-many" {
		count = 160
	}
	if mode == "environment-paged" {
		count = 10
	}
	if mode == "environment-new" {
		count = 0
	}
	for i := 0; i < count; i++ {
		kind, identifier, name, path, remotePath := "sync", fmt.Sprintf("sync_project%d", i), fmt.Sprintf("project-%d", i), filepath.Join(home, fmt.Sprintf("project-%d", i)), fmt.Sprintf("/workspace/%d", i)
		if i%2 == 1 {
			kind, identifier, name, path, remotePath = "forward", fmt.Sprintf("fwrd_forward%d", i), fmt.Sprintf("forward-%d", i), "tcp:127.0.0.1:5000", "tcp:127.0.0.1:8000"
		}
		source := map[string]any{"protocol": "local", "user": "", "host": "", "port": 0, "path": path}
		destination := map[string]any{"protocol": "ssh", "user": "agent", "host": "fixture.invalid", "port": 2222, "path": remotePath}
		sessions = append(sessions, map[string]any{"kind": kind, "identifier": identifier, "name": name, "paused": i%2 == 1, "source": source, "destination": destination, "configuration_fingerprint": mutagenDigest("d")})
	}
	plan := map[string]any{"home": home, "data_directory": filepath.Join(home, "mutagen-data"), "namespace_fingerprint": mutagenDigest("e"), "daemon_state": "running", "running_fingerprint": mutagenDigest("f"), "registration": "unmanaged", "previous": previous, "desired": binding, "tools": tools, "sessions": sessions, "inventory_fingerprint": mutagenDigest("1"), "marker_fingerprint": mutagenDigest("2"), "fingerprint": mutagenDigest("3"), "repairable": true, "error_code": nil}
	if mode == "environment-stopped" || mode == "environment-new" {
		plan["daemon_state"], plan["running_fingerprint"] = "stopped", nil
	}
	if mode == "environment-new" {
		plan["namespace_fingerprint"], plan["previous"], plan["marker_fingerprint"] = nil, nil, nil
	}
	if mode == "environment-external" || mode == "environment-blocked" {
		code := "sync_mutagen_inventory_invalid"
		if mode == "environment-external" {
			plan["registration"] = "external"
			code = "sync_mutagen_registration_external"
		}
		plan["repairable"], plan["error_code"] = false, code
	}
	return plan
}
func mutagenEnvironmentFixture(method, id string, params json.RawMessage, home, mode string) any {
	var input map[string]any
	if json.Unmarshal(params, &input) != nil {
		os.Exit(4)
	}
	if method == "mutagen.environment.apply" {
		if path := os.Getenv("DESKTOP_TEST_ENVIRONMENT_REQUEST"); path != "" {
			if os.WriteFile(path, params, 0600) != nil {
				os.Exit(4)
			}
		}
		waitControlBarrier("DESKTOP_TEST_ENVIRONMENT_RELEASE", "DESKTOP_TEST_ENVIRONMENT_STARTED")
	}
	plan := mutagenEnvironmentPlan(home, mode)
	result := plan
	if method == "mutagen.environment.apply" {
		encoded, _ := json.Marshal(plan)
		var post map[string]any
		if json.Unmarshal(encoded, &post) != nil {
			os.Exit(4)
		}
		post["daemon_state"], post["running_fingerprint"], post["namespace_fingerprint"] = "running", mutagenDigest("4"), mutagenDigest("e")
		post["previous"], post["marker_fingerprint"], post["fingerprint"] = post["desired"], mutagenDigest("5"), mutagenDigest("6")
		stop := "completed"
		if plan["daemon_state"] == "stopped" {
			stop = "skipped"
		}
		result = map[string]any{"operation_id": id, "home": home, "method": method, "plan": plan, "outcome": "completed", "stages": []any{map[string]any{"name": "stop", "state": stop}, map[string]any{"name": "start", "state": "completed"}, map[string]any{"name": "verify", "state": "completed"}, map[string]any{"name": "persist", "state": "completed"}}, "environment": post, "error_code": nil}
		if mode == "environment-partial" || mode == "environment-unknown" || mode == "environment-persist-unknown" {
			outcome, state, code := "partial", "failed", "sync_mutagen_unavailable"
			if mode != "environment-partial" {
				outcome, state, code = "unknown", "unknown", "sync_mutagen_timeout"
			}
			result["outcome"], result["error_code"], result["environment"] = outcome, code, nil
			if mode == "environment-persist-unknown" {
				result["stages"].([]any)[3].(map[string]any)["state"] = state
			} else {
				result["stages"] = []any{map[string]any{"name": "stop", "state": "completed"}, map[string]any{"name": "start", "state": state}}
			}
		}
	}
	switch mode {
	case "environment-bad-home":
		plan["home"] = filepath.Join(home, "wrong")
	case "environment-bad-field":
		plan["secret"] = "do-not-render"
	case "environment-missing-field":
		delete(plan, "marker_fingerprint")
	case "environment-bad-fingerprint":
		plan["fingerprint"] = "sha256:bad"
	case "environment-no-tools":
		plan["tools"] = []any{}
	case "environment-duplicate-tools":
		plan["tools"].([]any)[1] = plan["tools"].([]any)[0]
	case "environment-null-namespace":
		plan["namespace_fingerprint"] = nil
	case "environment-stopped-running":
		plan["daemon_state"] = "stopped"
	case "environment-duplicate-session":
		plan["sessions"].([]any)[1] = plan["sessions"].([]any)[0]
	case "environment-bad-endpoint":
		plan["sessions"].([]any)[0].(map[string]any)["source"].(map[string]any)["protocol"] = "future"
	case "environment-wrong-review":
		plan["fingerprint"] = mutagenDigest("7")
	case "environment-wrong-paused":
		result["environment"].(map[string]any)["sessions"].([]any)[1].(map[string]any)["paused"] = false
	case "environment-wrong-config":
		result["environment"].(map[string]any)["sessions"].([]any)[0].(map[string]any)["configuration_fingerprint"] = mutagenDigest("7")
	case "environment-missing-session":
		post := result["environment"].(map[string]any)
		post["sessions"] = post["sessions"].([]any)[:1]
	case "environment-wrong-tools":
		result["environment"].(map[string]any)["tools"].([]any)[0].(map[string]any)["fingerprint"] = mutagenDigest("7")
	case "environment-wrong-namespace":
		result["environment"].(map[string]any)["namespace_fingerprint"] = mutagenDigest("7")
	case "environment-wrong-binding":
		result["environment"].(map[string]any)["previous"] = nil
	case "environment-no-post":
		result["environment"] = nil
	case "environment-bad-stage":
		result["stages"].([]any)[1].(map[string]any)["name"] = "stop"
	case "environment-bad-operation":
		result["operation_id"] = "wrong"
	case "environment-partial-post":
		result["outcome"], result["error_code"] = "partial", "sync_mutagen_environment_write_failed"
		result["stages"].([]any)[3].(map[string]any)["state"] = "failed"
	}
	return result
}
