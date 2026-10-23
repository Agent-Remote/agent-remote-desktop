package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// SessionDeleteSelection is the immutable filter shown to the user when a
// deletion review is opened. It is sent alongside the explicit IDs so a
// server can reject a request whose reviewed selection no longer matches.
type SessionDeleteSelection struct {
	ToolType   string   `json:"tool_type"`
	Statuses   []string `json:"statuses"`
	ProjectKey *string  `json:"project_key"`
}

// SessionDeleteInput contains only the reviewed collection. The Rust bridge
// creates and journals the caller key; Desktop never invents a replacement
// key after an outcome becomes unknown.
type SessionDeleteInput struct {
	SchemaVersion     int                      `json:"schema_version"`
	Expected          cliadapter.ConfigContext `json:"expected_context"`
	ToolType          string                   `json:"tool_type"`
	SessionIDs        []string                 `json:"session_ids"`
	Selection         SessionDeleteSelection   `json:"selection"`
	SelectionDigest   string                   `json:"selection_digest"`
	IntentFingerprint string                   `json:"intent_fingerprint"`
	Reason            string                   `json:"reason"`
	Confirmed         bool                     `json:"confirmed"`
}

// SessionDeleteRequestInput is the read-only recovery operation. It accepts
// only the original request UUID and reviewed context.
type SessionDeleteRequestInput struct {
	RequestID string                   `json:"request_id"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	Wait      bool                     `json:"wait"`
	Timeout   uint64                   `json:"timeout"`
}

// SessionDeleteSessionContext is the immutable identity snapshot for one
// reviewed session. It survives deletion so a receipt cannot be mistaken for
// a different session that later reuses a project or account.
type SessionDeleteSessionContext struct {
	SessionID         string  `json:"session_id"`
	ToolAccountID     string  `json:"tool_account_id"`
	WorkspaceID       string  `json:"workspace_id"`
	ProjectKey        string  `json:"project_key"`
	NodeID            string  `json:"node_id"`
	RuntimeBackend    string  `json:"runtime_backend"`
	RuntimeResourceID *string `json:"runtime_resource_id"`
}

// SessionDeleteReceipt is an immutable collection deletion observation.
// Retention blockers are kept separate from process stop and Skill save
// observations so a completed stop cannot be presented as deletion success.
type SessionDeleteReceipt struct {
	SchemaVersion      int                           `json:"schema_version"`
	RequestID          string                        `json:"request_id"`
	Context            cliadapter.ConfigContext      `json:"context"`
	ToolType           string                        `json:"tool_type"`
	SessionIDs         []string                      `json:"session_ids"`
	Selection          SessionDeleteSelection        `json:"selection"`
	SelectionDigest    string                        `json:"selection_digest"`
	IntentFingerprint  string                        `json:"intent_fingerprint"`
	Reason             string                        `json:"reason"`
	Phase              string                        `json:"phase"`
	DeleteTaskID       *string                       `json:"delete_task_id"`
	DeleteTaskRecordID *string                       `json:"delete_task_record_id"`
	TaskStatus         *string                       `json:"task_status"`
	DeletedSessionIDs  []string                      `json:"deleted_session_ids"`
	SessionStatuses    map[string]string             `json:"session_statuses"`
	SessionContexts    []SessionDeleteSessionContext `json:"session_contexts"`
	ObservedAt         *string                       `json:"observed_at"`
	ErrorCode          *string                       `json:"error_code"`
	ResultStatus       *string                       `json:"result_status"`
	SessionDeleted     bool                          `json:"session_deleted"`
	CollectionMismatch bool                          `json:"collection_mismatch"`
	RetentionBlockers  []string                      `json:"retention_blockers"`
}
