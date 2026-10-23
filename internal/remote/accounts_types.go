package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

type AccountCreateInput struct {
	Confirmed bool                     `json:"confirmed"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	Tool      string                   `json:"tool"`
	Name      string                   `json:"name"`
	Region    string                   `json:"region"`
	Timezone  string                   `json:"timezone"`
	Locale    string                   `json:"locale"`
	Tags      []string                 `json:"tags"`
}
type AccountStatusInput struct {
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	AccountID string                   `json:"account_id"`
}
type AccountActionInput struct {
	Confirmed bool                     `json:"confirmed"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
	AccountID string                   `json:"account_id"`
}

// AccountDetails contains the CLI's safe projection, with no credential data.
type AccountDetails struct {
	ID             string   `json:"id"`
	Tool           string   `json:"tool_type"`
	Name           string   `json:"display_name"`
	Status         string   `json:"status"`
	Region         string   `json:"region_code"`
	Timezone       string   `json:"timezone"`
	Locale         string   `json:"locale"`
	Tags           []string `json:"preferred_node_tags"`
	AffinityNodeID *string  `json:"affinity_node_id"`
	RuntimeBackend *string  `json:"runtime_backend"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

// AccountBinding deliberately has no server command, verifier or raw error.
type AccountBinding struct {
	AccountID        string  `json:"tool_account_id"`
	Status           string  `json:"status"`
	NodeID           *string `json:"node_id"`
	BindingSessionID *string `json:"binding_session_id"`
	TmuxSessionName  *string `json:"tmux_session_name"`
	RemotePath       *string `json:"account_remote_path"`
	TaskID           *string `json:"task_id"`
	ErrorCode        *string `json:"error_code"`
}
type AccountStatus struct {
	Context cliadapter.ConfigContext `json:"context"`
	Account AccountDetails           `json:"account"`
	Binding AccountBinding           `json:"binding"`
}

// Accepted confirms the HTTP request was accepted, not task completion or login.
type AccountReceipt struct {
	OperationID string                   `json:"operation_id"`
	Context     cliadapter.ConfigContext `json:"context"`
	Accepted    bool                     `json:"accepted"`
	Account     *AccountDetails          `json:"account"`
	Binding     *AccountBinding          `json:"binding"`
}
