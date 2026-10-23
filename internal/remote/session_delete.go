package remote

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	SessionDeleteMethod        = "sessions.delete"
	SessionDeleteRequestMethod = "sessions.delete.request"
	SessionDeleteStatusMethod  = "sessions.delete.status"
)

func SessionDeleteSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SessionDeleteMethod, "write", "device", "exact_request")
}

func SessionDeleteQuerySupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SessionDeleteRequestMethod, "read", "device", "") ||
		supportsDesktopOperation(c, SessionDeleteStatusMethod, "read", "device", "")
}

func DeleteSessions(ctx context.Context, client cliadapter.Client, input SessionDeleteInput) (SessionDeleteReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if input.Reason == "" {
		input.Reason = "user_requested"
	}
	if err := validateSessionDeleteInput(input, client.Home); err != nil {
		return SessionDeleteReceipt{}, err
	}
	caps, err := sessionDeleteCapabilities(ctx, client)
	if err != nil {
		return SessionDeleteReceipt{}, err
	}
	if !SessionDeleteSupported(caps) {
		return SessionDeleteReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SessionDeleteMethod, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return SessionDeleteReceipt{}, err
		}
		return SessionDeleteReceipt{}, fault.SessionDeleteOutcomeUnknown
	}
	receipt, err := decodeSessionDeleteReceipt(raw)
	if err != nil || !sameDeleteIntent(receipt, input) {
		return SessionDeleteReceipt{}, fault.SessionDeleteOutcomeUnknown
	}
	return receipt, nil
}

func QuerySessionDelete(ctx context.Context, client cliadapter.Client, input SessionDeleteRequestInput) (SessionDeleteReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if input.Timeout == 0 {
		input.Timeout = 60
	}
	if !validAccountImportRequestID(input.RequestID) || input.Expected.Home != client.Home || !validAccountContext(input.Expected, client.Home) || input.Timeout < 1 || input.Timeout > 86400 {
		return SessionDeleteReceipt{}, fault.SessionDeleteInputInvalid
	}
	caps, err := sessionDeleteCapabilities(ctx, client)
	if err != nil {
		return SessionDeleteReceipt{}, err
	}
	method := SessionDeleteRequestMethod
	if !supportsDesktopOperation(caps, method, "read", "device", "") && supportsDesktopOperation(caps, SessionDeleteStatusMethod, "read", "device", "") {
		method = SessionDeleteStatusMethod
	}
	if !SessionDeleteQuerySupported(caps) {
		return SessionDeleteReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, method, input, &raw); err != nil {
		return SessionDeleteReceipt{}, err
	}
	receipt, err := decodeSessionDeleteReceipt(raw)
	if err != nil || receipt.RequestID != input.RequestID || !sameDefaultContext(receipt.Context, input.Expected) {
		return SessionDeleteReceipt{}, fault.SessionDeleteOutcomeUnknown
	}
	return receipt, nil
}

func sessionDeleteCapabilities(ctx context.Context, client cliadapter.Client) (cliadapter.Capabilities, error) {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return caps, err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return caps, fault.CLIProtocol
	}
	return caps, nil
}

func validateSessionDeleteInput(input SessionDeleteInput, home string) error {
	if input.SchemaVersion != 1 || !input.Confirmed || input.Reason != "user_requested" || input.Expected.Home != home || !validDefaultContext(input.Expected) || input.Expected.ServerURL == nil || input.Expected.DeviceID == nil {
		return fault.SessionDeleteInputInvalid
	}
	// The caller-key deletion bridge is intentionally Claude-only; accepting a
	// different tool here would produce a UI-confirmed request the Rust bridge
	// must reject after the user has already reviewed it.
	if input.ToolType != "claude" || input.ToolType != input.Selection.ToolType || !validSessionDeleteIDs(input.SessionIDs) || !validSessionDeleteSelection(input.Selection) || !validSyncFingerprint(input.SelectionDigest) || !validSyncFingerprint(input.IntentFingerprint) {
		return fault.SessionDeleteInputInvalid
	}
	if input.SelectionDigest != SessionDeleteSelectionDigest(input.ToolType, input.SessionIDs, input.Selection) {
		return fault.SessionDeleteInputInvalid
	}
	return nil
}

func validSessionDeleteIDs(ids []string) bool {
	if len(ids) == 0 || len(ids) > 256 {
		return false
	}
	for i, id := range ids {
		if !validAccountImportRequestID(id) || (i > 0 && ids[i-1] >= id) {
			return false
		}
	}
	return true
}

func validSessionDeleteSelection(selection SessionDeleteSelection) bool {
	if selection.ToolType != "claude" || len(selection.Statuses) == 0 || len(selection.Statuses) > 16 {
		return false
	}
	for i, status := range selection.Statuses {
		if !slices.Contains([]string{"failed", "interrupted", "stopped"}, status) || (i > 0 && selection.Statuses[i-1] >= status) {
			return false
		}
	}
	return selection.ProjectKey == nil || validAccountText(*selection.ProjectKey, 512)
}

var sessionDeletePhases = []string{"pending", "accepted", "deleting", "deleted", "failed", "unknown", "blocked", "interrupted"}
var sessionDeleteTaskStatuses = []string{"unknown", "not_requested", "pending", "accepted", "running", "succeeded", "failed", "cancelled"}

func decodeSessionDeleteReceipt(raw json.RawMessage) (SessionDeleteReceipt, error) {
	var value SessionDeleteReceipt
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil || len(fields) == 0 {
		return value, fault.CLIProtocol
	}
	// Decode with DisallowUnknownFields, while allowing nullable task/error
	// projections and keeping the exact collection visible in every receipt.
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&value) != nil || !value.Valid() {
		return SessionDeleteReceipt{}, fault.CLIProtocol
	}
	return value, nil
}

func (r SessionDeleteReceipt) Valid() bool {
	if r.SchemaVersion != 1 || r.Reason != "user_requested" || !validAccountImportRequestID(r.RequestID) || !slices.Contains(sessionDeletePhases, r.Phase) || !validDefaultTool(r.ToolType) || !validSessionDeleteIDs(r.SessionIDs) || !validSessionDeleteSelection(r.Selection) || r.Selection.ToolType != r.ToolType || !validSyncFingerprint(r.SelectionDigest) || !validSyncFingerprint(r.IntentFingerprint) || r.DeletedSessionIDs == nil || r.SessionStatuses == nil || (r.Phase != "unknown" && (r.SessionContexts == nil || len(r.SessionContexts) != len(r.SessionIDs))) || (r.TaskStatus != nil && !slices.Contains(sessionDeleteTaskStatuses, *r.TaskStatus)) {
		return false
	}
	if r.SelectionDigest != SessionDeleteSelectionDigest(r.ToolType, r.SessionIDs, r.Selection) {
		return false
	}
	seen := make(map[string]bool, len(r.DeletedSessionIDs))
	for _, id := range r.DeletedSessionIDs {
		if !validAccountImportRequestID(id) || seen[id] || !slices.Contains(r.SessionIDs, id) {
			return false
		}
		seen[id] = true
	}
	for id, status := range r.SessionStatuses {
		if !validAccountImportRequestID(id) || !slices.Contains(r.SessionIDs, id) || !validAccountText(status, 128) {
			return false
		}
	}
	seenContexts := make(map[string]bool, len(r.SessionContexts))
	for _, context := range r.SessionContexts {
		if !validAccountImportRequestID(context.SessionID) || seenContexts[context.SessionID] || !slices.Contains(r.SessionIDs, context.SessionID) || !validAccountImportRequestID(context.ToolAccountID) || !validAccountImportRequestID(context.WorkspaceID) || !validAccountText(context.ProjectKey, 4096) || !validAccountImportRequestID(context.NodeID) || !validAccountText(context.RuntimeBackend, 128) {
			return false
		}
		seenContexts[context.SessionID] = true
		if context.RuntimeResourceID != nil && !validAccountText(*context.RuntimeResourceID, 512) {
			return false
		}
	}
	for _, value := range []*string{r.DeleteTaskID, r.DeleteTaskRecordID, r.ErrorCode, r.ResultStatus} {
		if value != nil && !validAccountText(*value, 512) {
			return false
		}
	}
	if r.ObservedAt != nil && !validAccountText(*r.ObservedAt, 128) {
		return false
	}
	for _, blocker := range r.RetentionBlockers {
		if !validAccountText(blocker, 512) {
			return false
		}
	}
	return true
}

func sameDeleteIntent(receipt SessionDeleteReceipt, input SessionDeleteInput) bool {
	return receipt.Context.Home == input.Expected.Home && sameDefaultContext(receipt.Context, input.Expected) && receipt.ToolType == input.ToolType && slices.Equal(receipt.SessionIDs, input.SessionIDs) && receipt.SelectionDigest == input.SelectionDigest && receipt.IntentFingerprint == input.IntentFingerprint
}

// SessionDeleteSelectionDigest is deterministic across CLI and Desktop and
// deliberately excludes mutable status observations from the ID set itself.
func SessionDeleteSelectionDigest(tool string, ids []string, selection SessionDeleteSelection) string {
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	statuses := append([]string(nil), selection.Statuses...)
	sort.Strings(statuses)
	payload := map[string]any{
		"schema_version": 1,
		"tool_type":      tool,
		"session_ids":    ids,
		"selection": map[string]any{
			"tool_type":   selection.ToolType,
			"statuses":    statuses,
			"project_key": selection.ProjectKey,
		},
	}
	return sha256JSON(payload)
}

// SessionDeleteIntentFingerprint follows the shared CLI/Server canonical
// contract. Authentication context is checked separately on every receipt;
// it is intentionally not part of this cross-client digest.
func SessionDeleteIntentFingerprint(tool string, ids []string, selection SessionDeleteSelection) string {
	ids = append([]string(nil), ids...)
	sort.Strings(ids)
	statuses := append([]string(nil), selection.Statuses...)
	sort.Strings(statuses)
	payload := map[string]any{
		"schema_version": 1,
		"tool_type":      tool,
		"session_ids":    ids,
		"selection": map[string]any{
			"tool_type":   selection.ToolType,
			"statuses":    statuses,
			"project_key": selection.ProjectKey,
		},
		"reason": "user_requested",
	}
	return sha256JSON(payload)
}

func sha256JSON(value any) string {
	bytes, _ := json.Marshal(value)
	sum := sha256.Sum256(bytes)
	return "sha256:" + hex.EncodeToString(sum[:])
}
