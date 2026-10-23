package remote_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

const startupEvent = `{"schema_version":1,"kind":"port_forward_started","forward":{"id":"11111111-1111-4111-8111-111111111111","session_id":"22222222-2222-4222-8222-222222222222","node_id":"33333333-3333-4333-8333-333333333333","remote_port":4173,"requested_local_port":4173,"client_instance_id":"ci_abc","status":"active","bytes_up":0,"bytes_down":0,"connection_count":0,"expires_at":"2026-01-02T03:04:05Z","created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-02T03:04:05Z"},"local_port":4173,"listener":{"state":"bound","addresses":["127.0.0.1:4173"]},"remote_ready":false}`

func TestForwardRunnerWaitsForStartupAndStopsChild(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	script := "#!/bin/sh\nprintf '%s\\n' '" + startupEvent + "'\ntrap 'exit 0' INT TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner, err := remote.StartForward(context.Background(), path, dir, remote.ForwardLaunchInput{SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173})
	if err != nil {
		t.Fatalf("start runner: %v", err)
	}
	listener := runner.Listener()
	if listener.State != "bound" || listener.LocalPort != 4173 {
		t.Fatalf("unexpected runner listener: %+v", listener)
	}
	stopCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := runner.Stop(stopCtx); err != nil {
		t.Fatalf("stop runner: %v", err)
	}
	select {
	case <-runner.Done():
	case <-time.After(time.Second):
		t.Fatal("runner did not report process exit")
	}
}

func TestStartForwardResumePassesExactIDToCLIWithoutCreateInputs(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	argsPath := filepath.Join(dir, "args")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" > '" + argsPath + "'\nprintf '%s\\n' '" + startupEvent + "'\ntrap 'exit 0' INT TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner, err := remote.StartForward(context.Background(), path, dir, remote.ForwardLaunchInput{
		ForwardID: "11111111-1111-4111-8111-111111111111",
		LocalPort: 4173,
	})
	if err != nil {
		t.Fatalf("start resumed runner: %v", err)
	}
	defer runner.Stop(context.Background())
	raw, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	args := string(raw)
	if !strings.Contains(args, "--forward-id 11111111-1111-4111-8111-111111111111") || strings.Contains(args, "--session") || strings.Contains(args, "--caller-request-id") {
		t.Fatalf("resume runner passed unsafe or create-only arguments: %q", args)
	}
}

func TestStartForwardRejectsStartupIdentityMismatch(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	// The child reports a valid, bound event for another session. Format-only
	// validation would accept it; the runner must bind it to the reviewed
	// session before publishing a listener.
	bad := strings.Replace(startupEvent,
		`"session_id":"22222222-2222-4222-8222-222222222222"`,
		`"session_id":"99999999-9999-4999-8999-999999999999"`, 1)
	script := "#!/bin/sh\nprintf '%s\\n' '" + bad + "'\ntrap 'exit 0' INT TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := remote.StartForward(context.Background(), path, dir, remote.ForwardLaunchInput{
		SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173,
	}); err == nil || !strings.Contains(err.Error(), "session") {
		t.Fatalf("mismatched startup session was accepted: %v", err)
	}
}

func TestStartForwardMatchesCreateIdentityFields(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	requestID := "44444444-4444-4444-8444-444444444444"
	clientID := "client-instance-1"
	event := strings.Replace(startupEvent, `"client_instance_id":"ci_abc"`, `"client_instance_id":"`+clientID+`","caller_request_id":"`+requestID+`"`, 1)
	script := "#!/bin/sh\nprintf '%s\\n' '" + event + "'\ntrap 'exit 0' INT TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	runner, err := remote.StartForward(context.Background(), path, dir, remote.ForwardLaunchInput{
		SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173,
		ExpectedNodeID: "33333333-3333-4333-8333-333333333333", ClientInstanceID: clientID, CallerRequestID: requestID,
	})
	if err != nil {
		t.Fatalf("matching create startup identity rejected: %v", err)
	}
	defer runner.Stop(context.Background())
}

func TestStartForwardRejectsCanceledContextEvenWhenStartupEventIsReady(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	// The event is emitted immediately, making the startup/cancellation select
	// race realistic. A canceled launch must never publish a runner that the
	// caller can miss during its shutdown barrier.
	script := "#!/bin/sh\nprintf '%s\\n' '" + startupEvent + "'\ntrap 'exit 0' INT TERM\nwhile :; do sleep 1; done\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if runner, err := remote.StartForward(ctx, path, dir, remote.ForwardLaunchInput{SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173}); err == nil || runner != nil {
		t.Fatalf("canceled launch published runner: runner=%v err=%v", runner, err)
	}
}

func TestStartForwardRejectsMalformedIdentityBeforeStartingHelper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	marker := filepath.Join(dir, "started")
	script := "#!/bin/sh\nprintf started > '" + marker + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	cases := []remote.ForwardLaunchInput{
		{ForwardID: "forward-1", LocalPort: 4173},
		{SessionID: "session-1", RemotePort: 4173, LocalPort: 4173},
		{SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173, ExpectedNodeID: "node-1"},
		{SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173, CallerRequestID: "request-1"},
		{SessionID: "22222222-2222-4222-8222-222222222222", RemotePort: 4173, LocalPort: 4173, ClientInstanceID: "" + strings.Repeat("x", 129)},
	}
	for index, input := range cases {
		if _, err := remote.StartForward(context.Background(), path, dir, input); err == nil {
			t.Fatalf("case %d malformed launch identity was accepted", index)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("malformed launch started helper: stat=%v", err)
	}
}

func TestStartForwardRejectsCreateFieldsOnResumeBeforeStartingHelper(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell helper is covered by the native Windows runtime validation")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "agent-remote-forward-helper")
	marker := filepath.Join(dir, "started")
	script := "#!/bin/sh\nprintf started > '" + marker + "'\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	ttl := uint64(600)
	cases := []remote.ForwardLaunchInput{
		{ForwardID: "11111111-1111-4111-8111-111111111111", RemotePort: 4173, LocalPort: 4173},
		{ForwardID: "11111111-1111-4111-8111-111111111111", SessionID: "22222222-2222-4222-8222-222222222222", LocalPort: 4173},
		{ForwardID: "11111111-1111-4111-8111-111111111111", TTLSeconds: &ttl, LocalPort: 4173},
		{ForwardID: "11111111-1111-4111-8111-111111111111", CallerRequestID: "44444444-4444-4444-8444-444444444444", LocalPort: 4173},
	}
	for index, input := range cases {
		if _, err := remote.StartForward(context.Background(), path, dir, input); err == nil {
			t.Fatalf("case %d resume accepted create-only fields", index)
		}
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("invalid resume started helper: stat=%v", err)
	}
}

func TestParseForwardStartupRejectsNonLoopbackListener(t *testing.T) {
	line := strings.Replace(startupEvent, `"127.0.0.1:4173"`, `"0.0.0.0:4173"`, 1)
	if _, err := remote.ParseForwardStartupEvent(line, 1234); err == nil {
		t.Fatal("wildcard listener address was accepted")
	}
}

func TestParseForwardStartupRejectsListenerPortMismatch(t *testing.T) {
	line := strings.Replace(startupEvent, `"127.0.0.1:4173"`, `"127.0.0.1:4174"`, 1)
	if _, err := remote.ParseForwardStartupEvent(line, 1234); err == nil {
		t.Fatal("listener address with a mismatched port was accepted")
	}
}

func TestParseForwardStartupEventRequiresBoundListenerAndSafeProjection(t *testing.T) {
	line := startupEvent
	listener, err := remote.ParseForwardStartupEvent(line, 42)
	if err != nil {
		t.Fatalf("parse startup event: %v", err)
	}
	if listener.State != "bound" || listener.LocalPort != 4173 || listener.PID != 42 || listener.RemoteReady {
		t.Fatalf("unexpected listener: %+v", listener)
	}
	if _, err := remote.ParseForwardStartupEvent(`{"schema_version":1,"kind":"port_forward_started","local_port":4173,"listener":{"state":"connecting","addresses":["127.0.0.1:4173"]}}`, 1); err == nil {
		t.Fatal("connecting listener was accepted")
	}
}

func TestParseForwardStartupEventRejectsCredentialFields(t *testing.T) {
	line := `{"schema_version":1,"kind":"port_forward_started","forward":{"id":"11111111-1111-4111-8111-111111111111","session_id":"22222222-2222-4222-8222-222222222222","node_id":"33333333-3333-4333-8333-333333333333","remote_port":4173,"requested_local_port":4173,"client_instance_id":"ci_abc","status":"active","expires_at":"2026-01-02T03:04:05Z","created_at":"2026-01-02T03:04:05Z","updated_at":"2026-01-02T03:04:05Z","connection":{"token":"secret"}},"local_port":4173,"listener":{"state":"bound","addresses":["127.0.0.1:4173"]}}`
	if _, err := remote.ParseForwardStartupEvent(line, 1); err == nil {
		t.Fatal("credential-bearing startup event was accepted")
	}
}

func TestParseForwardStartupRejectsNonUUIDRuntimeIdentity(t *testing.T) {
	line := strings.Replace(startupEvent,
		`"node_id":"33333333-3333-4333-8333-333333333333"`,
		`"node_id":"node-1"`, 1)
	if _, err := remote.ParseForwardStartupEvent(line, 1); err == nil {
		t.Fatal("startup event with a non-UUID node identity was accepted")
	}
}
