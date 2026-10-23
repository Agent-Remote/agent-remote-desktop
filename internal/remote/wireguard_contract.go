package remote

import (
	"bytes"
	"encoding/json"
	"net/url"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

func decodeWireGuardPlan(raw json.RawMessage) (WireGuardConfigPlan, error) {
	var plan WireGuardConfigPlan
	if decodeWireGuardJSON(raw, &plan) != nil || !validWireGuardPlan(plan) {
		return WireGuardConfigPlan{}, fault.CLIProtocol
	}
	var required struct {
		Output struct {
			TargetIdentity json.RawMessage `json:"target_identity"`
		} `json:"output"`
	}
	if json.Unmarshal(raw, &required) != nil || len(required.Output.TargetIdentity) == 0 {
		return WireGuardConfigPlan{}, fault.CLIProtocol
	}
	return plan, nil
}

func validWireGuardPlan(plan WireGuardConfigPlan) bool {
	if plan.PlanVersion != 1 || !validWireGuardPath(plan.Home) || !validWireGuardContext(plan.Context) {
		return false
	}
	output := plan.Output
	if !validWireGuardPath(output.Path) || !validWireGuardPath(output.AnchorPath) || !validWireGuardIdentity(output.AnchorIdentity) || output.MissingDirectories == nil || len(output.MissingDirectories) > 128 {
		return false
	}
	parent := output.AnchorPath
	for _, component := range output.MissingDirectories {
		if component == "" || component == "." || component == ".." || strings.ContainsAny(component, "/\\") || strings.IndexFunc(component, unicode.IsControl) >= 0 {
			return false
		}
		parent = filepath.Join(parent, component)
	}
	if filepath.Dir(output.Path) != parent || output.Path == parent {
		return false
	}
	switch output.Action {
	case "create":
		return output.TargetIdentity == nil
	case "replace":
		return len(output.MissingDirectories) == 0 && output.TargetIdentity != nil && validWireGuardIdentity(*output.TargetIdentity)
	default:
		return false
	}
}

func validWireGuardPath(path string) bool {
	return path != "" && len(path) <= 32768 && filepath.IsAbs(path) && filepath.Clean(path) == path && strings.IndexFunc(path, unicode.IsControl) < 0
}

func validWireGuardContext(context WireGuardContext) bool {
	if context.DeviceID == "" || len(context.DeviceID) > 512 || strings.TrimSpace(context.DeviceID) != context.DeviceID || strings.IndexFunc(context.DeviceID, unicode.IsControl) >= 0 || len(context.ServerURL) > 8192 {
		return false
	}
	server, err := url.Parse(context.ServerURL)
	return err == nil && (server.Scheme == "https" || server.Scheme == "http") && server.Hostname() != "" && server.User == nil && server.RawQuery == "" && server.Fragment == "" && strings.IndexFunc(context.ServerURL, unicode.IsControl) < 0
}

func validWireGuardIdentity(identity string) bool {
	if len(identity) != 64 {
		return false
	}
	for _, character := range identity {
		if !(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f') {
			return false
		}
	}
	return true
}

func decodeWireGuardReceipt(raw json.RawMessage) (WireGuardConfigReceipt, error) {
	var receipt WireGuardConfigReceipt
	var required struct {
		Complete  *bool           `json:"complete"`
		PeerCount json.RawMessage `json:"peer_count"`
	}
	if decodeWireGuardJSON(raw, &receipt) != nil || json.Unmarshal(raw, &required) != nil || required.Complete == nil || len(required.PeerCount) == 0 || !validWireGuardReceipt(receipt) {
		return WireGuardConfigReceipt{}, fault.WireGuardOutcomeUnknown
	}
	return receipt, nil
}

func validWireGuardReceipt(receipt WireGuardConfigReceipt) bool {
	if receipt.OperationID == "" || !validWireGuardPath(receipt.Home) || !validWireGuardPath(receipt.OutputPath) || !validWireGuardContext(receipt.Context) || (receipt.PeerCount != nil && *receipt.PeerCount > 1024) {
		return false
	}
	keyReady := receipt.KeyState == "reused" || receipt.KeyState == "created"
	if receipt.Complete {
		return receipt.Code == "none" && keyReady && receipt.Enrollment == "confirmed" && receipt.Publication == "published" && receipt.PeerCount != nil && validWireGuardPeerReceipt(receipt)
	}
	switch receipt.Code {
	case "credential_unavailable", "wireguard_key_invalid":
		return receipt.KeyState == "not_checked" && receipt.Enrollment == "not_attempted" && receipt.Publication == "not_attempted" && receipt.PeerCount == nil
	case "wireguard_key_store_incomplete":
		return receipt.KeyState == "unknown" && receipt.Enrollment == "not_attempted" && receipt.Publication == "not_attempted" && receipt.PeerCount == nil
	case "wireguard_enrollment_rejected", "wireguard_enrollment_unknown":
		expected := "rejected"
		if receipt.Code == "wireguard_enrollment_unknown" {
			expected = "unknown"
		}
		return keyReady && receipt.Enrollment == expected && receipt.Publication == "not_attempted" && receipt.PeerCount == nil
	case "wireguard_config_unavailable", "wireguard_config_invalid":
		return keyReady && receipt.Enrollment == "confirmed" && receipt.Publication == "not_attempted" && receipt.PeerCount == nil && validWireGuardPeerReceipt(receipt)
	case "output_changed":
		return keyReady && receipt.Publication == "not_attempted" && ((receipt.Enrollment == "confirmed" && receipt.PeerCount != nil && validWireGuardPeerReceipt(receipt)) || (receipt.Enrollment == "not_attempted" && receipt.PeerCount == nil))
	case "wireguard_context_changed":
		return keyReady && (receipt.Enrollment == "not_attempted" || receipt.Enrollment == "confirmed") && receipt.Publication == "not_attempted" && receipt.PeerCount == nil
	case "wireguard_publication_incomplete":
		return keyReady && receipt.Enrollment == "confirmed" && (receipt.Publication == "failed" || receipt.Publication == "published") && receipt.PeerCount != nil
	default:
		return false
	}
}

func validWireGuardPeerReceipt(receipt WireGuardConfigReceipt) bool {
	if receipt.PeerID == nil || *receipt.PeerID == "" || len(*receipt.PeerID) > 128 || receipt.PeerVersion == nil || *receipt.PeerVersion == 0 {
		return false
	}
	for _, character := range *receipt.PeerID {
		if !(character >= 'a' && character <= 'z') && !(character >= 'A' && character <= 'Z') && !(character >= '0' && character <= '9') && character != '-' && character != '_' {
			return false
		}
	}
	return true
}

func decodeWireGuardJSON(raw json.RawMessage, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}
