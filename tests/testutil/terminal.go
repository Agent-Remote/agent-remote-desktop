package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// InstallTerminalFixture returns a native long-running Agent stand-in for
// Windows tests. POSIX tests keep their shell fixtures so their signal/trap
// assertions remain exact; Windows cannot execute those shell scripts as an
// Agent binary, so compile the small ConPTY fixture once per test.
func InstallTerminalFixture(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Fatal("InstallTerminalFixture is only valid on Windows")
	}
	root, err := repoRoot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "agent-fixture.exe")
	cmd := exec.Command("go", "build", "-o", path, "./tests/terminal-fixture")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build Windows terminal fixture: %v (%s)", err, output)
	}
	return path
}

func repoRoot() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", os.ErrNotExist
	}
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
