package workspace_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

func TestNestedSplitRemovalPreservesOtherSessions(t *testing.T) {
	p := &workspace.Pane{SessionID: "a"}
	if !p.Split("a", "b", false) || !p.Split("b", "c", true) {
		t.Fatal("split failed")
	}
	if p.Split("missing", "d", false) {
		t.Fatal("split replaced an unrelated pane")
	}
	p = p.Remove("b")
	ids := p.Leaves()
	if len(ids) != 2 || ids[0] != "a" || ids[1] != "c" {
		t.Fatalf("lost surviving pane: %v", ids)
	}
	p = p.Remove("a")
	if p.First != nil || p.SessionID != "c" {
		t.Fatal("single pane did not collapse")
	}
	if p.Remove("c") != nil {
		t.Fatal("last pane not removed")
	}
}

func TestProjectUsesCanonicalDirectory(t *testing.T) {
	dir := t.TempDir()
	p, err := workspace.NewProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.Path == "" || p.Name == "" || p.ID == "" {
		t.Fatalf("invalid project: %+v", p)
	}
	if _, err := workspace.NewProject(dir + "/missing"); err == nil {
		t.Fatal("nonexistent project accepted")
	}
}
