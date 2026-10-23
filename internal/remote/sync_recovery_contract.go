package remote

import (
	"encoding/hex"
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// Recovery targets intentionally lack a remote fingerprint: the Server resource
// no longer exists. Normal synchronization targets keep their stricter rule.
func decodeSyncRecoveryTarget(raw json.RawMessage) (SyncTarget, error) {
	var target SyncTarget
	_, err := decodeAccountObject(raw, &target, []string{"local_path", "project_key", "workspace_id", "sync_session_id", "fingerprint"}, []string{"workspace_id", "sync_session_id", "fingerprint"})
	if err != nil || !validWireGuardPath(target.LocalPath) || !validAccountText(target.ProjectKey, 8192) || !validDefaultID(target.WorkspaceID) || !validDefaultID(target.SyncSessionID) || target.Fingerprint != nil || target.SyncSessionID != nil && target.WorkspaceID == nil {
		return target, fault.CLIProtocol
	}
	return target, nil
}

func decodeSyncRecoveryMapping(raw json.RawMessage) (SyncRecoveryMapping, error) {
	var mapping SyncRecoveryMapping
	fields, err := decodeAccountObject(raw, &mapping, []string{"workspace", "sync"}, []string{"sync"})
	if err != nil {
		return mapping, err
	}
	w := &mapping.Workspace
	if _, err := decodeAccountObject(fields["workspace"], w, []string{"id", "server_url", "project_key", "local_path", "display_name", "remote_path"}, []string{"remote_path"}); err != nil || !validDefaultTool(w.ID) || !validAccountText(w.ServerURL, 8192) || !validAccountText(w.ProjectKey, 8192) || !validWireGuardPath(w.LocalPath) || !validSyncRecoveryText(w.DisplayName, 4096) || w.RemotePath != nil && !validSyncRecoveryText(*w.RemotePath, 32768) {
		return mapping, fault.CLIProtocol
	}
	if mapping.Sync != nil {
		s := mapping.Sync
		if _, err := decodeAccountObject(fields["sync"], s, []string{"id", "server_url", "workspace_id", "node_id", "status", "conflict_status", "mutagen_session_id", "remote_endpoint"}, []string{"node_id", "mutagen_session_id", "remote_endpoint"}); err != nil || !validDefaultTool(s.ID) || s.ServerURL != w.ServerURL || s.WorkspaceID != w.ID || !validDefaultID(s.NodeID) || !validAccountText(s.Status, 128) || !validAccountText(s.ConflictStatus, 128) || !validAccountOptional(s.MutagenSessionID, 512) || !validAccountOptional(s.RemoteEndpoint, 8192) {
			return mapping, fault.CLIProtocol
		}
	}
	return mapping, nil
}

func validSyncFingerprint(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") || value != strings.ToLower(value) {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil
}
func validSyncRecoveryError(code string) bool {
	return slices.Contains([]string{"sync_recovery_not_required", "sync_recovery_blocked", "sync_recovery_metadata_incomplete", "sync_target_invalid", "sync_target_changed", "sync_mutagen_missing", "sync_mutagen_unavailable", "sync_mutagen_not_running", "sync_mutagen_environment_invalid", "sync_mutagen_environment_mismatch", "sync_mutagen_response_invalid", "sync_mutagen_output_limit", "sync_mutagen_timeout", "sync_mutagen_target_changed", "sync_mutagen_ambiguous", "sync_mutagen_unverified", "sync_mutagen_proxy_missing", "sync_outcome_unknown", "state_unavailable", "configuration_changed", "configuration_unavailable", "authentication_required", "credential_unavailable", "permission_denied", "server_unavailable", "rate_limited", "sync_response_invalid", "canceled"}, code)
}
func decodeSyncRecoveryMutagen(raw json.RawMessage) (SyncRecoveryMutagen, error) {
	var observation SyncRecoveryMutagen
	fields, err := decodeAccountObject(raw, &observation, []string{"state", "recoverable", "error_code", "namespace_fingerprint", "target"}, []string{"error_code", "namespace_fingerprint", "target"})
	if err != nil || !slices.Contains([]string{"available", "session_missing", "unavailable", "missing", "blocked"}, observation.State) || observation.NamespaceFingerprint != nil && !validSyncFingerprint(*observation.NamespaceFingerprint) {
		return observation, fault.CLIProtocol
	}
	if observation.Recoverable != (observation.State == "available" || observation.State == "session_missing") || observation.Recoverable != (observation.ErrorCode == nil) || observation.Recoverable && observation.NamespaceFingerprint == nil || (observation.State == "available") != (observation.Target != nil) {
		return observation, fault.CLIProtocol
	}
	if observation.ErrorCode != nil && !validSyncRecoveryError(*observation.ErrorCode) {
		return observation, fault.CLIProtocol
	}
	if observation.Target != nil {
		target := observation.Target
		if _, err := decodeAccountObject(fields["target"], target, []string{"identifier", "name", "local_path", "remote_path", "endpoint", "configuration_fingerprint"}, nil); err != nil || !validMutagenIdentifier(target.Identifier) || !validMutagenName(target.Name) || !validWireGuardPath(target.LocalPath) || !validSyncRecoveryText(target.RemotePath, 32768) || !validSyncRecoveryText(target.Endpoint, 8192) || !validSyncFingerprint(target.ConfigurationFingerprint) {
			return observation, fault.CLIProtocol
		}
	}
	return observation, nil
}
func decodeSyncRecoveryPlan(raw json.RawMessage) (SyncRecoveryPlan, error) {
	var plan SyncRecoveryPlan
	fields, err := decodeAccountObject(raw, &plan, []string{"context", "target", "mapping", "mapping_fingerprint", "server_state", "mutagen", "recoverable", "error_code"}, []string{"error_code"})
	if err != nil {
		return plan, err
	}
	if plan.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return plan, err
	}
	if plan.Target, err = decodeSyncRecoveryTarget(fields["target"]); err != nil {
		return plan, err
	}
	if plan.Mapping, err = decodeSyncRecoveryMapping(fields["mapping"]); err != nil {
		return plan, err
	}
	if plan.Mutagen, err = decodeSyncRecoveryMutagen(fields["mutagen"]); err != nil {
		return plan, err
	}
	w, t := plan.Mapping.Workspace, plan.Target
	if !validSyncFingerprint(plan.MappingFingerprint) || !slices.Contains([]string{"sync_not_found", "workspace_not_found"}, plan.ServerState) || plan.Context.ServerURL == nil || *plan.Context.ServerURL != w.ServerURL || t.WorkspaceID == nil || *t.WorkspaceID != w.ID || t.ProjectKey != w.ProjectKey || t.LocalPath != w.LocalPath || plan.Recoverable != plan.Mutagen.Recoverable || !sameSyncPointer(plan.ErrorCode, plan.Mutagen.ErrorCode) {
		return plan, fault.CLIProtocol
	}
	if plan.Mapping.Sync == nil {
		if t.SyncSessionID != nil || plan.ServerState != "workspace_not_found" || plan.Recoverable {
			return plan, fault.CLIProtocol
		}
	} else if t.SyncSessionID == nil || *t.SyncSessionID != plan.Mapping.Sync.ID {
		return plan, fault.CLIProtocol
	}
	if mt := plan.Mutagen.Target; mt != nil {
		s := plan.Mapping.Sync
		if s == nil || s.MutagenSessionID == nil || s.RemoteEndpoint == nil || w.RemotePath == nil || mt.Name != *s.MutagenSessionID || mt.Endpoint != *s.RemoteEndpoint || mt.LocalPath != t.LocalPath || mt.RemotePath != *w.RemotePath {
			return plan, fault.CLIProtocol
		}
	}
	return plan, nil
}
func decodeSyncRecoveryReceipt(raw json.RawMessage) (SyncRecoveryReceipt, error) {
	var receipt SyncRecoveryReceipt
	fields, err := decodeAccountObject(raw, &receipt, []string{"operation_id", "context", "method", "plan", "result_target", "outcome", "stages", "mutagen", "error_code"}, []string{"mutagen", "error_code"})
	if err != nil {
		return receipt, err
	}
	if receipt.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return receipt, err
	}
	if receipt.Plan, err = decodeSyncRecoveryPlan(fields["plan"]); err != nil {
		return receipt, err
	}
	if receipt.ResultTarget, err = decodeSyncRecoveryTarget(fields["result_target"]); err != nil {
		return receipt, err
	}
	if receipt.Mutagen != nil {
		observation, err := decodeSyncRecoveryMutagen(fields["mutagen"])
		if err != nil {
			return receipt, err
		}
		receipt.Mutagen = &observation
	}
	if !validDefaultTool(receipt.OperationID) || receipt.Method != "sync.recovery.apply" || !receipt.Plan.Recoverable || !sameDefaultContext(receipt.Context, receipt.Plan.Context) || !slices.Contains([]string{"completed", "partial", "unknown"}, receipt.Outcome) || (receipt.Outcome == "completed") != (receipt.ErrorCode == nil) || receipt.ErrorCode != nil && !validSyncRecoveryError(*receipt.ErrorCode) {
		return receipt, fault.CLIProtocol
	}
	var stages []json.RawMessage
	if json.Unmarshal(fields["stages"], &stages) != nil || len(stages) < 2 || len(stages) > 3 {
		return receipt, fault.CLIProtocol
	}
	for index, rawStage := range stages {
		var stage SyncStage
		if _, err := decodeAccountObject(rawStage, &stage, []string{"name", "state"}, nil); err != nil || stage.Name != []string{"server", "mutagen", "persist"}[index] || !slices.Contains([]string{"completed", "failed", "unknown"}, stage.State) || index < len(stages)-1 && stage.State != "completed" {
			return receipt, fault.CLIProtocol
		}
	}
	if receipt.Stages[1].State == "completed" {
		if len(stages) != 3 || receipt.Mutagen == nil || receipt.Mutagen.State != "session_missing" || !sameSyncPointer(receipt.Mutagen.NamespaceFingerprint, receipt.Plan.Mutagen.NamespaceFingerprint) {
			return receipt, fault.CLIProtocol
		}
	} else if len(stages) != 2 || receipt.Mutagen != nil {
		return receipt, fault.CLIProtocol
	}
	last := receipt.Stages[len(receipt.Stages)-1]
	if receipt.Outcome == "completed" {
		wanted := copySyncTarget(receipt.Plan.Target)
		wanted.SyncSessionID = nil
		if receipt.Plan.ServerState == "workspace_not_found" {
			wanted.WorkspaceID = nil
		}
		if len(stages) != 3 || last.State != "completed" || !reflect.DeepEqual(receipt.ResultTarget, wanted) || receipt.Mutagen == nil || receipt.Mutagen.State != "session_missing" {
			return receipt, fault.CLIProtocol
		}
	} else if !reflect.DeepEqual(receipt.ResultTarget, receipt.Plan.Target) || last.State != map[string]string{"partial": "failed", "unknown": "unknown"}[receipt.Outcome] {
		return receipt, fault.CLIProtocol
	}
	return receipt, nil
}

func validSyncRecoveryText(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.IndexFunc(value, unicode.IsControl) < 0
}
func validMutagenIdentifier(value string) bool {
	if !strings.HasPrefix(value, "sync_") || len(value) <= 5 || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !asciiLetter(r) && !(r >= '0' && r <= '9') && r != '_' {
			return false
		}
	}
	return true
}
func asciiLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
func validMutagenName(value string) bool {
	if value == "" || len(value) > 128 || !asciiLetter(rune(value[0])) || value == "defaults" {
		return false
	}
	for _, r := range value {
		if !asciiLetter(r) && !(r >= '0' && r <= '9') && r != '-' {
			return false
		}
	}
	candidate := value
	if len(value) == 36 && value[8] == '-' && value[13] == '-' && value[18] == '-' && value[23] == '-' {
		candidate = strings.ReplaceAll(value, "-", "")
	}
	if len(candidate) == 32 {
		if _, err := hex.DecodeString(candidate); err == nil {
			return false
		}
	}
	return true
}
