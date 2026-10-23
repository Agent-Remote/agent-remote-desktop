package app_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestCredentialsPageShowsSecretFreeProfileMetadata(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.CredentialsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Studio workstation")
	if !tt.HasText("remote_login") || !tt.HasText("agent_forwarding") {
		t.Fatal("credential integration modes were not displayed")
	}
	if tt.HasText("user_id") || tt.HasText("private") {
		t.Fatal("secret or private credential fields leaked into the view")
	}
}

func TestCredentialsPageKeepsUnsupportedOperationExplicit(t *testing.T) {
	testutil.InstallControlHelper(t)
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "credentials-unsupported")
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if tt.HasText(locales.CredentialsTitle()) {
		t.Fatal("unsupported credential inventory was offered as a ready action")
	}
}

func TestCredentialsCreateUsesReviewedSecureControlRequest(t *testing.T) {
	testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.New(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	tt := ui.NewTester(a.View, 1100, 820)
	if err := tt.Click(locales.RemoteManagement()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "https://test.invalid")
	if err := tt.Click(locales.CredentialsTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, "Studio workstation")
	if err := tt.Click(locales.CredentialsCreateTitle()); err != nil {
		t.Fatal(err)
	}
	for _, field := range []struct{ label, value string }{{locales.CredentialsDisplayName(), "Desktop profile"}, {locales.CredentialsGitUserName(), "desktop-user"}, {locales.CredentialsGitUserEmail(), "desktop@example.test"}, {locales.CredentialsGitHubMode(), "remote_login"}, {locales.CredentialsSSHMode(), "agent_forwarding"}} {
		if err := tt.Click(field.label); err != nil {
			t.Fatal(err)
		}
		tt.Key(ui.Cmd, ui.KeyA)
		tt.Type(field.value)
	}
	if err := tt.Click(locales.CredentialsReview()); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(locales.CredentialsReviewHelp()) || tt.HasText("desktop-user") {
		t.Fatal("review exposed Git identity or omitted safety text")
	}
	capture := filepath.Join(t.TempDir(), "credential-request")
	t.Setenv("DESKTOP_TEST_CREDENTIAL_RECEIPT", capture)
	if err := tt.Click(locales.CredentialsConfirm()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 300 && !tt.HasText(locales.CredentialsMutationAccepted("create")); i++ {
		time.Sleep(50 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.CredentialsMutationAccepted("create")) {
		t.Fatalf("credential mutation receipt missing; texts=%v", tt.Texts())
	}
	data, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var request map[string]any
	if json.Unmarshal(data, &request) != nil || request["confirmed"] != true || request["display_name"] != "Desktop profile" {
		t.Fatalf("invalid credential request: %s", data)
	}
	if strings.Contains(string(data), "token") || strings.Contains(string(data), "private_key") {
		t.Fatalf("secret field crossed the control request: %s", data)
	}
}
