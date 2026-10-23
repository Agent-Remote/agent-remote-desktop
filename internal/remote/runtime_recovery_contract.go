package remote

import (
	"encoding/json"
	"net/url"
	"slices"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func validRuntimeServer(value string) bool {
	u, err := url.Parse(value)
	return validAccountText(value, 8192) && err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != "" && u.User == nil && u.RawQuery == "" && !u.ForceQuery && !strings.Contains(value, "#")
}

// Authority returns the CLI's canonical URL. Only known equivalent URL forms
// are accepted here; subsequent operations bind that canonical context exactly.
func sameRuntimeServer(a, b string) bool {
	if !validRuntimeServer(a) || !validRuntimeServer(b) {
		return false
	}
	left, _ := url.Parse(a)
	right, _ := url.Parse(b)
	port := func(u *url.URL) string {
		p := u.Port()
		if u.Scheme == "https" && p == "443" || u.Scheme == "http" && p == "80" {
			return ""
		}
		return p
	}
	return left.Scheme == right.Scheme && strings.EqualFold(left.Hostname(), right.Hostname()) && port(left) == port(right) && strings.TrimRight(left.EscapedPath(), "/") == strings.TrimRight(right.EscapedPath(), "/")
}
func validRuntimeContext(value RuntimeRecoveryContext, home string) bool {
	return value.Home == home && validWireGuardPath(home) && validRuntimeServer(value.ServerURL) && validAccountImportRequestID(value.ActorUserID)
}
func validRuntimeAction(value string) bool {
	return slices.Contains([]string{"verify_target", "verify_source", "repair_source"}, value)
}
func validRuntimeStatus(value string) bool {
	return slices.Contains([]string{"pending", "leased", "running", "succeeded", "failed", "cancelled", "expired"}, value)
}
func RuntimeRecoveryTerminal(value string) bool {
	return slices.Contains([]string{"succeeded", "failed", "cancelled", "expired"}, value)
}
func validRuntimeTask(value, prefix, account string) bool {
	prefix += ":" + account + ":"
	return strings.HasPrefix(value, prefix) && validAccountImportRequestID(strings.TrimPrefix(value, prefix))
}
func validRuntimeOriginal(value RuntimeRecoveryOriginal) bool {
	return validAccountImportRequestID(value.ToolAccountID) && validRuntimeTask(value.TaskID, "migrate_tool_account_runtime", value.ToolAccountID) && validAccountImportRequestID(value.TaskRecordID) && validAccountImportRequestID(value.UserID) && validAccountImportRequestID(value.NodeID) && value.ToolType == "claude" && slices.Contains([]string{"native", "docker_sandbox"}, value.SourceRuntimeBackend) && slices.Contains([]string{"native", "docker_sandbox"}, value.TargetRuntimeBackend) && value.SourceRuntimeBackend != value.TargetRuntimeBackend
}
func decodeRuntimeContext(raw json.RawMessage) (RuntimeRecoveryContext, error) {
	var value RuntimeRecoveryContext
	_, err := decodeAccountObject(raw, &value, []string{"home", "server_url", "actor_user_id"}, nil)
	if err != nil || !validRuntimeContext(value, value.Home) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func decodeRuntimeOriginal(raw json.RawMessage) (RuntimeRecoveryOriginal, error) {
	var value RuntimeRecoveryOriginal
	_, err := decodeAccountObject(raw, &value, []string{"task_id", "task_record_id", "user_id", "tool_account_id", "node_id", "tool_type", "source_runtime_backend", "target_runtime_backend"}, nil)
	if err != nil || !validRuntimeOriginal(value) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func decodeRuntimeBinding(raw json.RawMessage) (RuntimeRecoveryBinding, error) {
	var value RuntimeRecoveryBinding
	if json.Unmarshal(raw, &value) != nil {
		return value, fault.CLIProtocol
	}
	fields := []string{"version", "task_id", "task_record_id", "original_task_id", "original_task_record_id", "node_id", "user_id", "tool_account_id", "tool_type", "source_runtime_backend", "target_runtime_backend"}
	if value.Version != 1 {
		fields = append(fields, "action")
	}
	_, err := decodeAccountObject(raw, &value, fields, nil)
	if err != nil || value.Version < 1 || value.Version > 3 || !validRuntimeOriginal(value.Original()) || !validRuntimeTask(value.TaskID, "recover_tool_account_runtime", value.ToolAccountID) || !validAccountImportRequestID(value.TaskRecordID) || value.TaskRecordID == value.OriginalTaskRecordID || value.Version == 1 && value.Action != nil || value.Version != 1 && (value.Action == nil || *value.Action != value.RecoveryAction()) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func decodeRuntimePlan(raw json.RawMessage) (RuntimeRecoveryPlan, error) {
	var value RuntimeRecoveryPlan
	fields, err := decodeAccountObject(raw, &value, []string{"context", "account_id", "original", "action", "original_status", "account_status", "digest"}, nil)
	if err != nil || !validRuntimeAction(value.Action) || !validRuntimeStatus(value.OriginalStatus) || !validAccountState(value.AccountStatus) || !validSyncFingerprint(value.Digest) {
		return value, fault.CLIProtocol
	}
	if value.Context, err = decodeRuntimeContext(fields["context"]); err != nil {
		return value, err
	}
	if value.Original, err = decodeRuntimeOriginal(fields["original"]); err != nil {
		return value, err
	}
	if value.AccountID != value.Original.ToolAccountID {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func decodeRuntimeReceipt(raw json.RawMessage, mutation bool) (RuntimeRecoveryReceipt, error) {
	var value RuntimeRecoveryReceipt
	required := []string{"context", "account_id", "request_id", "digest", "recovery", "target_completion_confirmed", "source_restoration_confirmed"}
	if mutation {
		required = append(required, "operation_id")
	}
	fields, err := decodeAccountObject(raw, &value, required, []string{"digest"})
	if err != nil || !validAccountImportRequestID(value.AccountID) || !validAccountImportRequestID(value.RequestID) || value.Digest != nil && !validSyncFingerprint(*value.Digest) || mutation && (!validDefaultTool(value.OperationID) || value.Digest == nil) {
		return value, fault.CLIProtocol
	}
	if value.Context, err = decodeRuntimeContext(fields["context"]); err != nil {
		return value, err
	}
	recoveryFields, err := decodeAccountObject(fields["recovery"], &value.Recovery, []string{"binding", "status"}, nil)
	if err != nil || !validRuntimeStatus(value.Recovery.Status) {
		return value, fault.CLIProtocol
	}
	if value.Recovery.Binding, err = decodeRuntimeBinding(recoveryFields["binding"]); err != nil {
		return value, err
	}
	b := value.Recovery.Binding
	if b.ToolAccountID != value.AccountID || b.TaskID != "recover_tool_account_runtime:"+value.AccountID+":"+value.RequestID || value.TargetCompletionConfirmed != (value.Recovery.Status == "succeeded" && b.Version == 1) || value.SourceRestorationConfirmed != (value.Recovery.Status == "succeeded" && b.Version != 1) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
