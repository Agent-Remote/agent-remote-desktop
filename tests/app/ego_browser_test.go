package app_test

import (
	"errors"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/app"
	"github.com/Agent-Remote/agent-remote-desktop/locales"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

func TestEgoBrowserStatusUsesStructuredControlMethod(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", errors.New("test component not installed")
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := tt.Click(locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("ego-browser status")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserLocalState("installed", "present")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserLocalState("installed", "present")) {
		t.Fatalf("ego-browser page did not load: %v", tt.Texts())
	}
	if !tt.HasText(locales.EgoBrowserDevice("browser-device-1", "active")) {
		t.Fatal("structured ego-browser device projection missing")
	}
	if tt.HasText(locales.CliFormTitle()) {
		t.Fatal("ego-browser status fell through to generic terminal form")
	}
	if err := tt.Click(locales.EgoBrowserRequestsOpen()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserRequest("request-ledger-1", "active", "evaluate", "42")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserRequest("request-ledger-1", "active", "evaluate", "42")) {
		t.Fatalf("ego-browser request page did not load: %v", tt.Texts())
	}
	if err := tt.Click(locales.EgoBrowserRequestCancel()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.EgoBrowserBindingAction("pause")); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserBindingReview()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserBindingReviewTarget("binding-1", "pause")) {
		t.Fatalf("binding review did not load: %v", tt.Texts())
	}
	if err := tt.Click(locales.EgoBrowserBindingConfirm()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserBindingAccepted("pause", "paused")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserBindingAccepted("pause", "paused")) {
		t.Fatalf("binding lifecycle receipt did not load: %v", tt.Texts())
	}
}

func TestEgoBrowserTerminalDeletionUsesTargetReviewAndReceipt(t *testing.T) {
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "ego-browser-delete-terminal")
	binary := testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", errors.New("test component not installed")
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := tt.Click(locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("ego-browser status")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserDeleteBinding()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserDeleteBinding()) {
		t.Fatalf("terminal deletion action missing: %v", tt.Texts())
	}
	if err := tt.Click(locales.EgoBrowserDeleteBinding()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserDeleteReview()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserDeleteReviewTarget("binding", "binding-1", "2")) {
		t.Fatalf("deletion review missing exact target: %v", tt.Texts())
	}
	if err := tt.Click(locales.EgoBrowserDeleteConfirm()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserDeleteAccepted("binding", "binding-1")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserDeleteAccepted("binding", "binding-1")) {
		t.Fatalf("deletion receipt missing: %v", tt.Texts())
	}
}

func TestEgoBrowserClaimUsesStructuredReviewAndReceipt(t *testing.T) {
	binary := testutil.InstallControlHelper(t)
	i18n.SetLocaleFunc(func() string { return "en" })
	t.Cleanup(func() { i18n.SetLocaleFunc(nil) })
	a, err := app.NewWithOptions(t.TempDir(), app.Options{FindExecutable: func(name string) (string, error) {
		if name == "agent-remote" {
			return binary, nil
		}
		return "", errors.New("test component not installed")
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = a.Close() })
	tt := ui.NewTester(a.View, 1100, 820)
	tt.SetLocale("en")
	if err := tt.Click(locales.CapabilitiesTitle()); err != nil {
		t.Fatal(err)
	}
	waitRemoteText(t, tt, locales.CapabilitiesState("unverified"))
	if err := tt.Click(locales.CapabilitiesCommands()); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click(locales.CapabilitiesFilter()); err != nil {
		t.Fatal(err)
	}
	tt.Type("ego-browser connect")
	if err := tt.Click(locales.CliFormOpen()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserCandidateClaim()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserCandidateClaim()) {
		t.Fatalf("structured claim action missing: %v", tt.Texts())
	}
	if err := tt.Click(locales.EgoBrowserCandidateClaim()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserClaimReview()) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserClaimReviewTarget("session-1", "Claude Studio")) {
		t.Fatalf("claim review missing exact session: %v", tt.Texts())
	}
	if err := tt.Click(locales.EgoBrowserClaimConfirm()); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(15 * time.Second)
	for !tt.HasText(locales.EgoBrowserClaimAccepted("session-1")) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
		tt.Frame()
	}
	if !tt.HasText(locales.EgoBrowserClaimAccepted("session-1")) {
		t.Fatalf("claim receipt missing: %v", tt.Texts())
	}
}
