package remote

import (
	"encoding/hex"
	"encoding/json"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"reflect"
	"slices"
	"strings"
)

func validSyncMethod(s string) bool {
	return slices.Contains([]string{"sync.ensure", "sync.pause", "sync.resume", "sync.resolve", "sync.reset"}, s)
}
func validSyncTarget(t SyncTarget) bool {
	if !validWireGuardPath(t.LocalPath) || !validAccountText(t.ProjectKey, 8192) || !validDefaultID(t.WorkspaceID) || !validDefaultID(t.SyncSessionID) {
		return false
	}
	if t.SyncSessionID == nil {
		return t.Fingerprint == nil
	}
	if t.WorkspaceID == nil || t.Fingerprint == nil {
		return false
	}
	digest := strings.TrimPrefix(*t.Fingerprint, "sha256:")
	_, err := hex.DecodeString(digest)
	return len(digest) == 64 && *t.Fingerprint == "sha256:"+strings.ToLower(digest) && err == nil
}
func decodeSyncTarget(raw json.RawMessage) (SyncTarget, error) {
	var t SyncTarget
	_, err := decodeAccountObject(raw, &t, []string{"local_path", "project_key", "workspace_id", "sync_session_id", "fingerprint"}, []string{"workspace_id", "sync_session_id", "fingerprint"})
	if err != nil || !validSyncTarget(t) {
		return t, fault.CLIProtocol
	}
	return t, nil
}
func decodeMutagen(raw json.RawMessage) (MutagenObservation, error) {
	var m MutagenObservation
	_, err := decodeAccountObject(raw, &m, []string{"state", "conflicts", "paused", "status", "conflict_count"}, []string{"paused", "status", "conflict_count"})
	if err != nil || !slices.Contains([]string{"missing", "session_missing", "available", "unavailable"}, m.State) || !slices.Contains([]string{"unknown", "detected", "none_observed"}, m.Conflicts) {
		return m, fault.CLIProtocol
	}
	if m.State != "available" && (m.Conflicts != "unknown" || m.Paused != nil || m.Status != nil || m.ConflictCount != nil) {
		return m, fault.CLIProtocol
	}
	if m.State == "available" && (m.Paused == nil || m.Status == nil) {
		return m, fault.CLIProtocol
	}
	if m.Conflicts == "none_observed" && (m.ConflictCount == nil || *m.ConflictCount != 0) {
		return m, fault.CLIProtocol
	}
	if m.Conflicts == "unknown" && m.ConflictCount != nil {
		return m, fault.CLIProtocol
	}
	if m.Conflicts == "detected" && (m.ConflictCount == nil || *m.ConflictCount == 0) {
		return m, fault.CLIProtocol
	}
	if m.Paused != nil && *m.Paused && (m.Conflicts != "unknown" || m.ConflictCount != nil) {
		return m, fault.CLIProtocol
	}
	if m.Status != nil && !slices.Contains([]string{"watching", "scanning", "disconnected", "connecting-alpha", "connecting-beta", "reconciling", "staging-alpha", "staging-beta", "transitioning", "saving", "waiting-for-rescan", "halted-on-root-deletion", "halted-on-root-emptied", "halted-on-root-type-change", "unknown"}, *m.Status) {
		return m, fault.CLIProtocol
	}
	return m, nil
}
func decodeSyncDetails(raw json.RawMessage) (*SyncDetails, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var s SyncDetails
	_, err := decodeAccountObject(raw, &s, []string{"id", "workspace_id", "node_id", "local_path", "remote_path", "status", "conflict_status", "mutagen_session_id", "remote_endpoint", "sync_mode", "sync_git", "exclude"}, []string{"node_id", "mutagen_session_id", "remote_endpoint"})
	if err != nil || !validDefaultTool(s.ID) || !validDefaultTool(s.WorkspaceID) || !validDefaultID(s.NodeID) || !validWireGuardPath(s.LocalPath) || !validAccountText(s.RemotePath, 8192) || !validAccountText(s.Status, 128) || !validAccountText(s.ConflictStatus, 128) || !validAccountOptional(s.MutagenSessionID, 512) || !validAccountOptional(s.RemoteEndpoint, 8192) || !validAccountText(s.SyncMode, 128) || s.Exclude == nil || len(s.Exclude) > 256 {
		return nil, fault.CLIProtocol
	}
	for _, pattern := range s.Exclude {
		if !validAccountText(pattern, 4096) {
			return nil, fault.CLIProtocol
		}
	}
	return &s, nil
}
func matchesSyncTarget(t SyncTarget, s *SyncDetails) bool {
	if s == nil {
		return t.SyncSessionID == nil
	}
	return t.SyncSessionID != nil && *t.SyncSessionID == s.ID && t.WorkspaceID != nil && *t.WorkspaceID == s.WorkspaceID && t.LocalPath == s.LocalPath
}
func decodeSyncStatus(raw json.RawMessage) (SyncStatus, error) {
	var s SyncStatus
	f, err := decodeAccountObject(raw, &s, []string{"context", "target", "sync", "mutagen"}, []string{"sync"})
	if err != nil {
		return s, err
	}
	if s.Context, err = decodeAccountContext(f["context"]); err != nil {
		return s, err
	}
	if s.Target, err = decodeSyncTarget(f["target"]); err != nil {
		return s, err
	}
	if s.Sync, err = decodeSyncDetails(f["sync"]); err != nil {
		return s, err
	}
	if s.Mutagen, err = decodeMutagen(f["mutagen"]); err != nil {
		return s, err
	}
	if !matchesSyncTarget(s.Target, s.Sync) {
		return s, fault.CLIProtocol
	}
	return s, nil
}
func decodeSyncReceipt(raw json.RawMessage) (SyncReceipt, error) {
	var r SyncReceipt
	f, err := decodeAccountObject(raw, &r, []string{"operation_id", "context", "target", "result_target", "method", "dry_run", "outcome", "stages", "sync", "mutagen", "error_code"}, []string{"sync", "mutagen", "error_code"})
	if err != nil {
		return r, err
	}
	if r.Context, err = decodeAccountContext(f["context"]); err != nil {
		return r, err
	}
	if r.Target, err = decodeSyncTarget(f["target"]); err != nil {
		return r, err
	}
	if r.ResultTarget, err = decodeSyncTarget(f["result_target"]); err != nil {
		return r, err
	}
	if r.Sync, err = decodeSyncDetails(f["sync"]); err != nil {
		return r, err
	}
	if r.Mutagen != nil {
		m, err := decodeMutagen(f["mutagen"])
		if err != nil {
			return r, err
		}
		r.Mutagen = &m
	}
	if !validDefaultTool(r.OperationID) || !validSyncMethod(r.Method) || !slices.Contains([]string{"completed", "partial", "unknown"}, r.Outcome) || r.Stages == nil || len(r.Stages) > 32 || r.ResultTarget.LocalPath != r.Target.LocalPath || r.ResultTarget.ProjectKey != r.Target.ProjectKey {
		return r, fault.CLIProtocol
	}
	var stages []json.RawMessage
	if json.Unmarshal(f["stages"], &stages) != nil {
		return r, fault.CLIProtocol
	}
	for _, rawStage := range stages {
		var stage SyncStage
		if _, err := decodeAccountObject(rawStage, &stage, []string{"name", "state"}, nil); err != nil || !slices.Contains([]string{"workspace", "server", "mutagen", "persist"}, stage.Name) || !slices.Contains([]string{"completed", "skipped", "failed", "unknown"}, stage.State) {
			return r, fault.CLIProtocol
		}
		if r.Outcome == "completed" && (stage.State == "failed" || stage.State == "unknown") {
			return r, fault.CLIProtocol
		}
	}
	if (r.Outcome == "completed") != (r.ErrorCode == nil) {
		return r, fault.CLIProtocol
	}
	if r.ErrorCode != nil && !slices.Contains([]string{"sync_action_invalid", "sync_conflicts_remaining", "sync_conflicts_unknown", "sync_git_check_failed", "sync_git_check_limit", "sync_git_locked", "sync_mutagen_ambiguous", "sync_mutagen_environment_invalid", "sync_mutagen_environment_mismatch", "sync_mutagen_missing", "sync_mutagen_not_running", "sync_mutagen_output_limit", "sync_mutagen_proxy_missing", "sync_mutagen_response_invalid", "sync_mutagen_target_changed", "sync_mutagen_timeout", "sync_mutagen_unavailable", "sync_mutagen_unverified", "sync_target_invalid", "invalid_params", "confirmation_required", "configuration_changed", "configuration_unavailable", "authentication_required", "credential_unavailable", "permission_denied", "rate_limited", "server_unavailable", "sync_not_found", "sync_target_changed", "sync_response_invalid", "sync_rejected", "sync_outcome_unknown", "sync_local_failed", "sync_prepare_failed", "sync_prepare_timeout", "state_unavailable", "mutagen_unavailable", "canceled"}, *r.ErrorCode) {
		return r, fault.CLIProtocol
	}
	if r.Method != "sync.ensure" && !reflect.DeepEqual(r.Target, r.ResultTarget) {
		return r, fault.CLIProtocol
	}
	if r.Target.WorkspaceID != nil && !sameSyncPointer(r.Target.WorkspaceID, r.ResultTarget.WorkspaceID) {
		return r, fault.CLIProtocol
	}
	if r.Target.SyncSessionID != nil && (!sameSyncPointer(r.Target.SyncSessionID, r.ResultTarget.SyncSessionID) || !sameSyncPointer(r.Target.Fingerprint, r.ResultTarget.Fingerprint)) {
		return r, fault.CLIProtocol
	}
	if r.Outcome == "completed" {
		if r.Sync == nil {
			return r, fault.CLIProtocol
		}
		required := map[string]bool{"server": false, "persist": false, "mutagen": false}
		if r.Method == "sync.ensure" {
			required["workspace"] = false
		}
		for _, stage := range r.Stages {
			if stage.Name == "mutagen" {
				if (r.DryRun && stage.State != "skipped") || (!r.DryRun && stage.State != "completed") {
					return r, fault.CLIProtocol
				}
			} else if stage.State != "completed" {
				return r, fault.CLIProtocol
			}
			required[stage.Name] = true
		}
		for _, seen := range required {
			if !seen {
				return r, fault.CLIProtocol
			}
		}
	}
	if r.Sync != nil && !matchesSyncTarget(r.ResultTarget, r.Sync) {
		return r, fault.CLIProtocol
	}
	return r, nil
}

func sameSyncPointer(a, b *string) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
