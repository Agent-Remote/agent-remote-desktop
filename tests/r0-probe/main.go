// r0-probe gathers R0 evidence for the UI redesign (agent-remote docs,
// agent-remote-desktop-redesign.zh-CN.md §8): whether ⌥/Alt+letter chords
// reach the app while a terminal has focus without leaking bytes to the
// program, whether the modifier release is observable, and what a blurred
// overlay over a streaming terminal costs per frame. It injects OS-level key
// events into its own window and records exactly what the in-process "program"
// behind the terminal receives. It never starts an Agent or touches user state.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type report struct {
	Schema      string         `json:"schema"`
	Mode        string         `json:"mode"`
	GOOS        string         `json:"goos"`
	GOARCH      string         `json:"goarch"`
	HostRuntime bool           `json:"host_runtime"`
	Display     string         `json:"display,omitempty"`
	Injection   string         `json:"injection"`
	Status      string         `json:"status"` // passed | failed | not_run
	Reason      string         `json:"reason,omitempty"`
	Chord       []*chordReport `json:"chord,omitempty"`
	Blur        []blurVariant  `json:"blur,omitempty"`
	Captures    []string       `json:"captures,omitempty"`
	Extra       map[string]any `json:"extra,omitempty"`
}

// program is the in-process process behind the terminal: Write receives the
// bytes the terminal sends (what a real Agent would read from its PTY) and
// Read feeds the terminal its output.
type program struct {
	mu       sync.Mutex
	received []byte
	out      chan []byte
	pending  []byte
	closed   chan struct{}
}

func newProgram() *program {
	return &program{out: make(chan []byte, 256), closed: make(chan struct{})}
}

func (p *program) Write(b []byte) (int, error) {
	p.mu.Lock()
	p.received = append(p.received, b...)
	p.mu.Unlock()
	return len(b), nil
}

func (p *program) Read(b []byte) (int, error) {
	if len(p.pending) == 0 {
		select {
		case chunk := <-p.out:
			p.pending = chunk
		case <-p.closed:
			return 0, io.EOF
		}
	}
	n := copy(b, p.pending)
	p.pending = p.pending[n:]
	return n, nil
}

func (p *program) Close() error {
	select {
	case <-p.closed:
	default:
		close(p.closed)
	}
	return nil
}

func (p *program) Received() []byte {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]byte(nil), p.received...)
}

func main() {
	mode := flag.String("mode", "chord", "chord | blur")
	outDir := flag.String("out", "", "directory for PNG captures (optional)")
	manual := flag.Bool("manual", false, "chord: do not inject; print instructions and wait for a person to press the keys")
	seconds := flag.Float64("variant-seconds", 3, "blur: seconds per overlay variant")
	flag.Parse()

	// Frame statistics are read once when the ui package initialises, so the
	// blur run re-executes itself with MYGO_FRAME_STATS=all.
	if *mode == "blur" && os.Getenv("MYGO_FRAME_STATS") == "" {
		self, err := os.Executable()
		if err != nil {
			fail(*mode, err)
		}
		cmd := exec.Command(self, os.Args[1:]...)
		cmd.Env = append(os.Environ(), "MYGO_FRAME_STATS=all")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			os.Exit(1)
		}
		return
	}
	// Never change the user's persisted language preference.
	_ = os.Setenv("MYGO_LOCALE", "en")

	r := &report{Schema: "agent-remote-desktop.r0-probe/v1", Mode: *mode, GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		HostRuntime: true, Display: displayName(), Injection: injectorName(*manual)}
	if *outDir != "" {
		if err := os.MkdirAll(*outDir, 0o755); err != nil {
			fail(*mode, err)
		}
	}
	frames := newFrameLog()
	if *mode == "blur" {
		log.SetFlags(0)
		log.SetOutput(frames)
	}

	mygo.App.SetName("Agent Remote R0 Probe")
	mygo.App.OnWindowAllClosed(func() {})
	done := make(chan struct{})
	mygo.App.WhenReady(func() {
		var err error
		switch *mode {
		case "chord":
			err = runChord(r, *manual, *outDir, done)
		case "blur":
			err = runBlur(r, frames, time.Duration(*seconds*float64(time.Second)), *outDir, done)
		default:
			err = fmt.Errorf("unknown mode %q", *mode)
		}
		if err != nil {
			r.Status, r.Reason = "failed", err.Error()
			close(done)
		}
	})
	go func() { <-done; mygo.RunOnMain(func() { mygo.App.Quit() }) }()
	if err := mygo.App.Run(); err != nil {
		r.Status, r.Reason = "failed", err.Error()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(r)
	if r.Status == "failed" {
		os.Exit(1)
	}
}

func fail(mode string, err error) {
	fmt.Fprintf(os.Stdout, `{"schema":"agent-remote-desktop.r0-probe/v1","mode":%q,"status":"failed","reason":%q}`+"\n", mode, err.Error())
	os.Exit(1)
}

func displayName() string {
	switch runtime.GOOS {
	case "linux":
		if os.Getenv("WAYLAND_DISPLAY") != "" {
			return "wayland"
		}
		if d := os.Getenv("DISPLAY"); d != "" {
			return "x11 " + d
		}
		return "none"
	default:
		return "native"
	}
}

// waitFocused waits until the window is the key window, activating the app.
func waitFocused(win *mygo.Window, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if win.IsFocused() {
			return true
		}
		mygo.RunOnMain(func() { mygo.App.Focus(); win.Focus() })
		time.Sleep(100 * time.Millisecond)
	}
	return win.IsFocused()
}

func trimLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
