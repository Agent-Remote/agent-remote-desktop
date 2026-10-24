package terminalhost_test

import (
	"errors"
	"runtime"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
)

func TestFormatDroppedPathsPOSIXQuotesShellData(t *testing.T) {
	got, err := terminalhost.FormatDroppedPaths([]string{"/tmp/has space/it's;safe", "/tmp/$HOME/emoji-界.txt"}, false)
	if err != nil {
		t.Fatal(err)
	}
	want := "'/tmp/has space/it'\\''s;safe' '/tmp/$HOME/emoji-界.txt'"
	if got != want {
		t.Fatalf("formatted paths = %q, want %q", got, want)
	}
	if got == "/tmp/has space/it's;safe /tmp/$HOME/emoji-界.txt" {
		t.Fatal("paths were inserted without shell quoting")
	}
}

func TestFormatDroppedPathsRejectsRelativeAndExpansionPaths(t *testing.T) {
	for _, test := range []struct {
		name    string
		path    string
		windows bool
	}{
		{name: "relative", path: "./file.txt"},
		{name: "nul", path: "/tmp/a\x00b"},
		{name: "windows-percent", path: `C:\Users\A%PATH%\file.txt`, windows: true},
		{name: "windows-bang", path: `C:\Users\A!PATH!\file.txt`, windows: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := terminalhost.FormatDroppedPaths([]string{test.path}, test.windows); !errors.Is(err, terminalhost.ErrInvalidDroppedPath) {
				t.Fatalf("error = %v, want ErrInvalidDroppedPath", err)
			}
		})
	}
}

func TestFormatDroppedPathsWindowsEscapesCommandOperators(t *testing.T) {
	got, err := terminalhost.FormatDroppedPaths([]string{`C:\Users\A & B\file (1).txt`}, true)
	if err != nil {
		t.Fatal(err)
	}
	want := `"C:\Users\A ^& B\file ^(1^).txt"`
	if got != want {
		t.Fatalf("formatted Windows path = %q, want %q", got, want)
	}
	quoted, err := terminalhost.FormatDroppedPaths([]string{`C:\Users\A "Q".txt`}, true)
	if err != nil {
		t.Fatal(err)
	}
	if quoted != `"C:\Users\A \"Q\".txt"` {
		t.Fatalf("quoted Windows path = %q", quoted)
	}
}

func TestHostInsertDroppedPathsKeepsInputAsOneShellArgument(t *testing.T) {
	integration(t)
	command := []string{"/bin/sh", "-c", "read answer; printf 'reply:%s\\n' \"$answer\""}
	if runtime.GOOS == "windows" {
		// The native Windows probe covers command-prompt quoting. This keeps
		// the host-level test focused on the same round-trip on ConPTY.
		command = []string{"cmd.exe", "/Q", "/V:ON", "/C", "set /p answer= & echo reply:!answer!"}
	}
	h, err := terminalhost.New(session.Command{Argv: command, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })
	path := "/tmp/has space/it's-safe"
	want := "'/tmp/has space/it'\\''s-safe'"
	if runtime.GOOS == "windows" {
		path = `C:\Users\A B\file.txt`
		want = `"C:\Users\A B\file.txt"`
	}
	if err := h.InsertDroppedPaths([]string{path}); err != nil {
		t.Fatal(err)
	}
	h.Terminal.Send([]byte("\r"))
	waitText(t, h, want)
	select {
	case <-h.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("drop input did not finish the shell")
	}
}
