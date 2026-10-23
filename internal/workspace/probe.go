package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ProjectFacts are what the ⌥X wheel shows about a project. They come from
// two cheap local reads — .git/HEAD and a few marker files — never from the
// git executable or the network, and never claim a clean/dirty state.
type ProjectFacts struct {
	Branch   string // branch name, short commit for a detached HEAD, "" if not a repository
	Detached bool
	Language string // stable ID: go | rust | typescript | javascript | python | swift | cpp | ""
}

// markers are checked in order; the first present names the language.
var markers = []struct{ file, language string }{
	{"go.mod", "go"}, {"Cargo.toml", "rust"}, {"Package.swift", "swift"},
	{"pyproject.toml", "python"}, {"requirements.txt", "python"}, {"setup.py", "python"},
	{"tsconfig.json", "typescript"}, {"package.json", "javascript"}, {"CMakeLists.txt", "cpp"},
}

// Probe reads facts about the project at dir. Reads are bounded: HEAD is a
// single small file and markers are only stat-ed.
func Probe(dir string) ProjectFacts {
	var f ProjectFacts
	for _, m := range markers {
		if st, err := os.Stat(filepath.Join(dir, m.file)); err == nil && !st.IsDir() {
			f.Language = m.language
			break
		}
	}
	head := headPath(dir)
	if head == "" {
		return f
	}
	b, err := readSmall(head, 512)
	if err != nil {
		return f
	}
	line := strings.TrimSpace(string(b))
	if ref, ok := strings.CutPrefix(line, "ref: "); ok {
		f.Branch = strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/")
	} else if len(line) >= 7 && isHex(line) {
		f.Branch, f.Detached = line[:7], true
	}
	return f
}

// headPath finds HEAD for a repository or a worktree (.git as a file
// holding "gitdir: <path>").
func headPath(dir string) string {
	dot := filepath.Join(dir, ".git")
	st, err := os.Stat(dot)
	if err != nil {
		return ""
	}
	if st.IsDir() {
		return filepath.Join(dot, "HEAD")
	}
	b, err := readSmall(dot, 4096)
	if err != nil {
		return ""
	}
	gitdir, ok := strings.CutPrefix(strings.TrimSpace(string(b)), "gitdir: ")
	if !ok {
		return ""
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	return filepath.Join(gitdir, "HEAD")
}

func readSmall(path string, limit int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, limit)
	n, err := f.Read(buf)
	if n == 0 && err != nil {
		return nil, err
	}
	return buf[:n], nil
}

func isHex(s string) bool {
	for _, r := range s {
		if !strings.ContainsRune("0123456789abcdef", r) {
			return false
		}
	}
	return true
}

// ProbeCache keeps facts per project path and refreshes them off the UI
// thread. Get never blocks: it returns what is known and schedules a
// refresh when the entry is missing or older than maxAge; changed is called
// (from the worker goroutine) when new facts arrive.
type ProbeCache struct {
	mu      sync.Mutex
	entries map[string]probeEntry
	maxAge  time.Duration
	changed func()
	probe   func(string) ProjectFacts
}

type probeEntry struct {
	facts    ProjectFacts
	at       time.Time
	inFlight bool
}

func NewProbeCache(maxAge time.Duration, changed func()) *ProbeCache {
	return &ProbeCache{entries: map[string]probeEntry{}, maxAge: maxAge, changed: changed, probe: Probe}
}

func (c *ProbeCache) Get(dir string) (ProjectFacts, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[dir]
	if (!ok || time.Since(e.at) > c.maxAge) && !e.inFlight {
		e.inFlight = true
		c.entries[dir] = e
		go func() {
			facts := c.probe(dir)
			c.mu.Lock()
			c.entries[dir] = probeEntry{facts: facts, at: time.Now()}
			c.mu.Unlock()
			if c.changed != nil {
				c.changed()
			}
		}()
	}
	return e.facts, ok && !e.at.IsZero()
}
