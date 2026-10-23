package release_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestReleaseVerifierAcceptsCommunityPolicy(t *testing.T) {
	for _, target := range []string{"linux/amd64", "windows/amd64"} {
		t.Run(target, func(t *testing.T) {
			release := makeRelease(t, target, "1.2.3", false)
			output := runVerifier(t, release)
			if !strings.Contains(output, `"verified": true`) {
				t.Fatalf("unexpected verifier output: %s", output)
			}
		})
	}
}

func TestReleaseVerifierCanRequireExactFormalTargetSet(t *testing.T) {
	release := makeReleaseSet(t, []string{"darwin/arm64", "windows/amd64", "linux/amd64"}, "1.2.3", true)
	output := runVerifier(t, release, "--allow-adhoc", "--require-targets", "darwin/arm64,windows/amd64,linux/amd64")
	if !strings.Contains(output, `"verified": true`) {
		t.Fatalf("unexpected verifier output: %s", output)
	}
	if !strings.Contains(output, `"signature_check": "macOS ad-hoc smoke signature accepted by explicit opt-in"`) {
		t.Fatalf("verifier misreported the Darwin ad-hoc target: %s", output)
	}
	if command := verifierCommand(t, release); run(append(command, "--allow-adhoc", "--require-targets", "darwin/arm64,windows/amd64,linux/arm64")) == nil {
		t.Fatal("release with a different formal target set passed verification")
	}
	if command := verifierCommand(t, release); run(append(command, "--allow-adhoc", "--require-targets", "darwin/arm64,windows/amd64")) == nil {
		t.Fatal("release with an extra target passed exact target verification")
	}
	if command := verifierCommand(t, release); run(append(command, "--allow-adhoc", "--require-targets", "")) == nil {
		t.Fatal("release verifier accepted an empty required target set")
	}
}

func TestMyGoLinuxPackageDeclaresTrayRuntimeDependency(t *testing.T) {
	root := testutil.Root(t)
	data, err := os.ReadFile(filepath.Join(root, "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Linux struct {
			Depends []string `json:"depends"`
		} `json:"linux"`
	}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatal(err)
	}
	// mygo's Debian backend already adds GTK and WebKitGTK. The desktop also
	// creates an AppIndicator tray icon, so at least one of the two compatible
	// Debian runtime package names must be declared by the application config.
	var found bool
	for _, dependency := range config.Linux.Depends {
		if strings.Contains(dependency, "libayatana-appindicator3-1") && strings.Contains(dependency, "libappindicator3-1") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("mygo.json Linux depends must provide an AppIndicator alternative, got %v", config.Linux.Depends)
	}
}

func TestReleaseVerifierRequiresExplicitAdHocOptIn(t *testing.T) {
	release := makeRelease(t, "darwin/arm64", "1.2.3", true)
	if command := verifierCommand(t, release); run(command) == nil {
		t.Fatal("ad-hoc macOS release passed formal verification")
	}
	output := runVerifier(t, release, "--allow-adhoc")
	if !strings.Contains(output, `"verified": true`) {
		t.Fatalf("unexpected smoke verifier output: %s", output)
	}
}

func TestReleaseVerifierRejectsAdHocCertificateFingerprint(t *testing.T) {
	release := makeRelease(t, "darwin/arm64", "1.2.3", true)
	manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	policy := manifest["signing_policy"].(map[string]any)
	policy["macos"].(map[string]any)["certificate_sha1"] = strings.Repeat("B", 40)
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if command := verifierCommand(t, release); run(append(command, "--allow-adhoc")) == nil {
		t.Fatal("ad-hoc macOS release with a certificate fingerprint passed verification")
	}
}

func TestReleaseVerifierChecksCommunityCertificateSelfSignature(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.Root(t), "scripts", "verify-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, marker := range []string{
		"subject_issuer",
		"-issuer",
		"community signing certificate must be self-signed",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("release verifier omitted self-signature check marker %q", marker)
		}
	}
}

func TestReleaseVerifierRejectsPublisherSigningAndTampering(t *testing.T) {
	release := makeRelease(t, "linux/amd64", "1.2.3", false)
	manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	policy := manifest["signing_policy"].(map[string]any)
	policy["linux"].(map[string]any)["publisher_code_signing"] = true
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if command := verifierCommand(t, release); run(command) == nil {
		t.Fatal("release with Linux publisher signing passed verification")
	}

	// Restore policy, then alter a packaged byte without changing its digest.
	policy["linux"].(map[string]any)["publisher_code_signing"] = false
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(release, "artifacts", "linux-amd64", "agent"), []byte("tampered"), 0600); err != nil {
		t.Fatal(err)
	}
	if command := verifierCommand(t, release); run(command) == nil {
		t.Fatal("tampered release passed verification")
	}
}

func TestReleaseVerifierBindsManifestToDesktopIdentity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
		value string
	}{
		{name: "name", field: "name", value: "Another Desktop"},
		{name: "identifier", field: "identifier", value: "com.example.other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			release := makeRelease(t, "linux/amd64", "1.2.3", false)
			manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			manifest["app"].(map[string]any)[tc.field] = tc.value
			data, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifestPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			if command := verifierCommand(t, release); run(command) == nil {
				t.Fatalf("release with mismatched app %s passed verification", tc.field)
			}
		})
	}
}

func TestReleaseVerifierRejectsWindowsPublisherSigning(t *testing.T) {
	release := makeRelease(t, "windows/amd64", "1.2.3", false)
	manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	policy := manifest["signing_policy"].(map[string]any)
	policy["windows"].(map[string]any)["publisher_code_signing"] = true
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if command := verifierCommand(t, release); run(command) == nil {
		t.Fatal("release with Windows publisher signing passed verification")
	}
}

func TestReleaseVerifierRejectsNotarizationAndDeveloperIDPolicy(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(map[string]any)
	}{
		{name: "notarization", mutate: func(manifest map[string]any) {
			manifest["signing_policy"].(map[string]any)["macos"].(map[string]any)["notarization"] = true
		}},
		{name: "developer-id", mutate: func(manifest map[string]any) {
			mac := manifest["signing_policy"].(map[string]any)["macos"].(map[string]any)
			mac["identity"] = "Developer ID Application: Example"
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			release := makeRelease(t, "darwin/arm64", "1.2.3", false)
			manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			tc.mutate(manifest)
			data, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifestPath, data, 0600); err != nil {
				t.Fatal(err)
			}
			if command := verifierCommand(t, release); run(command) == nil {
				t.Fatalf("invalid macOS %s policy passed verification", tc.name)
			}
		})
	}
}

func TestReleaseVerifierRejectsUndeclaredTargetDirectory(t *testing.T) {
	release := makeRelease(t, "linux/amd64", "1.2.3", false)
	if err := os.MkdirAll(filepath.Join(release, "artifacts", "freebsd-amd64"), 0755); err != nil {
		t.Fatal(err)
	}
	if command := verifierCommand(t, release); run(command) == nil {
		t.Fatal("undeclared FreeBSD artifact directory passed verification")
	}
}

func TestReleaseVerifierRejectsUnexpectedArguments(t *testing.T) {
	release := makeRelease(t, "linux/amd64", "1.2.3", false)
	command := verifierCommand(t, release)
	command = append(command, "unexpected")
	if run(command) == nil {
		t.Fatal("release verifier accepted an unexpected positional argument")
	}
}

func TestReleaseVerifierRejectsDeclaredTargetWithoutArtifacts(t *testing.T) {
	release := makeRelease(t, "linux/amd64", "1.2.3", false)
	manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	manifest["targets"] = []string{"linux/amd64", "windows/amd64"}
	data, err = json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	if command := verifierCommand(t, release); run(command) == nil {
		t.Fatal("declared target without artifacts passed verification")
	}
}

func TestReleaseVerifierRequiresDesktopExecutableAndInstaller(t *testing.T) {
	for _, missing := range []string{"agent-remote-desktop-installer", "agent-remote-desktop"} {
		t.Run(missing, func(t *testing.T) {
			release := makeRelease(t, "linux/amd64", "1.2.3", false)
			manifestPath := filepath.Join(release, "metadata", "release-manifest.json")
			data, err := os.ReadFile(manifestPath)
			if err != nil {
				t.Fatal(err)
			}
			var manifest map[string]any
			if err := json.Unmarshal(data, &manifest); err != nil {
				t.Fatal(err)
			}
			entries := manifest["artifacts"].([]any)
			filtered := make([]any, 0, len(entries))
			for _, raw := range entries {
				entry := raw.(map[string]any)
				path := entry["path"].(string)
				if strings.HasSuffix(path, "/"+missing) {
					if err := os.Remove(filepath.Join(release, filepath.FromSlash(path))); err != nil {
						t.Fatal(err)
					}
					continue
				}
				filtered = append(filtered, raw)
			}
			manifest["artifacts"] = filtered
			data, err = json.Marshal(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(manifestPath, data, 0644); err != nil {
				t.Fatal(err)
			}
			sumsPath := filepath.Join(release, "SHA256SUMS")
			sums, err := os.ReadFile(sumsPath)
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(sums), "\n")
			kept := lines[:0]
			for _, line := range lines {
				if !strings.HasSuffix(strings.TrimSpace(line), "/"+missing) {
					kept = append(kept, line)
				}
			}
			if err := os.WriteFile(sumsPath, []byte(strings.Join(kept, "\n")), 0644); err != nil {
				t.Fatal(err)
			}
			if command := verifierCommand(t, release); run(command) == nil {
				t.Fatalf("release missing %s passed verification", missing)
			}
		})
	}
}

func TestVersionFlagMatchesPackagedManifestVersion(t *testing.T) {
	root := testutil.Root(t)
	configData, err := os.ReadFile(filepath.Join(root, "mygo.json"))
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatal(err)
	}
	if config.Version == "" {
		t.Fatal("mygo.json has no application version")
	}
	command := exec.Command("go", "run", ".", "--version")
	command.Dir = root
	command.Env = withoutBuildSelectors(os.Environ())
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("version probe failed: %v\n%s", err, output)
	}
	want := fmt.Sprintf("agent-remote-desktop %s\n", config.Version)
	if string(output) != want {
		t.Fatalf("version output = %q, want %q", output, want)
	}
}

func TestWindowsPackagedSmokeEntryPointIsPresent(t *testing.T) {
	path := filepath.Join(testutil.Root(t), "scripts", "release-native-smoke.ps1")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Windows packaged smoke entry point is missing: %v", err)
	}
	script := string(data)
	for _, marker := range []string{
		"RuntimeInformation",
		"windows/$arch",
		"--release",
		"--status",
		"--version",
		"GetTempPath",
		"publisher_code_signing",
		"ad-hoc/smoke",
		"certificate_sha1",
		"developer id",
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("Windows packaged smoke entry point omitted %q", marker)
		}
	}
}

func TestPackagedSmokeReadsNestedInstallerStatus(t *testing.T) {
	root := testutil.Root(t)
	sh, err := os.ReadFile(filepath.Join(root, "scripts", "release-native-smoke.sh"))
	if err != nil {
		t.Fatalf("Unix packaged smoke entry point is missing: %v", err)
	}
	if !strings.Contains(string(sh), "current.get(\"target\")") || !strings.Contains(string(sh), "current.get(\"version\")") {
		t.Fatal("Unix packaged smoke must validate target/version from installer status.current")
	}
	pwsh, err := os.ReadFile(filepath.Join(root, "scripts", "release-native-smoke.ps1"))
	if err != nil {
		t.Fatalf("Windows packaged smoke entry point is missing: %v", err)
	}
	if !strings.Contains(string(pwsh), "$status.current.target") || !strings.Contains(string(pwsh), "$status.current.version") {
		t.Fatal("Windows packaged smoke must validate target/version from installer status.current")
	}
}

func TestNativeAcceptanceClearsPreviousEvidenceBeforeRetry(t *testing.T) {
	root := testutil.Root(t)
	sh, err := os.ReadFile(filepath.Join(root, "scripts", "native-acceptance.sh"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`rm -rf "$EVIDENCE_DIR/wireguard"`,
		`"$EVIDENCE_DIR"/summary.txt`,
		`"$EVIDENCE_DIR"/wireguard.log`,
		`write_summary`,
		`RUN_STATUS="passed"`,
		`ui_display=`,
	} {
		if !strings.Contains(string(sh), marker) {
			t.Fatalf("native-acceptance.sh does not clear stale evidence marker %q", marker)
		}
	}
	pwsh, err := os.ReadFile(filepath.Join(root, "scripts", "native-acceptance.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, marker := range []string{
		`"wireguard.log"`,
		`Remove-Item -Recurse -Force -LiteralPath $wireguardEvidence`,
	} {
		if !strings.Contains(string(pwsh), marker) {
			t.Fatalf("native-acceptance.ps1 does not clear stale evidence marker %q", marker)
		}
	}
}

func TestCIHostEvidenceIncludesWorkbenchProbe(t *testing.T) {
	root := testutil.Root(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("CI workflow is missing: %v", err)
	}
	workflow := string(data)
	if !strings.Contains(workflow, "scripts/native-acceptance.sh") || !strings.Contains(workflow, "native-acceptance.ps1") {
		t.Fatal("CI workflow must run native acceptance on Unix and Windows hosts")
	}
	// The uploaded native evidence is later consumed by
	// verify-native-evidence.sh, which requires a successful workbench log.
	// Keep the workbench probe in the same acceptance directory so a green
	// standalone step cannot be mistaken for complete host evidence.
	if strings.Contains(workflow, "DESKTOP_NATIVE_SKIP_WORKBENCH") || strings.Contains(workflow, "-SkipWorkbench") {
		t.Fatal("CI native evidence must include the workbench probe")
	}
}

func TestCIHostEvidencePinsAcceptanceToMatrixTarget(t *testing.T) {
	root := testutil.Root(t)
	data, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "ci.yml"))
	if err != nil {
		t.Fatalf("CI workflow is missing: %v", err)
	}
	workflow := string(data)
	for _, marker := range []string{
		"- os: macos-latest\n            goos: darwin",
		"- os: ubuntu-latest\n            goos: linux",
		"- os: windows-latest\n            goos: windows",
		"run: scripts/native-acceptance.sh --target \"${{ matrix.goos }}\"",
		"run: ./scripts/native-acceptance.ps1 -EvidenceDir \"$env:DESKTOP_NATIVE_EVIDENCE_DIR\" -Target \"${{ matrix.goos }}\"",
	} {
		if !strings.Contains(workflow, marker) {
			t.Errorf("CI native acceptance omitted explicit matrix target contract %q", marker)
		}
	}
}

func TestUnixPackagedSmokeRejectsUnexpectedArguments(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required to run packaged smoke entry point: %v", err)
	}
	path := filepath.Join(testutil.Root(t), "scripts", "release-native-smoke.sh")
	if err := exec.Command(bash, path, t.TempDir(), "unexpected").Run(); err == nil {
		t.Fatal("packaged smoke entry point accepted an unexpected argument")
	}
}

func TestUnixPackagedSmokeVerifiesInstalledMacOSBundle(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.Root(t), "scripts", "release-native-smoke.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, marker := range []string{
		`if [[ "$goos" == darwin ]]; then`,
		`command -v codesign`,
		`while [[ "$app_bundle" != "$destination" && "$app_bundle" != *.app ]]`,
		`codesign --verify --deep --strict --verbose=2 "$app_bundle"`,
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("packaged smoke omitted installed macOS signature check marker %q", marker)
		}
	}
}

func TestBuildReleaseRejectsDuplicateTargetsBeforeBuild(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required to run release build entry point: %v", err)
	}
	root := testutil.Root(t)
	out := t.TempDir()
	command := exec.Command(bash, filepath.Join(root, "scripts", "build-release.sh"),
		"--platforms", "linux/amd64,linux/amd64", "--out", out)
	command.Dir = root
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("duplicate release targets unexpectedly passed: %s", output)
	}
	if !strings.Contains(string(output), "duplicate target 'linux/amd64'") {
		t.Fatalf("duplicate target error = %q", output)
	}
}

func TestBuildReleaseNormalizesWindowsExecutableName(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(testutil.Root(t), "scripts", "build-release.sh"))
	if err != nil {
		t.Fatal(err)
	}
	script := string(data)
	for _, marker := range []string{
		`windows_binary="$windows_dir/Agent Remote Desktop.exe"`,
		`normalized_binary="$windows_dir/agent-remote-desktop.exe"`,
		`mv -- "$windows_binary" "$normalized_binary"`,
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("build-release.sh omitted Windows executable normalization marker %q", marker)
		}
	}
}

func TestBuildReleaseDoesNotAllowExplicitAdHocIdentityWithoutOptIn(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required to run release build entry point: %v", err)
	}
	root := testutil.Root(t)
	out := t.TempDir()
	command := exec.Command(bash, filepath.Join(root, "scripts", "build-release.sh"),
		"--platforms", "darwin/arm64", "--out", out)
	command.Dir = root
	env := withoutBuildSelectors(os.Environ())
	env = append(env, "DESKTOP_MACOS_SIGNING_IDENTITY=-", "DESKTOP_ALLOW_ADHOC=0")
	command.Env = env
	output, err := command.CombinedOutput()
	if err == nil {
		t.Fatalf("explicit ad-hoc macOS identity unexpectedly passed formal build: %s", output)
	}
	if !strings.Contains(string(output), "macOS ad-hoc signing is smoke-only") {
		t.Fatalf("explicit ad-hoc identity error = %q", output)
	}
}

func withoutBuildSelectors(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, entry := range environment {
		if strings.HasPrefix(entry, "GOOS=") || strings.HasPrefix(entry, "GOARCH=") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func runVerifier(t *testing.T, release string, extra ...string) string {
	t.Helper()
	command := verifierCommand(t, release)
	command = append(command, extra...)
	output, err := exec.Command(command[0], command[1:]...).CombinedOutput()
	if err != nil {
		t.Fatalf("verifier failed: %v\n%s", err, output)
	}
	return string(output)
}

func verifierCommand(t *testing.T, release string) []string {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required to run release verifier: %v", err)
	}
	return []string{bash, filepath.Join(testutil.Root(t), "scripts", "verify-release.sh"), release}
}

func run(command []string) error {
	return exec.Command(command[0], command[1:]...).Run()
}

func makeRelease(t *testing.T, target, version string, adhoc bool) string {
	return makeReleaseSet(t, []string{target}, version, adhoc)
}

func makeReleaseSet(t *testing.T, targets []string, version string, adhoc bool) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "metadata"), 0755); err != nil {
		t.Fatal(err)
	}
	artifacts := make([]map[string]any, 0, len(targets))
	checksumLines := make([]string, 0, len(targets))
	for _, target := range targets {
		targetKey := strings.ReplaceAll(target, "/", "-")
		artifactDir := filepath.Join(root, "artifacts", targetKey)
		if err := os.MkdirAll(artifactDir, 0755); err != nil {
			t.Fatal(err)
		}
		installerName := "agent-remote-desktop-installer"
		if strings.HasPrefix(target, "windows/") {
			installerName += ".exe"
		}
		names := []string{installerName}
		switch {
		case strings.HasPrefix(target, "windows/"):
			names = append(names, "agent-remote-desktop.exe")
		case strings.HasPrefix(target, "linux/"):
			names = append(names, "agent-remote-desktop")
		case strings.HasPrefix(target, "darwin/"):
			names = append(names, "Agent Remote Desktop.app/Contents/MacOS/Agent Remote Desktop")
		}
		for _, name := range names {
			artifactPath := filepath.Join(artifactDir, filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(artifactPath), 0755); err != nil {
				t.Fatal(err)
			}
			contents := []byte("release artifact " + target + " " + name)
			if err := os.WriteFile(artifactPath, contents, 0644); err != nil {
				t.Fatal(err)
			}
			digest := sha256.Sum256(contents)
			hashText := hex.EncodeToString(digest[:])
			relative := filepath.ToSlash(filepath.Join("artifacts", targetKey, name))
			artifacts = append(artifacts, map[string]any{"path": relative, "size": len(contents), "sha256": hashText})
			checksumLines = append(checksumLines, hashText+"  "+relative+"\n")
		}
	}
	macMode, identity := "project-self-signed", "Agent Remote Community Code Signing"
	var certificate any = strings.Repeat("A", 40)
	if adhoc {
		macMode, identity, certificate = "ad-hoc/smoke", "-", nil
	}
	manifest := map[string]any{
		"schema_version": 1,
		"app":            map[string]string{"name": "Agent Remote Desktop", "version": version, "identifier": "dev.agent-remote.desktop"},
		"targets":        targets,
		"signing_policy": map[string]any{
			"macos":   map[string]any{"identity": identity, "mode": macMode, "certificate_sha1": certificate, "notarization": false},
			"windows": map[string]any{"publisher_code_signing": false},
			"linux":   map[string]any{"publisher_code_signing": false},
		},
		"artifacts": artifacts,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metadata", "release-manifest.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(strings.Join(checksumLines, "")), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}
