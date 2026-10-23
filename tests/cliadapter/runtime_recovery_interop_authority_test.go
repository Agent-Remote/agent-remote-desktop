package cliadapter_test

import (
	"context"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

func TestRuntimeRecoveryWithBuiltRustCLIManualHistoryRemainsReadOnly(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real manual recovery interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	f := newRecoveryInteropServer(t)
	client := loginRecoveryInterop(t, ctx, cli, f)
	f.mu.Lock()
	f.records[recoveryInteropKey] = recoveryInteropReceipt(recoveryInteropKey, "repair_source")
	f.mu.Unlock()
	before := recoveryInteropFiles(t, client.Home)
	authority, err := remote.GetRuntimeRecoveryAuthority(ctx, client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: f.server.URL + "/"})
	if err != nil {
		t.Fatal(err)
	}
	if authority.Context.ServerURL != f.server.URL {
		t.Fatal("authority did not preserve the canonical configured server identity")
	}
	input := remote.RuntimeRecoveryStatusInput{Expected: authority.Context, AccountID: recoveryInteropAccount, RequestID: recoveryInteropKey}
	status, err := remote.GetRuntimeRecoveryStatus(ctx, client, input)
	if err != nil || status.Digest != nil || !status.SourceRestorationConfirmed || status.TargetCompletionConfirmed {
		t.Fatalf("manual old-key observation: %+v %v", status, err)
	}
	input.ExpectedBinding = &status.Recovery.Binding
	if _, err := remote.GetRuntimeRecoveryStatus(ctx, client, input); err != nil {
		t.Fatalf("known manual binding could not be observed again: %v", err)
	}
	pending, err := remote.ListRuntimeRecoveryPending(ctx, client)
	if err != nil || len(pending) != 0 {
		t.Fatalf("manual history acquired a local submission record: %+v %v", pending, err)
	}
	if !reflect.DeepEqual(before, recoveryInteropFiles(t, client.Home)) {
		t.Fatal("manual history created or changed local state")
	}
	f.mu.Lock()
	f.records[recoveryInteropKey]["binding"].(map[string]any)["task_record_id"] = "99999999-9999-4999-8999-999999999999"
	f.mu.Unlock()
	if _, err := remote.GetRuntimeRecoveryStatus(ctx, client, input); err == nil {
		t.Fatal("manual status silently replaced its previously observed immutable binding")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.posts != 0 || f.authCalls != 2 {
		t.Fatalf("manual status submitted work or refreshed credentials: posts=%d auth=%d", f.posts, f.authCalls)
	}
}

func TestRuntimeRecoveryWithBuiltRustCLIRechecksAdministratorBeforeSubmission(t *testing.T) {
	cli := os.Getenv("DESKTOP_CLI_BINARY")
	if cli == "" {
		t.Skip("set DESKTOP_CLI_BINARY for real administrator revocation interoperability")
	}
	t.Setenv("AGENT_REMOTE_SECRET_BACKEND", "file")
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	f := newRecoveryInteropServer(t)
	client := loginRecoveryInterop(t, ctx, cli, f)
	before := recoveryInteropFiles(t, client.Home)
	authority, err := remote.GetRuntimeRecoveryAuthority(ctx, client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: f.server.URL})
	if err != nil {
		t.Fatal(err)
	}
	plan, err := remote.InspectRuntimeRecovery(ctx, client, remote.RuntimeRecoveryInspectInput{Expected: authority.Context, AccountID: recoveryInteropAccount, OriginalTaskID: recoveryInteropOriginalTask, Action: "verify_target"})
	if err != nil {
		t.Fatal(err)
	}
	f.mu.Lock()
	f.admin = false
	f.mu.Unlock()
	now, err := remote.GetRuntimeRecoveryAuthority(ctx, client, remote.RuntimeRecoveryAuthorityInput{ExpectedHome: client.Home, ExpectedServerURL: f.server.URL})
	if err != nil || now.Admin {
		t.Fatalf("ordinary user did not receive a read-only non-admin observation: %+v %v", now, err)
	}
	if _, err := remote.ApplyRuntimeRecovery(ctx, client, remote.RuntimeRecoveryApplyInput{Confirmed: true, ExpectedPlan: plan, RequestID: recoveryInteropKey}); err == nil {
		t.Fatal("revoked administrator submitted a reviewed operation")
	}
	pending, err := remote.ListRuntimeRecoveryPending(ctx, client)
	if err != nil || len(pending) != 0 {
		t.Fatalf("rejected authority invented a durable submission: %+v %v", pending, err)
	}
	// Apply may acquire its own existing-user coordination locks, but reads must
	// preserve all credential bytes and configuration captured before inspection.
	after := recoveryInteropFiles(t, client.Home)
	for path, digest := range before {
		if after[path] != digest {
			t.Fatalf("administrator recheck changed existing local data %q", path)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.posts != 0 || f.authCalls != 2 {
		t.Fatalf("revoked administrator submitted or refreshed: posts=%d auth=%d", f.posts, f.authCalls)
	}
}
