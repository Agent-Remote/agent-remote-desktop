package remote

import (
	"encoding/json"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

type WireGuardTargetObservation struct {
	LogicalName string                   `json:"logical_name"`
	State       string                   `json:"state"`
	Reason      *string                  `json:"reason"`
	StatusState string                   `json:"status_state"`
	Identity    *WireGuardTargetIdentity `json:"identity"`
}

// Exactly one platform variant is present. An identity is an observation, not
// proof of ownership, a runtime generation, or permission to modify a tunnel.
type WireGuardTargetIdentity struct {
	Kind    string
	Linux   *WireGuardLinuxTarget
	MacOS   *WireGuardMacOSTarget
	Windows *WireGuardWindowsTarget
}

type WireGuardLinuxTarget struct {
	Name      string  `json:"name"`
	Index     uint32  `json:"index"`
	Namespace string  `json:"namespace"`
	PublicKey *string `json:"public_key"`
}

type WireGuardMacOSTarget struct {
	Name            string  `json:"name"`
	Index           uint32  `json:"index"`
	MappingIdentity string  `json:"mapping_identity"`
	SocketIdentity  string  `json:"socket_identity"`
	PublicKey       *string `json:"public_key"`
}

type WireGuardWindowsTarget struct {
	ServiceName    string  `json:"service_name"`
	BinaryPath     string  `json:"binary_path"`
	ConfigPath     string  `json:"config_path"`
	ConfigIdentity *string `json:"config_identity"`
	ServiceState   uint32  `json:"service_state"`
	StartType      uint32  `json:"start_type"`
	Account        string  `json:"account"`
	PublicKey      *string `json:"public_key"`
}

func (identity WireGuardTargetIdentity) MarshalJSON() ([]byte, error) {
	switch {
	case identity.Kind == "linux_interface" && identity.Linux != nil && identity.MacOS == nil && identity.Windows == nil:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			*WireGuardLinuxTarget
		}{identity.Kind, identity.Linux})
	case identity.Kind == "macos_interface" && identity.MacOS != nil && identity.Linux == nil && identity.Windows == nil:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			*WireGuardMacOSTarget
		}{identity.Kind, identity.MacOS})
	case identity.Kind == "windows_service" && identity.Windows != nil && identity.Linux == nil && identity.MacOS == nil:
		return json.Marshal(struct {
			Kind string `json:"kind"`
			*WireGuardWindowsTarget
		}{identity.Kind, identity.Windows})
	default:
		return nil, fault.CLIProtocol
	}
}

func (identity *WireGuardTargetIdentity) UnmarshalJSON(raw []byte) error {
	var tag struct {
		Kind string `json:"kind"`
	}
	if json.Unmarshal(raw, &tag) != nil {
		return fault.CLIProtocol
	}
	*identity = WireGuardTargetIdentity{Kind: tag.Kind}
	switch tag.Kind {
	case "linux_interface":
		var value struct {
			Kind string `json:"kind"`
			WireGuardLinuxTarget
		}
		if _, ok := wireGuardObject(raw, "kind", "name", "index", "namespace", "public_key"); !ok || decodeWireGuardJSON(raw, &value) != nil {
			return fault.CLIProtocol
		}
		identity.Linux = &value.WireGuardLinuxTarget
	case "macos_interface":
		var value struct {
			Kind string `json:"kind"`
			WireGuardMacOSTarget
		}
		if _, ok := wireGuardObject(raw, "kind", "name", "index", "mapping_identity", "socket_identity", "public_key"); !ok || decodeWireGuardJSON(raw, &value) != nil {
			return fault.CLIProtocol
		}
		identity.MacOS = &value.WireGuardMacOSTarget
	case "windows_service":
		var value struct {
			Kind string `json:"kind"`
			WireGuardWindowsTarget
		}
		object, ok := wireGuardObject(raw, "kind", "service_name", "binary_path", "config_path", "config_identity", "service_state", "start_type", "account", "public_key")
		if !ok || !wireGuardRequiredNumbers(object, "service_state", "start_type") || decodeWireGuardJSON(raw, &value) != nil {
			return fault.CLIProtocol
		}
		identity.Windows = &value.WireGuardWindowsTarget
	default:
		return fault.CLIProtocol
	}
	return nil
}

func (identity WireGuardTargetIdentity) PublicKey() *string {
	switch {
	case identity.Linux != nil:
		return identity.Linux.PublicKey
	case identity.MacOS != nil:
		return identity.MacOS.PublicKey
	case identity.Windows != nil:
		return identity.Windows.PublicKey
	default:
		return nil
	}
}
