package platform_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
)

func TestExecutableRejectsBlankNames(t *testing.T) {
	for _, name := range []string{"", "   ", "."} {
		if _, err := platform.Executable(name); err == nil || !strings.Contains(err.Error(), fault.ExecutableMissing.Error()) {
			t.Fatalf("Executable(%q) error = %v, want executable-missing error", name, err)
		}
	}
}

func TestExecutableReturnsAbsolutePathFromPATH(t *testing.T) {
	name := "sh"
	if filepath.Separator == '\\' {
		name = "cmd.exe"
	}
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s is unavailable on this host: %v", name, err)
	}
	path, err := platform.Executable(name)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(path) {
		t.Fatalf("Executable(%q) returned non-absolute path %q", name, path)
	}
}
