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

// DeviceCodeCall consumes the bounded device authorization stream. A progress
// callback runs before completion, on the caller's goroutine. It must not block.
// Cancellation only stops this local CLI process; it cannot revoke authorization.
func (client Client) DeviceCodeCall(ctx context.Context, params, result any, progress func(json.RawMessage) error) error {
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
	}{ProtocolVersion, id, "login.device_code", params})
	if err != nil || len(input) > 64*1024 {
		return fault.CLIProtocol
	}
	defer clear(input)
	ctx, cancel := context.WithTimeout(ctx, 1980*time.Second)
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
	if err = cmd.Start(); err != nil {
		return fault.CLIUnavailable
	}
	// Closing the reader also unblocks a descendant that inherited stdout after
	// CommandContext killed its parent. WaitDelay alone cannot unblock Scanner.
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
	var protocolErr error
	var terminal, seenProgress bool
	var payload json.RawMessage
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
			if seenProgress || envelope.Phase != "awaiting_approval" || len(envelope.Result) != 0 || envelope.Error != nil || len(envelope.Progress) == 0 || bytes.Equal(envelope.Progress, []byte("null")) {
				protocolErr = fault.CLIProtocol
				break
			}
			seenProgress = true
			if progress != nil {
				protocolErr = progress(envelope.Progress)
			}
		case "result":
			if !seenProgress || envelope.Phase != "completed" || envelope.Error != nil || len(envelope.Progress) != 0 || len(envelope.Result) == 0 || bytes.Equal(envelope.Result, []byte("null")) {
				protocolErr = fault.CLIProtocol
				break
			}
			var receipt struct {
				ID string `json:"operation_id"`
			}
			if json.Unmarshal(envelope.Result, &receipt) != nil || receipt.ID != id {
				protocolErr = fault.CLIProtocol
				break
			}
			payload, terminal = envelope.Result, true
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
			// A terminal record must be followed promptly by EOF and process exit.
			// Keep reading during this grace period so trailing records are rejected.
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
	if !terminal {
		return fault.CLIProtocol
	}
	if operation != nil {
		var exit *exec.ExitError
		if !errors.As(runErr, &exit) {
			return fault.CLIProtocol
		}
		return operation
	}
	if runErr != nil || json.Unmarshal(payload, result) != nil {
		return fault.CLIProtocol
	}
	return nil
}
