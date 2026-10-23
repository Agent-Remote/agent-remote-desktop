package cliadapter_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestRuntimeRecoveryWithBuiltRustCLIRetainsMalformedAcceptanceAndForgetsRejectionOffline(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real recovery submission interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	for _, rejected := range []bool{false, true} {
		name := "malformed-acceptance"
		if rejected {
			name = "definite-rejection"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			f := newRecoveryInteropServer(t)
			client := loginRecoveryInterop(t, ctx, cli, f)
			authority, err := remote.GetRuntimeRecoveryAuthority(ctx, client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: f.server.URL})
			if err != nil {
				t.Fatal(err)
			}
			plan, err := remote.InspectRuntimeRecovery(ctx, client, remote.RuntimeRecoveryInspectInput{Expected: authority.Context, AccountID: recoveryInteropAccount, OriginalTaskID: recoveryInteropOriginalTask, Action: "repair_source"})
			if err != nil {
				t.Fatal(err)
			}
			f.mu.Lock()
			f.reviewChanged, f.corruptResponse = rejected, !rejected
			f.mu.Unlock()
			input := remote.RuntimeRecoveryApplyInput{Confirmed: true, ExpectedPlan: plan, RequestID: recoveryInteropKey}
			_, err = remote.ApplyRuntimeRecovery(ctx, client, input)
			var operation *cliadapter.OperationError
			code, phase := "runtime_recovery_outcome_unknown", "unknown"
			if rejected {
				code, phase = "runtime_recovery_review_changed", "rejected"
			}
			if !errors.As(err, &operation) || operation.Code != code {
				t.Fatalf("submission classification: %v, want %s", err, code)
			}
			pending, err := remote.ListRuntimeRecoveryPending(ctx, client)
			if err != nil || len(pending) != 1 || pending[0].Phase != phase || pending[0].Binding != nil {
				t.Fatalf("untrusted acceptance/rejection retention: %+v %v", pending, err)
			}
			ref := pending[0]
			var binding *remote.RuntimeRecoveryBinding
			if rejected {
				// Definite single-attempt rejection can be removed explicitly even
				// when the server and user session are no longer available.
				f.server.Close()
			} else {
				for _, key := range []string{recoveryInteropKey, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"} {
					input.RequestID = key
					if _, err := remote.ApplyRuntimeRecovery(ctx, client, input); err == nil {
						t.Fatal("unresolved request was replayed or bypassed with a new key")
					}
				}
				status, err := remote.GetRuntimeRecoveryStatus(ctx, client, remote.RuntimeRecoveryStatusInput{Expected: ref.Context, AccountID: ref.AccountID, RequestID: ref.RequestID, ExpectedDigest: &ref.Digest})
				if err != nil || status.Recovery.Status != "succeeded" {
					t.Fatalf("original immutable receipt recovery: %+v %v", status, err)
				}
				binding = &status.Recovery.Binding
			}
			if _, err := remote.ForgetRuntimeRecovery(ctx, client, remote.RuntimeRecoveryForgetInput{Confirmed: true, Expected: ref.Context, AccountID: ref.AccountID, RequestID: ref.RequestID, ExpectedDigest: &ref.Digest, ExpectedBinding: binding}); err != nil {
				t.Fatal(err)
			}
			pending, err = remote.ListRuntimeRecoveryPending(ctx, client)
			if err != nil || len(pending) != 0 {
				t.Fatalf("explicitly settled key remained: %+v %v", pending, err)
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			if f.posts != 1 || f.authCalls != 2 {
				t.Fatalf("unexpected submission or credential renewal: posts=%d auth=%d", f.posts, f.authCalls)
			}
		})
	}
}
