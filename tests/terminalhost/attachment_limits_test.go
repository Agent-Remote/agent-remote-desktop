package terminalhost_test

import (
	"errors"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/attachment"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/egoist/mygo/transfer"
)

func TestExactHostRejectsAttachmentPayloadsOverDeclaredLimits(t *testing.T) {
	if os.Getenv("DESKTOP_TERMINAL_TESTS") != "1" {
		t.Skip("set DESKTOP_TERMINAL_TESTS=1")
	}
	command := []string{"/bin/sh", "-c", "while read answer; do printf 'received:%s\\n' \"$answer\"; done"}
	if runtime.GOOS == "windows" {
		command = []string{"cmd.exe", "/Q", "/C", "for /L %i in (1,1,999) do (set /p answer= & echo received:%answer%)"}
	}
	h, err := terminalhost.New(session.Command{
		Argv: command, Directory: t.TempDir(), Remote: true, AttachmentTransfer: true,
		AttachmentLimits: attachment.Limits{MaxBytes: 4, MaxEntries: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })
	file := t.TempDir() + string(os.PathSeparator) + "payload.txt"
	if err := os.WriteFile(file, []byte("12345"), 0600); err != nil {
		t.Fatal(err)
	}
	data, err := transfer.FileData(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.PasteData(data); !errors.Is(err, attachment.ErrTooLarge) {
		t.Fatalf("oversized exact payload = %v, want ErrTooLarge", err)
	}
	select {
	case <-h.Done():
		t.Fatal("host exited while checking attachment limit")
	case <-time.After(100 * time.Millisecond):
	}
}
