package workspace_test

import (
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

func TestIndependentTabsRetainTheirSplitLayouts(t *testing.T) {
	b := workspace.Board{}
	one := b.Add("project", "a", "Codex", "", false)
	b.Add("project", "b", "Claude", "a", false)
	two := b.Add("project", "c", "Codex", "", false)
	if len(b.Tabs) != 2 || len(one.Root.Leaves()) != 2 {
		t.Fatal("new tab destroyed previous split")
	}
	b.Show("project", "b", "Claude")
	if b.Current("project") != one {
		t.Fatal("existing session was recreated in another tab")
	}
	b.Hide("a")
	if one.Root.SessionID != "b" || two.Root.SessionID != "c" {
		t.Fatal("closing one pane damaged another tab")
	}
	b.Hide("b")
	if b.Current("project") != two {
		t.Fatal("active tab did not fall back to remaining tab")
	}
}

func TestReorderAndReconnectPreserveTabAndPaneIdentity(t *testing.T) {
	b := workspace.Board{}
	one := b.Add("p", "a", "first", "", false)
	b.Add("p", "b", "second", "a", true)
	one.Root.Ratio = 38
	two := b.Add("p", "c", "third", "", false)
	foreign := b.Add("other", "d", "other", "", false)
	b.Select(one.ID)
	if !b.Move(one.ID, two.ID) || b.Tabs[1] != one || b.Current("p") != one || one.FocusedSession != "b" {
		t.Fatal("reorder changed tab identity or focus")
	}
	if b.Move(one.ID, foreign.ID) {
		t.Fatal("tab crossed project boundary")
	}
	if !b.Replace("b", "reconnected") || one.Root.Second.SessionID != "reconnected" || one.Root.Ratio != 38 || !one.Root.Vertical || one.FocusedSession != "reconnected" {
		t.Fatal("reconnect lost pane position")
	}
	clone := b.Clone()
	b.Hide("reconnected")
	if clone.Tabs[1].Root.Second.SessionID != "reconnected" || one.FocusedSession != "a" {
		t.Fatal("snapshot aliased live layout or focus not repaired")
	}
}
