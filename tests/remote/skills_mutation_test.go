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

func TestChangeSkillUsesExactGenerationAndReturnsSafeReceipt(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	receipt, err := remote.ChangeSkill(context.Background(), client, remote.SkillChangeInput{
		Confirmed: true, Expected: cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device},
		ExpectedGeneration: 7, Skill: "11111111-1111-4111-8111-111111111111", Action: "disable",
		Scope: remote.SkillScope{Tools: []string{}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Accepted || receipt.Action != "disable" || receipt.Generation != 8 || receipt.Skill == "" {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
}

func TestChangeSkillFailsClosedForUnsupportedOrChangedInput(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	_, err := remote.ChangeSkill(context.Background(), client, remote.SkillChangeInput{Confirmed: true, Expected: expected, ExpectedGeneration: 7, Skill: "11111111-1111-4111-8111-111111111111", Action: "enable", Scope: remote.SkillScope{}})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("unsupported change capability accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	_, err = remote.ChangeSkill(context.Background(), client, remote.SkillChangeInput{Confirmed: true, Expected: expected, ExpectedGeneration: 7, Skill: "11111111-1111-4111-8111-111111111111", Action: "enable", Scope: remote.SkillScope{Tools: []string{"zeta", "alpha"}}})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unsorted scope accepted: %v", err)
	}
}

func TestListSkillConflictsProjectsOnlySafeMetadata(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server := "https://test.invalid"
	catalog, err := remote.ListSkillConflicts(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: strptr("device-1")}, remote.SkillStateConflictsInput{
		AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.Publications.Items) != 1 || len(catalog.Migrations.Items) != 1 || catalog.Publications.Items[0].Status != "conflicted" {
		t.Fatalf("unexpected conflict projection: %+v", catalog)
	}
	if catalog.Migrations.Items[0].Name != "Review helper" || catalog.Migrations.Items[0].SkillID == "" {
		t.Fatalf("migration metadata missing: %+v", catalog.Migrations.Items[0])
	}
}

func TestListSkillConflictsRejectsAmbiguousOrUnsupportedScope(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server := "https://test.invalid"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: strptr("device-1")}
	skill := "11111111-1111-4111-8111-111111111111"
	_, err := remote.ListSkillConflicts(context.Background(), client, expected, remote.SkillStateConflictsInput{AccountID: expected.Home, Scope: "account-directory", Skill: &skill, Limit: 100})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("directory skill selector accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	_, err = remote.ListSkillConflicts(context.Background(), client, expected, remote.SkillStateConflictsInput{AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Limit: 100})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("unsupported conflict capability accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-conflicts-unknown")
	_, err = remote.ListSkillConflicts(context.Background(), client, expected, remote.SkillStateConflictsInput{AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Limit: 100})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unknown conflict field crossed boundary: %v", err)
	}
}

func strptr(value string) *string { return &value }

func TestListSkillStateProjectsCheckpointAndPendingMetadata(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server := "https://test.invalid"
	state, err := remote.ListSkillState(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: strptr("device-1")}, remote.SkillStateListInput{
		AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Checkpoints.Items) != 1 || !state.Checkpoints.Items[0].Retained || !state.Checkpoints.Items[0].IsHead {
		t.Fatalf("unexpected checkpoint history: %+v", state.Checkpoints)
	}
	if len(state.Pending.Items) != 1 || state.Pending.Items[0].Status != "upload_pending" || state.Pending.Items[0].ExportableFromServer {
		t.Fatalf("unexpected pending history: %+v", state.Pending)
	}
}

func TestListSkillStateAcceptsIndependentPageCursors(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server := "https://test.invalid"
	state, err := remote.ListSkillState(context.Background(), client, cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: strptr("device-1")}, remote.SkillStateListInput{
		AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Limit: 100,
		Cursor: strptr("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"), PendingCursor: strptr("bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Checkpoints.Items) != 1 || len(state.Pending.Items) != 1 {
		t.Fatalf("independent cursor query lost either page: %+v", state)
	}
}

func TestListSkillStateRejectsInvalidScopeAndUnsupportedCapability(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server := "https://test.invalid"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: strptr("device-1")}
	_, err := remote.ListSkillState(context.Background(), client, expected, remote.SkillStateListInput{AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Skill: strptr("11111111-1111-4111-8111-111111111111"), Limit: 100})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("directory skill selector accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	_, err = remote.ListSkillState(context.Background(), client, expected, remote.SkillStateListInput{AccountID: "11111111-1111-4111-8111-111111111111", Scope: "account-directory", Limit: 100})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("unsupported history capability accepted: %v", err)
	}
}

func TestDiffSkillStateProjectsBoundedPathMetadata(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	account := "11111111-1111-4111-8111-111111111111"
	diff, err := remote.DiffSkillState(context.Background(), client, expected, remote.SkillStateDiffInput{
		AccountID: account, Scope: "account-directory", Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	if diff.Mode != "current" || diff.CheckpointID == "" || len(diff.Items) != 1 || diff.Items[0].Path != "README.md" {
		t.Fatalf("unexpected diff: %+v", diff)
	}
	if diff.Items[0].Base == nil || diff.Items[0].Current == nil || diff.Items[0].Base.SHA256 == diff.Items[0].Current.SHA256 {
		t.Fatalf("diff metadata was not projected: %+v", diff.Items)
	}
}

func TestDiffSkillStateRejectsAmbiguousSelectorAndUnknownFields(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	checkpoint := "55555555-5555-4555-8555-555555555555"
	account := "11111111-1111-4111-8111-111111111111"
	_, err := remote.DiffSkillState(context.Background(), client, expected, remote.SkillStateDiffInput{
		AccountID: account, Scope: "account-directory", CheckpointID: &checkpoint, Limit: 100,
	})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("ambiguous diff selector accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-diff-unknown")
	_, err = remote.DiffSkillState(context.Background(), client, expected, remote.SkillStateDiffInput{AccountID: account, Scope: "account-directory", Limit: 100})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unknown diff field crossed boundary: %v", err)
	}
}

func TestSkillRetryPlansExactFailedTargetsAndControlsWithFingerprint(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	operationID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	plan, err := remote.PlanSkillRetry(context.Background(), client, expected, remote.SkillRetryPlanInput{Expected: expected, OperationID: operationID})
	if err != nil {
		t.Fatal(err)
	}
	if plan.TargetCount != 1 || plan.Generation != 7 || !plan.RequiresConfirm || len(plan.Targets) != 1 || plan.Targets[0].AttemptNumber != 1 {
		t.Fatalf("unexpected retry plan: %+v", plan)
	}
	receipt, err := remote.ControlSkillRetry(context.Background(), client, remote.SkillRetryControlInput{
		Confirmed: true, Expected: expected, OperationID: operationID, ExpectedGeneration: plan.Generation, ExpectedPlanFingerprint: plan.PlanFingerprint,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.Accepted || receipt.SourceOperationID != operationID || receipt.TargetCount != 1 || len(receipt.Targets) != 1 || receipt.Targets[0].Readiness != "pending" {
		t.Fatalf("unexpected retry receipt: %+v", receipt)
	}
}

func TestSkillRetryRejectsUnsupportedAndUnconfirmedRequests(t *testing.T) {
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: t.TempDir()}
	server, device := "https://test.invalid", "device-1"
	expected := cliadapter.ConfigContext{Home: client.Home, ServerURL: &server, DeviceID: &device}
	operationID := "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "skills-unsupported")
	_, err := remote.PlanSkillRetry(context.Background(), client, expected, remote.SkillRetryPlanInput{Expected: expected, OperationID: operationID})
	var operation *cliadapter.OperationError
	if !errors.As(err, &operation) || operation.Code != "unsupported_method" {
		t.Fatalf("unsupported retry capability accepted: %v", err)
	}
	t.Setenv("DESKTOP_TEST_CLI_SCENARIO", "")
	_, err = remote.ControlSkillRetry(context.Background(), client, remote.SkillRetryControlInput{Expected: expected, OperationID: operationID, ExpectedGeneration: 7, ExpectedPlanFingerprint: strings.Repeat("a", 64)})
	if !errors.As(err, &operation) || operation.Code != "invalid_params" {
		t.Fatalf("unconfirmed retry accepted: %v", err)
	}
}
