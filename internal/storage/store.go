// Package storage persists desktop metadata separately from the CLI's state.
package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

type State struct {
	Projects    []workspace.Project   `json:"projects"`
	Preferences workspace.Preferences `json:"preferences"`
	Board       workspace.Board       `json:"board"`
	Sessions    []workspace.Session   `json:"sessions"`
	// Operations is a safe, desktop-owned projection of recent operation
	// lifecycles. It deliberately excludes CLI request keys, IDs, paths,
	// argv, receipts and credentials; the CLI remains the durable source for
	// exact recovery data.
	Operations []OperationRecord `json:"operations,omitempty"`
	// DedicatedOperations is the safe projection of reviewed terminal
	// workflows. It contains only a fixed CLI command label and lifecycle
	// status. The managed terminal's argv, output, session identity and
	// credentials remain ephemeral or owned by the CLI journal.
	DedicatedOperations []DedicatedOperationRecord `json:"dedicated_operations,omitempty"`
	// RemoteStopRecovery is the reviewed, content-free pointer needed to query
	// one exact stop request after a desktop restart. The Rust CLI remains the
	// durable owner of the receipt and execution journal; this record contains
	// only the caller key and the frozen public identity used to scope a query.
	RemoteStopRecovery *RemoteStopRecovery `json:"remote_stop_recovery,omitempty"`
	// RemoteCreateRecovery is the reviewed, content-free pointer needed to
	// observe one exact caller-key creation request after a desktop restart.
	// The Rust CLI remains the durable owner of the receipt and journal.
	RemoteCreateRecovery *RemoteCreateRecovery `json:"remote_create_recovery,omitempty"`
	// RemoteDeleteRecovery is the reviewed, content-free pointer needed to
	// query one exact fixed collection deletion after a desktop restart. The
	// CLI remains the durable owner of the receipt and execution journal.
	RemoteDeleteRecovery *RemoteDeleteRecovery `json:"remote_delete_recovery,omitempty"`
}

// RemoteStopRecovery is intentionally smaller than a CLI receipt. It permits
// the desktop to restore a query affordance without persisting process output,
// argv, credentials, or an inferred remote result.
type RemoteStopRecovery struct {
	RequestID      string    `json:"request_id"`
	Home           string    `json:"home"`
	ServerURL      string    `json:"server_url"`
	DeviceID       string    `json:"device_id"`
	SessionID      string    `json:"session_id"`
	NodeID         string    `json:"node_id"`
	RuntimeBackend string    `json:"runtime_backend"`
	StopTaskID     string    `json:"stop_task_id,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// RemoteCreateRecovery deliberately contains only the caller key and the
// frozen configuration scope required by sessions.create.request. It never
// stores argv, sync plans, credentials, or a copied CLI receipt.
type RemoteCreateRecovery struct {
	RequestID string    `json:"request_id"`
	Home      string    `json:"home"`
	ServerURL string    `json:"server_url"`
	DeviceID  string    `json:"device_id"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RemoteDeleteRecovery deliberately stores only the original caller key,
// authentication context, and the immutable public selection identity. It
// never stores a receipt, argv, credentials, task output, or mutable session
// observations. A restarted desktop can therefore query exactly this request
// without replaying it or expanding the reviewed collection.
type RemoteDeleteRecovery struct {
	RequestID         string    `json:"request_id"`
	Home              string    `json:"home"`
	ServerURL         string    `json:"server_url"`
	DeviceID          string    `json:"device_id"`
	ToolType          string    `json:"tool_type"`
	SessionIDs        []string  `json:"session_ids"`
	SelectionDigest   string    `json:"selection_digest"`
	IntentFingerprint string    `json:"intent_fingerprint"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// OperationRecord is the minimal cross-restart journal entry used by the
// operation center. It is intentionally independent from any CLI receipt.
type OperationRecord struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	// Stages is a bounded, content-free projection of a multi-stage operation.
	// Exact request keys, paths, arguments and receipts remain in the CLI journal.
	Stages    []OperationStage `json:"stages,omitempty"`
	UpdatedAt time.Time        `json:"updated_at"`
}

type OperationStage struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// DedicatedOperationRecord deliberately cannot represent a CLI receipt. It
// exists only so the operation center can restore a lifecycle summary after a
// desktop restart and ask the user to inspect the CLI journal explicitly.
type DedicatedOperationRecord struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}

var operationIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// Operation stages are a deliberately small, non-sensitive projection of a
// CLI receipt. They let the operation center explain where an interrupted
// workflow stopped after a desktop restart without persisting the receipt,
// request key, paths, or user supplied content.
var remoteStopRequestIDPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
var remoteCreateRequestIDPattern = remoteStopRequestIDPattern
var remoteDeleteFingerprintPattern = regexp.MustCompile(`^sha256:[0-9a-fA-F]{64}$`)
var remoteStopTokenPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)

// Command labels are generated from a reviewed schema path and never include
// user arguments. Keep the executable and path-token shape constrained at the
// storage boundary so generic schema commands can retain a safe lifecycle
// summary without persisting arbitrary argv or user content. The CLI schema
// validator separately rejects whitespace and NUL in path components.
var dedicatedCommandPattern = regexp.MustCompile(`^(agent-remote|fclaude)( [A-Za-z0-9][A-Za-z0-9_-]*){0,8}$`)

func (r OperationRecord) Valid() bool {
	if !operationIDPattern.MatchString(r.ID) {
		return false
	}
	if !r.UpdatedAt.IsZero() {
		switch r.Status {
		case "running", "completed", "failed", "attention":
		default:
			return false
		}
	} else {
		return false
	}
	if len(r.Stages) > 8 {
		return false
	}
	for _, stage := range r.Stages {
		if !validOperationStage(stage) {
			return false
		}
	}
	return true
}

func validOperationStage(stage OperationStage) bool {
	if stage.Name != "workspace" && stage.Name != "server" && stage.Name != "mutagen" && stage.Name != "persist" && stage.Name != "stop" && stage.Name != "start" && stage.Name != "verify" {
		return false
	}
	switch stage.State {
	case "completed", "skipped", "failed", "unknown":
		return true
	default:
		return false
	}
}

func (r DedicatedOperationRecord) Valid() bool {
	if !operationIDPattern.MatchString(r.ID) || !dedicatedCommandPattern.MatchString(r.Command) {
		return false
	}
	switch r.Status {
	case "pending", "running", "completed", "failed", "attention":
		return !r.UpdatedAt.IsZero()
	default:
		return false
	}
}

func (r RemoteStopRecovery) Valid() bool {
	if !remoteStopRequestIDPattern.MatchString(r.RequestID) || !validRemoteStopPath(r.Home) ||
		!validRemoteStopURL(r.ServerURL) || !remoteStopTokenPattern.MatchString(r.DeviceID) ||
		!remoteStopTokenPattern.MatchString(r.SessionID) || !remoteStopTokenPattern.MatchString(r.NodeID) ||
		!remoteStopTokenPattern.MatchString(r.RuntimeBackend) || (r.StopTaskID != "" && !remoteStopTokenPattern.MatchString(r.StopTaskID)) || r.UpdatedAt.IsZero() {
		return false
	}
	return true
}

func (r RemoteCreateRecovery) Valid() bool {
	return remoteCreateRequestIDPattern.MatchString(r.RequestID) && validRemoteStopPath(r.Home) &&
		validRemoteStopURL(r.ServerURL) && remoteStopTokenPattern.MatchString(r.DeviceID) && !r.UpdatedAt.IsZero()
}

func (r RemoteDeleteRecovery) Valid() bool {
	if !remoteStopRequestIDPattern.MatchString(r.RequestID) || !validRemoteStopPath(r.Home) ||
		!validRemoteStopURL(r.ServerURL) || !remoteStopTokenPattern.MatchString(r.DeviceID) ||
		r.ToolType != "claude" || len(r.SessionIDs) == 0 || len(r.SessionIDs) > 256 ||
		!remoteDeleteFingerprintPattern.MatchString(r.SelectionDigest) ||
		!remoteDeleteFingerprintPattern.MatchString(r.IntentFingerprint) || r.UpdatedAt.IsZero() {
		return false
	}
	for i, id := range r.SessionIDs {
		if !remoteStopTokenPattern.MatchString(id) || (i > 0 && r.SessionIDs[i-1] >= id) {
			return false
		}
	}
	return true
}

func validRemoteStopPath(value string) bool {
	return filepath.IsAbs(value) && value != "" && !strings.ContainsAny(value, "\x00\r\n")
}

func validRemoteStopURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Hostname() != "" && parsed.User == nil && parsed.RawQuery == "" && !parsed.ForceQuery && parsed.Fragment == "" && len(value) <= 4096 && !strings.ContainsAny(value, "\x00\r\n\t ")
}

func Open(home string) (*Store, error) {
	dir := filepath.Join(home, "desktop")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "desktop.sqlite3")
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err = db.Exec(`PRAGMA busy_timeout=5000; CREATE TABLE IF NOT EXISTS desktop_state (id INTEGER PRIMARY KEY CHECK(id=1), version INTEGER NOT NULL, value TEXT NOT NULL);`); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

func (s *Store) Load() (State, error) {
	state := State{Preferences: workspace.Defaults()}
	var version int
	var raw string
	err := s.db.QueryRow("SELECT version,value FROM desktop_state WHERE id=1").Scan(&version, &raw)
	if err == sql.ErrNoRows {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	if version != 1 && version != 2 && version != 3 {
		return state, fmt.Errorf("%w: %d", fault.StorageVersion, version)
	}
	err = json.Unmarshal([]byte(raw), &state)
	if err != nil {
		return state, fmt.Errorf("%w: %v", fault.WorkspaceData, err)
	}
	if err := state.Validate(); err != nil {
		return state, err
	}
	state.restoreSelection()
	if state.Preferences.Theme == "" {
		state.Preferences.Theme = workspace.Defaults().Theme
	}
	if state.Preferences.FontSize < 10 || state.Preferences.FontSize > 32 {
		state.Preferences.FontSize = 14
	}
	if state.Preferences.SidebarWidth < 180 || state.Preferences.SidebarWidth > 400 {
		state.Preferences.SidebarWidth = 240
	}
	return state, err
}

func (s *Store) Save(state State) error {
	if err := state.Validate(); err != nil {
		return err
	}
	value, err := json.Marshal(state)
	if err != nil {
		return err
	}
	result, err := s.db.Exec("INSERT INTO desktop_state(id,version,value) VALUES(1,3,?) ON CONFLICT(id) DO UPDATE SET version=excluded.version,value=excluded.value WHERE desktop_state.version IN (1,2,3)", string(value))
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fault.StorageVersion
	}
	return nil
}

func (s *Store) Close() error { return s.db.Close() }
