package storage_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	_ "modernc.org/sqlite"
)

// Redesign doc §7.3: new installs follow the system appearance; a state
// saved before the setting existed keeps its explicit theme.
func TestFreshInstallFollowsSystemAppearance(t *testing.T) {
	s, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !state.Preferences.FollowSystemTheme || state.Preferences.ReduceTransparency {
		t.Fatalf("fresh preferences = %+v", state.Preferences)
	}
}

func TestLegacyStateKeepsExplicitTheme(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	db, err := sql.Open("sqlite", filepath.Join(home, "desktop", "desktop.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	// Exactly what a v3 build wrote: no follow_system_theme, no reduce_transparency.
	legacy := `{"projects":[],"preferences":{"theme":"sakura","focus_mode":false,"font_size":16,"sidebar_width":240,"selected_project":""},"board":{"tabs":[],"active":{}},"sessions":[]}`
	if _, err := db.Exec("INSERT INTO desktop_state VALUES(1,3,?)", legacy); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if state.Preferences.FollowSystemTheme || state.Preferences.Theme != "sakura" || state.Preferences.FontSize != 16 {
		t.Fatalf("legacy preferences changed: %+v", state.Preferences)
	}
}

func TestAppearancePreferencesRoundtrip(t *testing.T) {
	home := t.TempDir()
	s, err := storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	state, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	state.Preferences.Theme = "daybreak"
	state.Preferences.FollowSystemTheme = true
	state.Preferences.ReduceTransparency = true
	if err := s.Save(state); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = storage.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	p := got.Preferences
	if !p.FollowSystemTheme || !p.ReduceTransparency || p.Theme != "daybreak" {
		t.Fatalf("appearance preferences lost: %+v", p)
	}
}

// Redesign §7.3: the chord modifier round-trips, and a value this build
// does not know (a newer version's) reads as the default chord.
func TestChordModifierRoundtripAndUnknownValue(t *testing.T) {
	for stored, want := range map[string]string{"ctrl-alt": "ctrl-alt", "": "", "hyper": ""} {
		home := t.TempDir()
		s, err := storage.Open(home)
		if err != nil {
			t.Fatal(err)
		}
		state, err := s.Load()
		if err != nil {
			t.Fatal(err)
		}
		state.Preferences.ChordModifier = stored
		if err := s.Save(state); err != nil {
			t.Fatal(err)
		}
		s.Close()
		s, err = storage.Open(home)
		if err != nil {
			t.Fatal(err)
		}
		state, err = s.Load()
		s.Close()
		if err != nil {
			t.Fatal(err)
		}
		if state.Preferences.ChordModifier != want {
			t.Errorf("stored %q, read %q, want %q", stored, state.Preferences.ChordModifier, want)
		}
	}
}
