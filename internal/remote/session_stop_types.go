package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// SessionStopInput is the reviewed, content-free stop request sent to the
// Rust desktop-control adapter. The adapter owns caller-key generation when
// RequestID is empty; the returned key is required for any later recovery.
type SessionStopInput struct {
	SchemaVersion      int                      `json:"schema_version"`
	Expected           cliadapter.ConfigContext `json:"expected_context"`
	SessionID          string                   `json:"session_id"`
	ExpectedNodeID     string                   `json:"expected_node_id,omitempty"`
	ExpectedBackend    string                   `json:"expected_runtime_backend,omitempty"`
	ExpectedStopTaskID string                   `json:"expected_stop_task_id,omitempty"`
	Reason             string                   `json:"reason,omitempty"`
	Confirmed          bool                     `json:"confirmed"`
}

// SessionStopRequestInput is deliberately the only input accepted by the
// recovery query. It cannot submit a second stop request.
type SessionStopRequestInput struct {
	RequestID string                   `json:"request_id"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	Wait      bool                     `json:"wait"`
	Timeout   uint64                   `json:"timeout"`
}

// SessionStopReceipt is an immutable observation of the stop request. The
// process stop, Skill save, and sync state are intentionally separate fields;
// a terminal session status does not prove that its snapshot was published.
type SessionStopReceipt struct {
	SchemaVersion                int                      `json:"schema_version"`
	RequestID                    string                   `json:"request_id"`
	Phase                        string                   `json:"phase"`
	Context                      cliadapter.ConfigContext `json:"context"`
	SessionID                    string                   `json:"session_id"`
	NodeID                       string                   `json:"node_id"`
	RuntimeBackend               string                   `json:"runtime_backend"`
	RuntimeResourceID            *string                  `json:"runtime_resource_id"`
	StopTaskID                   *string                  `json:"stop_task_id"`
	StopTaskRecordID             *string                  `json:"stop_task_record_id"`
	StopTaskStatus               *string                  `json:"stop_task_status"`
	SessionStatus                *string                  `json:"session_status"`
	SkillFinalizationOperationID *string                  `json:"skill_finalization_operation_id"`
	SessionDeleted               bool                     `json:"session_deleted"`
	ObservedAt                   *string                  `json:"observed_at"`
	ErrorCode                    *string                  `json:"error_code"`
	IntentFingerprint            string                   `json:"intent_fingerprint"`
	Reason                       string                   `json:"reason"`
	ResultStatus                 *string                  `json:"result_status"`
	ProcessStopped               *bool                    `json:"process_stopped"`
	SaveStatus                   *string                  `json:"save_status"`
}
