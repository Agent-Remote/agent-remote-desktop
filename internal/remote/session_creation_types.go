package remote

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// SessionCreateInput is the public, content-free projection accepted by the
// desktop control protocol. The CLI owns caller-key generation and its
// durable journal; the desktop intentionally has no request_id field here.
// argv, environment and prompts never cross this boundary.
type SessionCreateInput struct {
	SchemaVersion int                      `json:"schema_version"`
	Expected      cliadapter.ConfigContext `json:"expected_context"`
	ToolType      string                   `json:"tool_type"`
	AccountID     string                   `json:"tool_account_id"`
	WorkspaceID   string                   `json:"workspace_id"`
	ProjectKey    string                   `json:"project_key"`
	ArgvDigest    string                   `json:"argv_digest"`
	Replacement   *SessionReplacement      `json:"replacement,omitempty"`
	SyncTarget    json.RawMessage          `json:"sync_target"`
	Confirmed     bool                     `json:"confirmed"`
}

// DecodeSessionCreateInput is used by protocol fixtures and future transport
// adapters. Unknown fields are rejected so a caller cannot smuggle raw user,
// credential or command data into the reviewed request.
func DecodeSessionCreateInput(raw []byte) (SessionCreateInput, error) {
	var input SessionCreateInput
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return SessionCreateInput{}, fault.SessionCreateInputInvalid
	}
	if decoder.Decode(new(any)) != io.EOF {
		return SessionCreateInput{}, fault.SessionCreateInputInvalid
	}
	return input, nil
}

type SessionReplacement struct {
	SessionID   string `json:"session_id"`
	AccountID   string `json:"tool_account_id"`
	WorkspaceID string `json:"workspace_id"`
	ProjectKey  string `json:"project_key"`
	NodeID      string `json:"node_id"`
	Backend     string `json:"runtime_backend"`
}

// SessionCreateRequestInput is deliberately the only way to query a caller
// key. It never submits a new create request.
type SessionCreateRequestInput struct {
	RequestID string                   `json:"request_id"`
	Expected  cliadapter.ConfigContext `json:"expected_context"`
}

type SessionCreateReceipt struct {
	SchemaVersion int                       `json:"schema_version"`
	RequestID     string                    `json:"request_id"`
	Phase         string                    `json:"phase"`
	Context       cliadapter.ConfigContext  `json:"context"`
	ToolType      string                    `json:"tool_type"`
	AccountID     string                    `json:"tool_account_id"`
	WorkspaceID   string                    `json:"workspace_id"`
	ProjectKey    string                    `json:"project_key"`
	ArgvDigest    string                    `json:"argv_digest"`
	NodeID        string                    `json:"node_id"`
	Backend       string                    `json:"runtime_backend"`
	SessionID     *string                   `json:"session_id"`
	TaskID        *string                   `json:"task_id"`
	TaskRecordID  *string                   `json:"task_record_id"`
	ErrorCode     *string                   `json:"error_code"`
	Session       *cliadapter.RemoteSession `json:"session"`
}

var sessionCreatePhases = map[string]bool{
	"prepared": true, "admission_pending": true, "accepted": true,
	"running": true, "failed": true, "unknown": true, "deleted": true,
}

func (r SessionCreateReceipt) Valid() bool {
	if r.SchemaVersion != 1 || !validAccountImportRequestID(r.RequestID) || !sessionCreatePhases[r.Phase] {
		return false
	}
	if r.SessionID != nil && *r.SessionID == "" {
		return false
	}
	if r.SessionID != nil && !validCanonicalUUID(*r.SessionID) {
		return false
	}
	if r.NodeID != "" && !validCanonicalUUID(r.NodeID) {
		return false
	}
	if r.AccountID != "" && !validCanonicalUUID(r.AccountID) {
		return false
	}
	if r.WorkspaceID != "" && !validCanonicalUUID(r.WorkspaceID) {
		return false
	}
	if r.TaskID != nil && *r.TaskID == "" {
		return false
	}
	if r.TaskRecordID != nil && *r.TaskRecordID == "" {
		return false
	}
	if r.ErrorCode != nil && (strings.TrimSpace(*r.ErrorCode) == "" || strings.ContainsAny(*r.ErrorCode, " \t\r\n")) {
		return false
	}
	if r.Session != nil && (r.Session.ID == "" || r.Session.ID != stringValue(r.SessionID)) {
		return false
	}
	return true
}

func stringValue(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func validateSessionCreateInput(input SessionCreateInput, home string) error {
	if input.SchemaVersion != 1 || !input.Confirmed || !validDefaultContext(input.Expected) || input.Expected.Home != home || input.Expected.ServerURL == nil || input.Expected.DeviceID == nil {
		return fault.SessionCreateInputInvalid
	}
	if !validDefaultTool(input.ToolType) || !validCanonicalUUID(input.AccountID) || !validCanonicalUUID(input.WorkspaceID) || !validDefaultTool(input.ProjectKey) || !validSyncFingerprint(input.ArgvDigest) || !validSessionCreateSyncTarget(input.SyncTarget) {
		return fault.SessionCreateInputInvalid
	}
	if input.Replacement != nil && (!validCanonicalUUID(input.Replacement.SessionID) || !validCanonicalUUID(input.Replacement.AccountID) || !validCanonicalUUID(input.Replacement.WorkspaceID) || !validDefaultTool(input.Replacement.ProjectKey) || !validCanonicalUUID(input.Replacement.NodeID) || !validDefaultTool(input.Replacement.Backend)) {
		return fault.SessionCreateInputInvalid
	}
	return nil
}

func validSessionCreateSyncTarget(raw json.RawMessage) bool {
	var fields map[string]json.RawMessage
	if len(raw) == 0 || uniqueDefaultJSON(raw) != nil || json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return false
	}
	for key, value := range fields {
		switch key {
		case "id", "workspace_id", "node_id":
			// The server schema uses UUIDs for the sync and workspace
			// identities. Decode as a string first so numbers/objects cannot
			// be smuggled through a permissive JSON projection.
			var id string
			if json.Unmarshal(value, &id) != nil || !validCanonicalUUID(id) {
				return false
			}
		case "local_path":
			var path string
			if json.Unmarshal(value, &path) != nil || !validWireGuardPath(path) {
				return false
			}
		case "remote_path":
			var path string
			if json.Unmarshal(value, &path) != nil || !validAccountText(path, 32768) {
				return false
			}
		case "mutagen_session_id":
			var id string
			if json.Unmarshal(value, &id) != nil || !validAccountText(id, 512) {
				return false
			}
		case "target_digest":
			var digest string
			if json.Unmarshal(value, &digest) != nil || !validSyncFingerprint(digest) {
				return false
			}
		default:
			return false
		}
	}
	for _, key := range []string{"id", "workspace_id", "local_path", "remote_path"} {
		if _, ok := fields[key]; !ok {
			return false
		}
	}
	return true
}

// SessionCreateSyncTargetValid exposes the same strict, content-free target
// contract used by CreateSession so the Go UI can reject malformed JSON before
// launching the CLI. The Rust CLI and Server remain authoritative; this is a
// local fail-closed guard against type confusion, path control characters and
// incomplete sync identity.
func SessionCreateSyncTargetValid(raw []byte) bool {
	return validSessionCreateSyncTarget(json.RawMessage(raw))
}
