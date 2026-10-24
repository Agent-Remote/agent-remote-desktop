package session_test

import (
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

func TestTerminalKeepsReadingWithoutVisibleView(t *testing.T) {
	if os.Getenv("DESKTOP_TERMINAL_TESTS") != "1" {
		t.Skip("set DESKTOP_TERMINAL_TESTS=1 to load the pinned native terminal library")
	}
	cmd := []string{"/bin/sh", "-c", "printf 'ready\\n'; read answer; printf 'reply:%s\\n' \"$answer\""}
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd.exe", "/Q", "/V:ON", "/C", "echo ready & set /p answer= & echo reply:!answer!"}
	}
	term, err := terminal.New(terminal.Options{Command: cmd, Dir: t.TempDir(), Scrollback: 1024 * 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer term.Close()
	// No terminal.View exists: startup and output consumption must remain independent.
	deadline := time.Now().Add(10 * time.Second)
	for !strings.Contains(term.Text(), "ready") && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(term.Text(), "ready") {
		t.Fatal("hidden terminal did not start or consume output")
	}
	term.Send([]byte("desktop-smoke\r"))
	select {
	case <-term.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("PTY process did not exit")
	}
	if !strings.Contains(term.Text(), "reply:desktop-smoke") {
		t.Fatalf("input/output roundtrip failed: %q", term.Text())
	}
	tt := ui.NewTester(func(c *ui.Context) { terminal.View(c, term).Fill() }, 800, 450)
	tt.SetSize(640, 400)
	if !strings.Contains(term.Text(), "reply:desktop-smoke") {
		t.Fatal("recreating view lost terminal output")
	}
}
