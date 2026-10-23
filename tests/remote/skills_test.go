package remote_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/cliadapter"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/remote"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestListSkillsUsesUserCapabilityAndProjectsSafeCatalog(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	catalog, err := remote.ListSkills(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}, remote.SkillListOptions{Effective: true, IncludeSystem: true})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.Generation != 7 || len(catalog.Items) != 1 || len(catalog.LocalItems) != 1 {
		t.Fatalf("unexpected catalog: %+v", catalog)
	}
	if catalog.Items[0].Effective == nil || !catalog.Items[0].Effective.Included || catalog.Items[0].Effective.RevisionID != "revision-1" {
		t.Fatalf("effective rule was not preserved: %+v", catalog.Items[0])
	}
}

func TestListSkillsFailsClosedWhenCapabilityOrContextChanges(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	_, err := remote.ListSkills(context.Background(), client, expected, remote.SkillListOptions{})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("unsupported skills capability was accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "context-changed")
	_, err = remote.ListSkills(context.Background(), client, expected, remote.SkillListOptions{})
	if !errors.Is(err, fault.CLIContextChanged) {
		t.Fatalf("changed context was accepted: %v", err)
	}
}

func TestListSkillsRejectsAmbiguousScopeAndMalformedCatalog(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	_, err := remote.ListSkills(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}, remote.SkillListOptions{Tool: "claude", AccountID: "account-1"})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("ambiguous scope was accepted: %v", err)
	}
}

func TestInfoSkillUsesExactUserScopeAndProjectsMetadataOnly(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	item, err := remote.InfoSkill(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}, remote.SkillInfoInput{Skill: "11111111-1111-4111-8111-111111111111"})
	if err != nil {
		t.Fatal(err)
	}
	if item.Kind != "library" || item.Name != "Review helper" || item.RevisionCount != 2 || item.Effective == nil || !item.Effective.Included {
		t.Fatalf("unexpected skill details: %+v", item)
	}
	if item.Status != "" || item.AccountID != "" || item.SourceCheckpoint != "" {
		t.Fatalf("unexpected private or local fields in library projection: %+v", item)
	}
}

func TestInfoSkillFailsClosedForAmbiguousScopeOrChangedContext(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	account := "11111111-1111-4111-8111-111111111111"
	_, err := remote.InfoSkill(context.Background(), client, expected, remote.SkillInfoInput{Skill: "review-helper", Tool: "claude", AccountID: account})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("ambiguous Skill scope was accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	_, err = remote.InfoSkill(context.Background(), client, expected, remote.SkillInfoInput{Skill: account})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("unsupported Skill info capability was accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "context-changed")
	_, err = remote.InfoSkill(context.Background(), client, expected, remote.SkillInfoInput{Skill: account})
	if !errors.Is(err, fault.CLIContextChanged) {
		t.Fatalf("changed context was accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-info-unknown")
	_, err = remote.InfoSkill(context.Background(), client, expected, remote.SkillInfoInput{Skill: account})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unknown Skill info field crossed boundary: %v", err)
	}
}

func TestInfoSkillRejectsNilUUIDBeforeCLI(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	_, err := remote.InfoSkill(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}, remote.SkillInfoInput{Skill: "00000000-0000-0000-0000-000000000000"})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("nil Skill UUID was accepted: %v", err)
	}
}

func TestInfoSkillStateProjectsOneCheckpointAndUsesUserCapability(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	checkpointID := "55555555-5555-4555-8555-555555555555"
	info, err := remote.InfoSkillState(context.Background(), client, expected, checkpointID)
	if err != nil {
		t.Fatal(err)
	}
	if info.Status != "ready" || info.Checkpoint == nil || info.Checkpoint.ID != checkpointID || info.Checkpoint.StorageLocation != "server" {
		t.Fatalf("unexpected checkpoint info: %+v", info)
	}
	if info.Checkpoint.ContentDigest != strings.Repeat("a", 64) {
		t.Fatalf("checkpoint digest missing: %+v", info.Checkpoint)
	}
}
