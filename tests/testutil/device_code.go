package testutil

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func deviceCodeFixture(id string, params json.RawMessage, mode string) {
	var input struct {
		Server    string `json:"server_url"`
		Confirmed bool   `json:"confirmed"`
	}
	if json.Unmarshal(params, &input) != nil || !input.Confirmed {
		os.Exit(4)
	}
	if path := os.Getenv("DESKTOP_TEST_DEVICE_CODE_INPUT"); path != "" {
		if os.WriteFile(path, params, 0600) != nil {
			os.Exit(5)
		}
	}
	if mode == "device-inherited-stdout" {
		child := exec.Command(os.Args[0], "--desktop-control")
		for _, value := range os.Environ() {
			if !strings.HasPrefix(value, "DESKTOP_TEST_CLI_SCENARIO=") {
				child.Env = append(child.Env, value)
			}
		}
		child.Env = append(child.Env, "DESKTOP_TEST_CLI_SCENARIO=device-hold-stdout")
		child.Stdout = os.Stdout
		if child.Start() != nil {
			os.Exit(6)
		}
		if path := os.Getenv("DESKTOP_TEST_CHILD_PID"); path != "" {
			_ = os.WriteFile(path, []byte(strconv.Itoa(child.Process.Pid)), 0600)
		}
		_ = child.Process.Release()
	}
	progress := map[string]any{"user_code": "ABCD-1234", "verification_url": "https://auth.test.invalid/cli?code=ABCD-1234", "expires_in": 300, "interval": 1}
	value := map[string]any{"protocol_version": 1, "request_id": id, "type": "progress", "phase": "awaiting_approval", "progress": progress}
	if mode == "device-wait-short" {
		progress["expires_in"] = 1
	}
	switch mode {
	case "device-wrong-id":
		value["request_id"] = "wrong"
	case "device-invalid-url":
		progress["verification_url"] = "file:///private/token"
	case "device-secret-progress":
		progress["device_code"] = "secret-device-code"
	case "device-invalid-code":
		progress["user_code"] = "unsafe\ncode"
	case "device-invalid-query":
		progress["verification_url"] = "https://auth.test.invalid/cli?access_token=secret"
	case "device-oversized":
		progress["expires_in"] = 2000
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.Encode(value)
	if mode == "device-early-eof" {
		_ = os.Stdout.Close()
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if path := os.Getenv("DESKTOP_TEST_PROGRESS_SENT"); path != "" {
		_ = os.WriteFile(path, nil, 0600)
	}
	if mode == "device-duplicate-progress" {
		encoder.Encode(value)
	}
	if mode == "device-wait" || mode == "device-wait-short" || mode == "device-wait-unknown" {
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if path := os.Getenv("DESKTOP_TEST_DEVICE_CODE_APPROVED"); path != "" {
				if _, err := os.Stat(path); err == nil {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	delete(value, "progress")
	value["request_id"], value["type"], value["phase"] = id, "result", "completed"
	operationID := id
	if mode == "device-wrong-operation" || mode == "device-wait-unknown" {
		operationID = "wrong"
	}
	value["result"] = map[string]any{"operation_id": operationID, "server_url": input.Server, "registered_device_id": "device-1", "credential_backend": "file", "expires_in": 3600}
	if mode == "device-expired" {
		delete(value, "result")
		value["type"], value["phase"], value["error"] = "error", "failed", map[string]any{"code": "login_expired", "retryable": false}
	}
	encoder.Encode(value)
	if mode == "device-terminal-eof" {
		_ = os.Stdout.Close()
		time.Sleep(30 * time.Second)
		os.Exit(0)
	}
	if mode == "device-trailing" {
		fmt.Fprintln(os.Stdout, `{"unexpected":true}`)
	}
	if mode == "device-expired" {
		os.Exit(1)
	}
	os.Exit(0)
}
