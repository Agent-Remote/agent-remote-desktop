package remote_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestMain(m *testing.M) {
	if testutil.ControlHelper() {
		return
	}
	os.Exit(m.Run())
}

func TestCredentialsListSupportRequiresReadOnlyDeviceOperation(t *testing.T) {
	platform := cliadapter.PlatformName(runtime.GOOS)
	caps := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform}
	if remote.CredentialsListSupported(caps) {
		t.Fatal("missing credentials.list was reported as supported")
	}
	caps.Operations = []cliadapter.Capability{{Method: remote.CredentialsListMethod, Effect: "write", Credential: "device"}}
	if remote.CredentialsListSupported(caps) {
		t.Fatal("write credentials.list was reported as read-only support")
	}
	caps.Operations = []cliadapter.Capability{{Method: remote.CredentialsListMethod, Effect: "read", Credential: "none"}}
	if remote.CredentialsListSupported(caps) {
		t.Fatal("credential-free credentials.list was reported as device support")
	}
	caps.Operations = []cliadapter.Capability{{Method: remote.CredentialsListMethod, Effect: "read", Credential: "device"}}
	if !remote.CredentialsListSupported(caps) {
		t.Fatal("valid credentials.list operation was not supported")
	}
}

func TestCredentialsListRejectsMissingExactContextBeforeCLI(t *testing.T) {
	client := cliadapter.Client{Binary: "unused", Home: "/tmp/desktop"}
	_, err := remote.ListCredentialProfiles(context.Background(), client, cliadapter.ConfigContext{Home: client.Home})
	if !errors.Is(err, fault.CLIContextChanged) {
		t.Fatalf("error = %v, want CLIContextChanged", err)
	}
}

func TestCredentialMutationsRequireExactDeviceWriteCapability(t *testing.T) {
	platform := cliadapter.PlatformName(runtime.GOOS)
	caps := cliadapter.Capabilities{Version: 1, CLIVersion: "test", Platform: platform}
	for _, method := range []string{remote.CredentialsCreateMethod, remote.CredentialsBindMethod, remote.CredentialsUnbindMethod} {
		if remote.CredentialMutationSupported(caps, method) {
			t.Fatalf("missing %s was reported as supported", method)
		}
		caps.Operations = []cliadapter.Capability{{Method: method, Effect: "write", Credential: "device"}}
		if remote.CredentialMutationSupported(caps, method) {
			t.Fatalf("%s without exact confirmation was reported as supported", method)
		}
		caps.Operations[0].Confirmation = "exact_request"
		if !remote.CredentialMutationSupported(caps, method) {
			t.Fatalf("valid %s was not supported", method)
		}
	}
}

func TestCredentialCreateValidationRejectsUnsafeMetadata(t *testing.T) {
	input := remote.CredentialCreateInput{DisplayName: "profile", GHMode: "remote_login", SSHMode: "agent_forwarding"}
	if err := remote.ValidateCredentialCreateInput(input); err != nil {
		t.Fatalf("valid profile rejected: %v", err)
	}
	secret := "token-value"
	input.GitUserName = &secret
	if err := remote.ValidateCredentialCreateInput(input); err != nil {
		t.Fatalf("git identity metadata rejected: %v", err)
	}
	input.GHMode = "access_token"
	if err := remote.ValidateCredentialCreateInput(input); err == nil {
		t.Fatal("unsupported credential mode accepted")
	}
	input.GHMode = "remote_login"
	input.DisplayName = "unsafe\nprofile"
	if err := remote.ValidateCredentialCreateInput(input); err == nil {
		t.Fatal("control character accepted in profile metadata")
	}
}

func TestCredentialMutationsUseStrictControlReceipts(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	home := t.TempDir()
	server := "https://test.invalid"
	device := "device-1"
	expected := cliadapter.ConfigContext{Home: home, ServerURL: &server, DeviceID: &device}
	capture := filepath.Join(t.TempDir(), "request.json")
	t.Setenv("DESKTOP_TEST_CREDENTIAL_RECEIPT", capture)
	name, email := "desktop-user", "desktop@example.test"
	create, err := remote.CreateCredentialProfile(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.CredentialCreateInput{Confirmed: true, Expected: expected, DisplayName: "Desktop profile", GitUserName: &name, GitUserEmail: &email, GHMode: "remote_login", SSHMode: "agent_forwarding"})
	if err != nil || create.Profile == nil || create.Action != "create" || create.Context.Home != home {
		t.Fatalf("create receipt = %#v, err=%v", create, err)
	}
	var request map[string]any
	data, err := os.ReadFile(capture)
	if err != nil || json.Unmarshal(data, &request) != nil || request["confirmed"] != true || request["display_name"] != "Desktop profile" {
		t.Fatalf("create request = %s, err=%v", data, err)
	}
	if _, ok := request["token"]; ok {
		t.Fatal("device token entered desktop-control params")
	}
	bind, err := remote.BindCredentialProfile(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.CredentialBindingInput{Confirmed: true, Expected: expected, AccountID: "account-1", ProfileID: "credential-1"})
	if err != nil || bind.Profile == nil || bind.AccountID == nil || *bind.AccountID != "account-1" {
		t.Fatalf("bind receipt = %#v, err=%v", bind, err)
	}
	unbound, err := remote.UnbindCredentialProfile(context.Background(), cliadapter.Client{Binary: binary, Home: home}, remote.CredentialUnbindInput{Confirmed: true, Expected: expected, AccountID: "account-1"})
	if err != nil || unbound.Profile != nil || unbound.AccountID == nil || *unbound.AccountID != "account-1" {
		t.Fatalf("unbind receipt = %#v, err=%v", unbound, err)
	}
}
