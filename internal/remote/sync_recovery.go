package remote

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func PreviewSyncRecovery(ctx context.Context, client cliadapter.Client, input SyncRecoveryPreviewInput) (SyncRecoveryPlan, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if !validAccountContext(input.Expected, client.Home) || !validWireGuardPath(input.LocalPath) {
		return SyncRecoveryPlan{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	if err := syncPreflight(ctx, client, "sync.recovery.preview"); err != nil {
		return SyncRecoveryPlan{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "sync.recovery.preview", input, &raw); err != nil {
		return SyncRecoveryPlan{}, err
	}
	plan, err := decodeSyncRecoveryPlan(raw)
	if err != nil || !sameDefaultContext(plan.Context, input.Expected) || plan.Target.LocalPath != input.LocalPath {
		return SyncRecoveryPlan{}, fault.CLIProtocol
	}
	return plan, nil
}

func ApplySyncRecovery(ctx context.Context, client cliadapter.Client, input SyncRecoveryApplyInput) (SyncRecoveryReceipt, error) {
	if !input.Confirmed {
		return SyncRecoveryReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	// Snapshot every pointer/slice before starting a subprocess. Revalidate the
	// reviewed plan without deriving a new one or silently dropping any field.
	input.Expected = accountContextSnapshot(input.Expected)
	encoded, err := json.Marshal(input.ExpectedPlan)
	if err != nil {
		return SyncRecoveryReceipt{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	plan, err := decodeSyncRecoveryPlan(encoded)
	if err != nil || !plan.Recoverable || !validAccountContext(input.Expected, client.Home) || !sameDefaultContext(plan.Context, input.Expected) {
		return SyncRecoveryReceipt{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	input.ExpectedPlan = plan
	if err := syncPreflight(ctx, client, "sync.recovery.apply"); err != nil {
		return SyncRecoveryReceipt{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "sync.recovery.apply", input, &raw); err != nil {
		var op *cliadapter.OperationError
		if errors.As(err, &op) {
			return SyncRecoveryReceipt{}, err
		}
		return SyncRecoveryReceipt{}, &cliadapter.OperationError{Code: "sync_outcome_unknown"}
	}
	receipt, err := decodeSyncRecoveryReceipt(raw)
	if err != nil || !sameDefaultContext(receipt.Context, input.Expected) || !reflect.DeepEqual(receipt.Plan, input.ExpectedPlan) {
		return SyncRecoveryReceipt{}, &cliadapter.OperationError{Code: "sync_outcome_unknown"}
	}
	return receipt, nil
}
