package cliadapter

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// ProgressEvent is a bounded, secret-free update emitted while a desktop
// control operation is running. The final result or error remains the only
// completion proof.
type ProgressEvent struct {
	Phase string
	Data  json.RawMessage
}

const (
	maxProgressEvents = 128
	maxProgressBytes  = 64 * 1024
)

// CallWithProgress is the streaming counterpart to Call. It opts into the
// desktop-control JSONL event stream and invokes progress for each validated
// progress envelope before consuming the terminal result/error envelope.
func (client Client) CallWithProgress(ctx context.Context, method string, params, result any, progress func(ProgressEvent) error) error {
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}
	id := hex.EncodeToString(nonce[:])
	input, err := json.Marshal(struct {
		Version int    `json:"protocol_version"`
		ID      string `json:"request_id"`
		Method  string `json:"method"`
		Stream  bool   `json:"stream_events"`
		Params  any    `json:"params"`
	}{ProtocolVersion, id, method, true, params})
	if err != nil || len(input) > 64*1024 {
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
	for _, env := range os.Environ() {
		key, _, _ := strings.Cut(env, "=")
		if !strings.EqualFold(key, "AGENT_REMOTE_HOME") {
			cmd.Env = append(cmd.Env, env)
		}
	}
	cmd.Env = append(cmd.Env, "AGENT_REMOTE_HOME="+client.Home)
	cmd.Stdin = bytes.NewReader(input)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fault.CLIUnavailable
	}
	if err := cmd.Start(); err != nil {
		return fault.CLIUnavailable
	}
	stopClosing := context.AfterFunc(ctx, func() { _ = stdout.Close() })
	defer stopClosing()

	var tailTimer *time.Timer
	var tailExpired atomic.Bool
	defer func() {
		if tailTimer != nil {
			tailTimer.Stop()
		}
	}()
	var operation *OperationError
	var terminalResult json.RawMessage
	var protocolErr error
	terminal := false
	eventCount := 0
	total := 0
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), maxSchemaBytes)
	for scanner.Scan() {
		total += len(scanner.Bytes()) + 1
		if total > maxSchemaBytes || terminal {
			protocolErr = fault.CLIProtocol
			break
		}
		var envelope struct {
			Version  int             `json:"protocol_version"`
			ID       string          `json:"request_id"`
			Type     string          `json:"type"`
			Phase    string          `json:"phase"`
			Progress json.RawMessage `json:"progress"`
			Result   json.RawMessage `json:"result"`
			Error    *OperationError `json:"error"`
		}
		if json.Unmarshal(scanner.Bytes(), &envelope) != nil || envelope.Version != ProtocolVersion || envelope.ID != id {
			protocolErr = fault.CLIProtocol
			break
		}
		switch envelope.Type {
		case "progress":
			if eventCount >= maxProgressEvents || !validProgressPhase(envelope.Phase) || len(envelope.Progress) == 0 || isJSONNull(envelope.Progress) || len(envelope.Result) != 0 || envelope.Error != nil || len(envelope.Progress) > maxProgressBytes || !jsonObject(envelope.Progress) {
				protocolErr = fault.CLIProtocol
				break
			}
			eventCount++
			if progress != nil {
				data := append(json.RawMessage(nil), envelope.Progress...)
				protocolErr = progress(ProgressEvent{Phase: envelope.Phase, Data: data})
			}
		case "result":
			if envelope.Phase != "completed" || envelope.Error != nil || len(envelope.Progress) != 0 || len(envelope.Result) == 0 || isJSONNull(envelope.Result) {
				protocolErr = fault.CLIProtocol
				break
			}
			if requiresReceipt(method) {
				var receipt struct {
					ID string `json:"operation_id"`
				}
				if json.Unmarshal(envelope.Result, &receipt) != nil || receipt.ID != id {
					protocolErr = fault.CLIProtocol
					break
				}
			}
			terminalResult, terminal = append(json.RawMessage(nil), envelope.Result...), true
		case "error":
			if envelope.Phase != "failed" || envelope.Error == nil || envelope.Error.Code == "" || len(envelope.Result) != 0 || len(envelope.Progress) != 0 {
				protocolErr = fault.CLIProtocol
				break
			}
			operation, terminal = envelope.Error, true
		default:
			protocolErr = fault.CLIProtocol
		}
		if protocolErr != nil {
			break
		}
		if terminal {
			tailTimer = time.AfterFunc(2*time.Second, func() {
				tailExpired.Store(true)
				_ = cmd.Process.Kill()
				_ = stdout.Close()
			})
		}
	}
	if protocolErr == nil && (scanner.Err() != nil || !terminal) {
		protocolErr = fault.CLIProtocol
	}
	if protocolErr != nil {
		_ = cmd.Process.Kill()
	}
	runErr := cmd.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if tailExpired.Load() {
		return fault.CLIProtocol
	}
	if protocolErr != nil {
		return protocolErr
	}
	if operation != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return fault.CLIProtocol
		}
		return operation
	}
	if runErr != nil || json.Unmarshal(terminalResult, result) != nil {
		return fault.CLIProtocol
	}
	return nil
}

func jsonObject(raw json.RawMessage) bool {
	var value map[string]json.RawMessage
	return json.Unmarshal(raw, &value) == nil && value != nil
}

func isJSONNull(raw []byte) bool { return bytes.Equal(bytes.TrimSpace(raw), []byte("null")) }

func validProgressPhase(phase string) bool {
	switch phase {
	case "started", "running", "awaiting_approval", "awaiting_input", "input_required", "awaiting_confirmation", "confirmation_required", "confirming", "retrying", "waiting", "stage":
		return true
	default:
		return false
	}
}

func requiresReceipt(method string) bool {
	return isRuntimeRecoveryMutation(method) || isImportMutation(method) || isSyncMutation(method) || isCredentialMutation(method) || isDeviceMutation(method) || isEgoBrowserRequestMutation(method) || method == "ego_browser.binding.control" || method == "ego_browser.claim.control" || method == "skills.change" || method == "skills.retry.control" || method == "login.password" || method == "logout" || method == "setup.local" || method == "wireguard.config.apply" || method == "wireguard.tunnel.apply" || method == "accounts.default.set" || method == "accounts.default.clear" || method == "accounts.create" || method == "accounts.bind" || method == "accounts.verify" || method == "accounts.disable"
}
