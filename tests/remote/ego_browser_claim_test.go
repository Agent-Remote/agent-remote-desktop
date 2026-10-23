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

func TestEgoBrowserClaimUsesFreshCandidateAndExactConfirmation(t *testing.T) {
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	expected := egoBrowserTestContext(home)
	candidates, err := remote.LoadEgoBrowserCandidates(context.Background(), client, remote.EgoBrowserCandidatesInput{Expected: expected})
	if err != nil || len(candidates.Items) != 1 || !candidates.Items[0].Controllable {
		t.Fatalf("candidate projection = %+v, err=%v", candidates, err)
	}
	plan, err := remote.LoadEgoBrowserClaimPlan(context.Background(), client, remote.EgoBrowserClaimPlanInput{Expected: expected, ToolSessionID: candidates.Items[0].ToolSessionID, CandidateFingerprint: "fp-1"})
	if err != nil {
		t.Fatalf("claim plan error = %v", err)
	}
	if !plan.Allowed || !plan.RequiresConfirmation || plan.LocalAdmission != "ready" {
		t.Fatalf("unexpected claim plan: %+v", plan)
	}
	receipt, err := remote.ControlEgoBrowserClaim(context.Background(), client, remote.EgoBrowserClaimControlInput{Confirmed: true, Expected: expected, ToolSessionID: plan.Candidate.ToolSessionID, CandidateFingerprint: plan.CandidateFingerprint})
	if err != nil {
		t.Fatalf("claim control error = %v", err)
	}
	if receipt.OperationID == "" || receipt.Status != "claim_submitted" || receipt.LocalAdmission != "open" {
		t.Fatalf("unexpected claim receipt: %+v", receipt)
	}
}

func TestEgoBrowserClaimRequiresExactUserCapabilityAndConfirmation(t *testing.T) {
	caps := cliadapter.Capabilities{Version: cliadapter.ProtocolVersion, CLIVersion: "test", Platform: "linux", Operations: []cliadapter.Capability{{Method: remote.EgoBrowserClaimControlMethod, Effect: "write", Credential: "device", Confirmation: "exact_request"}}}
	if remote.EgoBrowserClaimControlSupported(caps) {
		t.Fatal("device credential must not satisfy user claim contract")
	}
	home := t.TempDir()
	client := cliadapter.Client{Binary: testutil.InstallControlHelper(t), Home: home}
	_, err := remote.ControlEgoBrowserClaim(context.Background(), client, remote.EgoBrowserClaimControlInput{Expected: egoBrowserTestContext(home), ToolSessionID: "session-1", CandidateFingerprint: "fp-1"})
	if !errors.Is(err, fault.CLIProtocol) {
		t.Fatalf("unconfirmed claim must fail closed, got %v", err)
	}
}
