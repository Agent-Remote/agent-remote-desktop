package remote_test

import (
	"context"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
)

const exactIdentityDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func exactIdentityContext() cliadapter.ConfigContext {
	return cliadapter.ConfigContext{
		Home:      "/tmp/home",
		ServerURL: stringPtr("https://example.invalid"),
		DeviceID:  stringPtr("device-1"),
	}
}

func stringPtr(value string) *string { return &value }

func TestExactSessionOperationsRejectNonUUIDRuntimeIdentitiesBeforeCLI(t *testing.T) {
	ctx := exactIdentityContext()
	create := remote.SessionCreateInput{
		SchemaVersion: 1,
		Expected:      ctx,
		ToolType:      "claude",
		AccountID:     "account-1",
		WorkspaceID:   "22222222-2222-4222-8222-222222222222",
		ProjectKey:    "project",
		ArgvDigest:    exactIdentityDigest,
		SyncTarget:    []byte(`{"id":"sync","workspace_id":"workspace","local_path":"/tmp/project","remote_path":"/workspace"}`),
		Confirmed:     true,
	}
	if _, err := remote.CreateSession(context.Background(), cliadapter.Client{Home: ctx.Home}, create); err != fault.SessionCreateInputInvalid {
		t.Fatalf("non-UUID create account was accepted: %v", err)
	}

	stop := remote.SessionStopInput{
		SchemaVersion:   1,
		Expected:        ctx,
		SessionID:       "session-1",
		ExpectedNodeID:  "33333333-3333-4333-8333-333333333333",
		ExpectedBackend: "native",
		Confirmed:       true,
	}
	if _, err := remote.StopSession(context.Background(), cliadapter.Client{Home: ctx.Home}, stop); err != fault.SessionStopInputInvalid {
		t.Fatalf("non-UUID stop session was accepted: %v", err)
	}

	selection := remote.SessionDeleteSelection{ToolType: "claude", Statuses: []string{"stopped"}}
	deleteInput := remote.SessionDeleteInput{
		SchemaVersion:     1,
		Expected:          ctx,
		ToolType:          "claude",
		SessionIDs:        []string{"session-1"},
		Selection:         selection,
		SelectionDigest:   remote.SessionDeleteSelectionDigest("claude", []string{"session-1"}, selection),
		IntentFingerprint: remote.SessionDeleteIntentFingerprint("claude", []string{"session-1"}, selection),
		Reason:            "user_requested",
		Confirmed:         true,
	}
	if _, err := remote.DeleteSessions(context.Background(), cliadapter.Client{Home: ctx.Home}, deleteInput); err != fault.SessionDeleteInputInvalid {
		t.Fatalf("non-UUID delete session was accepted: %v", err)
	}
}
