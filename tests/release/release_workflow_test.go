package release_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

// Keep the release workflow's security and provenance boundary reviewable in
// the same repository as the scripts it invokes. This deliberately checks
// stable policy markers instead of parsing YAML with a second dependency.
func TestCommunityReleaseWorkflowIsTagBoundAndSignedOnMacOS(t *testing.T) {
	root := testutil.Root(t)
	path := filepath.Join(root, ".github", "workflows", "release.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("community release workflow is missing: %v", err)
	}
	workflow := string(data)
	for _, marker := range []string{
		"tags:",
		"- 'v*'",
		"refs/tags/v*",
		"actions/checkout@v4",
		"actions/setup-go@v6",
		"bash scripts/check.sh",
		"runs-on: macos-14",
		"environment: community-release",
		"COMMUNITY_MACOS_CERTIFICATE_P12_B64",
		"security import",
		"security set-key-partition-list",
		"DESKTOP_MACOS_SIGNING_IDENTITY",
		"DESKTOP_MACOS_SIGNER_CERTIFICATE_SHA1",
		"formal_targets='darwin/arm64,windows/amd64,linux/amd64'",
		"scripts/build-release.sh --platforms \"$formal_targets\" --out",
		"scripts/verify-release.sh \"$DESKTOP_RELEASE_OUT\" --require-targets \"$formal_targets\"",
		"Smoke test the installed macOS package",
		"scripts/release-native-smoke.sh \"$DESKTOP_RELEASE_OUT\"",
		"--verify-tag",
		"permissions:\n  contents: write",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("release workflow omitted %q", marker)
		}
	}
	if strings.Contains(workflow, "DESKTOP_ALLOW_ADHOC") {
		t.Error("formal release workflow must not opt into ad-hoc signing")
	}
	if strings.Contains(workflow, `echo "keychain_password=`) {
		t.Error("temporary keychain password must not be exported as a workflow output")
	}
	if strings.Contains(workflow, "Developer ID") || strings.Contains(workflow, "notar") {
		t.Error("formal release workflow must not introduce Developer ID or notarization")
	}
}

func TestCommunityReleaseWorkflowPublishesOnlyAfterOfflineVerification(t *testing.T) {
	root := testutil.Root(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "release.yml"))
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	verify := strings.Index(workflow, "scripts/verify-release.sh")
	smoke := strings.Index(workflow, "scripts/release-native-smoke.sh")
	archive := strings.Index(workflow, "Archive the verified release directory")
	publish := strings.Index(workflow, "Publish the tagged GitHub release")
	if verify < 0 || smoke < 0 || archive < 0 || publish < 0 || !(verify < smoke && smoke < archive && archive < publish) {
		t.Fatalf("release must verify and install-smoke before archive and publish (verify=%d smoke=%d archive=%d publish=%d)", verify, smoke, archive, publish)
	}
	if !strings.Contains(workflow, "actions/upload-artifact@v4") {
		t.Error("release workflow must retain the verified archive as workflow evidence")
	}
}

func TestCIWorkflowAggregatesAllNativeEvidence(t *testing.T) {
	root := testutil.Root(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("CI workflow is missing: %v", err)
	}
	workflow := string(data)
	job := strings.Index(workflow, "verify-native-evidence:")
	if job < 0 {
		t.Fatal("CI workflow must define a native evidence verifier job")
	}
	verifier := workflow[job:]
	for _, marker := range []string{
		"needs: test",
		"actions/download-artifact@v4",
		"native-acceptance-macos-latest",
		"native-acceptance-windows-latest",
		"native-acceptance-ubuntu-latest",
		"scripts/verify-native-evidence.sh",
		"--allow-virtual-display",
	} {
		if !strings.Contains(verifier, marker) {
			t.Errorf("native evidence verifier job omitted %q", marker)
		}
	}
	if !strings.Contains(workflow, "if-no-files-found: error") {
		t.Fatal("native acceptance artifact upload must fail when evidence is missing")
	}
	// The CI verifier is review evidence because Ubuntu runs under Xvfb. It
	// must still use the same fail-closed joiner and may not skip the workbench
	// or WireGuard checks by changing the acceptance inputs.
	if strings.Contains(verifier, "DESKTOP_NATIVE_SKIP_WORKBENCH") || strings.Contains(verifier, "-SkipWorkbench") {
		t.Fatal("native evidence verifier must consume complete acceptance records")
	}
}
