package session_test

import (
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

func TestRemoteAttachPreservesExactPublicDestinationAndHome(t *testing.T) {
	target := workspace.RemoteContext{Home: filepath.Join(t.TempDir(), "home with spaces"), ServerURL: "https://example.org/service", DeviceID: "33333333-3333-4333-8333-333333333333", SessionID: "11111111-1111-4111-8111-111111111111"}
	args, err := session.RemoteAttachArguments(target)
	if err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(t.TempDir(), "portable fclaude")
	dir := t.TempDir()
	cmd, err := session.PrepareWithResolver(session.Launch{Tool: session.RemoteClaude, Directory: dir, Home: target.Home, Arguments: args}, func(name string) (string, error) {
		if name != "fclaude" {
			t.Fatalf("wrong executable %s", name)
		}
		return binary, nil
	})
	want := []string{binary, "attach", target.SessionID, "--expected-server", target.ServerURL, "--expected-device", target.DeviceID}
	if err != nil || !reflect.DeepEqual(cmd.Argv, want) || cmd.Directory != dir || !cmd.Remote || !slices.Contains(cmd.Env, "AGENT_REMOTE_HOME="+target.Home) {
		t.Fatalf("lost reviewed destination: %+v %v", cmd, err)
	}
	if _, err := session.RecoveryArguments(session.RemoteClaude, target.SessionID); err == nil {
		t.Fatal("UUID-only history bypassed current-context review")
	}
}

func TestRemoteAttachRejectsAmbiguousOrSecretBearingIdentity(t *testing.T) {
	good := workspace.RemoteContext{Home: t.TempDir(), ServerURL: "https://example.org", DeviceID: "33333333-3333-4333-8333-333333333333", SessionID: "11111111-1111-4111-8111-111111111111"}
	for _, mutate := range []func(*workspace.RemoteContext){
		func(r *workspace.RemoteContext) { r.SessionID = "11111111" },
		func(r *workspace.RemoteContext) { r.SessionID = "00000000-0000-0000-0000-000000000000" },
		func(r *workspace.RemoteContext) { r.SessionID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaA" },
		func(r *workspace.RemoteContext) { r.SessionID = "--new" },
		func(r *workspace.RemoteContext) { r.SessionID += "; sh" },
		func(r *workspace.RemoteContext) { r.DeviceID = "device-1" },
		func(r *workspace.RemoteContext) { r.DeviceID = "00000000-0000-0000-0000-000000000000" },
		func(r *workspace.RemoteContext) { r.Home = "relative" },
		func(r *workspace.RemoteContext) { r.Home += "\x00" },
		func(r *workspace.RemoteContext) { r.ServerURL = "https://user:password@example.org" },
		func(r *workspace.RemoteContext) { r.ServerURL = "https://example.org?token=secret" },
		func(r *workspace.RemoteContext) { r.ServerURL = "https://example.org?" },
		func(r *workspace.RemoteContext) { r.ServerURL = "https://example.org#token" },
		func(r *workspace.RemoteContext) { r.ServerURL = "file:///server" },
		func(r *workspace.RemoteContext) { r.ServerURL = "https://example.org\n" },
	} {
		bad := good
		mutate(&bad)
		if args, err := session.RemoteAttachArguments(bad); err == nil || args != nil {
			t.Errorf("accepted unsafe destination %+v", bad)
		}
	}
}
