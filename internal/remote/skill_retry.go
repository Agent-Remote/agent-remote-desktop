package remote

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	SkillsRetryPlanMethod    = "skills.retry.plan"
	SkillsRetryControlMethod = "skills.retry.control"
)

type SkillRetryTarget struct {
	AccountID     string `json:"account_id"`
	AttemptID     string `json:"attempt_id"`
	AttemptNumber uint64 `json:"attempt_number"`
}

type SkillRetryPlan struct {
	SchemaVersion   uint8                    `json:"schema_version"`
	Operation       string                   `json:"operation"`
	Context         cliadapter.ConfigContext `json:"context"`
	OperationID     string                   `json:"operation_id"`
	Status          string                   `json:"status"`
	Retryable       bool                     `json:"retryable"`
	Generation      int64                    `json:"generation"`
	Targets         []SkillRetryTarget       `json:"targets"`
	TargetCount     int                      `json:"target_count"`
	PlanFingerprint string                   `json:"plan_fingerprint"`
	RequiresConfirm bool                     `json:"requires_confirmation"`
}

type SkillRetryPlanInput struct {
	Expected    cliadapter.ConfigContext
	OperationID string
}

type SkillRetryControlInput struct {
	Confirmed               bool
	Expected                cliadapter.ConfigContext
	OperationID             string
	ExpectedGeneration      int64
	ExpectedPlanFingerprint string
}

type SkillRetryReceipt struct {
	SchemaVersion     uint8                     `json:"schema_version"`
	Operation         string                    `json:"operation"`
	OperationID       string                    `json:"operation_id"`
	SourceOperationID string                    `json:"source_operation_id"`
	RequestID         string                    `json:"request_id"`
	Context           cliadapter.ConfigContext  `json:"context"`
	Status            string                    `json:"status"`
	Accepted          bool                      `json:"accepted"`
	Generation        int64                     `json:"generation"`
	TargetCount       int                       `json:"target_count"`
	PlanFingerprint   string                    `json:"plan_fingerprint"`
	Targets           []SkillRetryReceiptTarget `json:"targets"`
}

type SkillRetryReceiptTarget struct {
	AccountID     string  `json:"account_id"`
	AttemptID     *string `json:"attempt_id"`
	AttemptNumber *uint64 `json:"attempt_number"`
	Readiness     string  `json:"readiness"`
	Retryable     *bool   `json:"retryable"`
}

func SkillsRetryPlanSupported(c cliadapter.Capabilities) bool {
	return supportsSkillRetryOperation(c, SkillsRetryPlanMethod, "read", "user", "")
}

func SkillsRetryControlSupported(c cliadapter.Capabilities) bool {
	return supportsSkillRetryOperation(c, SkillsRetryControlMethod, "write", "user", "exact_request")
}

func supportsSkillRetryOperation(c cliadapter.Capabilities, method, effect, credential, confirmation string) bool {
	return supportsDesktopOperation(c, method, effect, credential, confirmation)
}

func PlanSkillRetry(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext, input SkillRetryPlanInput) (SkillRetryPlan, error) {
	if !sameContextHome(expected, client.Home) || !validRetryOperationID(input.OperationID) {
		return SkillRetryPlan{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillRetryPlan{}, err
	}
	if !SkillsRetryPlanSupported(caps) {
		return SkillRetryPlan{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := map[string]any{"expected_context": expected, "operation_id": input.OperationID}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsRetryPlanMethod, params, &raw); err != nil {
		return SkillRetryPlan{}, err
	}
	var plan SkillRetryPlan
	if err := decodeStrict(raw, &plan); err != nil {
		return SkillRetryPlan{}, fault.CLIProtocol
	}
	if err := validateRetryPlan(plan, expected, input.OperationID); err != nil {
		return SkillRetryPlan{}, err
	}
	return plan, nil
}

func ControlSkillRetry(ctx context.Context, client cliadapter.Client, input SkillRetryControlInput) (SkillRetryReceipt, error) {
	if !input.Confirmed || !sameContextHome(input.Expected, client.Home) || !validRetryOperationID(input.OperationID) || input.ExpectedGeneration < 0 || !validFingerprint(input.ExpectedPlanFingerprint) {
		return SkillRetryReceipt{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return SkillRetryReceipt{}, err
	}
	if !SkillsRetryControlSupported(caps) {
		return SkillRetryReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := map[string]any{
		"confirmed":                 true,
		"expected_context":          input.Expected,
		"operation_id":              input.OperationID,
		"expected_generation":       input.ExpectedGeneration,
		"expected_plan_fingerprint": input.ExpectedPlanFingerprint,
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SkillsRetryControlMethod, params, &raw); err != nil {
		return SkillRetryReceipt{}, err
	}
	var receipt SkillRetryReceipt
	if err := decodeStrict(raw, &receipt); err != nil {
		return SkillRetryReceipt{}, fault.CLIProtocol
	}
	if receipt.SchemaVersion != 1 || receipt.Operation != SkillsRetryControlMethod || receipt.SourceOperationID != input.OperationID || receipt.OperationID == "" || receipt.RequestID == "" || !sameSkillContext(receipt.Context, input.Expected) || !receipt.Accepted || receipt.Generation != input.ExpectedGeneration || receipt.TargetCount != len(receipt.Targets) || !validFingerprint(receipt.PlanFingerprint) || receipt.PlanFingerprint != input.ExpectedPlanFingerprint {
		return SkillRetryReceipt{}, fault.CLIProtocol
	}
	if len(receipt.Targets) == 0 || len(receipt.Targets) > 10000 {
		return SkillRetryReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

func validateRetryPlan(plan SkillRetryPlan, expected cliadapter.ConfigContext, operationID string) error {
	if plan.SchemaVersion != 1 || plan.Operation != SkillsRetryPlanMethod || !sameSkillContext(plan.Context, expected) || plan.OperationID != operationID || plan.Status != "failed" || !plan.Retryable || plan.Generation < 0 || plan.TargetCount != len(plan.Targets) || plan.TargetCount < 1 || plan.TargetCount > 10000 || !plan.RequiresConfirm || !validFingerprint(plan.PlanFingerprint) {
		return fault.CLIProtocol
	}
	seen := make(map[string]struct{}, len(plan.Targets))
	for _, target := range plan.Targets {
		if !validRetryOperationID(target.AccountID) || !validRetryOperationID(target.AttemptID) || target.AttemptNumber == 0 {
			return fault.CLIProtocol
		}
		if _, ok := seen[target.AccountID]; ok {
			return fault.CLIProtocol
		}
		seen[target.AccountID] = struct{}{}
	}
	return nil
}

func sameContextHome(expected cliadapter.ConfigContext, home string) bool {
	return strings.TrimSpace(expected.Home) != "" && strings.TrimRight(expected.Home, "/\\") == strings.TrimRight(home, "/\\") && expected.ServerURL != nil && strings.TrimSpace(*expected.ServerURL) != ""
}

func validRetryOperationID(value string) bool {
	return canonicalUUID(value)
}
