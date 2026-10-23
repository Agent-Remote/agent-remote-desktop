package release_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeEvidenceVerifierRejectsSimulatedWireGuardEvidence(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	writeWireGuardEvidence(t, mac, "simulated=true")
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("simulated WireGuard evidence passed verification")
	}
}

func TestNativeEvidenceVerifierRejectsWireGuardStatusOnlyAsFormalEvidence(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	writeWireGuardEvidence(t, mac, "")
	if _, err := runNativeEvidenceVerifier(t, mac, windows, linux); err == nil {
		t.Fatal("status-only WireGuard evidence passed formal verification")
	}
}

func TestNativeEvidenceVerifierCanRequireWireGuardEvidence(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	if _, err := runNativeEvidenceVerifier(t, "--require-wireguard", mac, windows, linux); err == nil {
		t.Fatal("missing required WireGuard evidence passed formal verification")
	}
}

func TestNativeEvidenceVerifierAcceptsPowerShellUtf8BomWireGuardEvidence(t *testing.T) {
	root := t.TempDir()
	mac := writeNativeEvidence(t, root, "darwin", "arm64", "host", false)
	windows := writeNativeEvidence(t, root, "windows", "amd64", "host", false)
	linux := writeNativeEvidence(t, root, "linux", "amd64", "host", false)
	for _, dir := range []string{mac, windows, linux} {
		writePassingWireGuardEvidence(t, dir, true)
	}
	if output, err := runNativeEvidenceVerifier(t, "--require-wireguard", mac, windows, linux); err != nil {
		t.Fatalf("PowerShell UTF-8 BOM WireGuard evidence failed: %v\n%s", err, output)
	}
}

func writeWireGuardEvidence(t *testing.T, desktopRoot, forbidden string) {
	t.Helper()
	dir := filepath.Join(desktopRoot, "wireguard")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	value := map[string]any{
		"schema_version": 1, "probe": "agent-remote-wireguard.native", "native": true, "cross_compilation": false,
		"runtime":      map[string]string{"goos": "darwin", "goarch": "arm64"},
		"installation": map[string]string{"status": "passed"}, "service_identity": map[string]string{"status": "passed"},
		"status_handshake": map[string]string{"status": "passed"}, "caller_authentication": map[string]string{"status": "passed"},
		"up_receipt": map[string]string{"status": "not_run"}, "down_receipt": map[string]string{"status": "not_run"},
		"replay_rejected": map[string]string{"status": "not_run"}, "service_restart": map[string]string{"status": "not_run"},
		"execution": map[string]string{"status": "status-only", "reason": map[bool]string{true: forbidden, false: "status-only"}[forbidden != ""]},
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence.json"), append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}

func writePassingWireGuardEvidence(t *testing.T, desktopRoot string, bom bool) {
	t.Helper()
	dir := filepath.Join(desktopRoot, "wireguard")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	value := map[string]any{
		"schema_version": 1, "probe": "agent-remote-wireguard.native", "native": true, "cross_compilation": false,
		"runtime":      map[string]string{"goos": "" + filepath.Base(desktopRoot), "goarch": "arm64"},
		"installation": map[string]string{"status": "passed"}, "service_identity": map[string]string{"status": "passed"},
		"status_handshake": map[string]string{"status": "passed"}, "caller_authentication": map[string]string{"status": "passed"},
		"up_receipt": map[string]string{"status": "passed"}, "down_receipt": map[string]string{"status": "passed"},
		"replay_rejected": map[string]string{"status": "passed"}, "service_restart": map[string]string{"status": "passed"},
		"execution": map[string]string{"status": "passed"},
	}
	// The test fixture receives its runtime directory as the OS name, so map
	// the three evidence roots back to the exact targets expected by the joiner.
	switch filepath.Base(desktopRoot) {
	case "darwin":
		value["runtime"] = map[string]string{"goos": "darwin", "goarch": "arm64"}
	case "windows":
		value["runtime"] = map[string]string{"goos": "windows", "goarch": "amd64"}
	case "linux":
		value["runtime"] = map[string]string{"goos": "linux", "goarch": "amd64"}
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if bom {
		encoded = append([]byte{0xef, 0xbb, 0xbf}, encoded...)
	}
	if err := os.WriteFile(filepath.Join(dir, "evidence.json"), append(encoded, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
}
