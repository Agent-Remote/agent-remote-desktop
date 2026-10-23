package main

import (
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chord"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

type chordReport struct {
	Route          string       `json:"route"` // shortcut: c.OnShortcut only (today's workbench); onkey: terminal Options.OnKey takes the chord
	Focused        bool         `json:"window_focused"`
	TerminalFocus  bool         `json:"terminal_had_focus"`
	Expected       string       `json:"expected_program_bytes"`
	Received       string       `json:"received_program_bytes"`
	Leaked         bool         `json:"leaked_bytes"`
	AltTapThenKey  string       `json:"alt_tap_then_key"` // reached | swallowed
	Steps          []stepResult `json:"steps"`
	ReleaseAfterMs []float64    `json:"release_observed_after_ms"`
	// DoubleDispatch counts window shortcuts that also fired for a chord
	// OnKey had already taken (onkey route); non-zero means R2 must dedupe.
	DoubleDispatch int  `json:"shortcut_also_fired"`
	Passed         bool `json:"passed"`
	// IME is recorded where the probe controls the input method (Windows).
	IME *imeReport `json:"ime,omitempty"`
}

type imeReport struct {
	InitiallyOpen bool     `json:"initially_open"`
	ChordEvents   []string `json:"chord_events_with_ime_open"`
	ChordBytes    string   `json:"bytes_from_chord_with_ime_open"`
	ComposedBytes string   `json:"bytes_from_typing_ni_space"`
	Error         string   `json:"error,omitempty"`
}

type stepResult struct {
	Name string   `json:"name"`
	Want []string `json:"want"`
	Got  []string `json:"got"`
	Pass bool     `json:"pass"`
}

var outcomeNames = map[chord.Outcome]string{
	chord.Opened: "Opened", chord.Cycled: "Cycled", chord.Retargeted: "Retargeted",
	chord.Committed: "Committed", chord.MadeSticky: "MadeSticky", chord.Closed: "Closed",
}

type chordProbe struct {
	route     string
	win       *mygo.Window
	doubles   atomic.Int32
	rec       chord.Recognizer // main thread only
	focus     string           // terminal.View focus binding
	mu        sync.Mutex
	events    []string
	latencies []float64
	altUpAt   atomic.Int64 // unix nanos of the last injected Alt release
	termFocus atomic.Bool
}

func (p *chordProbe) record(o chord.Outcome, layer string) {
	if o == chord.None {
		return
	}
	p.mu.Lock()
	p.events = append(p.events, outcomeNames[o]+":"+layer)
	p.mu.Unlock()
}

func (p *chordProbe) take() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := p.events
	p.events = nil
	return out
}

func (p *chordProbe) view(c *ui.Context, term *terminal.Terminal) {
	ui.Column(c).Fill().Background(ui.Hex("#09090B")).Padding(14).Children(func() {
		terminal.View(c.Key("term"), term).Grow(1).FillWidth().FocusBind(&p.focus, "term")
	})
	p.termFocus.Store(ui.FocusedValue(c, &p.focus) == "term")
	// The same root-scope registration the workbench uses for its commands.
	// A window shortcut that claims a key keeps its key-down from the focused
	// terminal, but the platform still delivers the text the key types (å on
	// macOS), which the terminal then forwards as input-method text. So the
	// onkey route registers the window shortcuts only while no terminal has
	// the focus; with a terminal focused, its OnKey takes the chord instead.
	if p.route == "shortcut" || !p.termFocus.Load() {
		for key, layer := range chordKeys {
			c.OnShortcut(ui.Alt, key, func() {
				if p.route == "onkey" && p.termFocus.Load() {
					p.doubles.Add(1) // focus moved into the terminal this frame
					return
				}
				p.record(p.rec.Press(layer, time.Now()), layer)
			})
		}
	}
	// Release is observed from the held modifiers, which every backend
	// updates with ModifiersChanged and which draws a frame when it changes.
	if p.rec.Held() && c.Modifiers()&ui.Alt == 0 {
		layer := p.rec.Layer
		p.record(p.rec.ModifierReleased(time.Now()), layer)
		if at := p.altUpAt.Load(); at != 0 {
			p.mu.Lock()
			p.latencies = append(p.latencies, float64(time.Since(time.Unix(0, at)).Microseconds())/1000)
			p.mu.Unlock()
		}
	}
}

var chordKeys = map[ui.Key]string{ui.KeyA: "agents", ui.KeyC: "create", ui.KeyX: "projects", ui.KeyS: "cc"}

// runChord runs the same injected scenario twice, each in a fresh window,
// terminal and program: first with window shortcuts alone (as the workbench
// registers commands today), then with the terminal's OnKey taking the chord.
func runChord(r *report, manual bool, outDir string, done chan struct{}) error {
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu},
		{Label: "Workbench", Submenu: []*mygo.MenuItem{{Label: "New session", Accelerator: "CmdOrCtrl+T"}}},
		{Role: mygo.RoleEditMenu}, {Role: mygo.RoleWindowMenu}}))
	go func() {
		defer close(done)
		for _, route := range []string{"shortcut", "onkey"} {
			rep, err := chordPass(r, route, manual, outDir)
			if err != nil {
				r.Status, r.Reason = "failed", err.Error()
				return
			}
			if rep == nil { // not_run or manual
				return
			}
			r.Chord = append(r.Chord, rep)
		}
		// The onkey route is the candidate design; the shortcut route is the
		// baseline that shows why it is needed.
		r.Status, r.Reason = "failed", "onkey route did not pass"
		if last := r.Chord[len(r.Chord)-1]; last.Route == "onkey" && last.Passed {
			r.Status, r.Reason = "passed", ""
		}
	}()
	return nil
}

func chordPass(r *report, route string, manual bool, outDir string) (*chordReport, error) {
	prog := newProgram()
	defer prog.Close()
	p := &chordProbe{route: route, focus: "term"}
	opts := terminal.Options{Conn: prog, Font: terminal.Font{Size: 14}}
	if route == "onkey" {
		// Runs on the main thread before the terminal encodes the key, so a
		// taken chord sends neither its key nor the text it would type.
		opts.OnKey = func(mods ui.Modifiers, key ui.Key) bool {
			layer, ok := chordKeys[key]
			if !ok || mods&^ui.Shift != ui.Alt {
				return false
			}
			p.record(p.rec.Press(layer, time.Now()), layer)
			if p.win != nil {
				p.win.Invalidate()
			}
			return true
		}
	}
	term, err := terminal.New(opts)
	if err != nil {
		return nil, err
	}
	created := make(chan *mygo.Window, 1)
	mygo.RunOnMain(func() {
		created <- mygo.NewWindow(mygo.WindowOptions{Title: "Agent Remote R0 Probe", Width: 1100, Height: 700,
			AutoHideMenuBar: true, Content: ui.View(func(c *ui.Context) { p.view(c, term) })})
	})
	win := <-created
	p.win = win
	defer mygo.RunOnMain(func() { win.Hide() })
	prog.out <- []byte("r0 chord probe (" + route + " route): the program behind this terminal records every byte it receives\r\n")
	rep := &chordReport{Route: route}
	rep.Focused = waitFocused(win, 5*time.Second)
	if manual {
		manualChord(r, p, prog)
		return nil, nil
	}
	inj, err := newInjector(win)
	if err != nil {
		r.Status, r.Reason = "not_run", "injector: "+err.Error()
		return nil, nil
	}
	r.Injection = describe(inj)
	scenario(r, rep, p, prog, inj)
	if r.Status == "not_run" {
		return nil, nil
	}
	rep.TerminalFocus = p.termFocus.Load()
	rep.DoubleDispatch = int(p.doubles.Load())
	if outDir != "" {
		if png, err := win.CapturePage(); err == nil {
			name := outDir + "/chord-" + r.GOOS + "-" + route + ".png"
			if os.WriteFile(name, png, 0o644) == nil {
				r.Captures = append(r.Captures, name)
			}
		}
	}
	return rep, nil
}

func scenario(r *report, rep *chordReport, p *chordProbe, prog *program, inj injector) {
	gap := func() { time.Sleep(45 * time.Millisecond) }
	tap := func(k rune) { inj.down(k); gap(); inj.up(k); gap() }
	typeText := func(s string) {
		for _, k := range s {
			tap(k)
		}
	}
	altDown := func() { inj.down(altKey); gap() }
	altUp := func() {
		p.altUpAt.Store(time.Now().UnixNano())
		inj.up(altKey)
		// A flagsChanged event may not schedule a redraw when the terminal is
		// otherwise idle; force the observation frame so release commits are
		// deterministic in the probe and in the native workbench.
		if p.win != nil {
			mygo.RunOnMain(func() { p.win.Invalidate() })
		}
	}
	settle := func() { time.Sleep(350 * time.Millisecond) }
	reset := func() {
		mygo.RunOnMain(func() { p.rec.Cancel() })
		time.Sleep(60 * time.Millisecond)
		p.take()
	}

	ic, hasIME := inj.(imeControl)
	if hasIME {
		was, err := ic.setIME(false)
		rep.IME = &imeReport{InitiallyOpen: was}
		if err != nil {
			rep.IME.Error = err.Error()
		}
		time.Sleep(100 * time.Millisecond)
	}
	typeText("hello world")
	time.Sleep(300 * time.Millisecond)
	if len(prog.Received()) == 0 {
		r.Status, r.Reason = "not_run", "injected key events were not delivered to the window (input permission or focus)"
		return
	}
	steps := []struct {
		name   string
		run    func()
		want   []string
		marker string
	}{
		{"plain_typing", func() {}, nil, ""},
		{"alt_a_tap", func() { altDown(); tap('a'); altUp() }, []string{"Opened:agents", "MadeSticky:agents"}, "q"},
		{"alt_c_hold", func() { altDown(); tap('c'); time.Sleep(400 * time.Millisecond); altUp() }, []string{"Opened:create", "Committed:create"}, "w"},
		{"alt_a_cycle", func() { altDown(); tap('a'); tap('a'); tap('a'); altUp() }, []string{"Opened:agents", "Cycled:agents", "Cycled:agents", "Committed:agents"}, "e"},
		{"alt_a_then_x", func() { altDown(); tap('a'); tap('x'); time.Sleep(400 * time.Millisecond); altUp() }, []string{"Opened:agents", "Retargeted:projects", "Committed:projects"}, "r"},
		{"alt_s_tap", func() { altDown(); tap('s'); altUp() }, []string{"Opened:cc", "MadeSticky:cc"}, "t"},
	}
	core := "hello world"
	pass := true
	for _, s := range steps {
		s.run()
		settle()
		got := p.take()
		ok := slices.Equal(got, s.want)
		pass = pass && ok
		rep.Steps = append(rep.Steps, stepResult{Name: s.name, Want: nonNil(s.want), Got: nonNil(got), Pass: ok})
		reset()
		typeText(s.marker)
		core += s.marker
	}
	// Alt pressed and released alone, then a plain key: on Windows this can
	// enter menu mode and swallow the key. Observation only, not a failure.
	altDown()
	altUp()
	time.Sleep(150 * time.Millisecond)
	tap('y')
	time.Sleep(400 * time.Millisecond)
	p.take()

	received := string(prog.Received())
	rep.Expected, rep.Received = core, received
	rest, ok := strings.CutPrefix(received, core)
	switch {
	case ok && rest == "y":
		rep.AltTapThenKey = "reached"
	case ok && rest == "":
		rep.AltTapThenKey = "swallowed"
	default:
		rep.Leaked = true
	}
	p.mu.Lock()
	rep.ReleaseAfterMs = append([]float64(nil), p.latencies...)
	p.mu.Unlock()
	rep.Passed = pass && !rep.Leaked
	if hasIME && rep.IME.Error == "" {
		// With the IME open and nothing composing, a chord must still reach
		// the app and send nothing; typing then composes as usual.
		_, _ = ic.setIME(true)
		time.Sleep(150 * time.Millisecond)
		before := len(prog.Received())
		altDown()
		tap('a')
		altUp()
		settle()
		rep.IME.ChordEvents = nonNil(p.take())
		mid := len(prog.Received())
		rep.IME.ChordBytes = string(prog.Received()[before:mid])
		mygo.RunOnMain(func() { p.rec.Cancel() })
		typeText("ni ")
		time.Sleep(500 * time.Millisecond)
		rep.IME.ComposedBytes = string(prog.Received()[mid:])
		_, _ = ic.setIME(rep.IME.InitiallyOpen)
	}
}

// manualChord lets a person press the keys when synthetic input is
// unavailable. It records what happened for 60 s without asserting.
func manualChord(r *report, p *chordProbe, prog *program) {
	fmt.Fprintln(os.Stderr, "r0-probe manual: type hello, then ⌥/Alt+A tap, ⌥C hold 0.5s, ⌥A A A, ⌥A then X; 60 s")
	prog.out <- []byte("\r\nmanual: type hello, then Alt+A tap, Alt+C hold, Alt+A A A, Alt+A then X (60 s)\r\n")
	time.Sleep(60 * time.Second)
	r.Status, r.Reason = "not_run", "manual session: inspect extra.events and received bytes"
	r.Extra = map[string]any{"events": p.take(), "received": string(prog.Received())}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
