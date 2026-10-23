package remote

// MutagenEnvironmentBinding is the recorded SSH binding, not a claim about the
// environment inherited by an independently started daemon.
type MutagenEnvironmentBinding struct {
	Home         string `json:"home"`
	SSHDirectory string `json:"ssh_directory"`
	Fingerprint  string `json:"fingerprint"`
}
type MutagenEnvironmentTool struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Fingerprint string `json:"fingerprint"`
}
type MutagenEnvironmentEndpoint struct {
	Protocol string `json:"protocol"`
	User     string `json:"user"`
	Host     string `json:"host"`
	Path     string `json:"path"`
	Port     uint32 `json:"port"`
}
type MutagenEnvironmentSession struct {
	Kind                     string                     `json:"kind"`
	Identifier               string                     `json:"identifier"`
	Name                     string                     `json:"name"`
	Paused                   bool                       `json:"paused"`
	Source                   MutagenEnvironmentEndpoint `json:"source"`
	Destination              MutagenEnvironmentEndpoint `json:"destination"`
	ConfigurationFingerprint string                     `json:"configuration_fingerprint"`
}
type MutagenEnvironmentPlan struct {
	Home                 string                      `json:"home"`
	DataDirectory        string                      `json:"data_directory"`
	NamespaceFingerprint *string                     `json:"namespace_fingerprint"`
	DaemonState          string                      `json:"daemon_state"`
	RunningFingerprint   *string                     `json:"running_fingerprint"`
	Registration         string                      `json:"registration"`
	Previous             *MutagenEnvironmentBinding  `json:"previous"`
	Desired              MutagenEnvironmentBinding   `json:"desired"`
	Tools                []MutagenEnvironmentTool    `json:"tools"`
	Sessions             []MutagenEnvironmentSession `json:"sessions"`
	InventoryFingerprint string                      `json:"inventory_fingerprint"`
	MarkerFingerprint    *string                     `json:"marker_fingerprint"`
	Fingerprint          string                      `json:"fingerprint"`
	Repairable           bool                        `json:"repairable"`
	ErrorCode            *string                     `json:"error_code"`
}
type MutagenEnvironmentApplyInput struct {
	Confirmed    bool
	ExpectedHome string
	ExpectedPlan MutagenEnvironmentPlan
}
type MutagenEnvironmentReceipt struct {
	OperationID string                  `json:"operation_id"`
	Home        string                  `json:"home"`
	Method      string                  `json:"method"`
	Plan        MutagenEnvironmentPlan  `json:"plan"`
	Outcome     string                  `json:"outcome"`
	Stages      []SyncStage             `json:"stages"`
	Environment *MutagenEnvironmentPlan `json:"environment"`
	ErrorCode   *string                 `json:"error_code"`
}
