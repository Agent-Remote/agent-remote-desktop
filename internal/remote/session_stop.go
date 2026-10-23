package remote

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	SessionStopMethod        = "sessions.stop"
	SessionStopRequestMethod = "sessions.stop.request"
	// SessionStopStatusMethod is accepted for clients that expose the more
	// explicit spelling. New CLI builds advertise sessions.stop.request.
	SessionStopStatusMethod = "sessions.stop.status"
)

func SessionStopSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SessionStopMethod, "write", "device", "exact_request")
}

func SessionStopQuerySupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SessionStopRequestMethod, "read", "device", "") ||
		supportsDesktopOperation(c, SessionStopStatusMethod, "read", "device", "")
}

func StopSession(ctx context.Context, client cliadapter.Client, input SessionStopInput) (SessionStopReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if input.Reason == "" {
		input.Reason = "user_requested"
	}
	if err := validateSessionStopInput(input, client.Home); err != nil {
		return SessionStopReceipt{}, err
	}
	caps, err := sessionStopCapabilities(ctx, client)
	if err != nil {
		return SessionStopReceipt{}, err
	}
	if !SessionStopSupported(caps) {
		return SessionStopReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SessionStopMethod, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return SessionStopReceipt{}, err
		}
		return SessionStopReceipt{}, fault.SessionStopOutcomeUnknown
	}
	receipt, err := decodeSessionStopReceipt(raw)
	if err != nil || receipt.Context.Home != client.Home || receipt.SessionID != input.SessionID || receipt.NodeID != input.ExpectedNodeID || receipt.RuntimeBackend != input.ExpectedBackend || (input.ExpectedStopTaskID != "" && (receipt.StopTaskID == nil || *receipt.StopTaskID != input.ExpectedStopTaskID)) || !sameDefaultContext(receipt.Context, input.Expected) {
		return SessionStopReceipt{}, fault.SessionStopOutcomeUnknown
	}
	return receipt, nil
}

func QuerySessionStop(ctx context.Context, client cliadapter.Client, input SessionStopRequestInput) (SessionStopReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if input.Timeout == 0 {
		input.Timeout = 60
	}
	if !validAccountImportRequestID(input.RequestID) || input.Expected.Home != client.Home || !validAccountContext(input.Expected, client.Home) || input.Timeout < 1 || input.Timeout > 86400 {
		return SessionStopReceipt{}, fault.SessionStopInputInvalid
	}
	caps, err := sessionStopCapabilities(ctx, client)
	if err != nil {
		return SessionStopReceipt{}, err
	}
	method := SessionStopRequestMethod
	if !supportsDesktopOperation(caps, method, "read", "device", "") && supportsDesktopOperation(caps, SessionStopStatusMethod, "read", "device", "") {
		method = SessionStopStatusMethod
	}
	if !SessionStopQuerySupported(caps) {
		return SessionStopReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, method, input, &raw); err != nil {
		return SessionStopReceipt{}, err
	}
	receipt, err := decodeSessionStopReceipt(raw)
	if err != nil || receipt.RequestID != input.RequestID || receipt.Context.Home != client.Home || !sameDefaultContext(receipt.Context, input.Expected) {
		return SessionStopReceipt{}, fault.SessionStopOutcomeUnknown
	}
	return receipt, nil
}

func sessionStopCapabilities(ctx context.Context, client cliadapter.Client) (cliadapter.Capabilities, error) {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return caps, err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return caps, fault.CLIProtocol
	}
	return caps, nil
}

func validateSessionStopInput(input SessionStopInput, home string) error {
	// The Rust bridge requires a complete reviewed runtime identity. Allowing an
	// empty node/backend here would only defer the rejection to the subprocess,
	// and would make a stop confirmation less precise than the session selected
	// by the user.
	if input.SchemaVersion != 1 || !input.Confirmed || input.Reason != "user_requested" || input.Expected.Home != home || !validDefaultContext(input.Expected) || !validCanonicalUUID(input.SessionID) || !validCanonicalUUID(input.ExpectedNodeID) || !validDefaultTool(input.ExpectedBackend) || !validDefaultIDString(input.ExpectedStopTaskID) {
		return fault.SessionStopInputInvalid
	}
	return nil
}

func validDefaultIDString(value string) bool { return value == "" || validDefaultTool(value) }

var sessionStopPhases = []string{"pending", "accepted", "stopping", "stopped", "saving", "saved", "failed", "unknown", "deleted", "interrupted", "running", "settled", "unresolved", "legacy"}
var sessionStopTaskStatuses = []string{"unknown", "not_requested", "pending", "accepted", "running", "succeeded", "failed", "cancelled"}
var sessionStopStatuses = []string{"unknown", "starting", "running", "active", "stopping", "stopped", "interrupted", "failed", "deleted"}

func decodeSessionStopReceipt(raw json.RawMessage) (SessionStopReceipt, error) {
	var receipt SessionStopReceipt
	var header struct {
		Phase string `json:"phase"`
	}
	var object map[string]json.RawMessage
	_ = json.Unmarshal(raw, &object)
	if json.Unmarshal(raw, &header) == nil && header.Phase == "unknown" && object["intent_fingerprint"] == nil {
		fields, err := decodeAccountObject(raw, &receipt,
			[]string{"schema_version", "request_id", "phase", "session_id", "node_id", "runtime_backend", "session_deleted", "error_code", "context"},
			[]string{"error_code"})
		if err != nil || receipt.SchemaVersion != 1 || !validAccountImportRequestID(receipt.RequestID) || !validCanonicalUUID(receipt.SessionID) || !validCanonicalUUID(receipt.NodeID) || !validDefaultTool(receipt.RuntimeBackend) || receipt.ErrorCode == nil || *receipt.ErrorCode != "SESSION_STOP_SUBMISSION_UNKNOWN" {
			return SessionStopReceipt{}, fault.CLIProtocol
		}
		if receipt.Context, err = decodeAccountContext(fields["context"]); err != nil {
			return SessionStopReceipt{}, err
		}
		return receipt, nil
	}
	fields, err := decodeAccountObject(raw, &receipt,
		[]string{"schema_version", "request_id", "phase", "context", "session_id", "node_id", "runtime_backend", "runtime_resource_id", "stop_task_id", "stop_task_record_id", "stop_task_status", "session_status", "skill_finalization_operation_id", "session_deleted", "observed_at", "error_code", "intent_fingerprint", "reason", "result_status", "process_stopped", "save_status"},
		[]string{"runtime_resource_id", "stop_task_id", "stop_task_record_id", "stop_task_status", "session_status", "skill_finalization_operation_id", "observed_at", "error_code", "result_status", "process_stopped", "save_status"})
	if err != nil || receipt.SchemaVersion != 1 || !validAccountImportRequestID(receipt.RequestID) || !validCanonicalUUID(receipt.SessionID) || !validCanonicalUUID(receipt.NodeID) || !validDefaultTool(receipt.RuntimeBackend) || !slices.Contains(sessionStopPhases, receipt.Phase) || !slices.Contains(sessionStopTaskStatuses, optionalStopValue(receipt.StopTaskStatus)) || !slices.Contains(sessionStopStatuses, optionalStopValue(receipt.SessionStatus)) || receipt.Reason != "user_requested" || !validSyncFingerprint(receipt.IntentFingerprint) {
		return SessionStopReceipt{}, fault.CLIProtocol
	}
	if receipt.Context, err = decodeAccountContext(fields["context"]); err != nil {
		return SessionStopReceipt{}, err
	}
	for _, value := range []*string{receipt.RuntimeResourceID, receipt.StopTaskID, receipt.StopTaskRecordID, receipt.SkillFinalizationOperationID, receipt.ErrorCode, receipt.ResultStatus, receipt.SaveStatus} {
		if value != nil && (strings.TrimSpace(*value) == "" || strings.IndexFunc(*value, func(r rune) bool { return r == '\r' || r == '\n' || r == '\t' }) >= 0) {
			return SessionStopReceipt{}, fault.CLIProtocol
		}
	}
	if receipt.SessionDeleted && optionalStopValue(receipt.SessionStatus) != "deleted" && receipt.Phase != "deleted" {
		return SessionStopReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

func optionalStopValue(value *string) string {
	if value == nil {
		return "unknown"
	}
	return *value
}

// Valid checks the public shape without requiring a particular CLI context.
// Callers must still compare Context and SessionID with their reviewed input.
func (r SessionStopReceipt) Valid() bool {
	if r.Phase == "unknown" && r.IntentFingerprint == "" {
		return r.SchemaVersion == 1 && validAccountImportRequestID(r.RequestID) && validCanonicalUUID(r.SessionID) && validCanonicalUUID(r.NodeID) && validDefaultTool(r.RuntimeBackend) && r.ErrorCode != nil
	}
	b, err := json.Marshal(r)
	if err != nil {
		return false
	}
	_, err = decodeSessionStopReceipt(b)
	return err == nil
}
