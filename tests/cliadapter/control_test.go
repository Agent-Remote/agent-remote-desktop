package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestMain(m *testing.M) {
	if wireGuardDumpHelper() {
		return
	}
	if testutil.ControlHelper() {
		return
	}
	os.Exit(m.Run())
}

func TestControlRejectsUncorrelatedAndMalformedResponses(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"wrong-id", "wrong-version", "wrong-phase", "null-result", "result-failed-exit", "trailing", "raw-error", "oversized"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			var result any
			err := client.Call(context.Background(), "context", struct{}{}, &result)
			if err == nil {
				t.Fatal("accepted invalid response")
			}
			if strings.Contains(err.Error(), "private diagnostic") {
				t.Fatal("leaked CLI stderr")
			}
		})
	}
}

func TestControlUsesSelectedHomeAndKeepsQueryFailuresDistinct(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	t.Setenv("AGENT_REMOTE_HOME", filepath.Join(t.TempDir(), "wrong-home"))
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "permission")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	snapshot, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Config.Home != client.Home {
		t.Fatal("used inherited home")
	}
	var denied *cliadapter.OperationError
	if !errors.As(snapshot.AccountsError, &denied) || denied.Code != "permission_denied" || denied.Retryable {
		t.Fatalf("lost authorization error: %v", snapshot.AccountsError)
	}
	if snapshot.SessionsError != nil || len(snapshot.Sessions) != 2 {
		t.Fatalf("account denial hid sessions: %+v", snapshot)
	}
	calls, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	if string(calls) != "capabilities\ncontext\naccounts.list\nsessions.list\n" {
		t.Fatalf("unexpected retry or command: %q", calls)
	}
}

func TestControlDoesNotQueryUnconfiguredServerOrTreatMissingDataAsEmpty(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "unconfigured")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	_, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	calls, _ := os.ReadFile(log)
	if string(calls) != "capabilities\ncontext\n" {
		t.Fatalf("queried absent configuration: %q", calls)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "missing-items")
	snapshot, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(snapshot.SessionsError, fault.CLIProtocol) {
		t.Fatal("missing items misrepresented as empty list")
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "context-changed")
	snapshot, err = remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(snapshot.AccountsError, fault.CLIContextChanged) || !errors.Is(snapshot.SessionsError, fault.CLIContextChanged) || len(snapshot.Accounts) != 0 || len(snapshot.Sessions) != 0 {
		t.Fatal("mixed results from a changed CLI environment")
	}
}

func TestControlCancellationTerminatesWaitingProcess(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "slow")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	var result any
	err := client.Call(ctx, "context", struct{}{}, &result)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("unexpected cancellation: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("cancel did not stop process")
	}
}

func TestControlWithProgressDeliversBoundedEventsBeforeResult(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	var result map[string]any
	var events []cliadapter.ProgressEvent
	err := client.CallWithProgress(context.Background(), "context", struct{}{}, &result, func(event cliadapter.ProgressEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Phase != "started" || string(events[0].Data) != `{"operation":"context","stage":"started"}` {
		t.Fatalf("unexpected progress events: %+v", events)
	}
	if result["home"] == nil {
		t.Fatalf("missing terminal result: %#v", result)
	}
}

func TestControlWithProgressRejectsMalformedProgress(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"progress-invalid-phase", "progress-nonobject", "progress-too-many"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			var result any
			if err := client.CallWithProgress(context.Background(), "context", struct{}{}, &result, nil); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("accepted malformed progress: %v", err)
			}
		})
	}
}

func TestControlWithBuiltRustCLI(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY to verify the real Rust control protocol")
	}
	client := cliadapter.Client{Binary: binary, Home: filepath.Join(t.TempDir(), "uninitialized")}
	snapshot, err := remote.Load(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Config.Home != client.Home || snapshot.Config.ServerURL != nil || !snapshot.Capabilities.Supports("sessions.list") {
		t.Fatalf("unexpected real CLI result: %+v", snapshot)
	}
	if _, err := os.Stat(client.Home); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("discovery initialized state: %v", err)
	}
	var result any
	err = client.Call(context.Background(), "accounts.list", struct{}{}, &result)
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "authentication_required" {
		t.Fatalf("real CLI authentication error lost: %v", err)
	}
}

func TestControlWithProgressBuiltRustCLI(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY to verify the real Rust streaming control protocol")
	}
	client := cliadapter.Client{Binary: binary, Home: filepath.Join(t.TempDir(), "uninitialized")}
	var result struct {
		Home string `json:"home"`
	}
	var events []cliadapter.ProgressEvent
	err := client.CallWithProgress(context.Background(), "context", struct{}{}, &result, func(event cliadapter.ProgressEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Phase != "started" {
		t.Fatalf("unexpected started event: %+v", events)
	}
	if result.Home != client.Home {
		t.Fatalf("unexpected context home: %q", result.Home)
	}
}
