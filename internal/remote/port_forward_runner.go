package remote

// This file owns the local process which holds a real loopback listener for a
// desktop initiated forward. The Rust CLI remains responsible for the SSH/
// HTTP2 tunnel, one shot credentials, TTL and bounded reconnect policy. Go
// only observes its secret free startup event and controls the process.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const forwardStartupTimeout = 30 * time.Second

type ForwardLaunchInput struct {
	// ForwardID resumes one exact existing server-side forward. When set, the
	// CLI uses its reconnect endpoint and never submits a create request.
	ForwardID              string
	SessionID              string
	RemotePort             uint16
	LocalPort              uint16
	TTLSeconds             *uint64
	ExpectedServerURL      string
	ExpectedDeviceID       string
	ExpectedNodeID         string
	ExpectedRuntimeBackend string
	// ClientInstanceID and CallerRequestID are carried for startup-event
	// matching. They are create-time identity, never a secret. On resume the
	// caller request is used only for validation and is intentionally omitted
	// from the CLI argv so reconnect cannot submit a new create intent.
	ClientInstanceID string
	CallerRequestID  string
}

type ForwardListener struct {
	Forward     PortForward
	LocalPort   uint16
	State       string
	PID         int
	RemoteReady bool
}

type ForwardRunner struct {
	cmd      *exec.Cmd
	done     chan struct{}
	startup  chan forwardStartup
	mu       sync.RWMutex
	listener ForwardListener
	waited   bool
	waitErr  error
}

type forwardStartup struct {
	Listener ForwardListener
	Err      error
}

// StartForward starts the normal Rust `forward` command and waits until the
// command has successfully bound its loopback listener. A returned runner
// represents a real child process; callers must retain it and stop it when the
// desktop explicitly exits.
func StartForward(ctx context.Context, binary, home string, input ForwardLaunchInput) (*ForwardRunner, error) {
	if err := validateForwardLaunchInput(binary, home, input); err != nil {
		return nil, err
	}
	args := []string{"--home", home, "--json", "forward"}
	if input.ForwardID != "" {
		args = append(args, "--forward-id", input.ForwardID, "--local-port", strconv.Itoa(int(input.LocalPort)))
	} else {
		args = append(args, strconv.Itoa(int(input.RemotePort)), "--session", input.SessionID, "--local-port", strconv.Itoa(int(input.LocalPort)))
	}
	if input.TTLSeconds != nil {
		args = append(args, "--ttl-seconds", strconv.FormatUint(*input.TTLSeconds, 10))
	}
	if input.ExpectedServerURL != "" {
		args = append(args, "--expected-server-url", input.ExpectedServerURL)
	}
	if input.ExpectedDeviceID != "" {
		args = append(args, "--expected-device-id", input.ExpectedDeviceID)
	}
	if input.ExpectedNodeID != "" {
		args = append(args, "--expected-node-id", input.ExpectedNodeID)
	}
	if input.ExpectedRuntimeBackend != "" {
		args = append(args, "--expected-runtime-backend", input.ExpectedRuntimeBackend)
	}
	if input.CallerRequestID != "" {
		if input.ForwardID == "" {
			args = append(args, "--caller-request-id", input.CallerRequestID)
		}
	}
	cmd := exec.Command(binary, args...)
	cmd.Env = filteredEnvironment(home)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	runner := &ForwardRunner{cmd: cmd, done: make(chan struct{}), startup: make(chan forwardStartup, 1)}
	go runner.readOutput(stdout)
	go drainOutput(stderr)
	go func() {
		err := cmd.Wait()
		runner.mu.Lock()
		runner.waited, runner.waitErr = true, err
		runner.mu.Unlock()
		close(runner.done)
	}()
	startupCtx, cancel := context.WithTimeout(ctx, forwardStartupTimeout)
	defer cancel()
	select {
	case event := <-runner.startup:
		// A startup event and context cancellation can become ready in the
		// same select window. Re-check the context before publishing the
		// runner so a close cannot receive a seemingly successful listener
		// after its launch worker was canceled.
		if err := startupCtx.Err(); err != nil {
			_ = runner.Stop(context.Background())
			return nil, err
		}
		if event.Err != nil {
			_ = runner.Stop(context.Background())
			return nil, event.Err
		}
		if err := validateForwardStartup(event.Listener, input); err != nil {
			_ = runner.Stop(context.Background())
			return nil, err
		}
		runner.mu.Lock()
		runner.listener = event.Listener
		runner.mu.Unlock()
		return runner, nil
	case <-runner.done:
		err := runner.waitError()
		if err == nil {
			return nil, errors.New("port-forward process exited before listener startup")
		}
		return nil, fmt.Errorf("port-forward process exited before listener startup: %w", safeProcessError(err))
	case <-startupCtx.Done():
		_ = runner.Stop(context.Background())
		return nil, fmt.Errorf("port-forward listener startup timed out: %w", startupCtx.Err())
	}
}

// validateForwardLaunchInput is intentionally run before exec.Cmd.Start. A
// malformed identity must never launch a helper with an argv that Rust would
// reject later, because a replacement helper could still bind a local port or
// emit a misleading startup event before the rejection is observed.
func validateForwardLaunchInput(binary, home string, input ForwardLaunchInput) error {
	if binary == "" || home == "" || input.LocalPort == 0 {
		return errors.New("invalid port-forward launch input")
	}
	if input.ForwardID != "" && !validCanonicalUUID(input.ForwardID) {
		return errors.New("invalid port-forward launch forward ID")
	}
	// The Rust CLI treats resume as a distinct mode: its exact forward ID
	// conflicts with every create-only identity selector (session, remote port,
	// TTL and caller key). The forward projection itself is the authoritative
	// binding for a resume; accepting a second session/port identity here would
	// let a stale review launch a helper before the CLI rejects its mixed argv.
	// Reject all create-only fields before spawning the helper.
	if input.ForwardID != "" && (input.SessionID != "" || input.RemotePort != 0 || input.TTLSeconds != nil || input.CallerRequestID != "") {
		return errors.New("invalid port-forward resume create fields")
	}
	if input.SessionID != "" && !validCanonicalUUID(input.SessionID) {
		return errors.New("invalid port-forward launch session ID")
	}
	if input.ForwardID == "" && (input.SessionID == "" || input.RemotePort == 0) {
		return errors.New("invalid port-forward launch create identity")
	}
	if input.ExpectedNodeID != "" && !validCanonicalUUID(input.ExpectedNodeID) {
		return errors.New("invalid port-forward launch node ID")
	}
	if input.CallerRequestID != "" && !validCanonicalUUID(input.CallerRequestID) {
		return errors.New("invalid port-forward launch caller request ID")
	}
	if input.ClientInstanceID != "" && !validPortForwardLaunchText(input.ClientInstanceID, 128) {
		return errors.New("invalid port-forward launch client instance ID")
	}
	if input.ExpectedRuntimeBackend != "" && !validPortForwardLaunchText(input.ExpectedRuntimeBackend, 64) {
		return errors.New("invalid port-forward launch runtime backend")
	}
	if input.TTLSeconds != nil && (*input.TTLSeconds < 60 || *input.TTLSeconds > 604800) {
		return errors.New("invalid port-forward launch TTL")
	}
	return nil
}

func validPortForwardLaunchText(value string, limit int) bool {
	return value != "" && len(value) <= limit && strings.IndexFunc(value, unicode.IsControl) < 0
}

// validateForwardStartup binds the real local listener to the exact intent
// reviewed by the desktop. The startup event is produced by the Rust CLI, but
// it is still an untrusted process boundary: a malformed or stale event must
// never make a different server-side forward appear attached to this pane.
func validateForwardStartup(listener ForwardListener, input ForwardLaunchInput) error {
	if listener.State != "bound" || listener.LocalPort == 0 || listener.Forward.ID == "" {
		return errors.New("invalid port-forward startup identity")
	}
	forward := listener.Forward
	if input.ForwardID != "" && forward.ID != input.ForwardID {
		return errors.New("port-forward startup ID does not match requested forward")
	}
	if input.SessionID != "" && forward.SessionID != input.SessionID {
		return errors.New("port-forward startup session does not match requested session")
	}
	if input.ExpectedNodeID != "" && forward.NodeID != input.ExpectedNodeID {
		return errors.New("port-forward startup node does not match requested node")
	}
	if input.RemotePort != 0 && forward.RemotePort != input.RemotePort {
		return errors.New("port-forward startup remote port does not match requested port")
	}
	if input.LocalPort == 0 || listener.LocalPort != input.LocalPort || forward.RequestedLocalPort != input.LocalPort {
		return errors.New("port-forward startup local port does not match requested port")
	}
	if input.ClientInstanceID != "" && forward.ClientInstanceID != input.ClientInstanceID {
		return errors.New("port-forward startup client instance does not match request")
	}
	if input.CallerRequestID != "" && (forward.CallerRequestID == nil || *forward.CallerRequestID != input.CallerRequestID) {
		return errors.New("port-forward startup caller request does not match request")
	}
	return nil
}

func filteredEnvironment(home string) []string {
	env := make([]string, 0, len(os.Environ())+1)
	for _, value := range os.Environ() {
		key, _, _ := strings.Cut(value, "=")
		if strings.EqualFold(key, "AGENT_REMOTE_HOME") {
			continue
		}
		env = append(env, value)
	}
	return append(env, "AGENT_REMOTE_HOME="+home)
}

func (r *ForwardRunner) readOutput(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 256*1024)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if listener, err := ParseForwardStartupEvent(string(line), r.cmd.Process.Pid); err == nil {
			r.sendStartup(forwardStartup{Listener: listener})
			return
		}
		if code := parseForwardError(string(line)); code != "" {
			r.sendStartup(forwardStartup{Err: errors.New(code)})
			return
		}
	}
	if err := scanner.Err(); err != nil {
		r.sendStartup(forwardStartup{Err: errors.New("port-forward output unavailable")})
	}
}

// ParseForwardStartupEvent validates the CLI's machine-readable startup
// event. It accepts only the schema and listener state needed by desktop and
// never exposes connection credentials.
func ParseForwardStartupEvent(line string, pid int) (ForwardListener, error) {
	var envelope struct {
		Kind      string          `json:"kind"`
		Schema    int             `json:"schema_version"`
		Forward   json.RawMessage `json:"forward"`
		LocalPort uint16          `json:"local_port"`
		Listener  struct {
			State     string   `json:"state"`
			Addresses []string `json:"addresses"`
		} `json:"listener"`
		RemoteReady bool `json:"remote_ready"`
	}
	envelopeDecoder := json.NewDecoder(strings.NewReader(line))
	envelopeDecoder.DisallowUnknownFields()
	if envelopeDecoder.Decode(&envelope) != nil || envelope.Kind != "port_forward_started" {
		return ForwardListener{}, errors.New("not a port-forward startup event")
	}
	if envelopeDecoder.Decode(new(any)) != io.EOF {
		return ForwardListener{}, errors.New("invalid port-forward startup event")
	}
	var forward PortForward
	decoder := json.NewDecoder(bytes.NewReader(envelope.Forward))
	decoder.DisallowUnknownFields()
	if envelope.Schema != 1 || envelope.LocalPort == 0 || envelope.Listener.State != "bound" || len(envelope.Listener.Addresses) == 0 || decoder.Decode(&forward) != nil || decoder.Decode(new(any)) != io.EOF || !validPortForward(forward) || !validLoopbackListenerAddresses(envelope.Listener.Addresses, envelope.LocalPort) {
		return ForwardListener{}, errors.New("invalid port-forward startup event")
	}
	return ForwardListener{Forward: forward, LocalPort: envelope.LocalPort, State: "bound", PID: pid, RemoteReady: envelope.RemoteReady}, nil
}

// validLoopbackListenerAddresses keeps the local listener guarantee at the
// process boundary. The Rust forward command intentionally binds only
// 127.0.0.1 (and may also bind ::1); a malformed or replaced helper must not
// be allowed to report a wildcard address that would expose the forward to
// the LAN. Every reported address is checked because a single unsafe dual
// stack listener is enough to violate the guarantee.
func validLoopbackListenerAddresses(addresses []string, expectedPort uint16) bool {
	if len(addresses) == 0 || expectedPort == 0 {
		return false
	}
	for _, address := range addresses {
		host, port, err := net.SplitHostPort(address)
		if err != nil || host == "" || port == "" {
			return false
		}
		parsedPort, err := strconv.Atoi(port)
		if err != nil || parsedPort != int(expectedPort) {
			return false
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			return false
		}
	}
	return true
}

func parseForwardError(line string) string {
	var value struct {
		Kind string `json:"kind"`
		Code string `json:"error_code"`
	}
	if json.Unmarshal([]byte(line), &value) != nil || value.Kind != "port_forward_error" {
		return ""
	}
	if value.Code == "" {
		return "port_forward_start_failed"
	}
	return value.Code
}

func (r *ForwardRunner) sendStartup(event forwardStartup) {
	select {
	case r.startup <- event:
	default:
	}
}

func drainOutput(reader io.Reader) { _, _ = io.Copy(io.Discard, reader) }

func safeProcessError(err error) error {
	if err == nil {
		return nil
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return fmt.Errorf("exit status %d", exit.ExitCode())
	}
	return errors.New("process wait failed")
}

func (r *ForwardRunner) Listener() ForwardListener {
	r.mu.RLock()
	defer r.mu.RUnlock()
	value := r.listener
	if r.waited {
		if r.waitErr != nil {
			value.State = "exited"
		} else {
			value.State = "stopped"
		}
	}
	return value
}

func (r *ForwardRunner) Done() <-chan struct{} { return r.done }

func (r *ForwardRunner) waitError() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return safeProcessError(r.waitErr)
}

// Stop asks the CLI to run its normal cleanup path (including the server stop
// request), then kills the child only if it does not exit promptly. This is
// deliberately process control, never a synthetic local status journal.
func (r *ForwardRunner) Stop(ctx context.Context) error {
	if r == nil || r.cmd == nil || r.cmd.Process == nil {
		return nil
	}
	if err := r.cmd.Process.Signal(os.Interrupt); err != nil {
		_ = r.cmd.Process.Kill()
	}
	select {
	case <-r.done:
		return r.waitError()
	case <-ctx.Done():
		_ = r.cmd.Process.Kill()
		return ctx.Err()
	case <-time.After(5 * time.Second):
		_ = r.cmd.Process.Kill()
		select {
		case <-r.done:
			return r.waitError()
		case <-time.After(time.Second):
			return errors.New("port-forward process did not stop")
		}
	}
}
