// installer-probe runs the local release installer against two synthetic
// releases. It exercises the same filesystem and rename operations used by a
// packaged upgrade, and prints a machine-readable success line. No network,
// privilege escalation, signing key, or user installation path is touched.
package main

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

	"github.com/Agent-Remote/agent-remote-desktop/internal/installer"
)

type artifact struct {
	Path string
	Data []byte
}

func main() {
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" && runtime.GOOS != "linux" {
		fail(fmt.Errorf("unsupported runtime OS %s; FreeBSD is not a release target", runtime.GOOS))
	}
	if runtime.GOARCH != "amd64" && runtime.GOARCH != "arm64" {
		fail(fmt.Errorf("unsupported runtime architecture %s", runtime.GOARCH))
	}
	root, err := os.MkdirTemp("", "agent-remote-installer-probe-")
	if err != nil {
		fail(err)
	}
	defer os.RemoveAll(root)
	target := runtime.GOOS + "/" + runtime.GOARCH
	v1, err := makeRelease(root, "1.0.0", target, artifact{Path: "agent-remote-desktop", Data: []byte("probe-v1")})
	if err != nil {
		fail(err)
	}
	v2, err := makeRelease(root, "1.1.0", target, artifact{Path: "agent-remote-desktop", Data: []byte("probe-v2")})
	if err != nil {
		fail(err)
	}
	destination := filepath.Join(root, "installed")
	if _, err = installer.Install(v1, runtime.GOOS, runtime.GOARCH, destination, false); err != nil {
		fail(fmt.Errorf("initial install: %w", err))
	}
	if err = assertFile(filepath.Join(destination, "agent-remote-desktop"), "probe-v1"); err != nil {
		fail(err)
	}
	if _, err = installer.Install(v2, runtime.GOOS, runtime.GOARCH, destination, false); err != nil {
		fail(fmt.Errorf("upgrade: %w", err))
	}
	if err = assertFile(filepath.Join(destination, "agent-remote-desktop"), "probe-v2"); err != nil {
		fail(err)
	}
	if _, err = installer.Install(v1, runtime.GOOS, runtime.GOARCH, destination, false); !errors.Is(err, installer.ErrDowngrade) {
		fail(fmt.Errorf("downgrade was not rejected: %v", err))
	}
	if _, err = installer.Rollback(destination); err != nil {
		fail(fmt.Errorf("rollback: %w", err))
	}
	if err = assertFile(filepath.Join(destination, "agent-remote-desktop"), "probe-v1"); err != nil {
		fail(err)
	}
	if _, err = os.Stat(destination + ".previous"); !errors.Is(err, os.ErrNotExist) {
		fail(fmt.Errorf("rollback retained previous tree: %v", err))
	}
	lock, err := installer.AcquireRunningLock(destination)
	if err != nil {
		fail(fmt.Errorf("running lock acquire: %w", err))
	}
	if second, secondErr := installer.AcquireRunningLock(destination); secondErr == nil {
		_ = second.Close()
		_ = lock.Close()
		fail(fmt.Errorf("running lock allowed concurrent owner"))
	} else if !errors.Is(secondErr, installer.ErrInstallBusy) {
		_ = lock.Close()
		fail(fmt.Errorf("running lock returned unexpected error: %w", secondErr))
	}
	if err := lock.Close(); err != nil {
		fail(fmt.Errorf("running lock release: %w", err))
	}
	lock, err = installer.AcquireRunningLock(destination)
	if err != nil {
		fail(fmt.Errorf("running lock reacquire: %w", err))
	}
	if err := lock.Close(); err != nil {
		fail(fmt.Errorf("running lock final release: %w", err))
	}
	result := map[string]any{
		"probe":        "installer",
		"platform":     runtime.GOOS,
		"architecture": runtime.GOARCH,
		"initial":      "1.0.0",
		"upgraded":     "1.1.0",
		"rolled_back":  "1.0.0",
		"running_lock": "passed",
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
}

func makeRelease(root, version, target string, items ...artifact) (string, error) {
	release := filepath.Join(root, "release-"+strings.ReplaceAll(version, ".", "-"))
	targetKey := strings.ReplaceAll(target, "/", "-")
	if err := os.MkdirAll(filepath.Join(release, "metadata", "unused"), 0755); err != nil {
		return "", err
	}
	artifactRoot := filepath.Join(release, "artifacts", targetKey)
	if err := os.MkdirAll(artifactRoot, 0755); err != nil {
		return "", err
	}
	entries := make([]map[string]any, 0, len(items))
	var sums strings.Builder
	for _, item := range items {
		path := filepath.Join(artifactRoot, filepath.FromSlash(item.Path))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return "", err
		}
		if err := os.WriteFile(path, item.Data, 0644); err != nil {
			return "", err
		}
		hash := sha256.Sum256(item.Data)
		hashText := hex.EncodeToString(hash[:])
		rel := filepath.ToSlash(filepath.Join("artifacts", targetKey, item.Path))
		entries = append(entries, map[string]any{"path": rel, "size": len(item.Data), "sha256": hashText})
		fmt.Fprintf(&sums, "%s  %s\n", hashText, rel)
	}
	manifest := map[string]any{
		"schema_version": 1,
		"app":            map[string]string{"name": "Agent Remote Desktop", "version": version, "identifier": "dev.agent-remote.desktop"},
		"targets":        []string{target},
		"artifacts":      entries,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(release, "metadata", "release-manifest.json"), data, 0644); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(release, "SHA256SUMS"), []byte(sums.String()), 0644); err != nil {
		return "", err
	}
	return release, nil
}

func assertFile(path, want string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if string(data) != want {
		return fmt.Errorf("%s = %q, want %q", path, data, want)
	}
	return nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "installer probe:", err)
	os.Exit(1)
}
