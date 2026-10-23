package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// SyncRecoveryMapping is the CLI-owned local record reviewed before recovery.
// Its fingerprint covers the entire record, including fields not in SyncTarget.
type SyncRecoveryMapping struct {
	Workspace SyncRecoveryWorkspace `json:"workspace"`
	Sync      *SyncRecoverySession  `json:"sync"`
}
type SyncRecoveryWorkspace struct {
	ID          string  `json:"id"`
	ServerURL   string  `json:"server_url"`
	ProjectKey  string  `json:"project_key"`
	LocalPath   string  `json:"local_path"`
	DisplayName string  `json:"display_name"`
	RemotePath  *string `json:"remote_path"`
}
type SyncRecoverySession struct {
	ID               string  `json:"id"`
	ServerURL        string  `json:"server_url"`
	WorkspaceID      string  `json:"workspace_id"`
	NodeID           *string `json:"node_id"`
	Status           string  `json:"status"`
	ConflictStatus   string  `json:"conflict_status"`
	MutagenSessionID *string `json:"mutagen_session_id"`
	RemoteEndpoint   *string `json:"remote_endpoint"`
}
type SyncRecoveryPreviewInput struct {
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	LocalPath string                   `json:"local_path"`
}

type SyncRecoveryMutagenTarget struct {
	Identifier               string `json:"identifier"`
	Name                     string `json:"name"`
	LocalPath                string `json:"local_path"`
	RemotePath               string `json:"remote_path"`
	Endpoint                 string `json:"endpoint"`
	ConfigurationFingerprint string `json:"configuration_fingerprint"`
}
type SyncRecoveryMutagen struct {
	State                string                     `json:"state"`
	Recoverable          bool                       `json:"recoverable"`
	ErrorCode            *string                    `json:"error_code"`
	NamespaceFingerprint *string                    `json:"namespace_fingerprint"`
	Target               *SyncRecoveryMutagenTarget `json:"target"`
}
type SyncRecoveryPlan struct {
	Context            cliadapter.ConfigContext `json:"context"`
	Target             SyncTarget               `json:"target"`
	Mapping            SyncRecoveryMapping      `json:"mapping"`
	MappingFingerprint string                   `json:"mapping_fingerprint"`
	ServerState        string                   `json:"server_state"`
	Mutagen            SyncRecoveryMutagen      `json:"mutagen"`
	Recoverable        bool                     `json:"recoverable"`
	ErrorCode          *string                  `json:"error_code"`
}
type SyncRecoveryApplyInput struct {
	Confirmed    bool                     `json:"confirmed"`
	Expected     cliadapter.ConfigContext `json:"expected_context"`
	ExpectedPlan SyncRecoveryPlan         `json:"expected_plan"`
}
type SyncRecoveryReceipt struct {
	OperationID  string                   `json:"operation_id"`
	Context      cliadapter.ConfigContext `json:"context"`
	Method       string                   `json:"method"`
	Plan         SyncRecoveryPlan         `json:"plan"`
	ResultTarget SyncTarget               `json:"result_target"`
	Outcome      string                   `json:"outcome"`
	Stages       []SyncStage              `json:"stages"`
	Mutagen      *SyncRecoveryMutagen     `json:"mutagen"`
	ErrorCode    *string                  `json:"error_code"`
}
