package installer_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/installer"
)

type artifact struct {
	path string
	data []byte
}

func makeRelease(t *testing.T, version string, target string, artifacts ...artifact) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "metadata"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "artifacts", strings.ReplaceAll(target, "/", "-")), 0755); err != nil {
		t.Fatal(err)
	}
	entries := make([]map[string]any, 0, len(artifacts))
	var sums strings.Builder
	for _, item := range artifacts {
		path := filepath.Join(root, "artifacts", strings.ReplaceAll(target, "/", "-"), filepath.FromSlash(item.path))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, item.data, 0644); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256(item.data)
		hexHash := hex.EncodeToString(hash[:])
		rel := filepath.ToSlash(filepath.Join("artifacts", strings.ReplaceAll(target, "/", "-"), item.path))
		entries = append(entries, map[string]any{"path": rel, "size": len(item.data), "sha256": hexHash})
		fmt.Fprintf(&sums, "%s  %s\n", hexHash, rel)
	}
	manifest := map[string]any{
		"schema_version": 1,
		"app":            map[string]string{"name": "Agent Remote Desktop", "version": version, "identifier": "dev.agent-remote.desktop"},
		"targets":        []string{target},
		"artifacts":      entries,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "metadata", "release-manifest.json"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "SHA256SUMS"), []byte(sums.String()), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestValidateReleaseChecksTargetAndHashes(t *testing.T) {
	releaseDir := makeRelease(t, "1.2.0", "linux/amd64", artifact{path: "agent-remote-desktop", data: []byte("v1")})
	release, err := installer.ValidateRelease(releaseDir, "linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	if release.Target != "linux/amd64" || release.Version != "1.2.0" || len(release.Files) != 1 {
		t.Fatalf("unexpected release: %+v", release)
	}
	if _, err := installer.ValidateRelease(releaseDir, "windows", "amd64"); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("wrong target error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(releaseDir, "artifacts", "linux-amd64", "agent-remote-desktop"), []byte("tampered"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.ValidateRelease(releaseDir, "linux", "amd64"); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("tampered release error = %v", err)
	}
}

func TestValidateReleaseBindsManifestToDesktopIdentity(t *testing.T) {
	for _, tc := range []struct {
		name  string
		field string
		value string
	}{
		{name: "name", field: "name", value: "Another Desktop"},
		{name: "identifier", field: "identifier", value: "com.example.other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			releaseDir := makeRelease(t, "1.0.0", "linux/amd64", artifact{path: "agent", data: []byte("app")})
			manifestPath := filepath.Join(releaseDir, "metadata", "release-manifest.json")
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
			if _, err := installer.ValidateRelease(releaseDir, "linux", "amd64"); !errors.Is(err, installer.ErrInvalidRelease) {
				t.Fatalf("manifest with mismatched app %s error = %v", tc.field, err)
			}
		})
	}
}

func TestValidateReleaseRejectsSymlinksAndUnknownFiles(t *testing.T) {
	releaseDir := makeRelease(t, "1.0.0", "darwin/arm64", artifact{path: "Agent Remote Desktop.app/Contents/MacOS/Agent Remote Desktop", data: []byte("app")})
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires elevated Windows privileges")
	}
	link := filepath.Join(releaseDir, "artifacts", "darwin-arm64", "link")
	if err := os.Symlink("Agent Remote Desktop.app", link); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.ValidateRelease(releaseDir, "darwin", "arm64"); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("symlink error = %v", err)
	}

	releaseDir = makeRelease(t, "1.0.0", "darwin/arm64", artifact{path: "app", data: []byte("app")})
	if err := os.WriteFile(filepath.Join(releaseDir, "artifacts", "darwin-arm64", "extra"), []byte("unlisted"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.ValidateRelease(releaseDir, "darwin", "arm64"); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("unknown file error = %v", err)
	}
}

func TestInstallUpgradeAndRollbackAreAtomic(t *testing.T) {
	target := "linux/amd64"
	destination := filepath.Join(t.TempDir(), "installed")
	status, err := installer.Inspect(destination)
	if err != nil {
		t.Fatal(err)
	}
	if status.Installed || status.Current != nil || status.PreviousAvailable {
		t.Fatalf("empty status: %+v", status)
	}
	v1 := makeRelease(t, "1.0.0", target, artifact{path: "agent", data: []byte("one")}, artifact{path: "desktop.desktop", data: []byte("desktop one")})
	receipt, err := installer.Install(v1, "linux", "amd64", destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Version != "1.0.0" || receipt.PreviousVersion != "" {
		t.Fatalf("initial receipt: %+v", receipt)
	}
	assertFile(t, filepath.Join(destination, "agent"), "one")

	v2 := makeRelease(t, "1.1.0", target, artifact{path: "agent", data: []byte("two")}, artifact{path: "desktop.desktop", data: []byte("desktop two")})
	receipt, err = installer.Install(v2, "linux", "amd64", destination, false)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.PreviousVersion != "1.0.0" {
		t.Fatalf("upgrade receipt: %+v", receipt)
	}
	assertFile(t, filepath.Join(destination, "agent"), "two")
	assertFile(t, filepath.Join(destination+".previous", "agent"), "one")
	status, err = installer.Inspect(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.Current == nil || status.Current.Version != "1.1.0" || status.Previous == nil || status.Previous.Version != "1.0.0" || !status.PreviousAvailable {
		t.Fatalf("upgraded status: %+v", status)
	}

	if _, err := installer.Install(v1, "linux", "amd64", destination, false); !errors.Is(err, installer.ErrDowngrade) {
		t.Fatalf("downgrade error = %v", err)
	}
	assertFile(t, filepath.Join(destination, "agent"), "two")

	receipt, err = installer.Rollback(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !receipt.RolledBack || receipt.Version != "1.0.0" || receipt.PreviousVersion != "1.1.0" {
		t.Fatalf("rollback receipt: %+v", receipt)
	}
	assertFile(t, filepath.Join(destination, "agent"), "one")
	status, err = installer.Inspect(destination)
	if err != nil {
		t.Fatal(err)
	}
	if !status.Installed || status.Current == nil || status.Current.Version != "1.0.0" || status.Previous != nil || status.PreviousAvailable {
		t.Fatalf("rolled back status: %+v", status)
	}
	if _, err := os.Stat(destination + ".previous"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("previous tree remained after rollback: %v", err)
	}
}

func TestInspectRejectsUnmanagedAndInterruptedInstallations(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "installed")
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Inspect(destination); !errors.Is(err, installer.ErrUnmanagedPath) {
		t.Fatalf("unmanaged status error = %v", err)
	}
	if err := os.RemoveAll(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(destination+".previous", 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Inspect(destination); !errors.Is(err, installer.ErrInterrupted) {
		t.Fatalf("stale previous status error = %v", err)
	}
}

func TestInstallDoesNotClobberUnmanagedPath(t *testing.T) {
	releaseDir := makeRelease(t, "1.0.0", "linux/amd64", artifact{path: "agent", data: []byte("one")})
	destination := filepath.Join(t.TempDir(), "installed")
	if err := os.MkdirAll(destination, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, "keep"), []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(releaseDir, "linux", "amd64", destination, false); !errors.Is(err, installer.ErrUnmanagedPath) {
		t.Fatalf("unmanaged path error = %v", err)
	}
	assertFile(t, filepath.Join(destination, "keep"), "keep")
}

func TestInstallRejectsStalePreviousWithoutCurrentState(t *testing.T) {
	releaseDir := makeRelease(t, "1.0.0", "linux/amd64", artifact{path: "agent", data: []byte("one")})
	destination := filepath.Join(t.TempDir(), "installed")
	if err := os.Mkdir(destination+".previous", 0755); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(releaseDir, "linux", "amd64", destination, false); !errors.Is(err, installer.ErrInterrupted) {
		t.Fatalf("stale previous install error = %v", err)
	}
}

func TestInstallRejectsRelativeAndSymlinkDestination(t *testing.T) {
	releaseDir := makeRelease(t, "1.0.0", "linux/amd64", artifact{path: "agent", data: []byte("one")})
	if _, err := installer.Install(releaseDir, "linux", "amd64", "relative", false); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("relative destination error = %v", err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("creating symlinks requires elevated Windows privileges")
	}
	parent := t.TempDir()
	real := filepath.Join(parent, "real")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(parent, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if _, err := installer.Install(releaseDir, "linux", "amd64", link, false); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("symlink destination error = %v", err)
	}
}

func TestInstallRejectsDestinationInsideReleaseDirectory(t *testing.T) {
	releaseDir := makeRelease(t, "1.0.0", "linux/amd64", artifact{path: "agent", data: []byte("one")})
	destination := filepath.Join(releaseDir, "installed")
	if _, err := installer.Install(releaseDir, "linux", "amd64", destination, false); !errors.Is(err, installer.ErrInvalidRelease) {
		t.Fatalf("release-contained destination error = %v", err)
	}
	if _, err := os.Stat(destination); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("release-contained destination was touched: %v", err)
	}
}

func TestRunningLockBlocksConcurrentInstallationAndReleasesAfterClose(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "installed")
	first, err := installer.AcquireRunningLock(destination)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installer.AcquireRunningLock(destination); !errors.Is(err, installer.ErrInstallBusy) {
		t.Fatalf("second running lock error = %v, want ErrInstallBusy", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := installer.AcquireRunningLock(destination)
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Close(); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, expected string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != expected {
		t.Fatalf("%s = %q, want %q", path, data, expected)
	}
}
