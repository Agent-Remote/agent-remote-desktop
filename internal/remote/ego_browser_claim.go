package remote

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	EgoBrowserCandidatesMethod   = "ego_browser.candidates"
	EgoBrowserClaimPlanMethod    = "ego_browser.claim.plan"
	EgoBrowserClaimControlMethod = "ego_browser.claim.control"
)

// EgoBrowserCandidate is a safe, bounded projection of one remote session
// eligible for full-trust browser binding. It contains no payload or secret.
type EgoBrowserCandidate struct {
	ToolSessionID               string  `json:"tool_session_id"`
	ToolType                    string  `json:"tool_type"`
	ToolAccountID               string  `json:"tool_account_id"`
	WorkspaceID                 string  `json:"workspace_id"`
	ProjectKey                  string  `json:"project_key"`
	DisplayName                 string  `json:"display_name"`
	Status                      string  `json:"status"`
	NodeID                      string  `json:"node_id"`
	RuntimeBackend              string  `json:"runtime_backend"`
	CurrentEgoBrowserDeviceID   *string `json:"current_ego_browser_device_id"`
	CurrentEgoBrowserDeviceName *string `json:"current_ego_browser_device_name"`
	BindingID                   *string `json:"binding_id"`
	Controllable                bool    `json:"controllable"`
}

type EgoBrowserCandidates struct {
	SchemaVersion int                      `json:"schema_version"`
	Operation     string                   `json:"operation"`
	Context       cliadapter.ConfigContext `json:"context"`
	Items         []EgoBrowserCandidate    `json:"items"`
}

type EgoBrowserCandidatesInput struct {
	Expected cliadapter.ConfigContext
}

type EgoBrowserClaimPlanInput struct {
	Expected             cliadapter.ConfigContext
	ToolSessionID        string
	CandidateFingerprint string
}

type EgoBrowserClaimPlan struct {
	SchemaVersion        int                      `json:"schema_version"`
	Operation            string                   `json:"operation"`
	Context              cliadapter.ConfigContext `json:"context"`
	Candidate            EgoBrowserCandidate      `json:"candidate"`
	CandidateFingerprint string                   `json:"candidate_fingerprint"`
	LocalAdmission       string                   `json:"local_admission"`
	RequiresConfirmation bool                     `json:"requires_confirmation"`
	Allowed              bool                     `json:"allowed"`
}

type EgoBrowserClaimControlInput struct {
	Confirmed            bool
	Expected             cliadapter.ConfigContext
	ToolSessionID        string
	CandidateFingerprint string
}

type EgoBrowserClaimReceipt struct {
	OperationID          string                   `json:"operation_id"`
	SchemaVersion        int                      `json:"schema_version"`
	Operation            string                   `json:"operation"`
	Context              cliadapter.ConfigContext `json:"context"`
	Candidate            EgoBrowserCandidate      `json:"candidate"`
	CandidateFingerprint string                   `json:"candidate_fingerprint"`
	Status               string                   `json:"status"`
	LocalAdmission       string                   `json:"local_admission"`
}

func EgoBrowserCandidatesSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserCandidatesMethod, "read", "user", "")
}

func EgoBrowserClaimPlanSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserClaimPlanMethod, "read", "user", "")
}

func EgoBrowserClaimControlSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserClaimControlMethod, "write", "user", "exact_request")
}

func LoadEgoBrowserCandidates(ctx context.Context, client cliadapter.Client, input EgoBrowserCandidatesInput) (EgoBrowserCandidates, error) {
	if !validEgoClaimContext(input.Expected, client.Home) {
		return EgoBrowserCandidates{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserCandidates{}, err
	}
	if !EgoBrowserCandidatesSupported(caps) {
		return EgoBrowserCandidates{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, EgoBrowserCandidatesMethod, struct {
		Expected cliadapter.ConfigContext `json:"expected_context"`
	}{input.Expected}, &raw); err != nil {
		return EgoBrowserCandidates{}, err
	}
	var result EgoBrowserCandidates
	if decodeStrict(raw, &result) != nil || result.SchemaVersion != 1 || result.Operation != EgoBrowserCandidatesMethod || !sameEgoContext(result.Context, input.Expected) || len(result.Items) > 128 {
		return EgoBrowserCandidates{}, fault.CLIProtocol
	}
	for _, item := range result.Items {
		if !validEgoCandidate(item) {
			return EgoBrowserCandidates{}, fault.CLIProtocol
		}
	}
	return result, nil
}

func LoadEgoBrowserClaimPlan(ctx context.Context, client cliadapter.Client, input EgoBrowserClaimPlanInput) (EgoBrowserClaimPlan, error) {
	if !validEgoClaimContext(input.Expected, client.Home) || !validIdentifier(input.ToolSessionID) || !validFingerprint(input.CandidateFingerprint) {
		return EgoBrowserClaimPlan{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserClaimPlan{}, err
	}
	if !EgoBrowserClaimPlanSupported(caps) {
		return EgoBrowserClaimPlan{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := struct {
		Expected             cliadapter.ConfigContext `json:"expected_context"`
		ToolSessionID        string                   `json:"tool_session_id"`
		CandidateFingerprint string                   `json:"candidate_fingerprint"`
	}{input.Expected, input.ToolSessionID, input.CandidateFingerprint}
	var raw json.RawMessage
	if err := client.Call(ctx, EgoBrowserClaimPlanMethod, params, &raw); err != nil {
		return EgoBrowserClaimPlan{}, err
	}
	var result EgoBrowserClaimPlan
	if decodeStrict(raw, &result) != nil || result.SchemaVersion != 1 || result.Operation != EgoBrowserClaimPlanMethod || !sameEgoContext(result.Context, input.Expected) || result.Candidate.ToolSessionID != input.ToolSessionID || result.CandidateFingerprint != input.CandidateFingerprint || result.LocalAdmission != "ready" || !result.RequiresConfirmation || !result.Allowed || !validEgoCandidate(result.Candidate) || result.Candidate.BindingID != nil || result.Candidate.CurrentEgoBrowserDeviceID != nil || !result.Candidate.Controllable {
		return EgoBrowserClaimPlan{}, fault.CLIProtocol
	}
	return result, nil
}

func ControlEgoBrowserClaim(ctx context.Context, client cliadapter.Client, input EgoBrowserClaimControlInput) (EgoBrowserClaimReceipt, error) {
	if !input.Confirmed || !validEgoClaimContext(input.Expected, client.Home) || !validIdentifier(input.ToolSessionID) || !validFingerprint(input.CandidateFingerprint) {
		return EgoBrowserClaimReceipt{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserClaimReceipt{}, err
	}
	if !EgoBrowserClaimControlSupported(caps) {
		return EgoBrowserClaimReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := struct {
		Confirmed            bool                     `json:"confirmed"`
		Expected             cliadapter.ConfigContext `json:"expected_context"`
		ToolSessionID        string                   `json:"tool_session_id"`
		CandidateFingerprint string                   `json:"candidate_fingerprint"`
	}{true, input.Expected, input.ToolSessionID, input.CandidateFingerprint}
	var receipt EgoBrowserClaimReceipt
	if err := client.Call(ctx, EgoBrowserClaimControlMethod, params, &receipt); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return EgoBrowserClaimReceipt{}, err
		}
		return EgoBrowserClaimReceipt{}, fault.EgoBrowserClaimOutcomeUnknown
	}
	if receipt.OperationID == "" || receipt.SchemaVersion != 1 || receipt.Operation != EgoBrowserClaimControlMethod || !sameEgoContext(receipt.Context, input.Expected) || receipt.Candidate.ToolSessionID != input.ToolSessionID || receipt.CandidateFingerprint != input.CandidateFingerprint || receipt.Status != "claim_submitted" || receipt.LocalAdmission != "open" || !validEgoCandidate(receipt.Candidate) {
		return EgoBrowserClaimReceipt{}, fault.EgoBrowserClaimOutcomeUnknown
	}
	return receipt, nil
}

func validEgoClaimContext(value cliadapter.ConfigContext, home string) bool {
	return validEgoContext(value, home) && value.ServerURL != nil
}

func validFingerprint(value string) bool {
	return value != "" && len(value) <= 2048 && !strings.ContainsFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f })
}

func validEgoCandidate(value EgoBrowserCandidate) bool {
	return validIdentifier(value.ToolSessionID) && validIdentifier(value.ToolType) && validIdentifier(value.ToolAccountID) && validIdentifier(value.WorkspaceID) && validIdentifier(value.ProjectKey) && validText(value.DisplayName, 256) && validText(value.Status, 64) && validIdentifier(value.NodeID) && validIdentifier(value.RuntimeBackend) && optionalText(value.CurrentEgoBrowserDeviceID, 256) && optionalText(value.CurrentEgoBrowserDeviceName, 256) && optionalText(value.BindingID, 256)
}
