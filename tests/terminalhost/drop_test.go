package terminalhost_test

import (
	"errors"
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
	h, err := terminalhost.New(session.Command{
		Argv:      []string{"/bin/sh", "-c", "read answer; printf 'reply:%s\\n' \"$answer\""},
		Directory: t.TempDir(),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })
	if err := h.InsertDroppedPaths([]string{"/tmp/has space/it's-safe"}); err != nil {
		t.Fatal(err)
	}
	h.Terminal.Send([]byte("\r"))
	waitText(t, h, "'/tmp/has space/it'\\''s-safe'")
	select {
	case <-h.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("drop input did not finish the shell")
	}
}
