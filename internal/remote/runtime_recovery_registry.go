package remote

import (
	"encoding/json"
	"slices"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func decodeRuntimePending(raw json.RawMessage, home string) ([]RuntimeRecoveryRecord, error) {
	var value struct {
		Home  string                  `json:"home"`
		Items []RuntimeRecoveryRecord `json:"items"`
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
	for i, rawItem := range items {
		r := &value.Items[i]
		fields, err := decodeAccountObject(rawItem, r, []string{"context", "account_id", "request_id", "digest", "original", "action", "binding", "phase", "rejection_code"}, []string{"binding", "rejection_code"})
		if err != nil || !validAccountImportRequestID(r.RequestID) || !validSyncFingerprint(r.Digest) || !validRuntimeAction(r.Action) || !slices.Contains([]string{"unknown", "accepted", "rejected"}, r.Phase) {
			return nil, fault.CLIProtocol
		}
		if r.Context, err = decodeRuntimeContext(fields["context"]); err != nil || r.Context.Home != home {
			return nil, fault.CLIProtocol
		}
		if r.Original, err = decodeRuntimeOriginal(fields["original"]); err != nil || r.AccountID != r.Original.ToolAccountID {
			return nil, fault.CLIProtocol
		}
		key := r.Context.ServerURL + "\x00" + r.Context.ActorUserID + "\x00" + r.AccountID + "\x00" + r.RequestID
		if seen[key] {
			return nil, fault.CLIProtocol
		}
		seen[key] = true
		if (r.Phase == "accepted") != (r.Binding != nil) || (r.Phase == "rejected") != (r.RejectionCode != nil) {
			return nil, fault.CLIProtocol
		}
		if r.RejectionCode != nil && !slices.Contains([]string{"runtime_recovery_rejected", "runtime_recovery_review_changed"}, *r.RejectionCode) {
			return nil, fault.CLIProtocol
		}
		if r.Binding != nil {
			binding, err := decodeRuntimeBinding(fields["binding"])
			if err != nil || binding.Original() != r.Original || binding.RecoveryAction() != r.Action || binding.TaskID != "recover_tool_account_runtime:"+r.AccountID+":"+r.RequestID {
				return nil, fault.CLIProtocol
			}
			r.Binding = &binding
		}
	}
	return value.Items, nil
}

func decodeRuntimeForget(raw json.RawMessage) (RuntimeRecoveryForgetReceipt, error) {
	var value RuntimeRecoveryForgetReceipt
	fields, err := decodeAccountObject(raw, &value, []string{"operation_id", "context", "account_id", "request_id", "digest", "forgotten"}, nil)
	if err != nil || !validDefaultTool(value.OperationID) || !validAccountImportRequestID(value.AccountID) || !validAccountImportRequestID(value.RequestID) || !validSyncFingerprint(value.Digest) || !value.Forgotten {
		return value, fault.CLIProtocol
	}
	value.Context, err = decodeRuntimeContext(fields["context"])
	return value, err
}
