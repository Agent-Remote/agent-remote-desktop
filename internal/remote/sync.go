package remote

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"reflect"
)

func SyncOperationSupported(c cliadapter.Capabilities, method string) bool {
	if c.Validate() != nil || !c.MatchesHostPlatform() {
		return false
	}
	for _, op := range c.Operations {
		if op.Method != method || op.Credential != "device" {
			continue
		}
		if method == "sync.status" || method == "sync.recovery.preview" {
			return op.Effect == "read" && op.Confirmation == ""
		}
		return (validSyncMethod(method) || method == "sync.recovery.apply") && op.Effect == "write" && op.Confirmation == "exact_request"
	}
	return false
}
func syncPreflight(ctx context.Context, client cliadapter.Client, method string) error {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return fault.CLIProtocol
	}
	if !SyncOperationSupported(caps, method) {
		return &cliadapter.OperationError{Code: "unsupported_method"}
	}
	return nil
}
func GetSyncStatus(ctx context.Context, client cliadapter.Client, input SyncStatusInput) (SyncStatus, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if !validAccountContext(input.Expected, client.Home) || !validWireGuardPath(input.LocalPath) {
		return SyncStatus{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	if err := syncPreflight(ctx, client, "sync.status"); err != nil {
		return SyncStatus{}, err
	}
	var raw json.RawMessage
	if err := client.Call(ctx, "sync.status", input, &raw); err != nil {
		return SyncStatus{}, err
	}
	result, err := decodeSyncStatus(raw)
	if err != nil || !sameDefaultContext(result.Context, input.Expected) || result.Target.LocalPath != input.LocalPath {
		return SyncStatus{}, fault.CLIProtocol
	}
	return result, nil
}
func RunSyncAction(ctx context.Context, client cliadapter.Client, method string, input SyncActionInput) (SyncReceipt, error) {
	return RunSyncActionWithProgress(ctx, client, method, input, nil)
}

// RunSyncActionWithProgress runs a reviewed synchronization mutation while
// forwarding the CLI's bounded, secret-free progress events to the caller.
// The final SyncReceipt remains the only completion proof; progress is only a
// live UI projection and must not be persisted as a receipt.
func RunSyncActionWithProgress(ctx context.Context, client cliadapter.Client, method string, input SyncActionInput, progress func(cliadapter.ProgressEvent) error) (SyncReceipt, error) {
	if !input.Confirmed {
		return SyncReceipt{}, &cliadapter.OperationError{Code: "confirmation_required"}
	}
	input.Expected = accountContextSnapshot(input.Expected)
	input.ExpectedTarget = copySyncTarget(input.ExpectedTarget)
	if !validSyncMethod(method) || !validAccountContext(input.Expected, client.Home) || !validSyncTarget(input.ExpectedTarget) || (method != "sync.ensure" && input.ExpectedTarget.SyncSessionID == nil) {
		return SyncReceipt{}, &cliadapter.OperationError{Code: "invalid_params"}
	}
	if err := syncPreflight(ctx, client, method); err != nil {
		return SyncReceipt{}, err
	}
	var raw json.RawMessage
	var err error
	if progress == nil {
		err = client.Call(ctx, method, input, &raw)
	} else {
		err = client.CallWithProgress(ctx, method, input, &raw, progress)
	}
	if err != nil {
		var op *cliadapter.OperationError
		if errors.As(err, &op) {
			return SyncReceipt{}, err
		}
		return SyncReceipt{}, &cliadapter.OperationError{Code: "sync_outcome_unknown"}
	}
	result, err := decodeSyncReceipt(raw)
	if err != nil || !sameDefaultContext(result.Context, input.Expected) || !reflect.DeepEqual(result.Target, input.ExpectedTarget) || result.Method != method || result.DryRun != input.DryRun {
		return SyncReceipt{}, &cliadapter.OperationError{Code: "sync_outcome_unknown"}
	}
	return result, nil
}
func copySyncTarget(t SyncTarget) SyncTarget {
	t.WorkspaceID = copyDefaultText(t.WorkspaceID)
	t.SyncSessionID = copyDefaultText(t.SyncSessionID)
	t.Fingerprint = copyDefaultText(t.Fingerprint)
	return t
}
