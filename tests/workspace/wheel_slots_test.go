package workspace_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

func TestAssignSlotFillsFreeSlotsThenEvictsLeastRecent(t *testing.T) {
	t0 := time.Unix(1_700_000_000, 0)
	var projects []workspace.Project
	for i := 0; i < workspace.WheelSlots; i++ {
		projects = append(projects, workspace.Project{ID: string(rune('a' + i)), LastOpened: t0.Add(time.Duration(i) * time.Hour)})
		workspace.AssignSlot(projects, i)
		if projects[i].Slot != i+1 {
			t.Fatalf("project %d got slot %d", i, projects[i].Slot)
		}
	}
	// The least recently opened (a, slot 1) leaves; others keep their slots.
	projects = append(projects, workspace.Project{ID: "new", LastOpened: t0.Add(100 * time.Hour)})
	workspace.AssignSlot(projects, len(projects)-1)
	if projects[len(projects)-1].Slot != 1 || projects[0].Slot != 0 {
		t.Fatalf("eviction: new=%d a=%d", projects[len(projects)-1].Slot, projects[0].Slot)
	}
	for i := 1; i < workspace.WheelSlots; i++ {
		if projects[i].Slot != i+1 {
			t.Fatalf("project %d moved to slot %d", i, projects[i].Slot)
		}
	}
	// A project already on the wheel keeps its slot.
	workspace.AssignSlot(projects, 3)
	if projects[3].Slot != 4 {
		t.Fatalf("assigned project moved to %d", projects[3].Slot)
	}
}

func TestProbeReadsBranchWorktreeDetachedAndLanguage(t *testing.T) {
	dir := t.TempDir()
	if got := workspace.Probe(dir); got != (workspace.ProjectFacts{}) {
		t.Fatalf("empty directory: %+v", got)
	}
	must := func(err error) {
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
	must(os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("ref: refs/heads/feature/zen\n"), 0o644))
	must(os.WriteFile(filepath.Join(dir, "Cargo.toml"), nil, 0o644))
	if got := workspace.Probe(dir); got.Branch != "feature/zen" || got.Detached || got.Language != "rust" {
		t.Fatalf("branch repo: %+v", got)
	}
	must(os.WriteFile(filepath.Join(dir, ".git", "HEAD"), []byte("0123456789abcdef0123456789abcdef01234567\n"), 0o644))
	if got := workspace.Probe(dir); got.Branch != "0123456" || !got.Detached {
		t.Fatalf("detached: %+v", got)
	}
	// A worktree's .git is a file naming the real git directory.
	wt, gitdir := t.TempDir(), t.TempDir()
	must(os.WriteFile(filepath.Join(gitdir, "HEAD"), []byte("ref: refs/heads/main\n"), 0o644))
	must(os.WriteFile(filepath.Join(wt, ".git"), []byte("gitdir: "+gitdir+"\n"), 0o644))
	must(os.WriteFile(filepath.Join(wt, "go.mod"), nil, 0o644))
	if got := workspace.Probe(wt); got.Branch != "main" || got.Language != "go" {
		t.Fatalf("worktree: %+v", got)
	}
}

func TestProbeCacheNeverBlocksAndReportsChanges(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	changed := make(chan struct{}, 1)
	cache := workspace.NewProbeCache(time.Minute, func() { changed <- struct{}{} })
	if _, known := cache.Get(dir); known {
		t.Fatal("first Get claimed to know the facts")
	}
	select {
	case <-changed:
	case <-time.After(5 * time.Second):
		t.Fatal("probe never reported")
	}
	if facts, known := cache.Get(dir); !known || facts.Language != "go" {
		t.Fatalf("cached facts = %+v %v", facts, known)
	}
}
