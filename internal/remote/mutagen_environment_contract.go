package remote

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func validMutagenEnvironmentError(code string) bool {
	return slices.Contains([]string{"sync_mutagen_environment_outcome_unknown", "sync_mutagen_environment_blocked", "sync_mutagen_environment_invalid", "sync_mutagen_environment_mismatch", "sync_mutagen_missing", "sync_mutagen_proxy_missing", "sync_mutagen_unavailable", "sync_mutagen_timeout", "sync_mutagen_output_limit", "sync_mutagen_response_invalid", "sync_mutagen_inventory_invalid", "sync_mutagen_inventory_limit", "sync_mutagen_inventory_changed", "sync_mutagen_daemon_busy", "sync_mutagen_daemon_lock_missing", "sync_mutagen_registration_external", "sync_mutagen_registration_unknown", "sync_mutagen_environment_changed", "sync_mutagen_environment_unverified", "sync_mutagen_environment_write_failed", "operation_busy", "sync_outcome_unknown"}, code)
}
func decodeMutagenBinding(raw json.RawMessage) (MutagenEnvironmentBinding, error) {
	var binding MutagenEnvironmentBinding
	if _, err := decodeAccountObject(raw, &binding, []string{"home", "ssh_directory", "fingerprint"}, nil); err != nil || !validWireGuardPath(binding.Home) || !validWireGuardPath(binding.SSHDirectory) || !validSyncFingerprint(binding.Fingerprint) {
		return binding, fault.CLIProtocol
	}
	return binding, nil
}
func decodeMutagenEndpoint(raw json.RawMessage) (MutagenEnvironmentEndpoint, error) {
	var endpoint MutagenEnvironmentEndpoint
	if _, err := decodeAccountObject(raw, &endpoint, []string{"protocol", "user", "host", "path", "port"}, nil); err != nil || !slices.Contains([]string{"local", "ssh", "docker"}, endpoint.Protocol) || !validSyncRecoveryText(endpoint.Path, 32768) || endpoint.Port > 65535 {
		return endpoint, fault.CLIProtocol
	}
	for _, text := range []string{endpoint.User, endpoint.Host} {
		if text != "" && !validSyncRecoveryText(text, 8192) {
			return endpoint, fault.CLIProtocol
		}
	}
	return endpoint, nil
}
func decodeMutagenEnvironmentSession(raw json.RawMessage) (MutagenEnvironmentSession, error) {
	var session MutagenEnvironmentSession
	fields, err := decodeAccountObject(raw, &session, []string{"kind", "identifier", "name", "paused", "source", "destination", "configuration_fingerprint"}, nil)
	if err != nil || !slices.Contains([]string{"sync", "forward"}, session.Kind) || !validSyncFingerprint(session.ConfigurationFingerprint) || session.Name != "" && !validMutagenName(session.Name) {
		return session, fault.CLIProtocol
	}
	identifier := session.Identifier
	if session.Kind == "forward" {
		if !strings.HasPrefix(identifier, "fwrd_") {
			return session, fault.CLIProtocol
		}
		identifier = "sync_" + strings.TrimPrefix(identifier, "fwrd_")
	}
	if !validMutagenIdentifier(identifier) {
		return session, fault.CLIProtocol
	}
	if session.Source, err = decodeMutagenEndpoint(fields["source"]); err != nil {
		return session, err
	}
	if session.Destination, err = decodeMutagenEndpoint(fields["destination"]); err != nil {
		return session, err
	}
	return session, nil
}
func decodeMutagenEnvironmentPlan(raw json.RawMessage) (MutagenEnvironmentPlan, error) {
	var plan MutagenEnvironmentPlan
	fields, err := decodeAccountObject(raw, &plan, []string{"home", "data_directory", "namespace_fingerprint", "daemon_state", "running_fingerprint", "registration", "previous", "desired", "tools", "sessions", "inventory_fingerprint", "marker_fingerprint", "fingerprint", "repairable", "error_code"}, []string{"namespace_fingerprint", "running_fingerprint", "previous", "marker_fingerprint", "error_code"})
	if err != nil || !validWireGuardPath(plan.Home) || !validWireGuardPath(plan.DataDirectory) || !slices.Contains([]string{"running", "stopped", "unavailable"}, plan.DaemonState) || !slices.Contains([]string{"unmanaged", "external", "unknown"}, plan.Registration) || !validSyncFingerprint(plan.InventoryFingerprint) || !validSyncFingerprint(plan.Fingerprint) {
		return plan, fault.CLIProtocol
	}
	for _, fingerprint := range []*string{plan.NamespaceFingerprint, plan.RunningFingerprint, plan.MarkerFingerprint} {
		if fingerprint != nil && !validSyncFingerprint(*fingerprint) {
			return plan, fault.CLIProtocol
		}
	}
	if (plan.DaemonState == "running") != (plan.RunningFingerprint != nil) || plan.DaemonState == "running" && plan.NamespaceFingerprint == nil || plan.Repairable != (plan.ErrorCode == nil) || plan.ErrorCode != nil && !validMutagenEnvironmentError(*plan.ErrorCode) {
		return plan, fault.CLIProtocol
	}
	if plan.Desired, err = decodeMutagenBinding(fields["desired"]); err != nil {
		return plan, err
	}
	if plan.Desired.Home != plan.Home {
		return plan, fault.CLIProtocol
	}
	if plan.Previous != nil {
		previous, err := decodeMutagenBinding(fields["previous"])
		if err != nil || plan.MarkerFingerprint == nil {
			return plan, fault.CLIProtocol
		}
		plan.Previous = &previous
	}
	var tools, sessions []json.RawMessage
	if json.Unmarshal(fields["tools"], &tools) != nil || tools == nil || len(tools) > 3 || json.Unmarshal(fields["sessions"], &sessions) != nil || sessions == nil || len(sessions) > 4096 {
		return plan, fault.CLIProtocol
	}
	seenTools := map[string]bool{}
	for _, rawTool := range tools {
		var tool MutagenEnvironmentTool
		if _, err := decodeAccountObject(rawTool, &tool, []string{"name", "path", "fingerprint"}, nil); err != nil || !slices.Contains([]string{"mutagen", "ssh", "scp"}, tool.Name) || seenTools[tool.Name] || !validWireGuardPath(tool.Path) || !validSyncFingerprint(tool.Fingerprint) {
			return plan, fault.CLIProtocol
		}
		seenTools[tool.Name] = true
	}
	seenSessions := map[string]bool{}
	for i, rawSession := range sessions {
		session, err := decodeMutagenEnvironmentSession(rawSession)
		if err != nil || seenSessions[session.Identifier] {
			return plan, fault.CLIProtocol
		}
		seenSessions[session.Identifier] = true
		plan.Sessions[i] = session
	}
	if plan.Repairable && (len(tools) != 3 || plan.Registration != "unmanaged" || plan.DaemonState == "unavailable" || plan.NamespaceFingerprint == nil && (plan.DaemonState != "stopped" || len(sessions) != 0)) {
		return plan, fault.CLIProtocol
	}
	return plan, nil
}
func decodeMutagenEnvironmentReceipt(raw json.RawMessage) (MutagenEnvironmentReceipt, error) {
	var receipt MutagenEnvironmentReceipt
	fields, err := decodeAccountObject(raw, &receipt, []string{"operation_id", "home", "method", "plan", "outcome", "stages", "environment", "error_code"}, []string{"environment", "error_code"})
	if err != nil || !validDefaultTool(receipt.OperationID) || receipt.Method != "mutagen.environment.apply" || !slices.Contains([]string{"completed", "partial", "unknown"}, receipt.Outcome) || (receipt.Outcome == "completed") != (receipt.ErrorCode == nil) || receipt.ErrorCode != nil && !validMutagenEnvironmentError(*receipt.ErrorCode) {
		return receipt, fault.CLIProtocol
	}
	if receipt.Plan, err = decodeMutagenEnvironmentPlan(fields["plan"]); err != nil {
		return receipt, err
	}
	if !receipt.Plan.Repairable || receipt.Home != receipt.Plan.Home {
		return receipt, fault.CLIProtocol
	}
	var stages []json.RawMessage
	if json.Unmarshal(fields["stages"], &stages) != nil || len(stages) < 1 || len(stages) > 4 {
		return receipt, fault.CLIProtocol
	}
	for i, rawStage := range stages {
		var stage SyncStage
		if _, err := decodeAccountObject(rawStage, &stage, []string{"name", "state"}, nil); err != nil || stage.Name != []string{"stop", "start", "verify", "persist"}[i] || !slices.Contains([]string{"completed", "skipped", "failed", "unknown"}, stage.State) {
			return receipt, fault.CLIProtocol
		}
		if i < len(stages)-1 || receipt.Outcome == "completed" {
			state := "completed"
			if i == 0 && receipt.Plan.DaemonState == "stopped" {
				state = "skipped"
			}
			if stage.State != state {
				return receipt, fault.CLIProtocol
			}
		} else if stage.State != map[string]string{"partial": "failed", "unknown": "unknown"}[receipt.Outcome] {
			return receipt, fault.CLIProtocol
		}
	}
	if receipt.Outcome != "completed" {
		if receipt.Environment != nil {
			return receipt, fault.CLIProtocol
		}
		return receipt, nil
	}
	if len(stages) != 4 || receipt.Environment == nil {
		return receipt, fault.CLIProtocol
	}
	post, err := decodeMutagenEnvironmentPlan(fields["environment"])
	if err != nil || !post.Repairable || post.DaemonState != "running" || post.Home != receipt.Home || post.DataDirectory != receipt.Plan.DataDirectory || post.Previous == nil || !reflect.DeepEqual(*post.Previous, receipt.Plan.Desired) || !reflect.DeepEqual(post.Desired, receipt.Plan.Desired) || !reflect.DeepEqual(post.Tools, receipt.Plan.Tools) || !reflect.DeepEqual(post.Sessions, receipt.Plan.Sessions) || receipt.Plan.NamespaceFingerprint != nil && !sameSyncPointer(receipt.Plan.NamespaceFingerprint, post.NamespaceFingerprint) {
		return receipt, fault.CLIProtocol
	}
	receipt.Environment = &post
	return receipt, nil
}
