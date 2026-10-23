//go:build unix

package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestAccountDefaultCancellationDeliversSIGTERMBeforeKillFallback(t *testing.T) {
	for _, method := range []string{"get", "set", "clear"} {
		t.Run(method, func(t *testing.T) {
			client, input := defaultClient(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "default-signal-wait")
			directory := t.TempDir()
			ready, cleanup := filepath.Join(directory, "ready"), filepath.Join(directory, "cleanup")
			t.Setenv("DESKTOP_TEST_DEFAULT_SIGNAL_READY", ready)
			t.Setenv("DESKTOP_TEST_DEFAULT_SIGNAL_CLEANUP", cleanup)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				var err error
				switch method {
				case "get":
					_, err = remote.GetAccountDefault(ctx, client, "claude")
				case "set":
					_, err = remote.SetAccountDefault(ctx, client, input)
				case "clear":
					_, err = remote.ClearAccountDefault(ctx, client, input)
				}
				done <- err
			}()
			waitWireGuardBarrier(t, ready)
			cancel()
			select {
			case err := <-done:
				want := error(fault.AccountDefaultUnknown)
				if method == "get" {
					want = context.Canceled
				}
				if !errors.Is(err, want) {
					t.Fatalf("cancel result: %v, want %v", err, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("CLI did not terminate within cancellation bound")
			}
			evidence, err := os.ReadFile(cleanup)
			if err != nil || string(evidence) != "SIGTERM cleanup completed" {
				t.Fatalf("SIGTERM cleanup did not run before process exit: %q, %v", evidence, err)
			}
		})
	}
}
