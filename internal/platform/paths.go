// Package platform keeps platform paths and executable discovery out of views.
package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
)

// Home follows the CLI's precedence without reading its credentials or database.
func Home(override string) (string, error) {
	if override == "" {
		override = os.Getenv("AGENT_REMOTE_HOME")
	}
	if override != "" {
		return filepath.Abs(override)
	}
	user, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	if runtime.GOOS == "windows" {
		base := os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = filepath.Join(user, "AppData", "Local")
		}
		return filepath.Join(base, "agent-remote"), nil
	}
	return filepath.Join(user, ".config", "agent-remote"), nil
}

// Executable also searches common user install locations when launched by Finder.
// No shell startup files or project-controlled commands are executed during discovery.
func Executable(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == string(filepath.Separator) {
		return "", fmt.Errorf("%w: empty executable name", fault.ExecutableMissing)
	}
	if p, err := exec.LookPath(name); err == nil {
		return filepath.Abs(p)
	}
	// Fallback directories contain executable names, never path fragments. An
	// explicit path was already handled by LookPath above; reducing it here
	// keeps the fallback deterministic without changing that path contract.
	name = filepath.Base(name)
	user, _ := os.UserHomeDir()
	dirs := []string{filepath.Join(user, ".local", "bin"), filepath.Join(user, ".cargo", "bin")}
	if runtime.GOOS == "darwin" {
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	}
	if runtime.GOOS == "linux" {
		// Desktop files are often launched without the login shell's PATH.
		// Keep the same fallback behavior as a terminal launch for the common
		// system install locations, while still preferring PATH and user bins.
		dirs = append(dirs, "/usr/local/bin", "/usr/bin")
	}
	if runtime.GOOS == "windows" {
		// LookPath already honors PATHEXT.  Keep an explicitly supplied .exe
		// intact for callers that pass a configured Windows path/name, while
		// still using the conventional extension for fallback directories.
		if !strings.EqualFold(filepath.Ext(name), ".exe") {
			name += ".exe"
		}
		appendEnvDir := func(key string, parts ...string) {
			if base := strings.TrimSpace(os.Getenv(key)); base != "" {
				dirs = append(dirs, filepath.Join(append([]string{base}, parts...)...))
			}
		}
		appendEnvDir("APPDATA", "npm")
		appendEnvDir("LOCALAPPDATA", "Programs")
		appendEnvDir("LOCALAPPDATA")
		appendEnvDir("USERPROFILE", "scoop", "shims")
		appendEnvDir("USERPROFILE", ".bun", "bin")
		appendEnvDir("ProgramFiles")
		appendEnvDir("ProgramFiles(x86)")
	}
	for _, d := range dirs {
		p := filepath.Join(d, name)
		if s, err := os.Stat(p); err == nil && s.Mode().IsRegular() && (runtime.GOOS == "windows" || s.Mode()&0111 != 0) {
			return p, nil
		}
	}
	return "", fmt.Errorf("%w: %s", fault.ExecutableMissing, name)
}
