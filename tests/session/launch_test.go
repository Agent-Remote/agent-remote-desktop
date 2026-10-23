package session_test

import (
	"errors"
	"reflect"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
)

func TestLaunchArgumentsPreserveLiteralShellCharacters(t *testing.T) {
	args, err := session.ParseArguments(`["--model","model name","$(touch should-not-exist)","; & |","", "中文"]`)
	want := []string{"--model", "model name", "$(touch should-not-exist)", "; & |", "", "中文"}
	if err != nil || !reflect.DeepEqual(args, want) {
		t.Fatalf("argv changed: %q %v", args, err)
	}
}

func TestHistoryUsesExplicitToolRecoverySemantics(t *testing.T) {
	for _, tc := range []struct {
		tool      session.Tool
		reference string
		want      []string
	}{
		{session.Codex, "", []string{"resume"}},
		{session.Claude, "", []string{"--resume"}},
		{session.Codex, "a session name", []string{"resume", "a session name"}},
		{session.Claude, "abc123", []string{"--resume", "abc123"}},
	} {
		got, err := session.RecoveryArguments(tc.tool, tc.reference)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%s: %q %v", tc.tool, got, err)
		}
	}
	for _, reference := range []string{"11111111-1111-4111-8111-111111111111", "a1b2c3d4", "", "--new", "$(launch)", "a\x00b", "a\nb"} {
		if _, err := session.RecoveryArguments(session.RemoteClaude, reference); !errors.Is(err, fault.ResumeReference) {
			t.Errorf("unsafe or missing remote target accepted: %q", reference)
		}
	}
}

func TestLaunchArgumentsRejectNonStringJSONAndNUL(t *testing.T) {
	for _, value := range []string{`null`, `{}`, `"--help"`, `[null]`, `[1]`, `[true]`, `[{}]`, `[[]]`, `["ok"] trailing`} {
		if _, err := session.ParseArguments(value); !errors.Is(err, fault.InvalidArguments) {
			t.Errorf("accepted %s: %v", value, err)
		}
	}
	if _, err := session.ParseArguments(`["\u0000"]`); !errors.Is(err, fault.ArgumentNUL) {
		t.Fatalf("accepted NUL: %v", err)
	}
	for _, value := range []string{"", "  ", "[]"} {
		if args, err := session.ParseArguments(value); err != nil || len(args) != 0 {
			t.Fatalf("empty args: %v %v", args, err)
		}
	}
}

func TestPortableResolverIsUsedForLaunchAndRecovery(t *testing.T) {
	directory := t.TempDir()
	for _, tool := range []session.Tool{session.Codex, session.Claude} {
		reference := "session-123"
		args, err := session.RecoveryArguments(tool, reference)
		if err != nil {
			t.Fatal(err)
		}
		expected := "/portable/" + string(tool)
		calls := 0
		command, err := session.PrepareWithResolver(session.Launch{Tool: tool, Directory: directory, Home: directory, Arguments: args}, func(name string) (string, error) {
			calls++
			if name != string(tool) {
				t.Fatal("resolved the wrong tool")
			}
			return expected, nil
		})
		if err != nil || calls != 1 || command.Argv[0] != expected || !reflect.DeepEqual(command.Argv[1:], args) || command.Remote != (tool == session.RemoteClaude) {
			t.Fatalf("portable recovery changed: %+v %v", command, err)
		}
		_, err = session.PrepareWithResolver(session.Launch{Tool: tool, Directory: directory, Home: directory}, func(string) (string, error) { return "", fault.ExecutableMissing })
		if !errors.Is(err, fault.ExecutableMissing) {
			t.Fatal("launch bypassed missing portable provider")
		}
	}
}

func TestAgentRemoteIsAPortableManagedTerminalTool(t *testing.T) {
	directory := t.TempDir()
	command, err := session.PrepareWithResolver(session.Launch{
		Tool:      session.AgentRemote,
		Directory: directory,
		Home:      directory,
		Arguments: []string{"deps", "status"},
	}, func(name string) (string, error) {
		if name != "agent-remote" {
			t.Fatalf("resolved executable %q", name)
		}
		return "/portable/agent-remote", nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(command.Argv, []string{"/portable/agent-remote", "deps", "status"}) || command.Remote {
		t.Fatalf("agent-remote launch = %+v", command)
	}
}
