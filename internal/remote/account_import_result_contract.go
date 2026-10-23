package remote

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func validAccountImportWarnings(codes []string) bool {
	if codes == nil || len(codes) > 2 {
		return false
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if !slices.Contains([]string{"CONFIG_IMPORT_EXECUTABLE_REVIEW_REQUIRED", "CONFIG_IMPORT_CUSTOM_MARKDOWN_REVIEW_REQUIRED"}, code) || seen[code] {
			return false
		}
		seen[code] = true
	}
	return true
}
func decodeAccountImportValidation(raw json.RawMessage) (AccountImportValidation, error) {
	var value AccountImportValidation
	fields, err := decodeAccountObject(raw, &value, []string{"operation_id", "context", "account_id", "selection", "source_home", "include", "exclude", "audit_written", "accepted", "rejected", "warning_codes"}, nil)
	if err != nil || !validDefaultTool(value.OperationID) || !validDefaultTool(value.AccountID) || !validWireGuardPath(value.SourceHome) || !validAccountImportPaths(value.Include) || !validAccountImportPaths(value.Exclude) || !validAccountImportPaths(value.Accepted) || !validAccountImportPaths(value.Rejected) || !value.AuditWritten || !validAccountImportWarnings(value.WarningCodes) {
		return value, fault.CLIProtocol
	}
	if value.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return value, err
	}
	if value.Selection, err = decodeAccountImportSelection(fields["selection"]); err != nil {
		return value, err
	}
	// Every reviewed scope entry must be accounted for exactly once.
	seen := map[string]bool{}
	for _, list := range [][]string{value.Accepted, value.Rejected} {
		for _, path := range list {
			if seen[path] || !slices.Contains(value.Include, path) {
				return value, fault.CLIProtocol
			}
			seen[path] = true
		}
	}
	if len(seen) != len(value.Include) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func validAccountImportTask(task *string, account string) bool {
	if task == nil {
		return true
	}
	prefix := "import_tool_account_config:" + account + ":"
	return strings.HasPrefix(*task, prefix) && validAccountImportRequestID(strings.TrimPrefix(*task, prefix))
}
func validAccountImportReference(ref AccountImportReference) bool {
	return validAccountContext(ref.Context, ref.Context.Home) && validDefaultTool(ref.AccountID) && validAccountImportRequestID(ref.RequestID) && validSyncFingerprint(ref.Digest) && validSyncFingerprint(ref.IntentFingerprint) && validAccountImportTask(ref.TaskID, ref.AccountID) && slices.Contains([]string{"pending", "accepted", "rejected", "succeeded", "failed", "cancelled", "expired"}, ref.Phase)
}
func decodeAccountImportPending(raw json.RawMessage, home string) ([]AccountImportReference, error) {
	var value struct {
		Home  string                   `json:"home"`
		Items []AccountImportReference `json:"items"`
	}
	fields, err := decodeAccountObject(raw, &value, []string{"home", "items"}, nil)
	if err != nil || value.Home != home || value.Items == nil || len(value.Items) > 256 {
		return nil, fault.CLIProtocol
	}
	var items []json.RawMessage
	if json.Unmarshal(fields["items"], &items) != nil {
		return nil, fault.CLIProtocol
	}
	seen := map[string]bool{}
	for _, rawItem := range items {
		var ref AccountImportReference
		fields, err := decodeAccountObject(rawItem, &ref, []string{"context", "account_id", "request_id", "digest", "intent_fingerprint", "task_id", "phase"}, []string{"task_id"})
		if err != nil || !validAccountImportReference(ref) || ref.Context.Home != home || seen[ref.RequestID] {
			return nil, fault.CLIProtocol
		}
		if _, err = decodeAccountContext(fields["context"]); err != nil {
			return nil, err
		}
		seen[ref.RequestID] = true
	}
	return value.Items, nil
}
func decodeAccountImportReceipt(raw json.RawMessage, apply bool) (AccountImportReceipt, error) {
	var value AccountImportReceipt
	required := []string{"context", "account_id", "request_id", "digest", "intent_fingerprint", "task_id", "phase", "accepted", "file_count", "warning_codes"}
	if apply {
		required = append(required, "operation_id")
	}
	fields, err := decodeAccountObject(raw, &value, required, []string{"task_id"})
	if err != nil || !validDefaultTool(value.AccountID) || !validAccountImportRequestID(value.RequestID) || !validSyncFingerprint(value.Digest) || !validSyncFingerprint(value.IntentFingerprint) || !validAccountImportTask(value.TaskID, value.AccountID) || value.TaskID == nil || !value.Accepted || value.FileCount > 2048 || !validAccountImportWarnings(value.WarningCodes) {
		return value, fault.CLIProtocol
	}
	if value.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return value, err
	}
	if apply {
		if value.Phase != "accepted" || !validDefaultTool(value.OperationID) {
			return value, fault.CLIProtocol
		}
	} else if !slices.Contains([]string{"pending", "leased", "running", "succeeded", "failed", "cancelled", "expired"}, value.Phase) {
		return value, fault.CLIProtocol
	}
	return value, nil
}
func decodeAccountImportForget(raw json.RawMessage) (AccountImportForgetReceipt, error) {
	var value AccountImportForgetReceipt
	fields, err := decodeAccountObject(raw, &value, []string{"operation_id", "context", "account_id", "request_id", "digest", "task_id", "forgotten"}, []string{"task_id"})
	if err != nil || !validDefaultTool(value.OperationID) || !validDefaultTool(value.AccountID) || !validAccountImportRequestID(value.RequestID) || !validSyncFingerprint(value.Digest) || !validAccountImportTask(value.TaskID, value.AccountID) || !value.Forgotten {
		return value, fault.CLIProtocol
	}
	value.Context, err = decodeAccountContext(fields["context"])
	return value, err
}
