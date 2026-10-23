package cliadapter_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func defaultClient(t *testing.T) (cliadapter.Client, remote.AccountDefaultInput) {
	t.Helper()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device, previous := "https://test.invalid", "device-1", "previous-account"
	return client, remote.AccountDefaultInput{Confirmed: true, Expected: cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}, Previous: &previous, Tool: "claude", AccountID: "account-1"}
}
func TestAccountDefaultStrictRead(t *testing.T) {
	for _, mode := range []string{"", "unconfigured", "default-none", "default-missing", "default-null-tool", "default-wrong-tool", "default-unknown-field", "default-wrong-home", "default-missing-context-field", "default-null-context", "default-duplicate"} {
		t.Run(mode, func(t *testing.T) {
			client, _ := defaultClient(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			result, err := remote.GetAccountDefault(context.Background(), client, "claude")
			if strings.HasPrefix(mode, "default-") && mode != "default-none" {
				if !errors.Is(err, fault.CLIProtocol) {
					t.Fatalf("accepted malformed result: %+v %v", result, err)
				}
				return
			}
			if err != nil || result.Tool != "claude" || result.Context.Home != client.Home {
				t.Fatalf("read: %+v %v", result, err)
			}
			if (mode == "unconfigured" || mode == "default-none") && result.AccountID != nil {
				t.Fatal("absent default became a value")
			}
		})
	}
}
func TestAccountDefaultMutationStrictReceipt(t *testing.T) {
	for _, clearDefault := range []bool{false, true} {
		for _, mode := range []string{"", "default-missing", "default-null-tool", "default-wrong-tool", "default-unknown-field", "default-wrong-home", "default-missing-context-field", "default-null-context", "default-missing-changed", "default-false-changed", "default-wrong-operation", "default-wrong-previous", "default-wrong-result", "default-duplicate"} {
			t.Run(mode+map[bool]string{true: "-clear", false: "-set"}[clearDefault], func(t *testing.T) {
				client, input := defaultClient(t)
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				var receipt remote.AccountDefaultReceipt
				var err error
				if clearDefault {
					receipt, err = remote.ClearAccountDefault(context.Background(), client, input)
				} else {
					receipt, err = remote.SetAccountDefault(context.Background(), client, input)
				}
				if mode != "" {
					if !errors.Is(err, fault.AccountDefaultUnknown) {
						t.Fatalf("malformed mutation receipt: %+v %v", receipt, err)
					}
					return
				}
				if err != nil || !receipt.Changed || receipt.Tool != "claude" || receipt.Previous == nil || *receipt.Previous != "previous-account" {
					t.Fatalf("receipt: %+v %v", receipt, err)
				}
				if clearDefault && receipt.AccountID != nil {
					t.Fatal("clear left account")
				}
			})
		}
	}
}
func TestAccountDefaultNoConfirmationNoMutation(t *testing.T) {
	client, input := defaultClient(t)
	input.Confirmed = false
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	for _, call := range []func(context.Context, cliadapter.Client, remote.AccountDefaultInput) (remote.AccountDefaultReceipt, error){remote.SetAccountDefault, remote.ClearAccountDefault} {
		_, err := call(context.Background(), client, input)
		var operation *cliadapter.OperationError
		if !errors.As(err, &operation) || operation.Code != "confirmation_required" {
			t.Fatalf("confirmation bypass: %v", err)
		}
	}
	if data, _ := os.ReadFile(log); len(data) != 0 {
		t.Fatalf("unconfirmed process: %s", data)
	}
}
func TestAccountDefaultOldCLINeverReceivesMutation(t *testing.T) {
	client, input := defaultClient(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "default-unsupported")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	_, err := remote.SetAccountDefault(context.Background(), client, input)
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("capability: %v", err)
	}
	data, _ := os.ReadFile(log)
	if string(data) != "capabilities\n" {
		t.Fatalf("unsupported mutation called: %s", data)
	}
}
func TestAccountDefaultCancellationUnknownAndNoRetry(t *testing.T) {
	client, input := defaultClient(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "default-slow")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	barrier := t.TempDir()
	started, release := filepath.Join(barrier, "started"), filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_DEFAULT_STARTED", started)
	t.Setenv("DESKTOP_TEST_DEFAULT_RELEASE", release)
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := remote.SetAccountDefault(ctx, client, input); done <- err }()
	waitWireGuardBarrier(t, started)
	cancel()
	err := <-done
	if !errors.Is(err, fault.AccountDefaultUnknown) {
		t.Fatalf("canceled mutation: %v", err)
	}
	data, _ := os.ReadFile(log)
	if strings.Count(string(data), "accounts.default.set") != 1 {
		t.Fatalf("mutation retried/missing: %s", data)
	}
}
func TestAccountDefaultKnownServiceFailures(t *testing.T) {
	for mode, want := range map[string]string{"default-stale": "default_changed", "default-context-changed": "configuration_changed", "default-denied": "permission_denied"} {
		t.Run(mode, func(t *testing.T) {
			client, input := defaultClient(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.SetAccountDefault(context.Background(), client, input)
			var operation *cliadapter.OperationError
			if !errors.As(err, &operation) || operation.Code != want {
				t.Fatalf("failure: %v", err)
			}
		})
	}
}

func TestAccountDefaultConfirmationFreezesCallerReferences(t *testing.T) {
	client, input := defaultClient(t)
	barrier := t.TempDir()
	started := filepath.Join(barrier, "started")
	release := filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
	done := make(chan error, 1)
	go func() { _, err := remote.SetAccountDefault(context.Background(), client, input); done <- err }()
	waitWireGuardBarrier(t, started)
	capture := filepath.Join(barrier, "confirmation")
	// The environment lock orders pointer edits after the worker's Environ call.
	t.Setenv("DESKTOP_TEST_DEFAULT_RECEIPT", capture)
	*input.Expected.ServerURL = "https://changed.invalid"
	*input.Expected.DeviceID = "other-device"
	*input.Previous = "other-account"
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var sent remote.AccountDefaultInput
	if json.Unmarshal(data, &sent) != nil {
		t.Fatal("capture")
	}
	if *sent.Expected.ServerURL != "https://test.invalid" || *sent.Expected.DeviceID != "device-1" || *sent.Previous != "previous-account" {
		t.Fatalf("mutated confirmation: %s", data)
	}
}
func TestAccountDefaultRejectsInvalidInputWithoutProcess(t *testing.T) {
	for _, id := range []string{"prefix...", "with space", strings.Repeat("x", 129), ""} {
		t.Run(id, func(t *testing.T) {
			client, input := defaultClient(t)
			input.AccountID = id
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			if _, err := remote.SetAccountDefault(context.Background(), client, input); !errors.Is(err, fault.CLIProtocol) {
				t.Fatalf("invalid ID accepted: %v", err)
			}
			if data, _ := os.ReadFile(log); len(data) != 0 {
				t.Fatalf("invalid input dispatched: %s", data)
			}
		})
	}
}
