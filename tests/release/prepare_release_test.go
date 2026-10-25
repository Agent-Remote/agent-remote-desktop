package release_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestPrepareReleaseWorkflowAndScripts(t *testing.T) {
	root := testutil.Root(t)
	for _, name := range []string{
		"scripts/prepare-release.sh",
		"scripts/update-changelog.sh",
		".github/workflows/prepare-release.yml",
		"CHANGELOG.md",
	} {
		if _, err := os.Stat(filepath.Join(root, name)); err != nil {
			t.Fatalf("release source file is missing: %s: %v", name, err)
		}
	}

	workflowData, err := os.ReadFile(filepath.Join(root, ".github/workflows/prepare-release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowData)
	for _, marker := range []string{
		"workflow_dispatch:",
		"description: Release version without the leading v",
		"scripts/prepare-release.sh",
		"bash scripts/check.sh",
		"git commit -m \"chore: release v${VERSION}\"",
		"git push origin \"HEAD:${GITHUB_REF_NAME}\"",
		"git tag \"v${VERSION}\"",
		"git push origin \"v${VERSION}\"",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("prepare-release workflow omitted %q", marker)
		}
	}

	prepare, err := os.ReadFile(filepath.Join(root, "scripts/prepare-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		"mygo.json",
		"github.com/egoist/mygo/cmd/mygo i18n check",
		"github.com/egoist/mygo/cmd/mygo generate",
		"scripts/update-changelog.sh",
	} {
		if !strings.Contains(string(prepare), marker) {
			t.Errorf("prepare-release script omitted %q", marker)
		}
	}
}

func TestChangelogStartsWithCurrentFormalVersion(t *testing.T) {
	root := testutil.Root(t)
	var config struct {
		Version string `json:"version"`
	}
	data, err := os.ReadFile(filepath.Join(root, "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	if config.Version == "" || strings.HasSuffix(config.Version, "-dev") {
		t.Fatalf("mygo.json must contain the prepared formal version, got %q", config.Version)
	}

	changelog, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(changelog), "## v"+config.Version+" - ") {
		t.Fatalf("CHANGELOG.md has no entry for mygo.json version %s", config.Version)
	}
}
