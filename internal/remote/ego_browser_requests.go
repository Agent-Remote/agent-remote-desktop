package remote

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const (
	EgoBrowserRequestsMethod      = "ego_browser.requests"
	EgoBrowserRequestCancelMethod = "ego_browser.request.cancel"
)

// EgoBrowserRequest is a bounded metadata projection. Browser payload bytes
// are represented by their size only and never cross the desktop boundary.
type EgoBrowserRequest struct {
	ID                string `json:"id"`
	BindingID         string `json:"binding_id"`
	Generation        uint64 `json:"generation"`
	BindingGeneration uint64 `json:"binding_generation"`
	RequestID         string `json:"request_id"`
	Sequence          uint64 `json:"sequence"`
	MessageType       string `json:"message_type"`
	PayloadBytes      uint64 `json:"payload_bytes"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
}

type EgoBrowserRequests struct {
	SchemaVersion     int                      `json:"schema_version"`
	Operation         string                   `json:"operation"`
	Context           cliadapter.ConfigContext `json:"context"`
	BindingID         string                   `json:"binding_id"`
	BindingGeneration uint64                   `json:"binding_generation"`
	Items             []EgoBrowserRequest      `json:"items"`
}

type EgoBrowserRequestListInput struct {
	Expected  cliadapter.ConfigContext
	BindingID string
}

type EgoBrowserRequestCancelInput struct {
	Confirmed         bool
	Expected          cliadapter.ConfigContext
	BindingID         string
	RequestID         string
	BindingGeneration uint64
	Generation        uint64
	Sequence          uint64
}

type EgoBrowserRequestCancelReceipt struct {
	OperationID   string                   `json:"operation_id"`
	SchemaVersion int                      `json:"schema_version"`
	Operation     string                   `json:"operation"`
	Context       cliadapter.ConfigContext `json:"context"`
	BindingID     string                   `json:"binding_id"`
	RequestID     string                   `json:"request_id"`
	Status        string                   `json:"status"`
	Item          EgoBrowserRequest        `json:"item"`
}

func EgoBrowserRequestsSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserRequestsMethod, "read", "user", "")
}

func EgoBrowserRequestCancelSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, EgoBrowserRequestCancelMethod, "write", "user", "exact_request")
}

func LoadEgoBrowserRequests(ctx context.Context, client cliadapter.Client, input EgoBrowserRequestListInput) (EgoBrowserRequests, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !validEgoContext(input.Expected, client.Home) || !validEgoRequestID(input.BindingID) {
		return EgoBrowserRequests{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserRequests{}, err
	}
	if !EgoBrowserRequestsSupported(caps) {
		return EgoBrowserRequests{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	params := struct {
		Expected  cliadapter.ConfigContext `json:"expected_context"`
		BindingID string                   `json:"binding_id"`
	}{input.Expected, input.BindingID}
	if err := client.Call(ctx, EgoBrowserRequestsMethod, params, &raw); err != nil {
		return EgoBrowserRequests{}, err
	}
	var result EgoBrowserRequests
	if decodeStrict(raw, &result) != nil || result.SchemaVersion != 1 || result.Operation != EgoBrowserRequestsMethod || result.BindingID != input.BindingID || !sameEgoContext(result.Context, input.Expected) || result.BindingGeneration == 0 || len(result.Items) > 128 {
		return EgoBrowserRequests{}, fault.CLIProtocol
	}
	for _, item := range result.Items {
		if !validEgoBrowserRequest(item, input.BindingID) {
			return EgoBrowserRequests{}, fault.CLIProtocol
		}
	}
	return result, nil
}

func CancelEgoBrowserRequest(ctx context.Context, client cliadapter.Client, input EgoBrowserRequestCancelInput) (EgoBrowserRequestCancelReceipt, error) {
	input.Expected.ServerURL = cloneText(input.Expected.ServerURL)
	input.Expected.DeviceID = cloneText(input.Expected.DeviceID)
	if !input.Confirmed || !validEgoContext(input.Expected, client.Home) || !validEgoRequestID(input.BindingID) || !validEgoRequestID(input.RequestID) || input.BindingGeneration == 0 || input.Generation == 0 {
		return EgoBrowserRequestCancelReceipt{}, fault.CLIProtocol
	}
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return EgoBrowserRequestCancelReceipt{}, err
	}
	if !EgoBrowserRequestCancelSupported(caps) {
		return EgoBrowserRequestCancelReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	params := struct {
		Confirmed         bool                     `json:"confirmed"`
		Expected          cliadapter.ConfigContext `json:"expected_context"`
		BindingID         string                   `json:"binding_id"`
		RequestID         string                   `json:"request_id"`
		BindingGeneration uint64                   `json:"binding_generation"`
		Generation        uint64                   `json:"generation"`
		Sequence          uint64                   `json:"sequence"`
	}{true, input.Expected, input.BindingID, input.RequestID, input.BindingGeneration, input.Generation, input.Sequence}
	var receipt EgoBrowserRequestCancelReceipt
	if err := client.Call(ctx, EgoBrowserRequestCancelMethod, params, &receipt); err != nil {
		var operation *cliadapter.OperationError
		if errors.As(err, &operation) {
			return EgoBrowserRequestCancelReceipt{}, err
		}
		return EgoBrowserRequestCancelReceipt{}, fault.EgoBrowserRequestOutcomeUnknown
	}
	if receipt.OperationID == "" || receipt.SchemaVersion != 1 || receipt.Operation != EgoBrowserRequestCancelMethod || !sameEgoContext(receipt.Context, input.Expected) || receipt.BindingID != input.BindingID || receipt.RequestID != input.RequestID || receipt.Status == "" || !validEgoBrowserRequest(receipt.Item, input.BindingID) || receipt.Item.ID != input.RequestID || receipt.Item.Generation != input.Generation || receipt.Item.BindingGeneration != input.BindingGeneration || receipt.Item.Sequence != input.Sequence {
		return EgoBrowserRequestCancelReceipt{}, fault.EgoBrowserRequestOutcomeUnknown
	}
	return receipt, nil
}

func validEgoBrowserRequest(item EgoBrowserRequest, bindingID string) bool {
	return item.BindingID == bindingID && validEgoRequestID(item.ID) && validEgoRequestID(item.RequestID) && item.Generation > 0 && item.BindingGeneration > 0 && validText(item.MessageType, 128) && validText(item.Status, 32) && validText(item.CreatedAt, 128)
}

// validEgoRequestID mirrors the requests desktop-control module's narrower
// identifier contract. Binding/request IDs are validated with <=128 ASCII
// alphanumerics, '-' and '_' there, while other ego-browser projections use a
// broader metadata identifier contract (which also permits '.'). Keeping a
// separate helper prevents a request cancel review from launching with an ID
// that the Rust bridge would reject after the user confirmed it.
func validEgoRequestID(value string) bool {
	if value == "" || len(value) > 128 {
		return false
	}
	for _, r := range value {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
