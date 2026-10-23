package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// AccountImportSelection matches the CLI defaults: no resume history, skills included.
type AccountImportSelection struct {
	Tool                 string `json:"tool"`
	IncludeResumeHistory bool   `json:"include_resume_history"`
	ExcludeSkills        bool   `json:"exclude_skills"`
}
type AccountImportDiscovery struct {
	SourceHome string                 `json:"source_home"`
	Selection  AccountImportSelection `json:"selection"`
	Include    []string               `json:"include"`
	Exclude    []string               `json:"exclude"`
}
type AccountImportFile struct {
	Path   string `json:"path"`
	Size   uint64 `json:"size"`
	SHA256 string `json:"sha256"`
	Mode   uint32 `json:"mode"`
}

// AccountImportPlan contains metadata only. File contents never enter Go.
type AccountImportPlan struct {
	Context           cliadapter.ConfigContext `json:"context"`
	AccountID         string                   `json:"account_id"`
	Selection         AccountImportSelection   `json:"selection"`
	SourceHome        string                   `json:"source_home"`
	Include           []string                 `json:"include"`
	Exclude           []string                 `json:"exclude"`
	Manifest          []AccountImportFile      `json:"manifest"`
	TotalBytes        uint64                   `json:"total_bytes"`
	SourceFingerprint string                   `json:"source_fingerprint"`
	IntentFingerprint string                   `json:"intent_fingerprint"`
	Digest            string                   `json:"digest"`
}
type AccountImportPrepareInput struct {
	Expected           cliadapter.ConfigContext `json:"expected_context"`
	AccountID          string                   `json:"account_id"`
	Selection          AccountImportSelection   `json:"selection"`
	ExpectedSourceHome string                   `json:"expected_source_home"`
}
type AccountImportValidateInput struct {
	Confirmed          bool                     `json:"confirmed"`
	Expected           cliadapter.ConfigContext `json:"expected_context"`
	AccountID          string                   `json:"account_id"`
	Selection          AccountImportSelection   `json:"selection"`
	ExpectedSourceHome string                   `json:"expected_source_home"`
	ExpectedInclude    []string                 `json:"expected_include"`
}
type AccountImportApplyInput struct {
	Confirmed    bool
	ExpectedPlan AccountImportPlan
	RequestID    string
}
type AccountImportStatusInput struct {
	Expected       cliadapter.ConfigContext `json:"expected_context"`
	AccountID      string                   `json:"account_id"`
	RequestID      string                   `json:"request_id"`
	ExpectedDigest string                   `json:"expected_digest"`
	ExpectedTaskID *string                  `json:"expected_task_id"`
}

type AccountImportValidation struct {
	OperationID  string                   `json:"operation_id"`
	Context      cliadapter.ConfigContext `json:"context"`
	AccountID    string                   `json:"account_id"`
	Selection    AccountImportSelection   `json:"selection"`
	SourceHome   string                   `json:"source_home"`
	Include      []string                 `json:"include"`
	Exclude      []string                 `json:"exclude"`
	AuditWritten bool                     `json:"audit_written"`
	Accepted     []string                 `json:"accepted"`
	Rejected     []string                 `json:"rejected"`
	WarningCodes []string                 `json:"warning_codes"`
}

// AccountImportReference is the CLI-owned durable public recovery reference.
// It contains no local source or manifest paths and no file contents.
type AccountImportReference struct {
	Context           cliadapter.ConfigContext `json:"context"`
	AccountID         string                   `json:"account_id"`
	RequestID         string                   `json:"request_id"`
	Digest            string                   `json:"digest"`
	IntentFingerprint string                   `json:"intent_fingerprint"`
	TaskID            *string                  `json:"task_id"`
	Phase             string                   `json:"phase"`
}
type AccountImportReceipt struct {
	OperationID       string                   `json:"operation_id,omitempty"`
	Context           cliadapter.ConfigContext `json:"context"`
	AccountID         string                   `json:"account_id"`
	RequestID         string                   `json:"request_id"`
	Digest            string                   `json:"digest"`
	IntentFingerprint string                   `json:"intent_fingerprint"`
	TaskID            *string                  `json:"task_id"`
	Phase             string                   `json:"phase"`
	Accepted          bool                     `json:"accepted"`
	FileCount         uint64                   `json:"file_count"`
	WarningCodes      []string                 `json:"warning_codes"`
}

func (r AccountImportReceipt) Reference() AccountImportReference {
	return AccountImportReference{Context: accountContextSnapshot(r.Context), AccountID: r.AccountID, RequestID: r.RequestID, Digest: r.Digest, IntentFingerprint: r.IntentFingerprint, TaskID: copyDefaultText(r.TaskID), Phase: r.Phase}
}

type AccountImportForgetInput struct {
	Confirmed      bool                     `json:"confirmed"`
	Expected       cliadapter.ConfigContext `json:"expected_context"`
	AccountID      string                   `json:"account_id"`
	RequestID      string                   `json:"request_id"`
	ExpectedDigest string                   `json:"expected_digest"`
	ExpectedTaskID *string                  `json:"expected_task_id"`
}
type AccountImportForgetReceipt struct {
	OperationID string                   `json:"operation_id"`
	Context     cliadapter.ConfigContext `json:"context"`
	AccountID   string                   `json:"account_id"`
	RequestID   string                   `json:"request_id"`
	Digest      string                   `json:"digest"`
	TaskID      *string                  `json:"task_id"`
	Forgotten   bool                     `json:"forgotten"`
}
