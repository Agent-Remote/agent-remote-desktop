package testutil

import (
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

func SavedWorkspace(t *testing.T) storage.State {
	t.Helper()
	one, two := t.TempDir(), t.TempDir()
	state := storage.State{Preferences: workspace.Defaults(), Projects: []workspace.Project{{ID: "one", Name: "First project", Path: one}, {ID: "two", Name: "Second project", Path: two}}}
	state.Preferences.SelectedProject = "one"
	state.Preferences.SidebarWidth = 260
	for _, item := range []struct{ id, project, tool, dir string }{
		{"a", "one", "codex", one}, {"b", "one", "claude", one}, {"c", "one", "fclaude", one}, {"hidden", "one", "codex", one}, {"d", "two", "claude", two},
	} {
		state.Sessions = append(state.Sessions, workspace.Session{ID: item.id, ProjectID: item.project, Tool: item.tool, Directory: item.dir, Started: time.Date(2026, 10, 18, 1, 2, 3, 0, time.UTC)})
	}
	tab := state.Board.Add("one", "a", "Review", "", false)
	state.Board.Add("one", "b", "Review", "a", false)
	tab.Root.Ratio = 64
	state.Board.Add("one", "c", "Remote", "", false)
	state.Board.Add("two", "d", "Other", "", false)
	state.Board.Select(tab.ID)
	return state
}

func StoreWorkspace(t *testing.T, home string, state storage.State) {
	t.Helper()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := s.Save(state); err != nil {
		t.Fatal(err)
	}
}

func LoadWorkspace(t *testing.T, home string) storage.State {
	t.Helper()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	return state
}
