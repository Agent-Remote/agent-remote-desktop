package app

import (
	"context"
	"errors"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func sameRuntimeRecord(a, b remote.RuntimeRecoveryRecord) bool {
	return a.Context == b.Context && a.AccountID == b.AccountID && a.RequestID == b.RequestID
}
func runtimeRecoveryPreSubmitError(err error) bool {
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) {
		return false
	}
	switch operation.Code {
	case "invalid_params", "confirmation_required", "unsupported_method", "configuration_changed", "configuration_unavailable", "runtime_recovery_context_changed", "credential_unavailable", "authentication_required", "permission_denied":
		return true
	}
	return false
}
func (a *App) runtimeHasRetainedRequest() bool {
	p := &a.runtimeRecovery
	if p.authority == nil {
		return false
	}
	for _, record := range p.records {
		if record.reference.Context == p.authority.Context && record.reference.AccountID == p.accountID {
			return true
		}
	}
	return false
}
func (a *App) loadRuntimePending() {
	a.startRuntimeRecovery("pending", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		records, err := remote.ListRuntimeRecoveryPending(ctx, client)
		return runtimeRecoveryResult{pending: records, err: err}
	})
}
func (a *App) mergeRuntimePending(records []remote.RuntimeRecoveryRecord) {
	p := &a.runtimeRecovery
	p.pendingLoaded = true
	for _, reference := range records {
		found := false
		for i := range p.records {
			r := &p.records[i]
			if !sameRuntimeRecord(r.reference, reference) {
				continue
			}
			found = true
			if r.reference.Digest != reference.Digest || r.reference.Original != reference.Original || r.reference.Action != reference.Action || r.reference.Binding != nil && !reflect.DeepEqual(r.reference.Binding, reference.Binding) {
				p.err = fault.CLIProtocol
				break
			}
			r.reference = reference
			r.notSubmitted = false
			break
		}
		if !found {
			p.records = append(p.records, runtimeRecoveryRecord{reference: reference})
		}
	}
}
func (a *App) updateRuntimeRecord(result runtimeRecoveryResult) {
	for i := range a.runtimeRecovery.records {
		r := &a.runtimeRecovery.records[i]
		if !sameRuntimeRecord(r.reference, result.reference) {
			continue
		}
		r.err = result.err
		r.notSubmitted = result.notSubmitted
		if result.err == nil {
			b := result.receipt.Recovery.Binding
			if b.Original() != r.reference.Original || b.RecoveryAction() != r.reference.Action {
				r.err = fault.CLIProtocol
				return
			}
			r.receipt = &result.receipt
			if result.kind == "apply" {
				r.reference.Binding = &b
				r.reference.Phase = "accepted"
			}
		}
		return
	}
}
func (a *App) queryRuntimeRecord(record runtimeRecoveryRecord) {
	ref := record.reference
	a.runtimeRecovery.activeReference = ref
	binding := ref.Binding
	if record.receipt != nil {
		binding = &record.receipt.Recovery.Binding
	}
	input := remote.RuntimeRecoveryStatusInput{Expected: ref.Context, AccountID: ref.AccountID, RequestID: ref.RequestID, ExpectedDigest: &ref.Digest, ExpectedBinding: binding}
	a.startRuntimeRecovery("status", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		value, err := remote.GetRuntimeRecoveryStatus(ctx, client, input)
		return runtimeRecoveryResult{receipt: value, reference: ref, err: err}
	})
}
func (a *App) queryRuntimeManual() {
	p := &a.runtimeRecovery
	if p.authority == nil || !p.authority.Admin {
		return
	}
	input := remote.RuntimeRecoveryStatusInput{Expected: p.authority.Context, AccountID: p.manualAccount, RequestID: p.manualKey}
	if p.manualReceipt != nil {
		binding := p.manualReceipt.Recovery.Binding
		input.ExpectedBinding = &binding
	}
	a.startRuntimeRecovery("manual", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		value, err := remote.GetRuntimeRecoveryStatus(ctx, client, input)
		return runtimeRecoveryResult{receipt: value, err: err}
	})
}
func (a *App) reviewRuntimeForget(record runtimeRecoveryRecord) {
	ref := record.reference
	binding := ref.Binding
	if record.receipt != nil {
		binding = &record.receipt.Recovery.Binding
	}
	if ref.Phase == "rejected" {
		binding = nil
	}
	a.runtimeRecovery.forget = &remote.RuntimeRecoveryForgetInput{Confirmed: true, Expected: ref.Context, AccountID: ref.AccountID, RequestID: ref.RequestID, ExpectedDigest: &ref.Digest, ExpectedBinding: binding}
	a.runtimeRecovery.review = "forget"
}
func (a *App) forgetRuntimeRecord() {
	p := &a.runtimeRecovery
	if p.review != "forget" || p.forget == nil {
		return
	}
	input := *p.forget
	ref := remote.RuntimeRecoveryRecord{Context: input.Expected, AccountID: input.AccountID, RequestID: input.RequestID}
	a.startRuntimeRecovery("forget", func(ctx context.Context, client cliadapter.Client) runtimeRecoveryResult {
		_, err := remote.ForgetRuntimeRecovery(ctx, client, input)
		return runtimeRecoveryResult{reference: ref, err: err}
	})
}
