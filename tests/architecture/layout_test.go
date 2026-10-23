package architecture_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestTestFilesRemainOutsideProductionDirectories(t *testing.T) {
	root := testutil.Root(t)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			switch rel {
			case ".git", "vendor", "dist", "build", "artifacts", "tests":
				return filepath.SkipDir
			}
			if entry.Name() == "testdata" || entry.Name() == "testutil" || entry.Name() == "native-probe" {
				t.Errorf("test-only directory must be under tests/: %s", rel)
			}
		} else if strings.HasSuffix(entry.Name(), "_test.go") {
			t.Errorf("test file must be under tests/: %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
