package session_test

import (
	"reflect"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
)

const bindingAccount = "22222222-2222-4222-8222-222222222222"
const bindingAttempt = "bind-222222222222-0123456789ab"

func TestBindingLaunchUsesExactCLIArgvAndCannotResumeHistory(t *testing.T) {
	args, err := session.BindingArguments("https://example.org", "device-1", bindingAccount, bindingAttempt)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	cmd, err := session.PrepareWithResolver(session.Launch{Tool: session.AccountBinding, Directory: dir, Home: dir, Arguments: args}, func(name string) (string, error) {
		if name != "agent-remote" {
			t.Fatalf("wrong executable %s", name)
		}
		return "/portable/agent-remote", nil
	})
	want := []string{"/portable/agent-remote", "account", "attach", bindingAccount, "--binding-session", bindingAttempt, "--expected-server", "https://example.org", "--expected-device", "device-1"}
	if err != nil || !cmd.Remote || !reflect.DeepEqual(cmd.Argv, want) {
		t.Fatalf("wrong command: %+v %v", cmd, err)
	}
	ref, err := session.BindingReference(bindingAccount, bindingAttempt)
	if err != nil {
		t.Fatal(err)
	}
	account, err := session.BindingAccount(ref)
	if err != nil || account != bindingAccount {
		t.Fatalf("lost public identity: %s %v", account, err)
	}
	if _, err := session.RecoveryArguments(session.AccountBinding, ref); err == nil {
		t.Fatal("history resumed stale attempt")
	}
}

func TestBindingLaunchRejectsArbitraryCommandsAndMismatchedGenerations(t *testing.T) {
	valid, err := session.BindingArguments("https://example.org", "device-1", bindingAccount, bindingAttempt)
	if err != nil {
		t.Fatal(err)
	}
	cases := [][]string{nil, {"attach", bindingAttempt}, append(append([]string{}, valid...), "--help")}
	for _, change := range []struct {
		index int
		value string
	}{
		{0, "session"}, {1, "run"}, {2, "22222222"}, {3, "--command"},
		{4, "bind-333333333333-0123456789ab"}, {4, bindingAttempt + ";sh"},
		{4, "bind-222222222222-0123456789AB"}, {4, "bind-222222222222-中文中文"},
		{5, "--server"}, {6, "https://user:secret@example.org"}, {6, "file:///tmp/server"},
		{6, "https://example.org?token=secret"}, {6, "https://example.org#fragment"},
		{7, "--device"}, {8, "-other"}, {8, "device\nother"}, {8, ""},
	} {
		args := append([]string{}, valid...)
		args[change.index] = change.value
		cases = append(cases, args)
	}
	for _, args := range cases {
		_, err := session.PrepareWithResolver(session.Launch{Tool: session.AccountBinding, Directory: t.TempDir(), Arguments: args}, func(string) (string, error) {
			t.Fatal("invalid binding reached executable lookup")
			return "", nil
		})
		if err == nil {
			t.Errorf("accepted unsafe args %q", args)
		}
	}
	for _, ref := range []string{"", bindingAccount, "other/" + bindingAttempt, bindingAccount + "/" + bindingAttempt + "/extra"} {
		if _, err := session.BindingAccount(ref); err == nil {
			t.Errorf("accepted malformed history %q", ref)
		}
	}
}
