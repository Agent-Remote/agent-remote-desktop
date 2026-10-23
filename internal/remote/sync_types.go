package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// SyncTarget binds a review to the selected directory and its CLI mapping.
type SyncTarget struct {
	LocalPath     string  `json:"local_path"`
	ProjectKey    string  `json:"project_key"`
	WorkspaceID   *string `json:"workspace_id"`
	SyncSessionID *string `json:"sync_session_id"`
	Fingerprint   *string `json:"fingerprint"`
}
type SyncDetails struct {
	ID               string   `json:"id"`
	WorkspaceID      string   `json:"workspace_id"`
	NodeID           *string  `json:"node_id"`
	LocalPath        string   `json:"local_path"`
	RemotePath       string   `json:"remote_path"`
	SyncMode         string   `json:"sync_mode"`
	SyncGit          bool     `json:"sync_git"`
	Exclude          []string `json:"exclude"`
	Status           string   `json:"status"`
	ConflictStatus   string   `json:"conflict_status"`
	MutagenSessionID *string  `json:"mutagen_session_id"`
	RemoteEndpoint   *string  `json:"remote_endpoint"`
}
type MutagenObservation struct {
	Paused        *bool   `json:"paused"`
	Status        *string `json:"status"`
	ConflictCount *uint64 `json:"conflict_count"`
	State         string  `json:"state"`
	Conflicts     string  `json:"conflicts"`
}
type SyncStatusInput struct {
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	LocalPath string                   `json:"local_path"`
}
type SyncStatus struct {
	Context cliadapter.ConfigContext `json:"context"`
	Target  SyncTarget               `json:"target"`
	Sync    *SyncDetails             `json:"sync"`
	Mutagen MutagenObservation       `json:"mutagen"`
}
type SyncActionInput struct {
	Confirmed      bool                     `json:"confirmed"`
	Expected       cliadapter.ConfigContext `json:"expected_context"`
	ExpectedTarget SyncTarget               `json:"expected_target"`
	DryRun         bool                     `json:"dry_run"`
}
type SyncStage struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// SyncReceipt preserves partial progress. Completed is not proof of no conflicts.
type SyncReceipt struct {
	OperationID  string                   `json:"operation_id"`
	Context      cliadapter.ConfigContext `json:"context"`
	Target       SyncTarget               `json:"target"`
	ResultTarget SyncTarget               `json:"result_target"`
	Method       string                   `json:"method"`
	DryRun       bool                     `json:"dry_run"`
	Outcome      string                   `json:"outcome"`
	Stages       []SyncStage              `json:"stages"`
	Sync         *SyncDetails             `json:"sync"`
	Mutagen      *MutagenObservation      `json:"mutagen"`
	ErrorCode    *string                  `json:"error_code"`
}
