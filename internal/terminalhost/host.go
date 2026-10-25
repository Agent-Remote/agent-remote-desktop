// Package terminalhost owns PTY processes separately from the terminal renderer.
package terminalhost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Agent-Remote/agent-remote-desktop/internal/attachment"
	"github.com/Agent-Remote/agent-remote-desktop/internal/fault"
	"github.com/Agent-Remote/agent-remote-desktop/internal/session"
	"github.com/charmbracelet/x/xpty"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/transfer"
	"github.com/egoist/mygo/ui"
)

// ErrStopTimeout leaves the process and terminal intact until explicit force approval.
var ErrStopTimeout = fault.StopTimeout

type Host struct {
	Terminal           *terminal.Terminal
	pty                xpty.Pty
	cmd                *exec.Cmd
	done               chan struct{}
	exit               atomic.Int32
	stopMu             sync.Mutex
	closeOnce          sync.Once
	group              processGroup
	attachmentTransfer bool
	attachmentLimits   attachment.Limits
	titleMu            sync.RWMutex
	title              string
	// lastOutput is when the program last wrote, in Unix nanoseconds. It is
	// the only activity signal the workbench shows: output arriving is a
	// fact, whereas "thinking" or "waiting" cannot be told from a PTY.
	lastOutput atomic.Int64
}

// Options are the workbench hooks of a terminal. OnKey runs on the main
// thread before the terminal encodes a key and takes it by returning true:
// neither the key nor the text it types reaches the program (R0 showed that
// a window shortcut alone leaks Option text on macOS).
type Options struct {
	OnKey func(mods ui.Modifiers, key ui.Key) bool
}

// conn closes only the stream. Terminal.Close must not decide process policy.
type conn struct {
	xpty.Pty
	once sync.Once
	read func(n int)
}

func (c *conn) Read(p []byte) (int, error) {
	n, err := c.Pty.Read(p)
	if n > 0 && c.read != nil {
		c.read(n)
	}
	return n, err
}

func (c *conn) Close() error { var err error; c.once.Do(func() { err = c.Pty.Close() }); return err }

func New(command session.Command) (*Host, error) { return NewWithOptions(command, Options{}) }

// NewWithOptions starts command with the workbench's terminal hooks.
func NewWithOptions(command session.Command, hooks Options) (*Host, error) {
	if len(command.Argv) == 0 {
		return nil, fault.ExecutableMissing
	}
	p, err := xpty.NewPty(100, 30)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(command.Argv[0], command.Argv[1:]...)
	cmd.Dir = command.Directory
	cmd.Env = environment(os.Environ(), append([]string{"TERM=xterm-256color", "COLORTERM=truecolor"}, command.Env...))
	configureProcess(cmd)
	h := &Host{pty: p, cmd: cmd, done: make(chan struct{}), attachmentTransfer: command.Remote && command.AttachmentTransfer, attachmentLimits: command.AttachmentLimits}
	h.exit.Store(-1)
	stream := &conn{Pty: p, read: func(int) { h.lastOutput.Store(time.Now().UnixNano()) }}
	// Keep arbitrary OSC 52/5522 reads denied, including for exact remote
	// sessions. PasteData offers are answered independently by mygo when the
	// user explicitly pastes or drops a payload, so the fclaude attachment
	// bridge does not need ambient clipboard read access.
	clipboardRead := terminal.ClipboardDeny
	var term *terminal.Terminal
	options := terminal.Options{
		Conn: stream, Scrollback: 4 * 1024 * 1024,
		Font:          terminal.Font{Family: "JetBrains Mono, Menlo, Consolas, monospace", Size: 14},
		ClipboardRead: clipboardRead,
		// Agent TUIs commonly publish their current mode or project through an
		// OSC title. Keep that transient status at the host boundary so the
		// workbench can show it without persisting terminal-controlled text.
		OnTitle: h.setTitle,
		OnKey:   hooks.OnKey,
	}
	if command.Remote && !command.AttachmentTransfer {
		// A remote terminal without the exact attachment contract remains
		// text-only even when the program enables Kitty clipboard reads. The
		// terminal plugin otherwise offers every native clipboard format in
		// mode 5522, which would bypass the desktop capability gate.
		options.OnPaste = func(data transfer.Data) bool {
			filtered, handled := FilterRemotePaste(data)
			if !handled {
				return false
			}
			if term != nil && len(filtered.Formats()) > 0 {
				term.PasteData(filtered)
			}
			return true
		}
	}
	if command.Remote && command.AttachmentTransfer {
		// Native clipboard paste is read on the UI thread and offered later by
		// the terminal goroutine. Reject an over-limit file/image before that
		// offer is created; returning true consumes the event so it cannot fall
		// back to text and accidentally execute a path in the remote shell.
		options.OnPaste = func(data transfer.Data) bool {
			if !attachment.ContainsPayload(data) {
				return false
			}
			return h.attachmentLimits.ValidateData(data) != nil
		}
	}
	t, err := terminal.New(options)
	if err != nil {
		stream.Close()
		return nil, err
	}
	term = t
	h.Terminal = t
	if err = p.Start(cmd); err != nil {
		t.Close()
		return nil, err
	}
	if err = h.group.attach(cmd); err != nil {
		cmd.Process.Kill()
		_ = xpty.WaitProcess(context.Background(), cmd)
		t.Close()
		return nil, err
	}
	// The parent's slave descriptor otherwise keeps Unix reads open after child exit.
	if unix, ok := p.(*xpty.UnixPty); ok {
		_ = unix.Slave().Close()
	}
	go func() {
		_ = xpty.WaitProcess(context.Background(), cmd)
		if cmd.ProcessState != nil {
			h.exit.Store(int32(cmd.ProcessState.ExitCode()))
		}
		close(h.done)
	}()
	return h, nil
}

// FilterRemotePaste keeps a legacy or unverified remote session text-only.
// It returns handled=true for any clipboard payload that contains a format
// outside ordinary textual representations. In that case only text/plain is
// offered to the terminal; if no text exists, the paste is consumed without
// sending an image, file list, URI list, or custom MIME payload.
func FilterRemotePaste(data transfer.Data) (filtered transfer.Data, handled bool) {
	formats := data.Formats()
	for _, format := range formats {
		switch format {
		case transfer.Text, transfer.HTML, transfer.Markdown:
			continue
		default:
			text, err := data.Read(transfer.Text)
			if err != nil {
				return transfer.Data{}, true
			}
			return transfer.TextData(string(text)), true
		}
	}
	return transfer.Data{}, false
}

func environment(base, overrides []string) []string {
	result := append([]string(nil), base...)
	for _, value := range overrides {
		name, _, ok := strings.Cut(value, "=")
		if !ok {
			continue
		}
		replaced := false
		for i, entry := range result {
			key, _, _ := strings.Cut(entry, "=")
			if environmentKeyEqual(key, name) {
				result[i] = value
				replaced = true
				break
			}
		}
		if !replaced {
			result = append(result, value)
		}
	}
	return result
}

func (h *Host) Done() <-chan struct{} { return h.done }

// LastOutput is when the program last wrote to the terminal; zero if never.
func (h *Host) LastOutput() time.Time {
	if n := h.lastOutput.Load(); n != 0 {
		return time.Unix(0, n)
	}
	return time.Time{}
}
func (h *Host) ExitCode() int { return int(h.exit.Load()) }

// Title returns the latest transient OSC title emitted by the managed
// program. It is presentation state only: callers must not use it as an
// identity, command, or recovery reference, and it is never persisted.
func (h *Host) Title() string {
	if h == nil {
		return ""
	}
	h.titleMu.RLock()
	defer h.titleMu.RUnlock()
	return h.title
}

func (h *Host) setTitle(value string) {
	value = normalizeTitle(value)
	h.titleMu.Lock()
	h.title = value
	h.titleMu.Unlock()
}

func normalizeTitle(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, value)
	value = strings.TrimSpace(value)
	if len(value) <= 256 {
		return value
	}
	value = value[:256]
	for len(value) > 0 && !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

// Resize changes the native PTY/ConPTY size in terminal cells. The terminal
// renderer normally calls this through its Conn interface when a visible
// terminal view is laid out; exposing the same operation on Host keeps the
// process boundary explicit for callers that resize a session while its pane
// is hidden or before the first frame is painted.
//
// Both dimensions are validated here so a transient zero-sized native window
// cannot send an invalid resize request to Unix PTY or Windows ConPTY.
func (h *Host) Resize(cols, rows int) error {
	if cols < 1 || rows < 1 {
		return fmt.Errorf("terminal size must be positive: %dx%d", cols, rows)
	}
	return h.pty.Resize(cols, rows)
}

// Size reports the native PTY/ConPTY size in terminal cells.
func (h *Host) Size() (cols, rows int, err error) {
	return h.pty.Size()
}

func (h *Host) AttachmentTransferEnabled() bool {
	return h != nil && h.attachmentTransfer
}

// PasteData offers native clipboard/drag representations to the process in
// this terminal. It is used only for a capability-gated exact fclaude
// transfer; callers keep the terminal's regular text paste behavior intact.
func (h *Host) PasteData(data transfer.Data) error {
	if h == nil || h.Terminal == nil || len(data.Formats()) == 0 {
		return errors.New("terminal is unavailable")
	}
	if h.attachmentTransfer && attachment.ContainsPayload(data) {
		if err := h.attachmentLimits.ValidateData(data); err != nil {
			return err
		}
	}
	h.Terminal.PasteData(data)
	return nil
}

func (h *Host) Stop(force bool) error {
	h.stopMu.Lock()
	defer h.stopMu.Unlock()
	select {
	case <-h.done:
		// A graceful leader may have left a foreground helper in the process
		// group. The leader's exit code is already captured; force-clean the
		// remaining group without changing that recorded status.
		if !force {
			_ = h.group.signal(h.cmd, h.pty, true)
		}
		return nil
	default:
	}
	if err := h.group.signal(h.cmd, h.pty, force); err != nil && !errors.Is(err, os.ErrProcessDone) {
		select {
		case <-h.done:
			return nil
		default:
			return err
		}
	}
	select {
	case <-h.done:
		if !force {
			_ = h.group.signal(h.cmd, h.pty, true)
		}
		return nil
	case <-time.After(gracefulStopTimeout()):
		return ErrStopTimeout
	}
}

// Close releases an exited process. It never silently forces a live process.
func (h *Host) Close() error {
	select {
	case <-h.done:
	default:
		if err := h.Stop(false); err != nil {
			return err
		}
	}
	var result error
	h.closeOnce.Do(func() {
		if err := h.Terminal.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
			result = fmt.Errorf("close terminal: %w", err)
		}
		h.group.close()
	})
	return result
}
