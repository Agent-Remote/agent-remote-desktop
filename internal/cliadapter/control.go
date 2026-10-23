package cliadapter

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

const ProtocolVersion = 1

// OperationError contains only protocol codes, never CLI stderr or secret material.
type OperationError struct {
	Code       string `json:"code"`
	Retryable  bool   `json:"retryable"`
	HTTPStatus *int   `json:"http_status,omitempty"`
}

func (e *OperationError) Error() string { return e.Code }

type Client struct{ Binary, Home string }

// Call runs one operation in an isolated CLI process without shell interpretation
// or automatic retries. Context cancellation ends local waiting only.
func (client Client) Call(ctx context.Context, method string, params, result any) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(nonce[:])
	input, err := json.Marshal(struct {
		Version int    `json:"protocol_version"`
		ID      string `json:"request_id"`
		Method  string `json:"method"`
		Params  any    `json:"params"`
	}{ProtocolVersion, id, method, params})
	if err != nil {
		return fault.CLIProtocol
	}
	if len(input) > 64*1024 {
		return fault.CLIProtocol
	}
	defer clear(input)
	timeout := 90 * time.Second
	if isRuntimeRecoveryMutation(method) || isImportMutation(method) || isSyncMutation(method) || isSessionCreationMutation(method) || isSessionStopMutation(method) || isPortForwardMutation(method) || isCredentialMutation(method) || isDeviceMutation(method) || isEgoBrowserRequestMutation(method) || method == "ego_browser.binding.control" || method == "ego_browser.claim.control" || method == "skills.change" || method == "skills.retry.control" || method == "login.password" || method == "logout" || method == "setup.local" || method == "doctor.repair" || method == "deps.repair" || method == "wireguard.config.apply" || method == "wireguard.tunnel.apply" || method == "accounts.default.set" || method == "accounts.default.clear" || method == "accounts.create" || method == "accounts.bind" || method == "accounts.verify" || method == "accounts.disable" {
		timeout = 150 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, client.Binary, "--desktop-control")
	cmd.WaitDelay = time.Second
	if strings.HasPrefix(method, "accounts.runtime_recovery.") || strings.HasPrefix(method, "accounts.import.") || isSyncMutation(method) || isSessionCreationMutation(method) || isSessionStopMutation(method) || isPortForwardMethod(method) || isCredentialMutation(method) || isDeviceMethod(method) || isEgoBrowserRequestMethod(method) || method == "ego_browser.status" || method == "ego_browser.candidates" || method == "ego_browser.claim.plan" || method == "ego_browser.claim.control" || method == "ego_browser.binding.plan" || method == "ego_browser.binding.control" || method == "skills.change" || method == "skills.retry.plan" || method == "skills.retry.control" || method == "sync.status" || method == "sync.recovery.preview" || method == "mutagen.environment.preview" || method == "diagnostics.inspect" || method == "dependencies.inspect" || method == "status.inspect" || method == "doctor.inspect" || method == "doctor.repair" || method == "deps.inspect" || method == "deps.repair" || method == "ssh.inspect" || method == "wireguard.config.apply" || method == "wireguard.tunnel.apply" || method == "wireguard.status" || method == "wireguard.tunnel.plan" || method == "accounts.default.get" || method == "accounts.default.set" || method == "accounts.default.clear" || method == "accounts.create" || method == "accounts.bind" || method == "accounts.verify" || method == "accounts.disable" || method == "accounts.status" {
		configureDiagnosticCancellation(cmd)
	}
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if !strings.EqualFold(key, "AGENT_REMOTE_HOME") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "AGENT_REMOTE_HOME="+client.Home)
	cmd.Stdin = bytes.NewReader(input)
	out := boundedBuffer{limit: maxSchemaBytes}
	out.onLimit = func() { _ = cmd.Process.Kill() }
	cmd.Stdout = &out
	runErr := cmd.Run()
	if out.exceeded {
		return fault.CLIProtocol
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if runErr != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return fault.CLIUnavailable
		}
	}
	var envelope struct {
		Version int             `json:"protocol_version"`
		ID      string          `json:"request_id"`
		Type    string          `json:"type"`
		Phase   string          `json:"phase"`
		Result  json.RawMessage `json:"result"`
		Error   *OperationError `json:"error"`
	}
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	if decoder.Decode(&envelope) != nil || envelope.Version != ProtocolVersion || envelope.ID != id {
		return fault.CLIProtocol
	}
	if decoder.Decode(new(any)) != io.EOF {
		return fault.CLIProtocol
	}
	switch envelope.Type {
	case "result":
		if runErr != nil || envelope.Phase != "completed" || envelope.Error != nil || len(envelope.Result) == 0 || bytes.Equal(envelope.Result, []byte("null")) {
			return fault.CLIProtocol
		}
		if json.Unmarshal(envelope.Result, result) != nil {
			return fault.CLIProtocol
		}
		if isRuntimeRecoveryMutation(method) || isImportMutation(method) || isSyncMutation(method) || isCredentialMutation(method) || isDeviceMutation(method) || isEgoBrowserRequestMutation(method) || method == "ego_browser.binding.control" || method == "ego_browser.claim.control" || method == "skills.change" || method == "skills.retry.control" || method == "login.password" || method == "logout" || method == "setup.local" || method == "wireguard.config.apply" || method == "wireguard.tunnel.apply" || method == "accounts.default.set" || method == "accounts.default.clear" || method == "accounts.create" || method == "accounts.bind" || method == "accounts.verify" || method == "accounts.disable" {
			var receipt struct {
				ID string `json:"operation_id"`
			}
			// The protected WireGuard broker returns its own digest-bound
			// operation ID, which is deliberately different from the outer
			// desktop request ID. The remote layer validates that opaque receipt
			// against its fixed digest shape; every other mutation must echo the
			// request ID to prove that the response belongs to this call.
			if json.Unmarshal(envelope.Result, &receipt) != nil || receipt.ID == "" || (method != "wireguard.tunnel.apply" && receipt.ID != id) {
				return fault.CLIProtocol
			}
		}
		return nil
	case "error":
		if runErr == nil || envelope.Phase != "failed" || envelope.Error == nil || envelope.Error.Code == "" || len(envelope.Result) != 0 {
			return fault.CLIProtocol
		}
		return envelope.Error
	default:
		return fault.CLIProtocol
	}
}

func isSessionCreationMutation(method string) bool { return method == "sessions.create" }

func isCredentialMutation(method string) bool {
	return method == "credentials.create" || method == "credentials.bind" || method == "credentials.unbind"
}

func isDeviceMutation(method string) bool {
	return method == "device.revoke" || method == "device.rotate_token"
}

func isEgoBrowserRequestMutation(method string) bool { return method == "ego_browser.request.cancel" }

func isEgoBrowserRequestMethod(method string) bool {
	return method == "ego_browser.requests" || isEgoBrowserRequestMutation(method)
}

func isDeviceMethod(method string) bool {
	return method == "device.status" || method == "device.local.status" || method == "device.local.plan" || isDeviceMutation(method)
}

func isSessionStopMutation(method string) bool { return method == "sessions.stop" }

func isPortForwardMutation(method string) bool {
	return method == "port_forwards.create" || method == "port_forwards.stop" || method == "port_forwards.reconnect"
}

func isPortForwardMethod(method string) bool {
	return isPortForwardMutation(method) || method == "port_forwards.list"
}

type Capabilities struct {
	Version            int                          `json:"protocol_version"`
	CLIVersion         string                       `json:"cli_version"`
	Platform           string                       `json:"platform"`
	AttachmentTransfer AttachmentTransferCapability `json:"attachment_transfer"`
	Operations         []Capability                 `json:"operations"`
	TerminalOperations []TerminalCapability         `json:"terminal_operations,omitempty"`
}

// AttachmentTransferCapability is the local CLI declaration for the
// identity-bound exact-session file/image bridge. It is only an implementation
// signal: the Server and Node still perform admission and lease validation
// when fclaude uploads a payload.
type AttachmentTransferCapability struct {
	Supported       bool  `json:"supported"`
	ProtocolVersion int   `json:"protocol_version"`
	MaxBytes        int64 `json:"max_bytes"`
	MaxEntries      int   `json:"max_entries"`
}

func (c AttachmentTransferCapability) Valid() bool {
	return c.Supported && c.ProtocolVersion == 1 && c.MaxBytes > 0 && c.MaxBytes <= 256*1024*1024 && c.MaxEntries > 0 && c.MaxEntries <= 32
}

// TerminalCapability describes an argv/PTY contract, never a control method.
type TerminalCapability struct {
	Method       string `json:"method"`
	Version      int    `json:"protocol_version"`
	Effect       string `json:"effect,omitempty"`
	Credential   string `json:"credential,omitempty"`
	Confirmation string `json:"confirmation,omitempty"`
	Workflow     string `json:"workflow,omitempty"`
}
type Capability struct {
	Method       string `json:"method"`
	Effect       string `json:"effect"`
	Credential   string `json:"credential"`
	Confirmation string `json:"confirmation,omitempty"`
}

func (c Capabilities) Supports(method string) bool {
	return c.SupportsEffect(method, "read")
}

func (c Capabilities) SupportsEffect(method, effect string) bool {
	for _, operation := range c.Operations {
		if operation.Method == method && operation.Effect == effect {
			return true
		}
	}
	return false
}

type ConfigContext struct {
	Home      string  `json:"home"`
	ServerURL *string `json:"server_url"`
	DeviceID  *string `json:"active_device_id"`
}
type Account struct {
	ID      string  `json:"id"`
	Tool    string  `json:"tool_type"`
	Name    string  `json:"display_name"`
	Status  string  `json:"status"`
	Region  string  `json:"region_code"`
	Backend *string `json:"runtime_backend"`
}

// CredentialProfile is the secret-free projection returned by the CLI's
// credentials.list control method. It intentionally contains no user ID,
// Git identity, tokens, or provider credentials.
type CredentialProfile struct {
	ID            string `json:"id"`
	DisplayName   string `json:"display_name"`
	Status        string `json:"status"`
	GitHubCLIMode string `json:"github_cli_mode"`
	SSHMode       string `json:"ssh_mode"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}
type RemoteSession struct {
	ID                           string  `json:"id"`
	Tool                         string  `json:"tool_type"`
	AccountID                    string  `json:"tool_account_id"`
	WorkspaceID                  string  `json:"workspace_id"`
	ProjectKey                   string  `json:"project_key"`
	LocalPath                    *string `json:"workspace_local_path"`
	RemotePath                   *string `json:"workspace_remote_path"`
	NodeID                       string  `json:"node_id"`
	Status                       string  `json:"status"`
	Backend                      string  `json:"runtime_backend"`
	RuntimeResourceID            *string `json:"runtime_resource_id"`
	StopTaskID                   *string `json:"stop_task_id"`
	SkillFinalizationOperationID *string `json:"skill_finalization_operation_id"`
}

// NodeRecord is the bounded, secret-free managed Node projection returned by
// nodes.list. SSH metadata is descriptive only; join codes and credentials
// never cross the desktop-control boundary.
type NodeRecord struct {
	ID                   string  `json:"id"`
	Name                 string  `json:"name"`
	Status               string  `json:"status"`
	SSHHost              *string `json:"ssh_host"`
	SSHPort              *uint16 `json:"ssh_port"`
	SSHUser              *string `json:"ssh_user"`
	Version              *string `json:"version"`
	EgoBrowserEnabled    bool    `json:"ego_browser_enabled"`
	ConfiguredEnabled    bool    `json:"configured_enabled"`
	EffectiveEnabled     bool    `json:"effective_enabled"`
	NodeExecutionAllowed bool    `json:"node_execution_allowed"`
	EnrollmentAdmission  bool    `json:"enrollment_admission"`
	ExecutionAdmission   bool    `json:"execution_admission"`
}

func isSyncMutation(method string) bool {
	switch method {
	case "sync.ensure", "sync.pause", "sync.resume", "sync.resolve", "sync.reset", "sync.recovery.apply", "mutagen.environment.apply":
		return true
	}
	return false
}

func isImportMutation(method string) bool {
	return method == "accounts.import.validate" || method == "accounts.import.apply" || method == "accounts.import.forget"
}

func isRuntimeRecoveryMutation(method string) bool {
	return method == "accounts.runtime_recovery.apply" || method == "accounts.runtime_recovery.forget"
}
