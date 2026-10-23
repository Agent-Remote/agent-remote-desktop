//go:build unix

package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestAccountsCancellationDeliversSIGTERMAndPreservesUnknownMutation(t *testing.T) {
	for _, method := range []string{"status", "create", "bind", "verify", "disable"} {
		t.Run(method, func(t *testing.T) {
			client, create, action := accountInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "account-signal-wait")
			directory := t.TempDir()
			ready, cleanup, log := filepath.Join(directory, "ready"), filepath.Join(directory, "cleanup"), filepath.Join(directory, "calls")
			t.Setenv("DESKTOP_TEST_DEFAULT_SIGNAL_READY", ready)
			t.Setenv("DESKTOP_TEST_DEFAULT_SIGNAL_CLEANUP", cleanup)
			t.Setenv("DESKTOP_TEST_CLI_LOG", log)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch method {
				case "status":
					_, err = remote.GetAccountStatus(ctx, client, remote.AccountStatusInput{Expected: action.Expected, AccountID: action.AccountID})
				case "create":
					_, err = remote.CreateAccount(ctx, client, create)
				case "bind":
					_, err = remote.BindAccount(ctx, client, action)
				case "verify":
					_, err = remote.VerifyAccount(ctx, client, action)
				case "disable":
					_, err = remote.DisableAccount(ctx, client, action)
				}
				done <- err
			}()
			waitWireGuardBarrier(t, ready)
			cancel()
			select {
			case err := <-done:
				want := error(fault.AccountOutcomeUnknown)
				if method == "status" {
					want = context.Canceled
				}
				if !errors.Is(err, want) {
					t.Fatalf("cancel result: %v, want %v", err, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("cancellation retained CLI")
			}
			evidence, err := os.ReadFile(cleanup)
			if err != nil || string(evidence) != "SIGTERM cleanup completed" {
				t.Fatalf("missing graceful signal cleanup: %q %v", evidence, err)
			}
			calls, _ := os.ReadFile(log)
			if strings.Count(string(calls), "accounts."+method) != 1 {
				t.Fatalf("request retried: %s", calls)
			}
		})
	}
}
