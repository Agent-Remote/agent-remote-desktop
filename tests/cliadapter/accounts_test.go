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
)

func accountInputs(t *testing.T) (cliadapter.Client, remote.AccountCreateInput, remote.AccountActionInput) {
	t.Helper()
	client, base := defaultClient(t)
	create := remote.AccountCreateInput{Confirmed: true, Expected: base.Expected, Tool: "claude", Name: "Tokyo Studio", Region: "JP", Timezone: "Asia/Tokyo", Locale: "ja_JP.UTF-8", Tags: []string{"gpu", "east", "east"}}
	return client, create, remote.AccountActionInput{Confirmed: true, Expected: base.Expected, AccountID: "account-1"}
}
func callAccount(t *testing.T, method string, client cliadapter.Client, create remote.AccountCreateInput, action remote.AccountActionInput) (remote.AccountReceipt, error) {
	t.Helper()
	switch method {
	case "create":
		return remote.CreateAccount(context.Background(), client, create)
	case "bind":
		return remote.BindAccount(context.Background(), client, action)
	case "verify":
		return remote.VerifyAccount(context.Background(), client, action)
	case "disable":
		return remote.DisableAccount(context.Background(), client, action)
	}
	t.Fatal("unknown test action")
	return remote.AccountReceipt{}, nil
}
func TestAccountsStrictStatusProjection(t *testing.T) {
	for _, mode := range []string{"", "account-missing", "account-bad-context", "account-null-field", "account-raw-command", "account-unknown-state", "account-wrong-id", "account-missing-context-field", "account-null-context", "account-duplicate"} {
		t.Run(mode, func(t *testing.T) {
			client, _, action := accountInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			status, err := remote.GetAccountStatus(context.Background(), client, remote.AccountStatusInput{Expected: action.Expected, AccountID: action.AccountID})
			if mode != "" {
				if !errors.Is(err, fault.CLIProtocol) {
					t.Fatalf("unsafe/incomplete status accepted: %+v %v", status, err)
				}
				return
			}
			if err != nil || status.Account.ID != "account-1" || status.Binding.AccountID != "account-1" || status.Account.Status == status.Binding.Status || *status.Binding.TaskID != "account:verify:task-1" {
				t.Fatalf("status observations: %+v %v", status, err)
			}
		})
	}
}
func TestAccountsMutationReceiptsMatchRequestAndAcceptance(t *testing.T) {
	for _, method := range []string{"create", "bind", "verify", "disable"} {
		modes := []string{"", "account-missing", "account-bad-context", "account-null-field", "account-raw-command", "account-unknown-state", "account-no-accepted", "account-false-accepted", "account-wrong-operation", "account-mixed", "account-missing-context-field", "account-null-context", "account-duplicate"}
		if method == "create" {
			modes = append(modes, "account-wrong-create-fields")
		} else {
			modes = append(modes, "account-wrong-id")
		}
		if method == "bind" || method == "verify" {
			modes = append(modes, "account-no-task")
		}
		for _, mode := range modes {
			t.Run(method+"/"+mode, func(t *testing.T) {
				client, create, action := accountInputs(t)
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				result, err := callAccount(t, method, client, create, action)
				if mode != "" {
					if !errors.Is(err, fault.AccountOutcomeUnknown) {
						t.Fatalf("untrusted mutation receipt: %+v %v", result, err)
					}
					return
				}
				if err != nil || !result.Accepted {
					t.Fatalf("acceptance missing: %+v %v", result, err)
				}
				if method == "bind" || method == "verify" {
					if result.Account != nil || result.Binding == nil || result.Binding.TaskID == nil {
						t.Fatal("task acceptance lost")
					}
				} else if result.Account == nil || result.Binding != nil {
					t.Fatal("account receipt lost")
				}
			})
		}
	}
}
func TestAccountsNoConfirmationNoProcess(t *testing.T) {
	client, create, action := accountInputs(t)
	create.Confirmed, action.Confirmed = false, false
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	for _, method := range []string{"create", "bind", "verify", "disable"} {
		_, err := callAccount(t, method, client, create, action)
		var operation *cliadapter.OperationError
		if !errors.As(err, &operation) || operation.Code != "confirmation_required" {
			t.Fatalf("%s confirmation bypass: %v", method, err)
		}
	}
	if data, _ := os.ReadFile(log); len(data) != 0 {
		t.Fatalf("unconfirmed process: %s", data)
	}
}
func TestAccountsOldCLINeverReceivesMutation(t *testing.T) {
	client, create, action := accountInputs(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "account-unsupported")
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	for _, method := range []string{"create", "bind", "verify", "disable"} {
		_, err := callAccount(t, method, client, create, action)
		var operation *cliadapter.OperationError
		if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
			t.Fatalf("%s unsupported: %v", method, err)
		}
	}
	data, _ := os.ReadFile(log)
	if strings.Contains(string(data), "accounts.") {
		t.Fatalf("unsupported operation dispatched: %s", data)
	}
}
func TestAccountsCreateSnapshotsAllMutableConfirmationFields(t *testing.T) {
	client, create, _ := accountInputs(t)
	barrier := t.TempDir()
	started, release := filepath.Join(barrier, "started"), filepath.Join(barrier, "release")
	t.Setenv("DESKTOP_TEST_CAPABILITIES_STARTED", started)
	t.Setenv("DESKTOP_TEST_CAPABILITIES_RELEASE", release)
	done := make(chan error, 1)
	go func() { _, err := remote.CreateAccount(context.Background(), client, create); done <- err }()
	waitWireGuardBarrier(t, started)
	capture := filepath.Join(barrier, "request")
	// Synchronize with the worker's earlier environment read before editing pointers.
	t.Setenv("DESKTOP_TEST_ACCOUNT_RECEIPT", capture)
	*create.Expected.ServerURL = "https://changed.invalid"
	*create.Expected.DeviceID = "other-device"
	create.Tags[0] = "changed"
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
	var sent remote.AccountCreateInput
	if json.Unmarshal(data, &sent) != nil {
		t.Fatal("capture")
	}
	if *sent.Expected.ServerURL != "https://test.invalid" || *sent.Expected.DeviceID != "device-1" || sent.Tags[0] != "gpu" {
		t.Fatalf("confirmation mutated: %s", data)
	}
}
func TestAccountsCreateInvalidFieldsNeverStartProcess(t *testing.T) {
	for _, field := range []string{"tool", "name", "region", "timezone", "locale", "tags", "tag-control", "tag-limit"} {
		t.Run(field, func(t *testing.T) {
			client, create, _ := accountInputs(t)
			switch field {
			case "tool":
				create.Tool = "invalid.tool"
			case "name":
				create.Name = " "
			case "region":
				create.Region = strings.Repeat("x", 65)
			case "timezone":
				create.Timezone = "zone\n"
			case "locale":
				create.Locale = ""
			case "tags":
				create.Tags = nil
			case "tag-control":
				create.Tags = []string{"bad\x00"}
			case "tag-limit":
				create.Tags = make([]string, 65)
			}
			log := filepath.Join(t.TempDir(), "calls")
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			if _, err := remote.CreateAccount(context.Background(), client, create); !errors.Is(err, fault.AccountInputInvalid) {
				t.Fatalf("invalid fields accepted: %v", err)
			}
			if data, _ := os.ReadFile(log); len(data) != 0 {
				t.Fatalf("invalid fields dispatched: %s", data)
			}
		})
	}
}
func TestAccountsPreserveKnownRejectionAndUnknownAcceptance(t *testing.T) {
	for mode, want := range map[string]string{"account-rejected": "account_rejected", "account-unknown": "account_outcome_unknown", "account-context-changed": "configuration_changed", "account-denied": "permission_denied", "account-not-found": "account_not_found"} {
		t.Run(mode, func(t *testing.T) {
			client, _, input := accountInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.BindAccount(context.Background(), client, input)
			var operation *cliadapter.OperationError
			if !errors.As(err, &operation) || operation.Code != want {
				t.Fatalf("outcome code lost: %v", err)
			}
		})
	}
}
