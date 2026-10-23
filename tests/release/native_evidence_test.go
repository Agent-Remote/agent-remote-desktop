package release_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/tests/testutil"
)

func TestNativeEvidenceVerifierRequiresAllPhysicalReleasePlatforms(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)

	output, err := runNativeEvidenceVerifier(t, mac, windows, linux)
	if err != nil {
		t.Fatalf("native evidence verifier failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), `"verified": true`) || !strings.Contains(string(output), `"cross_compilation": false`) {
		t.Fatalf("unexpected verifier output: %s", output)
	}

	if _, err := runNativeEvidenceVerifier(t, mac, windows); err == nil {
		t.Fatal("incomplete platform evidence passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsNonReleaseArchitecture(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	// Windows ARM64 is a supported smoke target, but it is outside the
	// formal release matrix and must not satisfy the release evidence gate.
	windows := writeNativeEvidence(t, root, "windows", "arm64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("non-release architecture passed formal native evidence verification")
	}
}

func TestNativeEvidenceVerifierCanRequireLinuxX11AndWaylandPair(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linuxX11 := writeNativeEvidenceNamed(t, root, "linux-x11", "linux", "amd64", "host", false)
	linuxWayland := writeNativeEvidenceNamed(t, root, "linux-wayland", "linux", "amd64", "host", false)
	linuxX11Duplicate := writeNativeEvidenceNamed(t, root, "linux-x11-duplicate", "linux", "amd64", "host", false)

	if _, err := runNativeEvidenceVerifier(t, "--require-linux-display-pair", mac, windows, linuxX11); err == nil {
		t.Fatal("single Linux display protocol passed pair gate")
	}
	if _, err := runNativeEvidenceVerifier(t, "--require-linux-display-pair", mac, windows, linuxX11, linuxX11Duplicate); err == nil {
		t.Fatal("duplicate Linux display protocol passed pair gate")
	}
	if output, err := runNativeEvidenceVerifier(t, "--require-linux-display-pair", mac, windows, linuxX11, linuxWayland); err != nil {
		t.Fatalf("X11 and Wayland native evidence pair failed: %v\n%s", err, output)
	}
}

func TestNativeAcceptanceTargetMismatchIsNotRunAndFailsClosed(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required for shell acceptance entry: %v", err)
	}
	evidence := t.TempDir()
	target := "windows"
	if runtime.GOOS == "windows" {
		target = "linux"
	}
	command := exec.Command(bash, filepath.Join(testutil.Root(t), "scripts", "native-acceptance.sh"), "--target", target)
	command.Env = append(os.Environ(), "DESKTOP_NATIVE_EVIDENCE_DIR="+evidence)
	if output, err := command.CombinedOutput(); err == nil {
		t.Fatalf("target mismatch unexpectedly passed: %s", output)
	} else if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 3 {
		t.Fatalf("target mismatch exit = %v, output: %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(evidence, "summary.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "status=not_run") || !strings.Contains(string(data), "failed_step=target-preflight") {
		t.Fatalf("target mismatch did not leave fail-closed summary: %s", data)
	}
}

func TestNativeEvidenceVerifierRequiresHostDisplayUnlessExplicitlyVirtual(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "xvfb", false)

	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("virtual Linux display passed formal evidence verification")
	}
	if _, err := runNativeEvidenceVerifier(t, "--allow-virtual-display", mac, windows, linux); err != nil {
		t.Fatalf("explicit virtual-display review failed: %v", err)
	}
}

func TestNativeEvidenceVerifierRequiresDisplayProtocolMetadata(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	metadataPath := filepath.Join(linux, "metadata.txt")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "display_protocol=x11\n", "", 1))
	if err := os.WriteFile(metadataPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("evidence without display protocol metadata passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsCrossCompilationEvidence(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", true)
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("cross-compilation evidence passed verification")
	}
}

func TestNativeEvidenceVerifierRequiresStructuredPlatformProbe(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	if err := os.WriteFile(filepath.Join(mac, "platform.log"), []byte("cross-compiled probe\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("evidence without a structured host platform report passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsAmbiguousPlatformProbeLogs(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	platformPath := filepath.Join(mac, "platform.log")
	data, err := os.ReadFile(platformPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(platformPath, append(data, data...), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("ambiguous duplicate platform reports passed verification")
	}
}

func TestNativeEvidenceVerifierRequiresSuccessfulWindowAndWorkbenchMarkers(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	if err := os.WriteFile(filepath.Join(windows, "workbench.log"), []byte("workbench=skipped\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("skipped workbench evidence passed formal verification")
	}
}

func TestNativeEvidenceVerifierRequiresStructuredInstallerProbe(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	if err := os.WriteFile(filepath.Join(mac, "installer.log"), []byte("{\"probe\":\"installer\"}"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("incomplete installer probe passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsInstallerProbeWithUnexpectedSequence(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	path := filepath.Join(windows, "installer.log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"upgraded":"1.1.0"`, `"upgraded":"claimed"`, 1))
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("installer evidence with an unexpected version sequence passed verification")
	}
}

func TestNativeEvidenceVerifierRequiresSuccessfulTrayProbe(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	if err := os.WriteFile(filepath.Join(windows, "tray.log"), []byte("{\"probe\":\"agent-remote-desktop.tray\",\"goos\":\"windows\",\"goarch\":\"amd64\",\"host_runtime\":true,\"cross_compilation\":false,\"status\":\"failed\"}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("failed tray probe passed formal verification")
	}
}

func TestNativeEvidenceVerifierRequiresCompleteWorkbenchSuccessContract(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	path := filepath.Join(windows, "workbench.log")
	if err := os.WriteFile(path, []byte("workbench launch, focus, drag, shutdown: true\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("partial workbench success marker passed formal verification")
	}
}

func TestNativeEvidenceVerifierRequiresLinuxDisplayRequirement(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	platformPath := filepath.Join(linux, "platform.log")
	data, err := os.ReadFile(platformPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "    {\"id\": \"display\", \"status\": \"pass\", \"required\": true},\n", "", 1))
	if err := os.WriteFile(platformPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("Linux platform probe without display requirement passed verification")
	}
}

func TestNativeEvidenceVerifierRequiresWindowsWebView2Observation(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	platformPath := filepath.Join(windows, "platform.log")
	data, err := os.ReadFile(platformPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), "    {\"id\": \"webview2\", \"status\": \"unknown\", \"required\": false},\n", "", 1))
	if err := os.WriteFile(platformPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("Windows platform probe without WebView2 observation passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsInvalidPlatformRequirementStatus(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	platformPath := filepath.Join(mac, "platform.log")
	data, err := os.ReadFile(platformPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"id": "shell", "status": "pass"`, `"id": "shell", "status": "partial"`, 1))
	if err := os.WriteFile(platformPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("platform evidence with an unknown status passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsUnsupportedTraySchema(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	trayPath := filepath.Join(windows, "tray.log")
	data, err := os.ReadFile(trayPath)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"schema_version":1`, `"schema_version":2`, 1))
	if err := os.WriteFile(trayPath, data, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("tray evidence with an unsupported schema passed verification")
	}
}

func TestNativeEvidenceVerifierAcceptsPowerShellUtf8Bom(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	// Windows PowerShell 5 emits a BOM for `Set-Content -Encoding utf8`,
	// while PowerShell 7 emits BOM-less UTF-8.  Native evidence must accept
	// both forms when joining host records.
	for _, dir := range []string{mac, windows, linux} {
		for _, name := range []string{"summary.txt", "metadata.txt"} {
			path := filepath.Join(dir, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append([]byte{0xef, 0xbb, 0xbf}, data...), 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if output, err := runNativeEvidenceVerifier(t, mac, windows, linux); err != nil {
		t.Fatalf("PowerShell UTF-8 BOM evidence failed: %v\n%s", err, output)
	}
}

func runNativeEvidenceVerifier(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Skipf("bash is required to run native evidence verifier: %v", err)
	}
	commandArgs := append([]string{filepath.Join(testutil.Root(t), "scripts", "verify-native-evidence.sh")}, args...)
	return exec.Command(bash, commandArgs...).CombinedOutput()
}

func writeNativeEvidence(t *testing.T, root, goos, goarch, display string, cross bool) string {
	return writeNativeEvidenceNamed(t, root, goos, goos, goarch, display, cross)
}

func writeNativeEvidenceNamed(t *testing.T, root, name, goos, goarch, display string, cross bool) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	crossValue := "false"
	if cross {
		crossValue = "true"
	}
	if err := os.WriteFile(filepath.Join(dir, "summary.txt"), []byte(strings.Join([]string{
		"status=passed",
		"exit_code=0",
		"failed_step=complete",
		"runtime=" + goos + "/" + goarch,
		"cross_compilation=" + crossValue,
		"evidence=" + dir,
	}, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "metadata.txt"), []byte(strings.Join([]string{
		"runtime_goos=" + goos,
		"runtime_goarch=" + goarch,
		"mygo_version=v0.5.6-test",
		"ui_display=" + display,
		"display_protocol=" + evidenceDisplayProtocol(name, goos, display),
	}, "\n")+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"platform.log", "installer.log", "native-window.log", "tray.log", "workbench.log"} {
		contents := []byte("probe passed\n")
		if name == "platform.log" {
			// The evidence joiner validates the actual structured host report,
			// rather than trusting a summary and a present log file.
			displayRequirement := ""
			if goos == "linux" {
				displayRequirement = "    {\"id\": \"display\", \"status\": \"pass\", \"required\": true},\n" +
					"    {\"id\": \"gtk3\", \"status\": \"pass\", \"required\": true},\n" +
					"    {\"id\": \"webkitgtk\", \"status\": \"unknown\", \"required\": false},\n" +
					"    {\"id\": \"tray\", \"status\": \"pass\", \"required\": true},\n"
			} else if goos == "windows" {
				displayRequirement = "    {\"id\": \"webview2\", \"status\": \"unknown\", \"required\": false},\n"
			}
			contents = []byte(fmt.Sprintf(`{
  "schema_version": 1,
  "probe": "agent-remote-desktop.platform",
  "goos": %q,
  "goarch": %q,
  "platform": %q,
  "host_runtime": true,
  "cross_compilation": false,
  "supported": true,
  "requirements": [
    {"id": "supported-target", "status": "pass", "required": true},
    {"id": "shell", "status": "pass", "required": true},
    {"id": "terminal", "status": "pass", "required": true},
%s    {"id": "optional", "status": "unknown", "required": false}
  ]
}
`, goos, goarch, map[string]string{"darwin": "macos", "windows": "windows", "linux": "linux"}[goos], displayRequirement))
		} else if name == "installer.log" {
			contents = []byte(`{"probe":"installer","platform":"` + goos + `","architecture":"` + goarch + `","initial":"1.0.0","upgraded":"1.1.0","rolled_back":"1.0.0","running_lock":"passed"}
`)
		} else if name == "native-window.log" {
			contents = []byte("native locale switch and window hide/show: true PTY exit: 0\n")
		} else if name == "tray.log" {
			contents = []byte("{\"schema_version\":1,\"probe\":\"agent-remote-desktop.tray\",\"goos\":\"" + goos + "\",\"goarch\":\"" + goarch + "\",\"host_runtime\":true,\"cross_compilation\":false,\"status\":\"passed\"}\n")
		} else if name == "workbench.log" {
			contents = []byte("workbench launch, focus, drag, shutdown, exact remote context/reuse/history replacement, exact binding attachment/reuse and history refresh: true\n")
		}
		if err := os.WriteFile(filepath.Join(dir, name), contents, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func evidenceDisplayProtocol(name, goos, display string) string {
	switch goos {
	case "darwin":
		return "appkit"
	case "windows":
		return "win32"
	case "linux":
		if strings.Contains(strings.ToLower(name), "wayland") {
			return "wayland"
		}
		// Xvfb and ordinary host display fixtures both model X11 evidence.
		if display == "xvfb" || display == "host" {
			return "x11"
		}
	}
	return "unknown"
}
