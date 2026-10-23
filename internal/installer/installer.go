// Package installer validates and atomically installs a desktop release.
//
// The installer intentionally handles only files that are already present in a
// release directory. It does not download releases, elevate privileges, stop
// processes, or install the external Agent CLIs. Callers must obtain a release
// through their normal distribution channel and pass its manifest directory to
// Validate/Install.
package installer

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	ErrInvalidRelease = errors.New("invalid desktop release")
	ErrDowngrade      = errors.New("desktop release downgrade is not allowed")
	ErrUnmanagedPath  = errors.New("destination is not managed by the desktop installer")
	ErrInterrupted    = errors.New("desktop installation has an unfinished replacement")
	ErrInstallBusy    = errors.New("desktop installation is in use")
)

const stateVersion = 1

// Product identity is part of the release contract.  The installer is also
// shipped to Windows hosts where Bash/Python may be unavailable, so it must
// enforce the same identity binding as the offline release verifier itself.
const (
	ProductName       = "Agent Remote Desktop"
	ProductIdentifier = "dev.agent-remote.desktop"
)

type manifest struct {
	SchemaVersion int `json:"schema_version"`
	App           struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Identifier string `json:"identifier"`
	} `json:"app"`
	Targets   []string `json:"targets"`
	Artifacts []struct {
		Path   string `json:"path"`
		Size   int64  `json:"size"`
		SHA256 string `json:"sha256"`
	} `json:"artifacts"`
}

// Release is the validated subset of a release selected for one OS/CPU pair.
type Release struct {
	Root         string
	Target       string
	Version      string
	Identifier   string
	Name         string
	ArtifactRoot string
	Files        []string
}

// InstallState is written as a sidecar next to the installation directory.
// It contains no account, project, terminal, or credential data.
type InstallState struct {
	SchemaVersion int    `json:"schema_version"`
	Identifier    string `json:"identifier"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Target        string `json:"target"`
	InstalledAt   string `json:"installed_at"`
	Previous      bool   `json:"previous_available"`
}

// Receipt describes the result of an atomic installation or rollback.
type Receipt struct {
	Version         string `json:"version"`
	PreviousVersion string `json:"previous_version"`
	Target          string `json:"target"`
	Destination     string `json:"destination"`
	RolledBack      bool   `json:"rolled_back"`
}

// InstallStatus is a read-only snapshot of an installation managed by this
// package. A nil Current means that destination has not been installed yet.
// Previous is populated when an upgrade can be rolled back. The snapshot never
// reads or exposes application files, credentials, or release contents.
type InstallStatus struct {
	Destination       string        `json:"destination"`
	Installed         bool          `json:"installed"`
	Current           *InstallState `json:"current,omitempty"`
	Previous          *InstallState `json:"previous,omitempty"`
	PreviousAvailable bool          `json:"previous_available"`
}

// ValidateRelease verifies the release manifest, checksums, selected target,
// and every regular file under the selected target. Symlinks and paths outside
// the release root are rejected before any file is copied.
func ValidateRelease(releaseDir, goos, goarch string) (Release, error) {
	releaseDir, err := absoluteDirectory(releaseDir)
	if err != nil {
		return Release{}, err
	}
	goos = strings.ToLower(strings.TrimSpace(goos))
	goarch = strings.ToLower(strings.TrimSpace(goarch))
	if !validTargetPart(goos) || !validTargetPart(goarch) {
		return Release{}, fmt.Errorf("%w: invalid target %q/%q", ErrInvalidRelease, goos, goarch)
	}
	target := goos + "/" + goarch
	targetKey := goos + "-" + goarch
	manifestPath := filepath.Join(releaseDir, "metadata", "release-manifest.json")
	var m manifest
	if err := readJSON(manifestPath, &m); err != nil {
		return Release{}, fmt.Errorf("%w: manifest: %v", ErrInvalidRelease, err)
	}
	if m.SchemaVersion != 1 || m.App.Identifier != ProductIdentifier || m.App.Name != ProductName || !validVersion(m.App.Version) {
		return Release{}, fmt.Errorf("%w: incomplete manifest", ErrInvalidRelease)
	}
	if !contains(m.Targets, target) {
		return Release{}, fmt.Errorf("%w: target %s is not declared", ErrInvalidRelease, target)
	}
	artifactRoot := filepath.Join(releaseDir, "artifacts", targetKey)
	if err := ensureDirectoryNoSymlink(artifactRoot); err != nil {
		return Release{}, fmt.Errorf("%w: target artifacts: %v", ErrInvalidRelease, err)
	}
	checksums, err := readChecksums(filepath.Join(releaseDir, "SHA256SUMS"))
	if err != nil {
		return Release{}, fmt.Errorf("%w: checksums: %v", ErrInvalidRelease, err)
	}
	manifestFiles := make(map[string]string)
	manifestSizes := make(map[string]int64)
	for _, entry := range m.Artifacts {
		rel, err := cleanRelative(entry.Path)
		if err != nil {
			return Release{}, fmt.Errorf("%w: artifact path: %v", ErrInvalidRelease, err)
		}
		if !strings.HasPrefix(rel, "artifacts/"+targetKey+"/") {
			continue
		}
		if !validHash(entry.SHA256) || entry.Size < 0 {
			return Release{}, fmt.Errorf("%w: invalid artifact entry %q", ErrInvalidRelease, entry.Path)
		}
		if _, exists := manifestFiles[rel]; exists {
			return Release{}, fmt.Errorf("%w: duplicate artifact %q", ErrInvalidRelease, rel)
		}
		manifestFiles[rel] = strings.ToLower(entry.SHA256)
		manifestSizes[rel] = entry.Size
		if c, ok := checksums[rel]; !ok || c != strings.ToLower(entry.SHA256) {
			return Release{}, fmt.Errorf("%w: checksum manifest mismatch for %q", ErrInvalidRelease, rel)
		}
	}
	if len(manifestFiles) == 0 {
		return Release{}, fmt.Errorf("%w: target has no artifacts", ErrInvalidRelease)
	}
	var files []string
	err = filepath.WalkDir(artifactRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", path)
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular artifact: %s", path)
		}
		rel, err := filepath.Rel(releaseDir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		hash, ok := manifestFiles[rel]
		if !ok {
			return fmt.Errorf("artifact is absent from manifest: %s", rel)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Size() != manifestSizes[rel] {
			return fmt.Errorf("artifact size mismatch: got %d, want %d", info.Size(), manifestSizes[rel])
		}
		if err := verifyFile(path, hash); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return Release{}, fmt.Errorf("%w: %v", ErrInvalidRelease, err)
	}
	if len(files) != len(manifestFiles) {
		return Release{}, fmt.Errorf("%w: manifest has missing target artifacts", ErrInvalidRelease)
	}
	sort.Strings(files)
	return Release{Root: releaseDir, Target: target, Version: m.App.Version, Identifier: m.App.Identifier, Name: m.App.Name, ArtifactRoot: artifactRoot, Files: files}, nil
}

// Install validates releaseDir and atomically replaces destination. The
// destination must be an absolute path. An existing destination is replaced
// only when its sidecar was written by this package. The previous tree remains
// available beside it for Rollback.
func Install(releaseDir, goos, goarch, destination string, allowDowngrade bool) (Receipt, error) {
	release, err := ValidateRelease(releaseDir, goos, goarch)
	if err != nil {
		return Receipt{}, err
	}
	destination, err = absolutePath(destination)
	if err != nil {
		return Receipt{}, err
	}
	if pathWithin(release.Root, destination) {
		return Receipt{}, fmt.Errorf("%w: destination must be outside the release directory", ErrInvalidRelease)
	}
	if err := ensureNoSymlinkAncestors(destination); err != nil {
		return Receipt{}, err
	}
	lock, err := acquireMutationLock(destination)
	if err != nil {
		return Receipt{}, err
	}
	defer lock.Close()
	current, currentErr := readState(destination)
	if currentErr != nil && !errors.Is(currentErr, os.ErrNotExist) {
		return Receipt{}, currentErr
	}
	if currentErr == nil {
		if !pathExists(destination) {
			return Receipt{}, fmt.Errorf("%w: state exists without installation", ErrInterrupted)
		}
		if current.Identifier != release.Identifier || current.Target != release.Target {
			return Receipt{}, fmt.Errorf("%w: identifier or target changed", ErrInvalidRelease)
		}
		cmp, cmpErr := compareVersions(release.Version, current.Version)
		if cmpErr != nil {
			return Receipt{}, cmpErr
		}
		if cmp < 0 && !allowDowngrade {
			return Receipt{}, fmt.Errorf("%w: %s -> %s", ErrDowngrade, current.Version, release.Version)
		}
	} else if pathExists(destination) {
		return Receipt{}, fmt.Errorf("%w: %v", ErrUnmanagedPath, currentErr)
	}
	previousExists := pathExists(previousPath(destination))
	previousStateExists := pathExists(previousStatePath(destination))
	if previousExists != previousStateExists || (currentErr != nil && previousExists) || (currentErr == nil && previousExists != current.Previous) {
		return Receipt{}, fmt.Errorf("%w: stale previous tree", ErrInterrupted)
	}
	parent := filepath.Dir(destination)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return Receipt{}, err
	}
	staging, err := temporarySibling(parent, ".agent-remote-desktop-staging-")
	if err != nil {
		return Receipt{}, err
	}
	defer os.RemoveAll(staging)
	if err := copyTree(release.ArtifactRoot, staging); err != nil {
		return Receipt{}, fmt.Errorf("%w: copy release: %v", ErrInvalidRelease, err)
	}
	if err := syncTree(staging); err != nil {
		return Receipt{}, err
	}
	previousVersion := ""
	if currentErr == nil {
		previousVersion = current.Version
		if previousExists {
			if err := os.RemoveAll(previousPath(destination)); err != nil {
				return Receipt{}, fmt.Errorf("remove old previous installation: %w", err)
			}
			if err := os.Remove(previousStatePath(destination)); err != nil {
				return Receipt{}, fmt.Errorf("remove old previous state: %w", err)
			}
		}
		if err := os.Rename(destination, previousPath(destination)); err != nil {
			return Receipt{}, fmt.Errorf("move current installation: %w", err)
		}
		if err := os.Rename(statePath(destination), previousStatePath(destination)); err != nil {
			_ = os.Rename(previousPath(destination), destination)
			return Receipt{}, fmt.Errorf("move current state: %w", err)
		}
		// Directory fsync is best effort after the state-preserving rename. If
		// the filesystem rejects it, the tree is still a valid installation and
		// returning an error here would leave the previous tree detached.
		_ = syncDirectory(parent)
	}
	if err := os.Rename(staging, destination); err != nil {
		if currentErr == nil {
			_ = os.Rename(previousPath(destination), destination)
			_ = os.Rename(previousStatePath(destination), statePath(destination))
		}
		return Receipt{}, fmt.Errorf("publish installation: %w", err)
	}
	_ = syncDirectory(parent)
	state := InstallState{SchemaVersion: stateVersion, Identifier: release.Identifier, Name: release.Name, Version: release.Version, Target: release.Target, InstalledAt: time.Now().UTC().Format(time.RFC3339), Previous: currentErr == nil}
	if err := writeState(statePath(destination), state); err != nil {
		_ = os.RemoveAll(destination)
		if currentErr == nil {
			_ = os.Rename(previousPath(destination), destination)
			_ = os.Rename(previousStatePath(destination), statePath(destination))
		}
		return Receipt{}, fmt.Errorf("write installation state: %w", err)
	}
	_ = syncDirectory(parent)
	return Receipt{Version: release.Version, PreviousVersion: previousVersion, Target: release.Target, Destination: destination}, nil
}

// Inspect returns the current installation metadata without changing any
// files. An empty, unmanaged destination is reported as Installed=false so a
// caller can use this as a safe preflight before presenting an upgrade.
func Inspect(destination string) (InstallStatus, error) {
	destination, err := absolutePath(destination)
	if err != nil {
		return InstallStatus{}, err
	}
	if err := ensureNoSymlinkAncestors(destination); err != nil {
		return InstallStatus{}, err
	}
	status := InstallStatus{Destination: destination}
	current, currentErr := readState(destination)
	if errors.Is(currentErr, os.ErrNotExist) {
		if pathExists(destination) {
			return InstallStatus{}, fmt.Errorf("%w: destination has no installer state", ErrUnmanagedPath)
		}
		if pathExists(previousPath(destination)) || pathExists(previousStatePath(destination)) {
			return InstallStatus{}, fmt.Errorf("%w: stale previous tree", ErrInterrupted)
		}
		return status, nil
	}
	if currentErr != nil {
		return InstallStatus{}, currentErr
	}
	if !pathExists(destination) {
		return InstallStatus{}, fmt.Errorf("%w: state exists without installation", ErrInterrupted)
	}
	status.Installed = true
	status.Current = &current
	previousExists := pathExists(previousPath(destination))
	previousStateExists := pathExists(previousStatePath(destination))
	if previousExists != previousStateExists || previousExists != current.Previous {
		return InstallStatus{}, fmt.Errorf("%w: stale previous tree", ErrInterrupted)
	}
	if previousExists {
		previous, previousErr := readState(previousPath(destination))
		if previousErr != nil {
			return InstallStatus{}, fmt.Errorf("%w: previous state: %v", ErrInterrupted, previousErr)
		}
		if current.Identifier != previous.Identifier || current.Target != previous.Target {
			return InstallStatus{}, fmt.Errorf("%w: state identity changed", ErrInvalidRelease)
		}
		status.Previous = &previous
		status.PreviousAvailable = true
	}
	return status, nil
}

// Rollback atomically restores the tree saved by the last successful upgrade.
// The current tree is retained under a unique .rollback-* sibling so a failed
// rollback never destroys a release that can still be inspected.
func Rollback(destination string) (Receipt, error) {
	destination, err := absolutePath(destination)
	if err != nil {
		return Receipt{}, err
	}
	if err := ensureNoSymlinkAncestors(destination); err != nil {
		return Receipt{}, err
	}
	lock, err := acquireMutationLock(destination)
	if err != nil {
		return Receipt{}, err
	}
	defer lock.Close()
	current, err := readState(destination)
	if err != nil {
		return Receipt{}, err
	}
	previous, err := readState(previousPath(destination))
	if err != nil {
		return Receipt{}, fmt.Errorf("%w: previous state: %v", ErrInterrupted, err)
	}
	if current.Identifier != previous.Identifier || current.Target != previous.Target {
		return Receipt{}, fmt.Errorf("%w: state identity changed", ErrInvalidRelease)
	}
	if !current.Previous {
		return Receipt{}, fmt.Errorf("%w: current installation has no rollback history", ErrInterrupted)
	}
	if err := ensureDirectoryNoSymlink(previousPath(destination)); err != nil {
		return Receipt{}, err
	}
	backup, err := temporarySibling(filepath.Dir(destination), ".agent-remote-desktop-rollback-")
	if err != nil {
		return Receipt{}, err
	}
	if err := os.Remove(backup); err != nil {
		return Receipt{}, err
	}
	backupState := backup + ".install.json"
	if err := os.Rename(destination, backup); err != nil {
		return Receipt{}, err
	}
	if err := os.Rename(statePath(destination), backupState); err != nil {
		_ = os.Rename(backup, destination)
		return Receipt{}, err
	}
	if err := os.Rename(previousPath(destination), destination); err != nil {
		_ = os.Rename(backupState, statePath(destination))
		_ = os.Rename(backup, destination)
		return Receipt{}, err
	}
	if err := os.Rename(previousStatePath(destination), statePath(destination)); err != nil {
		_ = os.Rename(destination, previousPath(destination))
		_ = os.Rename(backupState, statePath(destination))
		_ = os.Rename(backup, destination)
		return Receipt{}, err
	}
	_ = syncDirectory(filepath.Dir(destination))
	// Keep backup and delete only after the swap succeeded. This is a deliberate
	// best-effort cleanup; retaining it is safe and gives operators recovery data.
	_ = os.RemoveAll(backup)
	_ = os.Remove(backupState)
	return Receipt{Version: previous.Version, PreviousVersion: current.Version, Target: previous.Target, Destination: destination, RolledBack: true}, nil
}

func statePath(destination string) string         { return destination + ".install.json" }
func previousPath(destination string) string      { return destination + ".previous" }
func previousStatePath(destination string) string { return destination + ".previous.install.json" }

func readState(path string) (InstallState, error) {
	var state InstallState
	if err := readJSON(statePath(path), &state); err != nil {
		return InstallState{}, err
	}
	if state.SchemaVersion != stateVersion || state.Identifier == "" || state.Target == "" || !validVersion(state.Version) {
		return InstallState{}, fmt.Errorf("%w: invalid installation state", ErrInvalidRelease)
	}
	return state, nil
}

func writeState(path string, state InstallState) error {
	if err := ensureNoSymlinkAncestors(path); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	tmpDir, err := temporarySibling(filepath.Dir(path), ".agent-remote-desktop-state-")
	if err != nil {
		return err
	}
	tmp := filepath.Join(tmpDir, filepath.Base(path))
	defer os.RemoveAll(tmpDir)
	if err := os.WriteFile(tmp, append(data, '\n'), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readJSON(path string, value any) error {
	if err := ensureRegularFile(path); err != nil {
		return err
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	decoder := json.NewDecoder(io.LimitReader(f, 8<<20))
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON data")
		}
		return err
	}
	return nil
}

func readChecksums(path string) (map[string]string, error) {
	if err := ensureRegularFile(path); err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	result := map[string]string{}
	scanner := bufio.NewScanner(io.LimitReader(f, 8<<20))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "  ", 2)
		if len(fields) != 2 || !validHash(fields[0]) {
			return nil, fmt.Errorf("invalid checksum line")
		}
		rel, err := cleanRelative(filepath.ToSlash(strings.TrimSpace(fields[1])))
		if err != nil {
			return nil, err
		}
		if _, exists := result[rel]; exists {
			return nil, fmt.Errorf("duplicate checksum path %q", rel)
		}
		result[rel] = strings.ToLower(fields[0])
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func verifyFile(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(f, 512<<20)); err != nil {
		return err
	}
	actual := hex.EncodeToString(h.Sum(nil))
	if actual != strings.ToLower(expected) {
		return fmt.Errorf("checksum mismatch: got %s, want %s", actual, expected)
	}
	return nil
}

func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink is not allowed: %s", path)
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("non-regular file: %s", path)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		mode := info.Mode().Perm()
		if mode == 0 {
			mode = 0644
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, mode)
		var closeErr error
		if err == nil {
			_, err = io.Copy(out, in)
			closeErr = out.Close()
		}
		closeErr = errors.Join(closeErr, in.Close())
		return errors.Join(err, closeErr)
	})
}

func syncTree(root string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		err = f.Sync()
		return errors.Join(err, f.Close())
	})
}

func temporarySibling(parent, prefix string) (string, error) {
	path, err := temporarySiblingPath(parent, prefix)
	if err != nil {
		return "", err
	}
	if err := os.Mkdir(path, 0700); err != nil {
		return "", err
	}
	return path, nil
}

func temporarySiblingPath(parent, prefix string) (string, error) {
	for i := 0; i < 20; i++ {
		name := fmt.Sprintf("%s%d-%d", prefix, time.Now().UnixNano(), rand.Int63())
		path := filepath.Join(parent, name)
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			return path, nil
		} else if err != nil {
			return "", err
		}
	}
	return "", errors.New("unable to allocate temporary sibling")
}

func absoluteDirectory(path string) (string, error) {
	absolute, err := absolutePath(path)
	if err != nil {
		return "", err
	}
	if err := ensureDirectoryNoSymlink(absolute); err != nil {
		return "", err
	}
	return absolute, nil
}

func absolutePath(path string) (string, error) {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." || !filepath.IsAbs(path) {
		return "", fmt.Errorf("%w: path must be absolute", ErrInvalidRelease)
	}
	return path, nil
}

func ensureNoSymlinkAncestors(path string) error {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return fmt.Errorf("%w: path must be absolute", ErrInvalidRelease)
	}
	root := filepath.VolumeName(path) + string(filepath.Separator)
	current := root
	parts := strings.Split(strings.TrimPrefix(path, root), string(filepath.Separator))
	for index, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			// macOS exposes these fixed system roots as aliases. They are not
			// user-controlled path components; every other symlink ancestor is
			// rejected to prevent replacement through an untrusted directory.
			if runtime.GOOS == "darwin" && index < len(parts)-1 &&
				(current == filepath.Join(root, "var") || current == filepath.Join(root, "tmp")) {
				continue
			}
			if index == len(parts)-1 {
				return fmt.Errorf("%w: destination is a symlink %s", ErrInvalidRelease, current)
			}
			return fmt.Errorf("%w: symlink path component %s", ErrInvalidRelease, current)
		}
	}
	return nil
}

func ensureRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("expected a regular file: %s", path)
	}
	return ensureNoSymlinkAncestors(path)
}

func ensureDirectoryNoSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("expected a regular directory: %s", path)
	}
	return ensureNoSymlinkAncestors(path)
}

func cleanRelative(path string) (string, error) {
	path = filepath.ToSlash(strings.TrimSpace(path))
	if path == "" || filepath.IsAbs(path) || strings.HasPrefix(path, "../") || path == ".." || strings.Contains(path, "\\") {
		return "", fmt.Errorf("invalid relative path %q", path)
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	if clean == "." || clean != path || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("invalid relative path %q", path)
	}
	return clean, nil
}

func validTargetPart(value string) bool {
	if value == "" || value == "." || value == ".." || strings.ContainsAny(value, `/\\`) {
		return false
	}
	for _, r := range value {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '_' {
			return false
		}
	}
	return true
}

func validHash(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func validVersion(value string) bool {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	if value == "" || strings.ContainsAny(value, "/\\\x00\n\r") {
		return false
	}
	return true
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func pathExists(path string) bool { _, err := os.Lstat(path); return err == nil }

func pathWithin(root, candidate string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(candidate))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func compareVersions(a, b string) (int, error) {
	parse := func(value string) ([]int, string, error) {
		value = strings.TrimPrefix(strings.TrimSpace(value), "v")
		parts := strings.SplitN(value, "-", 2)
		main := strings.Split(parts[0], ".")
		result := make([]int, len(main))
		for i, token := range main {
			if token == "" {
				return nil, "", fmt.Errorf("%w: invalid version %q", ErrInvalidRelease, value)
			}
			n, err := strconv.Atoi(token)
			if err != nil || n < 0 {
				return nil, "", fmt.Errorf("%w: invalid version %q", ErrInvalidRelease, value)
			}
			result[i] = n
		}
		pre := ""
		if len(parts) == 2 {
			pre = parts[1]
		}
		return result, pre, nil
	}
	aa, ap, err := parse(a)
	if err != nil {
		return 0, err
	}
	bb, bp, err := parse(b)
	if err != nil {
		return 0, err
	}
	for len(aa) < len(bb) {
		aa = append(aa, 0)
	}
	for len(bb) < len(aa) {
		bb = append(bb, 0)
	}
	for i := range aa {
		if aa[i] < bb[i] {
			return -1, nil
		}
		if aa[i] > bb[i] {
			return 1, nil
		}
	}
	if ap == bp {
		return 0, nil
	}
	if ap == "" {
		return 1, nil
	}
	if bp == "" {
		return -1, nil
	}
	if ap < bp {
		return -1, nil
	}
	return 1, nil
}
