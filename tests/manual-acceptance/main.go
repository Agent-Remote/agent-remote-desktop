// manual-acceptance prepares a disposable workbench for the redesign's
// hands-on checks (agent-remote docs/agent-remote-desktop-redesign.zh-CN.md
// §10): a temporary home with two projects, and stand-in "codex", "claude"
// and "fclaude" executables that show every byte the terminal sends them.
// A chord that leaks shows up at once as bytes on screen. Nothing touches
// the user's own desktop state, and no real Agent starts.
//
//	go run ./tests/manual-acceptance            # prepare and launch
//	go run ./tests/manual-acceptance -prepare   # only print the paths
//
// Run from the repository root. The same binary, named codex/claude/fclaude,
// is the stand-in Agent.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/storage"
	"github.com/Agent-Remote/agent-remote-desktop/internal/workspace"
)

var agents = []string{"codex", "claude", "fclaude"}

func main() {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	for _, a := range agents {
		if name == a {
			echoAgent(name)
			return
		}
	}
	prepareOnly := flag.Bool("prepare", false, "prepare the home and print how to launch, without launching")
	root := flag.String("root", filepath.Join(os.TempDir(), "agent-remote-manual-acceptance"), "directory for the disposable home, projects and stand-in Agents")
	desktop := flag.String("desktop", "", "desktop executable to launch (default: go run . from the current directory)")
	flag.Parse()
	home, bin, err := prepare(*root)
	if err != nil {
		fmt.Fprintln(os.Stderr, "manual-acceptance:", err)
		os.Exit(2)
	}
	fmt.Printf("home:   %s\nagents: %s (codex, claude, fclaude show the bytes they receive)\n", home, bin)
	path := bin + string(os.PathListSeparator) + os.Getenv("PATH")
	if *prepareOnly {
		fmt.Printf("launch: PATH=%q <desktop> -home %q\n", path, home)
		return
	}
	cmd := exec.Command("go", "run", ".", "-home", home)
	if *desktop != "" {
		cmd = exec.Command(*desktop, "-home", home)
	}
	cmd.Env = append(os.Environ(), "PATH="+path)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "manual-acceptance:", err)
		os.Exit(1)
	}
}

// prepare makes a fresh home with two projects (a Go module and a Python
// one, so the ⌥X HUD shows a language) and copies this binary in as the
// stand-in Agents.
func prepare(root string) (home, bin string, err error) {
	if err = os.RemoveAll(root); err != nil {
		return
	}
	home, bin = filepath.Join(root, "home"), filepath.Join(root, "bin")
	projects := []struct{ name, marker string }{{"alpha-service", "go.mod"}, {"beta-scripts", "pyproject.toml"}}
	state := storage.State{Preferences: workspace.Defaults()}
	for i, p := range projects {
		dir := filepath.Join(root, "projects", p.name)
		if err = os.MkdirAll(dir, 0o700); err != nil {
			return
		}
		if err = os.WriteFile(filepath.Join(dir, p.marker), []byte("# manual acceptance\n"), 0o600); err != nil {
			return
		}
		project, perr := workspace.NewProject(dir)
		if perr != nil {
			return "", "", perr
		}
		project.LastOpened = time.Now().UTC().Add(-time.Duration(i) * time.Hour)
		state.Projects = append(state.Projects, project)
		workspace.AssignSlot(state.Projects, i)
	}
	state.Preferences.SelectedProject = state.Projects[0].ID
	if err = os.MkdirAll(bin, 0o700); err != nil {
		return
	}
	self, err := os.Executable()
	if err != nil {
		return
	}
	for _, a := range agents {
		if runtime.GOOS == "windows" {
			a += ".exe"
		}
		if err = copyFile(self, filepath.Join(bin, a)); err != nil {
			return
		}
	}
	store, err := storage.Open(home)
	if err != nil {
		return
	}
	defer store.Close()
	err = store.Save(state)
	return
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(to, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o700)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, in)
	return errors.Join(err, out.Close())
}

// echoAgent puts its terminal in raw mode and prints, for each read, the
// bytes as hex and as text. Ctrl+D (0x04) twice exits.
func echoAgent(name string) {
	restore := rawMode()
	defer restore()
	fmt.Printf("\r\n  %s stand-in — every byte the terminal sends appears below.\r\n", name)
	fmt.Printf("  A chord (⌥A/⌥C/⌥X/⌥S, Alt+… on Windows/Linux) must print NOTHING here.\r\n")
	fmt.Printf("  Type normally to see the bytes arrive. Ctrl+D twice to exit.\r\n\r\n")
	// AGENT_REMOTE_ACCEPTANCE_LOG, when set, also receives the raw bytes,
	// so a scripted check can assert exactly what reached the Agent.
	var log *os.File
	if path := os.Getenv("AGENT_REMOTE_ACCEPTANCE_LOG"); path != "" {
		var err error
		if log, err = os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err != nil {
			fmt.Printf("  (byte log unavailable: %v)\r\n", err)
		} else {
			defer log.Close()
		}
		// A ready mark, so a scripted check starts typing once the Agent reads.
		if mark := path + ".ready"; log != nil {
			_ = os.WriteFile(mark, []byte(name), 0o600)
		}
	}
	buf := make([]byte, 256)
	eot := 0
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 && log != nil {
			_, _ = log.Write(buf[:n])
		}
		if n > 0 {
			var hex, text strings.Builder
			for _, b := range buf[:n] {
				fmt.Fprintf(&hex, "%02x ", b)
				if b >= 0x20 && b < 0x7f {
					text.WriteByte(b)
				} else {
					text.WriteByte('.')
				}
			}
			fmt.Printf("  %s  received %d byte(s): %s |%s|\r\n", time.Now().Format("15:04:05.000"), n, strings.TrimSpace(hex.String()), text.String())
			if n == 1 && buf[0] == 0x04 {
				if eot++; eot == 2 {
					return
				}
			} else {
				eot = 0
			}
		}
		if err != nil {
			return
		}
	}
}
