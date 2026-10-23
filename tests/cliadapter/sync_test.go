package cliadapter_test

import (
	"context"
	"errors"
	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func syncInputs(t *testing.T) (cliadapter.Client, remote.SyncActionInput) {
	t.Helper()
	client, base := defaultClient(t)
	workspace, syncID, fingerprint := "workspace-1", "sync-1", "sha256:"+strings.Repeat("a", 64)
	return client, remote.SyncActionInput{Confirmed: true, Expected: base.Expected, ExpectedTarget: remote.SyncTarget{LocalPath: t.TempDir(), ProjectKey: "project-key", WorkspaceID: &workspace, SyncSessionID: &syncID, Fingerprint: &fingerprint}}
}
func TestSyncStatusRejectsUntrustedProjection(t *testing.T) {
	for _, mode := range []string{"", "sync-missing", "sync-null", "sync-unknown-field", "sync-wrong-path", "sync-wrong-context", "sync-bad-fingerprint", "sync-contradictory-state", "sync-unknown-zero", "sync-paused-zero"} {
		t.Run(mode, func(t *testing.T) {
			client, input := syncInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			result, err := remote.GetSyncStatus(context.Background(), client, remote.SyncStatusInput{Expected: input.Expected, LocalPath: input.ExpectedTarget.LocalPath})
			if mode != "" {
				if !errors.Is(err, fault.CLIProtocol) {
					t.Fatalf("accepted unsafe projection: %+v %v", result, err)
				}
				return
			}
			if err != nil || result.Mutagen.Conflicts != "none_observed" {
				t.Fatalf("status: %+v %v", result, err)
			}
		})
	}
}
func TestSyncReceiptsPreservePartialAndUnknown(t *testing.T) {
	for _, method := range []string{"ensure", "pause", "resume", "resolve", "reset"} {
		for _, mode := range []string{"", "sync-partial", "sync-unknown"} {
			t.Run(method+mode, func(t *testing.T) {
				client, input := syncInputs(t)
				t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
				result, err := remote.RunSyncAction(context.Background(), client, "sync."+method, input)
				if err != nil || result.Method != "sync."+method || len(result.Stages) == 0 {
					t.Fatalf("receipt %+v %v", result, err)
				}
				if mode != "" && result.Outcome == "completed" {
					t.Fatal("lost partial outcome")
				}
			})
		}
	}
}

func TestSyncActionForwardsProgressBeforeReceipt(t *testing.T) {
	client, input := syncInputs(t)
	var events []cliadapter.ProgressEvent
	receipt, err := remote.RunSyncActionWithProgress(context.Background(), client, "sync.reset", input, func(event cliadapter.ProgressEvent) error {
		events = append(events, event)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Phase != "started" || string(events[0].Data) != `{"operation":"sync.reset","stage":"started"}` {
		t.Fatalf("unexpected progress events: %+v", events)
	}
	if receipt.Method != "sync.reset" || receipt.Outcome == "" {
		t.Fatalf("missing final receipt: %+v", receipt)
	}
}

func TestSyncActionProgressCallbackFailureFailsClosed(t *testing.T) {
	client, input := syncInputs(t)
	// The remote layer deliberately collapses transport and callback failures
	// into the same safe unknown-outcome code; it must never expose callback
	// internals or imply that the mutation can be replayed.
	_, err := remote.RunSyncActionWithProgress(context.Background(), client, "sync.reset", input, func(cliadapter.ProgressEvent) error {
		return errors.New("stop progress")
	})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "sync_outcome_unknown" {
		t.Fatalf("callback failure was not closed as unknown outcome: %v", err)
	}
}

func TestSyncMalformedMutationIsUnknown(t *testing.T) {
	for _, mode := range []string{"sync-missing", "sync-null", "sync-unknown-field", "sync-wrong-path", "sync-wrong-context", "sync-bad-fingerprint", "sync-wrong-method", "sync-wrong-id", "sync-wrong-dry-run", "sync-empty-stages", "sync-bad-stage", "sync-completed-failed"} {
		t.Run(mode, func(t *testing.T) {
			client, input := syncInputs(t)
			t.Setenv("DESKTOP_TEST_CLI_SCENARIO", mode)
			_, err := remote.RunSyncAction(context.Background(), client, "sync.reset", input)
			var op *cliadapter.OperationError
			if !errors.As(err, &op) || op.Code != "sync_outcome_unknown" {
				t.Fatalf("not unknown: %v", err)
			}
		})
	}
}
func TestSyncUnconfirmedCannotStartProcess(t *testing.T) {
	client, input := syncInputs(t)
	input.Confirmed = false
	log := filepath.Join(t.TempDir(), "calls")
	t.Setenv("DESKTOP_TEST_CLI_LOG", log)
	_, err := remote.RunSyncAction(context.Background(), client, "sync.reset", input)
	var op *cliadapter.OperationError
	if !errors.As(err, &op) || op.Code != "confirmation_required" {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(log); len(data) > 0 {
		t.Fatalf("spawned: %s", data)
	}
}
