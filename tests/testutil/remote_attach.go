package testutil

import "os"

const RemoteAttachDeviceID = "33333333-3333-4333-8333-333333333333"

func applyRemoteContextFixture(context map[string]any) {
	if device := os.Getenv("DESKTOP_TEST_REMOTE_DEVICE"); device != "" {
		context["active_device_id"] = device
	}
	if server := os.Getenv("DESKTOP_TEST_REMOTE_SERVER"); server != "" {
		context["server_url"] = server
	}
}

func remoteSessionFixture(result map[string]any, mode string) {
	items := result["items"].([]any)
	first := items[0].(map[string]any)
	first["tool_account_id"] = "44444444-4444-4444-8444-444444444444"
	first["workspace_id"] = "66666666-6666-4666-8666-666666666666"
	first["project_key"] = "project-alpha"
	first["node_id"] = "55555555-5555-4555-8555-555555555555"
	switch mode {
	case "remote-delete", "remote-delete-unknown":
		first["status"] = "stopped"
		if len(items) > 1 {
			items[1].(map[string]any)["status"] = "interrupted"
			items[1].(map[string]any)["tool_account_id"] = first["tool_account_id"]
			items[1].(map[string]any)["workspace_id"] = "77777777-7777-4777-8777-777777777777"
			items[1].(map[string]any)["project_key"] = "project-beta"
			items[1].(map[string]any)["node_id"] = first["node_id"]
		}
	case "remote-attach-missing":
		result["items"] = []any{}
	case "remote-attach-stopped":
		first["status"] = "stopped"
	case "remote-attach-active":
		first["status"] = "active"
	case "remote-attach-prefix":
		first["id"] = "11111111"
	case "remote-attach-duplicate":
		result["items"] = append(items, first)
	case "remote-session-malformed":
		first["status"] = "running\nprivate"
	}
}
