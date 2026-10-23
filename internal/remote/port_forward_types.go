package remote

import "github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"

// PortForward is the safe projection exposed by the Rust desktop-control
// adapter. Connection tokens and SSH details never cross into the desktop UI.
type PortForward struct {
	ID                 string  `json:"id"`
	SessionID          string  `json:"session_id"`
	NodeID             string  `json:"node_id"`
	RemotePort         uint16  `json:"remote_port"`
	RequestedLocalPort uint16  `json:"requested_local_port"`
	ClientInstanceID   string  `json:"client_instance_id"`
	CallerRequestID    *string `json:"caller_request_id,omitempty"`
	Status             string  `json:"status"`
	BytesUp            uint64  `json:"bytes_up"`
	BytesDown          uint64  `json:"bytes_down"`
	ConnectionCount    uint64  `json:"connection_count"`
	LastConnectedAt    *string `json:"last_connected_at,omitempty"`
	LeaseExpiresAt     *string `json:"lease_expires_at,omitempty"`
	ExpiresAt          string  `json:"expires_at"`
	StoppedAt          *string `json:"stopped_at,omitempty"`
	StopReason         *string `json:"stop_reason,omitempty"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
}

type PortForwardList struct {
	Context cliadapter.ConfigContext `json:"context"`
	Items   []PortForward            `json:"items"`
}

type PortForwardListInput struct {
	Expected cliadapter.ConfigContext `json:"expected_context"`
}

type PortForwardCreateInput struct {
	SchemaVersion          int                      `json:"schema_version"`
	Expected               cliadapter.ConfigContext `json:"expected_context"`
	SessionID              string                   `json:"session_id"`
	ExpectedNodeID         string                   `json:"expected_node_id"`
	ExpectedRuntimeBackend string                   `json:"expected_runtime_backend"`
	RemotePort             uint16                   `json:"remote_port"`
	LocalPort              uint16                   `json:"local_port"`
	ClientInstanceID       string                   `json:"client_instance_id"`
	CallerRequestID        string                   `json:"caller_request_id"`
	TTLSeconds             *uint64                  `json:"ttl_seconds,omitempty"`
	Confirmed              bool                     `json:"confirmed"`
}

type PortForwardActionInput struct {
	SchemaVersion     int                      `json:"schema_version"`
	Expected          cliadapter.ConfigContext `json:"expected_context"`
	ForwardID         string                   `json:"forward_id"`
	ExpectedSessionID string                   `json:"expected_session_id"`
	ExpectedNodeID    string                   `json:"expected_node_id"`
	Confirmed         bool                     `json:"confirmed"`
}

type PortForwardReceipt struct {
	SchemaVersion   int                      `json:"schema_version"`
	RequestID       string                   `json:"request_id,omitempty"`
	Context         cliadapter.ConfigContext `json:"context"`
	Forward         PortForward              `json:"forward"`
	NodeWireguardIP string                   `json:"node_wireguard_ip,omitempty"`
	SSHUser         string                   `json:"ssh_user,omitempty"`
	SSHPort         uint16                   `json:"ssh_port,omitempty"`
}
