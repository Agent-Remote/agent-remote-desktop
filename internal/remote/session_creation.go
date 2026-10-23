package remote

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	SessionCreateMethod        = "sessions.create"
	SessionCreateRequestMethod = "sessions.create.request"
)

func SessionCreateSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SessionCreateMethod, "write", "device", "exact_request")
}

func SessionCreateQuerySupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, SessionCreateRequestMethod, "read", "device", "")
}

func CreateSession(ctx context.Context, client cliadapter.Client, input SessionCreateInput) (SessionCreateReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if err := validateSessionCreateInput(input, client.Home); err != nil {
		return SessionCreateReceipt{}, err
	}
	if !SessionCreateSupportedForClient(ctx, client) {
		return SessionCreateReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SessionCreateMethod, input, &raw); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return SessionCreateReceipt{}, err
		}
		return SessionCreateReceipt{}, fault.SessionCreateOutcomeUnknown
	}
	receipt, err := decodeSessionCreateReceipt(raw)
	if err != nil || !sameDefaultContext(receipt.Context, input.Expected) || receipt.ToolType != input.ToolType || receipt.AccountID != input.AccountID || receipt.WorkspaceID != input.WorkspaceID || receipt.ProjectKey != input.ProjectKey || receipt.ArgvDigest != input.ArgvDigest {
		return SessionCreateReceipt{}, fault.SessionCreateOutcomeUnknown
	}
	return receipt, nil
}

func QuerySessionCreate(ctx context.Context, client cliadapter.Client, input SessionCreateRequestInput) (SessionCreateReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if !validAccountImportRequestID(input.RequestID) || input.Expected.Home != client.Home || !validDefaultContext(input.Expected) || input.Expected.ServerURL == nil || input.Expected.DeviceID == nil {
		return SessionCreateReceipt{}, fault.SessionCreateInputInvalid
	}
	if !SessionCreateQuerySupportedForClient(ctx, client) {
		return SessionCreateReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, SessionCreateRequestMethod, input, &raw); err != nil {
		return SessionCreateReceipt{}, err
	}
	receipt, err := decodeSessionCreateReceipt(raw)
	if err != nil || receipt.RequestID != input.RequestID || !sameDefaultContext(receipt.Context, input.Expected) {
		return SessionCreateReceipt{}, fault.SessionCreateOutcomeUnknown
	}
	return receipt, nil
}

func SessionCreateSupportedForClient(ctx context.Context, client cliadapter.Client) bool {
	return sessionCreatePreflight(ctx, client, SessionCreateMethod)
}

func SessionCreateQuerySupportedForClient(ctx context.Context, client cliadapter.Client) bool {
	return sessionCreatePreflight(ctx, client, SessionCreateRequestMethod)
}

func sessionCreatePreflight(ctx context.Context, client cliadapter.Client, method string) bool {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return false
	}
	if method == SessionCreateMethod {
		return SessionCreateSupported(caps)
	}
	return SessionCreateQuerySupported(caps)
}

func decodeSessionCreateReceipt(raw []byte) (SessionCreateReceipt, error) {
	var receipt SessionCreateReceipt
	if err := json.Unmarshal(raw, &receipt); err != nil || !receipt.Valid() {
		return SessionCreateReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}
