package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// FindFreeLoopbackPort asks the host for an available loopback TCP port.
//
// The desktop control protocol deliberately carries an explicit local port,
// while the human CLI accepts the more convenient "auto" spelling.  Keeping
// the allocation here lets the Go UI offer the same behavior without sending
// a zero port through the device-scoped protocol (zero is rejected there).
// The returned port is only a short-lived reservation: the supervised CLI
// process binds it immediately afterwards and must still handle a race with
// another local process claiming it first.
func FindFreeLoopbackPort() (uint16, error) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("find free loopback port: %w", err)
	}
	address, ok := listener.Addr().(*net.TCPAddr)
	if !ok || address.Port < 1 || address.Port > 65535 {
		_ = listener.Close()
		return 0, errors.New("find free loopback port: invalid listener address")
	}
	if err := listener.Close(); err != nil {
		return 0, fmt.Errorf("release free loopback port: %w", err)
	}
	return uint16(address.Port), nil
}

// ResolveLocalPort accepts the human CLI's explicit numeric form and its
// "auto" form. Blank input is also auto so the desktop form can provide a
// useful default without inventing a control-protocol zero value.
func ResolveLocalPort(value string) (uint16, error) {
	value = strings.TrimSpace(value)
	if value == "" || strings.EqualFold(value, "auto") {
		return FindFreeLoopbackPort()
	}
	port, err := strconv.ParseUint(value, 10, 16)
	if err != nil || port == 0 {
		if err == nil {
			err = errors.New("local port must be between 1 and 65535")
		}
		return 0, err
	}
	return uint16(port), nil
}

// The Rust adapter owns the local caller-key journal, listener and one-shot
// connection token. Go only submits reviewed, context-bound intents.
const (
	PortForwardListMethod      = "port_forwards.list"
	PortForwardCreateMethod    = "port_forwards.create"
	PortForwardStopMethod      = "port_forwards.stop"
	PortForwardReconnectMethod = "port_forwards.reconnect"
)

func PortForwardListSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, PortForwardListMethod, "read", "device", "")
}

func PortForwardCreateSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, PortForwardCreateMethod, "write", "device", "exact_request")
}

func PortForwardStopSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, PortForwardStopMethod, "write", "device", "exact_request")
}

func PortForwardReconnectSupported(c cliadapter.Capabilities) bool {
	return supportsDesktopOperation(c, PortForwardReconnectMethod, "write", "device", "exact_request")
}

func ListPortForwards(ctx context.Context, client cliadapter.Client, expected cliadapter.ConfigContext) (PortForwardList, error) {
	expected = accountContextSnapshot(expected)
	if !validPortForwardContext(expected, client.Home) {
		return PortForwardList{}, fault.CLIContextChanged
	}
	caps, err := portForwardCapabilities(ctx, client)
	if err != nil {
		return PortForwardList{}, err
	}
	if !PortForwardListSupported(caps) {
		return PortForwardList{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, PortForwardListMethod, PortForwardListInput{Expected: expected}, &raw); err != nil {
		return PortForwardList{}, err
	}
	result, err := decodePortForwardList(raw)
	if err != nil {
		return PortForwardList{}, err
	}
	if !sameDefaultContext(result.Context, expected) || result.Items == nil || len(result.Items) > 256 || duplicatePortForwardIDs(result.Items) {
		return PortForwardList{}, fault.CLIProtocol
	}
	for _, item := range result.Items {
		if !validPortForward(item) {
			return PortForwardList{}, fault.CLIProtocol
		}
	}
	return result, nil
}

func CreatePortForward(ctx context.Context, client cliadapter.Client, input PortForwardCreateInput) (PortForwardReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if err := validatePortForwardCreate(input, client.Home); err != nil {
		return PortForwardReceipt{}, err
	}
	caps, err := portForwardCapabilities(ctx, client)
	if err != nil {
		return PortForwardReceipt{}, err
	}
	if !PortForwardCreateSupported(caps) {
		return PortForwardReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, PortForwardCreateMethod, input, &raw); err != nil {
		return PortForwardReceipt{}, portForwardMutationError(err)
	}
	receipt, err := decodePortForwardReceipt(raw)
	if err != nil || !sameDefaultContext(receipt.Context, input.Expected) || receipt.Forward.SessionID != input.SessionID || (input.ExpectedNodeID != "" && receipt.Forward.NodeID != input.ExpectedNodeID) || receipt.Forward.RemotePort != input.RemotePort || receipt.Forward.RequestedLocalPort != input.LocalPort {
		return PortForwardReceipt{}, fault.PortForwardOutcomeUnknown
	}
	return receipt, nil
}

func StopPortForward(ctx context.Context, client cliadapter.Client, input PortForwardActionInput) (PortForwardReceipt, error) {
	return portForwardAction(ctx, client, PortForwardStopMethod, PortForwardStopSupported, input)
}

func ReconnectPortForward(ctx context.Context, client cliadapter.Client, input PortForwardActionInput) (PortForwardReceipt, error) {
	return portForwardAction(ctx, client, PortForwardReconnectMethod, PortForwardReconnectSupported, input)
}

func portForwardAction(ctx context.Context, client cliadapter.Client, method string, supported func(cliadapter.Capabilities) bool, input PortForwardActionInput) (PortForwardReceipt, error) {
	input.Expected = accountContextSnapshot(input.Expected)
	if err := validatePortForwardAction(input, client.Home); err != nil {
		return PortForwardReceipt{}, err
	}
	caps, err := portForwardCapabilities(ctx, client)
	if err != nil {
		return PortForwardReceipt{}, err
	}
	if !supported(caps) {
		return PortForwardReceipt{}, &cliadapter.OperationError{Code: "unsupported_method"}
	}
	var raw json.RawMessage
	if err := client.Call(ctx, method, input, &raw); err != nil {
		return PortForwardReceipt{}, portForwardMutationError(err)
	}
	receipt, err := decodePortForwardReceipt(raw)
	if err != nil || !sameDefaultContext(receipt.Context, input.Expected) || receipt.Forward.ID != input.ForwardID || (input.ExpectedSessionID != "" && receipt.Forward.SessionID != input.ExpectedSessionID) || (input.ExpectedNodeID != "" && receipt.Forward.NodeID != input.ExpectedNodeID) {
		return PortForwardReceipt{}, fault.PortForwardOutcomeUnknown
	}
	return receipt, nil
}

func portForwardCapabilities(ctx context.Context, client cliadapter.Client) (cliadapter.Capabilities, error) {
	var caps cliadapter.Capabilities
	if err := client.Call(ctx, "capabilities", struct{}{}, &caps); err != nil {
		return caps, err
	}
	if caps.Validate() != nil || !caps.MatchesHostPlatform() {
		return caps, fault.CLIProtocol
	}
	return caps, nil
}

func validatePortForwardCreate(input PortForwardCreateInput, home string) error {
	if input.SchemaVersion != 1 || !input.Confirmed || !validPortForwardContext(input.Expected, home) || !validAccountImportRequestID(input.SessionID) || !validCanonicalIDString(input.ExpectedNodeID) || !validDefaultIDString(input.ExpectedRuntimeBackend) || !validDefaultTool(input.ClientInstanceID) || (input.CallerRequestID != "" && !validAccountImportRequestID(input.CallerRequestID)) || input.RemotePort == 0 || input.LocalPort == 0 || (input.TTLSeconds != nil && (*input.TTLSeconds < 60 || *input.TTLSeconds > 604800)) {
		return fault.PortForwardInputInvalid
	}
	return nil
}

func validatePortForwardAction(input PortForwardActionInput, home string) error {
	if input.SchemaVersion != 1 || !input.Confirmed || !validPortForwardContext(input.Expected, home) || !validAccountImportRequestID(input.ForwardID) || !validCanonicalIDString(input.ExpectedSessionID) || !validCanonicalIDString(input.ExpectedNodeID) {
		return fault.PortForwardInputInvalid
	}
	return nil
}

// validCanonicalIDString matches the CLI's optional UUID fields: an omitted
// expected identity is allowed, while a supplied one must be a canonical,
// non-zero UUID. This keeps port-forward stop/reconnect binding checks from
// accepting a human slug that Rust would reject after the review.
func validCanonicalIDString(value string) bool {
	return value == "" || validCanonicalUUID(value)
}

func validPortForwardContext(value cliadapter.ConfigContext, home string) bool {
	return value.Home == home && validDefaultContext(value) && value.ServerURL != nil && value.DeviceID != nil
}

func decodePortForwardReceipt(raw []byte) (PortForwardReceipt, error) {
	var receipt PortForwardReceipt
	// Create responses include a one-shot connection token. Strip it before
	// decoding so the desktop never retains or displays credentials.
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return PortForwardReceipt{}, fault.CLIProtocol
	}
	delete(fields, "connection")
	clean, err := json.Marshal(fields)
	if err != nil {
		return PortForwardReceipt{}, fault.CLIProtocol
	}
	decoder := json.NewDecoder(strings.NewReader(string(clean)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&receipt) != nil || decoder.Decode(new(any)) != io.EOF || !validPortForward(receipt.Forward) || receipt.SchemaVersion != 1 || !validPortForwardReceiptMetadata(receipt) {
		return PortForwardReceipt{}, fault.CLIProtocol
	}
	return receipt, nil
}

func decodePortForwardList(raw json.RawMessage) (PortForwardList, error) {
	var result PortForwardList
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF {
		return PortForwardList{}, fault.CLIProtocol
	}
	return result, nil
}

func validPortForward(value PortForward) bool {
	// The Rust projection intentionally omits device_id and user_id. Device
	// binding is checked against the reviewed context before the projection is
	// returned, so requiring those private identifiers here would reject valid
	// receipts while leaking no additional identity into the UI.
	if !validCanonicalUUID(value.ID) || !validCanonicalUUID(value.SessionID) || !validCanonicalUUID(value.NodeID) || !validPortForwardText(value.ClientInstanceID, 128) || value.RemotePort == 0 || value.RequestedLocalPort == 0 || !validPortForwardText(value.Status, 64) || !validPortForwardText(value.ExpiresAt, 128) || !validPortForwardText(value.CreatedAt, 128) || !validPortForwardText(value.UpdatedAt, 128) {
		return false
	}
	if value.CallerRequestID != nil && !validAccountImportRequestID(*value.CallerRequestID) {
		return false
	}
	for _, optional := range []*string{value.LastConnectedAt, value.LeaseExpiresAt, value.StoppedAt} {
		if optional != nil && !validPortForwardText(*optional, 128) {
			return false
		}
	}
	if value.StopReason != nil && !validPortForwardText(*value.StopReason, 512) {
		return false
	}
	return true
}

func validPortForwardText(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.IndexFunc(value, func(r rune) bool { return r < 0x20 || r == 0x7f }) < 0
}

func validPortForwardReceiptMetadata(receipt PortForwardReceipt) bool {
	if receipt.RequestID != "" && !validAccountImportRequestID(receipt.RequestID) {
		return false
	}
	if receipt.NodeWireguardIP != "" && !validPortForwardText(receipt.NodeWireguardIP, 128) {
		return false
	}
	if receipt.SSHUser != "" && !validPortForwardText(receipt.SSHUser, 128) {
		return false
	}
	return true
}

func duplicatePortForwardIDs(items []PortForward) bool {
	seen := make(map[string]bool, len(items))
	for _, item := range items {
		if seen[item.ID] {
			return true
		}
		seen[item.ID] = true
	}
	return false
}

func portForwardMutationError(err error) error {
	var operation *cliadapter.OperationError
	if errors.As(err, &operation) {
		return err
	}
	return fault.PortForwardOutcomeUnknown
}
