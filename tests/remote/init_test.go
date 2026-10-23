package remote_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestInitContextFreezesSelectedHomeAndPreservesEmptyAuth(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "unconfigured")
	got, err := remote.InitContext(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if got.Home != home || got.ServerURL != nil || got.DeviceID != nil {
		t.Fatalf("unexpected unauthenticated context: %+v", got)
	}
}

func TestInitContextRejectsMalformedRemoteIdentity(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	t.Setenv("DESKTOP_TEST_CONTEXT_SERVER", "https://test.invalid/\ninvalid")
	if _, err := remote.InitContext(context.Background(), client); !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("malformed server identity error = %v, want CLI protocol", err)
	}
}

func TestAssessInitReadinessKeepsListFailuresSeparate(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "permission")
	readiness, err := remote.AssessInitReadiness(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if readiness.Ready {
		t.Fatal("permission failure was reported ready")
	}
	var operation *cliadapter.OperationError
	if !errors.As(readiness.Overview.AccountsError, &operation) || operation.Code != "permission_denied" {
		t.Fatalf("account query error = %v", readiness.Overview.AccountsError)
	}
	if readiness.Overview.SessionsError != nil || len(readiness.Overview.Sessions) == 0 {
		// The session query is independently successful; the account failure must
		// not erase its result or turn it into a synthetic empty success.
		t.Fatalf("session query was not preserved: %v", readiness.Overview.SessionsError)
	}
}
