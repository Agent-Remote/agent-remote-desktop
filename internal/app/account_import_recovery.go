package app

import (
	"context"
	"errors"
	"reflect"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

// These errors are returned before reservation/POST by the CLI apply contract.
// Unknown outcomes, transport failures and generic protocol errors never qualify.
func accountImportPreSubmitError(err error) bool {
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) {
		return false
	}
	switch operation.Code {
	case "account_import_source_changed", "account_import_empty", "invalid_params", "confirmation_required", "unsupported_method":
		return true
	default:
		return false
	}
}

func (a *App) loadAccountImportPending() {
	a.startAccountImport("pending", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		value, err := remote.ListAccountImportPending(ctx, client)
		return accountImportResult{pending: value, err: err}
	})
}
func (a *App) queryAccountImport(reference remote.AccountImportReference) {
	a.accountImport.activeReference = reference
	input := remote.AccountImportStatusInput{Expected: reference.Context, AccountID: reference.AccountID, RequestID: reference.RequestID, ExpectedDigest: reference.Digest, ExpectedTaskID: reference.TaskID}
	a.startAccountImport("status", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		value, err := remote.GetAccountImportStatus(ctx, client, input)
		return accountImportResult{receipt: value, reference: reference, err: err}
	})
}
func (a *App) updateAccountImportRecord(result accountImportResult) {
	p := &a.accountImport
	for i := range p.records {
		record := &p.records[i]
		if record.reference.RequestID != result.reference.RequestID {
			continue
		}
		record.err = result.err
		record.notSubmitted = result.notSubmitted
		if result.err == nil && record.reference.IntentFingerprint != result.receipt.IntentFingerprint {
			record.err = fault.CLIProtocol
			return
		}
		if result.err == nil {
			record.receipt = &result.receipt
			record.reference = result.receipt.Reference()
		}
		return
	}
}
func (a *App) forgetAccountImport() {
	p := &a.accountImport
	if p.forget == nil || p.review != "forget" {
		return
	}
	reference := *p.forget
	p.activeReference = reference
	input := remote.AccountImportForgetInput{Confirmed: true, Expected: reference.Context, AccountID: reference.AccountID, RequestID: reference.RequestID, ExpectedDigest: reference.Digest, ExpectedTaskID: reference.TaskID}
	a.startAccountImport("forget", func(ctx context.Context, client cliadapter.Client) accountImportResult {
		value, err := remote.ForgetAccountImport(ctx, client, input)
		return accountImportResult{reference: reference, forgotten: value, err: err}
	})
}

func (a *App) mergeAccountImportPending(pending []remote.AccountImportReference) {
	p := &a.accountImport
	p.pendingLoaded = true
	for _, reference := range pending {
		found := false
		for i := range p.records {
			old := p.records[i].reference
			if old.RequestID != reference.RequestID {
				continue
			}
			found = true
			if old.AccountID != reference.AccountID || old.Digest != reference.Digest || old.IntentFingerprint != reference.IntentFingerprint || !reflect.DeepEqual(old.Context, reference.Context) {
				p.err = fault.CLIProtocol
				break
			}
			if old.TaskID == nil {
				p.records[i].reference = reference
			}
			break
		}
		if !found {
			p.records = append(p.records, accountImportRecord{reference: reference})
		}
	}
}
func (a *App) accountImportHasRetainedRequest() bool {
	p := &a.accountImport
	for _, record := range p.records {
		if record.reference.AccountID == p.accountID && reflect.DeepEqual(record.reference.Context, p.expected) {
			return true
		}
	}
	return false
}
