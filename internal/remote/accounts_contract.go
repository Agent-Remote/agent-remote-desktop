package remote

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func validAccountText(s string, limit int) bool {
	return s != "" && len(s) <= limit && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}
func validAccountOptional(s *string, limit int) bool { return s == nil || validAccountText(*s, limit) }
func validAccountTags(tags []string) bool {
	if tags == nil || len(tags) > 64 {
		return false
	}
	for _, tag := range tags {
		if !validAccountText(tag, 128) {
			return false
		}
	}
	return true
}
func validAccountState(state string) bool {
	return slices.Contains([]string{"binding_requested", "binding_session_starting", "binding_waiting_user_login", "binding_verifying", "active", "expired", "disabled", "failed", "node_unavailable", "migrating"}, state)
}
func decodeAccountObject(raw json.RawMessage, out any, required, nullable []string) (map[string]json.RawMessage, error) {
	if uniqueDefaultJSON(raw) != nil {
		return nil, fault.CLIProtocol
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(out) != nil {
		return nil, fault.CLIProtocol
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) != len(required) {
		return nil, fault.CLIProtocol
	}
	for _, key := range required {
		field, exists := fields[key]
		if !exists || (bytes.Equal(field, []byte("null")) && !slices.Contains(nullable, key)) {
			return nil, fault.CLIProtocol
		}
	}
	return fields, nil
}
func decodeAccountDetails(raw json.RawMessage) (AccountDetails, error) {
	var a AccountDetails
	if _, err := decodeAccountObject(raw, &a, []string{"id", "tool_type", "display_name", "status", "region_code", "timezone", "locale", "preferred_node_tags", "affinity_node_id", "runtime_backend", "created_at", "updated_at"}, []string{"affinity_node_id", "runtime_backend"}); err != nil {
		return a, err
	}
	if !validDefaultTool(a.ID) || !validAccountState(a.Status) || !validDefaultID(a.AffinityNodeID) || !validAccountText(a.CreatedAt, 128) || !validAccountText(a.UpdatedAt, 128) || (a.RuntimeBackend != nil && *a.RuntimeBackend != "native" && *a.RuntimeBackend != "docker_sandbox") {
		return AccountDetails{}, fault.CLIProtocol
	}
	if ValidateAccountCreate(AccountCreateInput{Tool: a.Tool, Name: a.Name, Region: a.Region, Timezone: a.Timezone, Locale: a.Locale, Tags: a.Tags}) != nil {
		return AccountDetails{}, fault.CLIProtocol
	}
	return a, nil
}
func decodeAccountBinding(raw json.RawMessage) (AccountBinding, error) {
	var b AccountBinding
	if _, err := decodeAccountObject(raw, &b, []string{"tool_account_id", "status", "node_id", "binding_session_id", "tmux_session_name", "account_remote_path", "task_id", "error_code"}, []string{"node_id", "binding_session_id", "tmux_session_name", "account_remote_path", "task_id", "error_code"}); err != nil {
		return b, err
	}
	if !validDefaultTool(b.AccountID) || !validAccountState(b.Status) || !validDefaultID(b.NodeID) || !validAccountOptional(b.BindingSessionID, 512) || !validAccountOptional(b.TmuxSessionName, 512) || !validAccountOptional(b.RemotePath, 4096) || !validAccountOptional(b.TaskID, 512) || (b.ErrorCode != nil && *b.ErrorCode != "binding_failed") {
		return AccountBinding{}, fault.CLIProtocol
	}
	return b, nil
}
func decodeAccountContext(raw json.RawMessage) (cliadapter.ConfigContext, error) {
	var c cliadapter.ConfigContext
	if strictDefaultJSON(raw, &c, "home", "server_url", "active_device_id") != nil || !validAccountContext(c, c.Home) {
		return c, fault.CLIProtocol
	}
	return c, nil
}
func decodeAccountStatus(raw json.RawMessage) (AccountStatus, error) {
	var status AccountStatus
	fields, err := decodeAccountObject(raw, &status, []string{"context", "account", "binding"}, nil)
	if err != nil {
		return status, err
	}
	if status.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return AccountStatus{}, err
	}
	if status.Account, err = decodeAccountDetails(fields["account"]); err != nil {
		return AccountStatus{}, err
	}
	if status.Binding, err = decodeAccountBinding(fields["binding"]); err != nil {
		return AccountStatus{}, err
	}
	return status, nil
}
func decodeAccountReceipt(raw json.RawMessage) (AccountReceipt, error) {
	var receipt AccountReceipt
	fields, err := decodeAccountObject(raw, &receipt, []string{"operation_id", "context", "accepted", "account", "binding"}, []string{"account", "binding"})
	if err != nil {
		return receipt, err
	}
	if receipt.OperationID == "" || !receipt.Accepted {
		return AccountReceipt{}, fault.CLIProtocol
	}
	if receipt.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return AccountReceipt{}, err
	}
	if receipt.Account != nil {
		account, err := decodeAccountDetails(fields["account"])
		if err != nil {
			return AccountReceipt{}, err
		}
		receipt.Account = &account
	}
	if receipt.Binding != nil {
		binding, err := decodeAccountBinding(fields["binding"])
		if err != nil {
			return AccountReceipt{}, err
		}
		receipt.Binding = &binding
	}
	return receipt, nil
}
