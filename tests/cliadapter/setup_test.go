package cliadapter_test

import (
	"context"
	"database/sql"
	"encoding/json"
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
	_ "modernc.org/sqlite"
)

func TestLocalSetupRequiresConfirmationAndMatchingHomeBeforeInvokingCLI(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	for _, input := range []remote.LocalSetupInput{
		{ExpectedHome: client.Home},
		{Confirmed: true, ExpectedHome: filepath.Join(client.Home, "other")},
		{Confirmed: true, ExpectedHome: "relative-home"},
	} {
		if _, err := remote.SetupLocal(context.Background(), client, input); err == nil {
			t.Fatal("accepted unconfirmed or changed target")
		}
	}
	if _, err := os.Stat(log); !os.IsNotExist(err) {
		t.Fatalf("invalid request invoked the CLI: %v", err)
	}
}

func TestLocalSetupChecksCapabilitiesBeforeMutation(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"setup-unsupported", "setup-readonly", "other-platform", "duplicate-capability", "invalid-effect"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			if _, err := remote.SetupLocal(context.Background(), client, remote.LocalSetupInput{Confirmed: true, ExpectedHome: client.Home}); err == nil || errors.Is(err, fault.LocalSetupUnknown) {
				t.Fatalf("preflight should reject without claiming an unknown mutation: %v", err)
			}
			data, err := os.ReadFile(log)
			if err != nil || string(data) != "capabilities\n" {
				t.Fatalf("preflight executed a mutation: %q %v", data, err)
			}
		})
	}
}

func TestLocalSetupPreservesConfirmedPreWriteFailure(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "setup-configuration-unavailable")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	_, err := remote.SetupLocal(context.Background(), client, remote.LocalSetupInput{Confirmed: true, ExpectedHome: client.Home})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "configuration_unavailable" || errors.Is(err, fault.LocalSetupUnknown) {
		t.Fatalf("known pre-write rejection became an unknown mutation: %v", err)
	}
	calls, readErr := os.ReadFile(log)
	if readErr != nil || string(calls) != "capabilities\nsetup.local\n" {
		t.Fatalf("pre-write failure was replayed: %q %v", calls, readErr)
	}
}

func TestLocalSetupPreservesPartialReceiptAndRejectsMalformedResultsWithoutRetry(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	for _, mode := range []string{"", "setup-partial", "setup-missing-complete", "setup-wrong-operation", "setup-wrong-home", "setup-wrong-path", "setup-inconsistent-complete", "setup-duplicate-step", "setup-unknown-code", "setup-unknown-status", "setup-truncated-steps"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			log := filepath.Join(t.TempDir(), "calls")
			captured := filepath.Join(t.TempDir(), "request")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			t.Setenv("DESKTOP_TEST_SETUP_RECEIPT", captured)
			result, err := remote.SetupLocal(context.Background(), client, remote.LocalSetupInput{Confirmed: true, ExpectedHome: client.Home})
			if mode == "" || mode == "setup-partial" {
				if err != nil || len(result.Steps) != 8 || result.Complete != (mode == "") {
					t.Fatalf("lost complete or partial receipt: %+v %v", result, err)
				}
				if mode == "setup-partial" && (result.Steps[6].Code != "unsupported_state_version" || result.Steps[7].Status != "applied") {
					t.Fatal("one failed step hid independent completed work")
				}
			} else if !errors.Is(err, fault.LocalSetupUnknown) {
				t.Fatalf("malformed mutation receipt should remain unknown: %v", err)
			}
			data, _ := os.ReadFile(log)
			if string(data) != "capabilities\nsetup.local\n" {
				t.Fatalf("unexpected retries or calls: %q", data)
			}
			data, err = os.ReadFile(captured)
			var input map[string]any
			if err != nil || json.Unmarshal(data, &input) != nil || len(input) != 2 || input["confirmed"] != true || input["expected_home"] != client.Home {
				t.Fatalf("mutation sent wrong scope: %s %v", data, err)
			}
		})
	}
}

func TestLocalSetupCancellationAfterDispatchDoesNotReplayOrClaimRollback(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	barrier := t.TempDir()
	started := filepath.Join(barrier, "started")
	t.Setenv("DESKTOP_TEST_SETUP_STARTED", started)
	t.Setenv("DESKTOP_TEST_SETUP_RELEASE", filepath.Join(barrier, "release"))
	log := filepath.Join(barrier, "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := remote.SetupLocal(ctx, client, remote.LocalSetupInput{Confirmed: true, ExpectedHome: client.Home})
		done <- err
	}()
	deadline := time.Now().Add(15 * time.Second)
	for {
		if _, err := os.Stat(started); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("setup never reached dispatch barrier")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, fault.LocalSetupUnknown) {
			t.Fatalf("canceled mutation lost its unknown outcome: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancellation retained CLI process")
	}
	data, _ := os.ReadFile(log)
	if strings.Count(string(data), "setup.local\n") != 1 {
		t.Fatalf("cancellation replayed local work: %q", data)
	}
}

func TestLocalSetupWithBuiltRustCLIPreservesStateAndRejectsFutureDatabase(t *testing.T) {
	binary := os.Getenv("DESKTOP_CLI_BINARY")
	if binary == "" {
		t.Skip("set DESKTOP_CLI_BINARY for the real local setup contract")
	}
	home := filepath.Join(t.TempDir(), "new-home")
	client := cliadapter.Client{Binary: binary, Home: home}
	input := remote.LocalSetupInput{Confirmed: true, ExpectedHome: home}
	first, err := remote.SetupLocal(context.Background(), client, input)
	if err != nil || !first.Complete {
		t.Fatalf("initial local setup: %+v %v", first, err)
	}
	config := []byte("invalid toml: untouched-fixture-marker")
	if err := os.WriteFile(filepath.Join(home, "config.toml"), config, 0600); err != nil {
		t.Fatal(err)
	}
	second, err := remote.SetupLocal(context.Background(), client, input)
	if err != nil || !second.Complete {
		t.Fatalf("existing valid metadata must remain usable despite unreadable configuration: %+v %v", second, err)
	}
	for _, step := range second.Steps {
		if step.Status != "ready" {
			t.Fatalf("repeated setup rewrote ready metadata: %+v", step)
		}
	}
	db, err := sql.Open("sqlite", filepath.Join(home, "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	third, err := remote.SetupLocal(context.Background(), client, input)
	if err != nil || third.Complete || third.Steps[6].Code != "unsupported_state_version" {
		t.Fatalf("future database did not produce a partial receipt: %+v %v", third, err)
	}
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil || version != 99 {
		t.Fatalf("future schema was changed: %d %v", version, err)
	}
	after, err := os.ReadFile(filepath.Join(home, "config.toml"))
	if err != nil || string(after) != string(config) {
		t.Fatal("local metadata repair changed configuration")
	}
	encoded, _ := json.Marshal(third)
	if strings.Contains(string(encoded), "untouched-fixture-marker") {
		t.Fatal("local repair leaked configuration content")
	}
}
