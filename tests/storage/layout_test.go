package storage_test

import (
	"database/sql"
	"errors"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestFrozenLayoutSurvivesLaterUIEditsAndReopening(t *testing.T) {
	home := t.TempDir()
	state := testutil.SavedWorkspace(t)
	exit := 7
	state.Sessions[0].ExitCode = &exit
	snapshot := state.Clone()
	state.Board.Tabs[0].Root.Ratio = 22
	state.Board.Hide("b")
	state.Board.Move(state.Board.Tabs[0].ID, state.Board.Tabs[1].ID)
	state.Sessions[0].Tool = "changed"
	exit = 42
	testutil.StoreWorkspace(t, home, snapshot)
	got := testutil.LoadWorkspace(t, home)
	if !reflect.DeepEqual(got, snapshot) {
		t.Fatalf("saved snapshot was changed by UI edits:\ngot %+v\nwant %+v", got, snapshot)
	}
	if got.Board.Tabs[0].Root.Ratio != 64 || got.Board.Tabs[0].FocusedSession != "b" || *got.Sessions[0].ExitCode != 7 {
		t.Fatal("split geometry, focus or exit metadata was lost")
	}
}

func TestVersionOnePreferencesUpgradeWithoutLosingProjects(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	raw := `{"projects":[{"id":"old","name":"Old project","path":"/old/project"}],"preferences":{"theme":"daybreak","font_size":18,"sidebar_width":300,"selected_project":"old"}}`
	if _, err := db.Exec("INSERT INTO desktop_state VALUES(1,1,?)", raw); err != nil {
		t.Fatal(err)
	}
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(state.Projects) != 1 || state.Preferences.Theme != "daybreak" || state.Preferences.FontSize != 18 {
		t.Fatal("legacy preferences changed")
	}
	if err := s.Save(state); err != nil {
		t.Fatal(err)
	}
	var version int
	if err := db.QueryRow("SELECT version FROM desktop_state").Scan(&version); err != nil || version != 3 {
		t.Fatalf("upgrade version %d: %v", version, err)
	}
}

func TestInvalidLayoutDoesNotOverwriteValidHistory(t *testing.T) {
	home := t.TempDir()
	good := testutil.SavedWorkspace(t)
	testutil.StoreWorkspace(t, home, good)
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, change := range []func(*storage.State){
		func(s *storage.State) { s.Board.Tabs[0].Root.Second.SessionID = "missing" },
		func(s *storage.State) { s.Board.Tabs[0].Root.Second.SessionID = "a" },
		func(s *storage.State) { s.Board.Tabs[0].Root.Second = nil },
		func(s *storage.State) { s.Board.Tabs[0].Root.Ratio = 100 },
		func(s *storage.State) { s.Board.Tabs[0].Root.First = &workspace.Pane{SessionID: "d"} },
	} {
		bad := good.Clone()
		change(&bad)
		if err := s.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid layout accepted: %v", err)
		}
	}
	if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got, good) {
		t.Fatal("invalid write damaged previous history")
	}
}

func TestInvalidPreferencesDoNotOverwriteValidHistory(t *testing.T) {
	home := t.TempDir()
	good := testutil.SavedWorkspace(t)
	testutil.StoreWorkspace(t, home, good)
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, change := range []func(*storage.State){
		func(s *storage.State) { s.Preferences.Theme = "unknown-theme" },
		func(s *storage.State) { s.Preferences.FontSize = float32(math.NaN()) },
		func(s *storage.State) { s.Preferences.FontSize = float32(math.Inf(1)) },
		func(s *storage.State) { s.Preferences.FontSize = 9 },
		func(s *storage.State) { s.Preferences.SidebarWidth = float32(math.NaN()) },
		func(s *storage.State) { s.Preferences.SidebarWidth = 401 },
		func(s *storage.State) { s.Preferences.SelectedProject = "project\nname" },
	} {
		bad := good.Clone()
		change(&bad)
		if err := s.Save(bad); !errors.Is(err, fault.WorkspaceData) {
			t.Fatalf("invalid preferences accepted: %v", err)
		}
	}
	if got := testutil.LoadWorkspace(t, home); !reflect.DeepEqual(got, good) {
		t.Fatal("invalid preferences damaged previous history")
	}
}

func TestZeroPreferencesRemainCompatibleWithInMemoryState(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Save(storage.State{}); err != nil {
		s.Close()
		t.Fatalf("zero-value state should remain saveable: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	defaults := workspace.Defaults()
	// A saved state without follow_system_theme made an explicit theme
	// choice before the setting existed; it must not start following.
	defaults.FollowSystemTheme = false
	if state.Preferences != defaults {
		t.Fatalf("zero preferences were not normalized: got %+v want %+v", state.Preferences, defaults)
	}
}
