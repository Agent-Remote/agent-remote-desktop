package terminalhost_test

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/platform"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/Agent-Remote/agent-remote-desktop/internal/terminalhost"
	"github.com/egoist/mygo/transfer"
)

func integration(t *testing.T) {
	t.Helper()
	if os.Getenv("DESKTOP_TERMINAL_TESTS") != "1" {
		t.Skip("set DESKTOP_TERMINAL_TESTS=1")
	}
}

func TestHostInputOutputAndExitStatus(t *testing.T) {
	integration(t)
	cmd := []string{"/bin/sh", "-c", "printf 'ready\\n'; read answer; printf 'reply:%s\\n' \"$answer\"; exit 7"}
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd.exe", "/Q", "/V:ON", "/C", "echo ready & set /p answer= & echo reply:!answer! & exit /b 7"}
	}
	h, err := terminalhost.New(session.Command{Argv: cmd, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Stop(true); h.Close() })
	waitText(t, h, "ready")
	h.Terminal.Send([]byte("managed-pty\r"))
	select {
	case <-h.Done():
	case <-time.After(10 * time.Second):
		t.Fatal("process did not exit")
	}
	waitText(t, h, "reply:managed-pty")
	if h.ExitCode() != 7 {
		t.Fatalf("exit code %d", h.ExitCode())
	}
}

func TestHostCapturesTransientUnicodeTerminalTitle(t *testing.T) {
	integration(t)
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell fixture; ConPTY title handling is covered by the native Windows probe")
	}
	cmd := []string{"/bin/sh", "-c", "printf '\\033]0;工作台 🚀\007'; printf 'title-ready\\n'; read answer"}
	h, err := terminalhost.New(session.Command{Argv: cmd, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })
	waitText(t, h, "title-ready")
	deadline := time.Now().Add(3 * time.Second)
	for h.Title() != "工作台 🚀" && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if got := h.Title(); got != "工作台 🚀" {
		t.Fatalf("transient OSC title = %q, want %q", got, "工作台 🚀")
	}
	// The title is presentation state only and remains available while the
	// process is hidden behind a workbench view; it is not tied to output text.
	if got := h.Title(); got != "工作台 🚀" {
		t.Fatalf("title changed while terminal remained alive: %q", got)
	}
}

func TestHostResizeUpdatesNativePTYSize(t *testing.T) {
	integration(t)
	cmd := []string{"/bin/sh", "-c", "read answer"}
	if runtime.GOOS == "windows" {
		cmd = []string{"cmd.exe", "/Q", "/C", "set /p answer="}
	}
	h, err := terminalhost.New(session.Command{Argv: cmd, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })

	if cols, rows, err := h.Size(); err != nil {
		t.Fatal(err)
	} else if cols != 100 || rows != 30 {
		t.Fatalf("initial PTY size = %dx%d, want 100x30", cols, rows)
	}
	if err := h.Resize(132, 41); err != nil {
		t.Fatal(err)
	}
	if cols, rows, err := h.Size(); err != nil {
		t.Fatal(err)
	} else if cols != 132 || rows != 41 {
		t.Fatalf("resized PTY size = %dx%d, want 132x41", cols, rows)
	}
	if err := h.Resize(0, 41); err == nil {
		t.Fatal("zero-column resize unexpectedly succeeded")
	}
}

func TestHostAttachmentTransferRequiresExplicitRemoteLaunchFlag(t *testing.T) {
	integration(t)
	command := []string{"/bin/sh", "-c", "printf 'ready\\n'"}
	if runtime.GOOS == "windows" {
		command = []string{"cmd.exe", "/Q", "/C", "echo ready"}
	}
	for _, tc := range []struct {
		name     string
		remote   bool
		transfer bool
		expected bool
	}{
		{name: "local transfer flag is ignored", remote: false, transfer: true},
		{name: "remote legacy host stays disabled", remote: true, transfer: false},
		{name: "remote exact host enables transfer", remote: true, transfer: true, expected: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h, err := terminalhost.New(session.Command{
				Argv: command, Directory: t.TempDir(), Remote: tc.remote,
				AttachmentTransfer: tc.transfer,
			})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })
			if got := h.AttachmentTransferEnabled(); got != tc.expected {
				t.Fatalf("AttachmentTransferEnabled() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestRestrictRemotePasteKeepsUnverifiedSessionsTextOnly(t *testing.T) {
	textOnly := transfer.TextData("prompt")
	if _, handled := terminalhost.FilterRemotePaste(textOnly); handled {
		t.Fatal("plain text paste was intercepted")
	}

	filtered, handled := terminalhost.FilterRemotePaste(transfer.New(
		transfer.NewItem(transfer.Bytes(transfer.Text, []byte("file text")), transfer.Bytes(transfer.FileList, []byte("file:///tmp/secret.txt"))),
	))
	if !handled || string(mustReadTransferText(t, filtered)) != "file text" {
		t.Fatalf("file payload was not reduced to text: handled=%v formats=%v", handled, filtered.Formats())
	}

	image := transfer.New(transfer.NewItem(transfer.Bytes(transfer.PNG, []byte{1, 2, 3})))
	filtered, handled = terminalhost.FilterRemotePaste(image)
	if !handled || len(filtered.Formats()) != 0 {
		t.Fatalf("image-only payload was not consumed: handled=%v formats=%v", handled, filtered.Formats())
	}
}

func mustReadTransferText(t *testing.T, data transfer.Data) []byte {
	t.Helper()
	value, err := data.Read(transfer.Text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestGracefulTimeoutRequiresSeparateForce(t *testing.T) {
	integration(t)
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal trap fixture")
	}
	h, err := terminalhost.New(session.Command{Argv: []string{"/bin/sh", "-c", "trap '' TERM; printf 'ignoring-term\\n'; while :; do sleep 1; done"}, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { h.Stop(true); h.Close() })
	waitText(t, h, "ignoring-term")
	if err = h.Stop(false); !errors.Is(err, terminalhost.ErrStopTimeout) {
		t.Fatalf("expected confirmation timeout, got %v", err)
	}
	select {
	case <-h.Done():
		t.Fatal("process was killed without force approval")
	default:
	}
	if err = h.Stop(true); err != nil {
		t.Fatal(err)
	}
	select {
	case <-h.Done():
	case <-time.After(time.Second):
		t.Fatal("forced process still alive")
	}
}

func TestGracefulStopPublishesFinalExitCodeBeforeClose(t *testing.T) {
	integration(t)
	if runtime.GOOS == "windows" {
		t.Skip("POSIX signal trap fixture")
	}
	h, err := terminalhost.New(session.Command{Argv: []string{"/bin/sh", "-c", "trap 'exit 42' TERM; while :; do sleep 1; done"}, Directory: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Stop(true); _ = h.Close() })
	// Let the shell install its TERM trap before exercising the graceful stop
	// path; otherwise a signal delivered during exec setup is legitimately
	// reported as signal termination rather than the fixture's exit 42.
	time.Sleep(100 * time.Millisecond)
	if err := h.Stop(false); err != nil {
		t.Fatal(err)
	}
	if h.ExitCode() != 42 {
		t.Fatalf("graceful stop exit code = %d, want 42; terminal=%q", h.ExitCode(), h.Terminal.Text())
	}
	if err := h.Close(); err != nil {
		t.Fatal(err)
	}
}

func waitText(t *testing.T, h *terminalhost.Host, text string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(h.Terminal.Text(), text) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("terminal did not contain %q; got %q", text, h.Terminal.Text())
}

func TestInstalledAgentsLaunchInsideManagedPTY(t *testing.T) {
	if os.Getenv("DESKTOP_LOCAL_AGENT_TESTS") != "1" {
		t.Skip("set DESKTOP_LOCAL_AGENT_TESTS=1 for installed-tool version probes")
	}
	for _, name := range []string{"codex", "claude", "fclaude"} {
		t.Run(name, func(t *testing.T) {
			binary, err := platform.Executable(name)
			if err != nil {
				t.Skip(err)
			}
			dir := t.TempDir()
			h, err := terminalhost.New(session.Command{Argv: []string{binary, "--version"}, Directory: dir, Env: []string{"AGENT_REMOTE_HOME=" + dir}})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { h.Stop(true); h.Close() })
			select {
			case <-h.Done():
			case <-time.After(20 * time.Second):
				t.Fatal("Agent version probe timed out")
			}
			if h.ExitCode() != 0 {
				t.Fatalf("Agent exit: %d", h.ExitCode())
			}
		})
	}
}
