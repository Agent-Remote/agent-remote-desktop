package remote

// RuntimeRecoveryContext identifies the current administrator, never a device.
type RuntimeRecoveryContext struct {
	Home        string `json:"home"`
	ServerURL   string `json:"server_url"`
	ActorUserID string `json:"actor_user_id"`
}
type RuntimeRecoveryOriginal struct {
	TaskID               string `json:"task_id"`
	TaskRecordID         string `json:"task_record_id"`
	UserID               string `json:"user_id"`
	ToolAccountID        string `json:"tool_account_id"`
	NodeID               string `json:"node_id"`
	ToolType             string `json:"tool_type"`
	SourceRuntimeBackend string `json:"source_runtime_backend"`
	TargetRuntimeBackend string `json:"target_runtime_backend"`
}
type RuntimeRecoveryBinding struct {
	Version              int     `json:"version"`
	Action               *string `json:"action,omitempty"`
	TaskID               string  `json:"task_id"`
	TaskRecordID         string  `json:"task_record_id"`
	OriginalTaskID       string  `json:"original_task_id"`
	OriginalTaskRecordID string  `json:"original_task_record_id"`
	NodeID               string  `json:"node_id"`
	UserID               string  `json:"user_id"`
	ToolAccountID        string  `json:"tool_account_id"`
	ToolType             string  `json:"tool_type"`
	SourceRuntimeBackend string  `json:"source_runtime_backend"`
	TargetRuntimeBackend string  `json:"target_runtime_backend"`
}

func (b RuntimeRecoveryBinding) Original() RuntimeRecoveryOriginal {
	return RuntimeRecoveryOriginal{b.OriginalTaskID, b.OriginalTaskRecordID, b.UserID, b.ToolAccountID, b.NodeID, b.ToolType, b.SourceRuntimeBackend, b.TargetRuntimeBackend}
}
func (b RuntimeRecoveryBinding) RecoveryAction() string {
	switch b.Version {
	case 1:
		return "verify_target"
	case 2:
		return "verify_source"
	case 3:
		return "repair_source"
	default:
		return ""
	}
}

type RuntimeRecoveryAuthorityInput struct {
	ExpectedHome      string `json:"expected_home"`
	ExpectedServerURL string `json:"expected_server_url"`
}
type RuntimeRecoveryAuthority struct {
	Context RuntimeRecoveryContext `json:"context"`
	Admin   bool                   `json:"admin"`
}
type RuntimeRecoveryInspectInput struct {
	Expected       RuntimeRecoveryContext `json:"expected_context"`
	AccountID      string                 `json:"account_id"`
	OriginalTaskID string                 `json:"original_task_id"`
	Action         string                 `json:"action"`
}
type RuntimeRecoveryPlan struct {
	Context        RuntimeRecoveryContext  `json:"context"`
	AccountID      string                  `json:"account_id"`
	Original       RuntimeRecoveryOriginal `json:"original"`
	Action         string                  `json:"action"`
	OriginalStatus string                  `json:"original_status"`
	AccountStatus  string                  `json:"account_status"`
	Digest         string                  `json:"digest"`
}
type RuntimeRecoveryApplyInput struct {
	Confirmed    bool
	ExpectedPlan RuntimeRecoveryPlan
	RequestID    string
}
type RuntimeRecoveryStatusInput struct {
	Expected        RuntimeRecoveryContext  `json:"expected_context"`
	AccountID       string                  `json:"account_id"`
	RequestID       string                  `json:"request_id"`
	ExpectedDigest  *string                 `json:"expected_digest"`
	ExpectedBinding *RuntimeRecoveryBinding `json:"expected_binding"`
}
type RuntimeRecovery struct {
	Binding RuntimeRecoveryBinding `json:"binding"`
	Status  string                 `json:"status"`
}
type RuntimeRecoveryReceipt struct {
	OperationID                string                 `json:"operation_id,omitempty"`
	Context                    RuntimeRecoveryContext `json:"context"`
	AccountID                  string                 `json:"account_id"`
	RequestID                  string                 `json:"request_id"`
	Digest                     *string                `json:"digest"`
	Recovery                   RuntimeRecovery        `json:"recovery"`
	TargetCompletionConfirmed  bool                   `json:"target_completion_confirmed"`
	SourceRestorationConfirmed bool                   `json:"source_restoration_confirmed"`
}
type RuntimeRecoveryRecord struct {
	Context       RuntimeRecoveryContext  `json:"context"`
	AccountID     string                  `json:"account_id"`
	RequestID     string                  `json:"request_id"`
	Digest        string                  `json:"digest"`
	Original      RuntimeRecoveryOriginal `json:"original"`
	Action        string                  `json:"action"`
	Binding       *RuntimeRecoveryBinding `json:"binding"`
	Phase         string                  `json:"phase"`
	RejectionCode *string                 `json:"rejection_code"`
}
type RuntimeRecoveryForgetInput struct {
	Confirmed       bool                    `json:"confirmed"`
	Expected        RuntimeRecoveryContext  `json:"expected_context"`
	AccountID       string                  `json:"account_id"`
	RequestID       string                  `json:"request_id"`
	ExpectedDigest  *string                 `json:"expected_digest"`
	ExpectedBinding *RuntimeRecoveryBinding `json:"expected_binding"`
}
type RuntimeRecoveryForgetReceipt struct {
	OperationID string                 `json:"operation_id"`
	Context     RuntimeRecoveryContext `json:"context"`
	AccountID   string                 `json:"account_id"`
	RequestID   string                 `json:"request_id"`
	Digest      string                 `json:"digest"`
	Forgotten   bool                   `json:"forgotten"`
}
