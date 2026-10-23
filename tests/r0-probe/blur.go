package main

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// blurVariant summarises frames logged by MYGO_FRAME_STATS=all while one
// overlay variant was shown over a terminal streaming ~400 lines/s.
type blurVariant struct {
	Name    string  `json:"name"`
	Frames  int     `json:"frames"`
	FPS     float64 `json:"fps"`
	P50Ms   float64 `json:"p50_ms"`
	P95Ms   float64 `json:"p95_ms"`
	MaxMs   float64 `json:"max_ms"`
	PaintMs float64 `json:"paint_p95_ms"`
	Path    string  `json:"path"`
}

type frameSample struct {
	variant      int32
	total, paint float64
	path         string
}

// frameLog receives the engine's frame log lines (log.SetOutput).
type frameLog struct {
	mu      sync.Mutex
	variant atomic.Int32 // -1 while warming up between variants
	samples []frameSample
}

func newFrameLog() *frameLog { f := &frameLog{}; f.variant.Store(-1); return f }

var frameLine = regexp.MustCompile(`took ([0-9.]+) ms: build [0-9.]+[^,]*, layout [0-9.]+, paint ([0-9.]+), present [0-9.]+ \(([^)]*)\)`)

func (f *frameLog) Write(b []byte) (int, error) {
	if m := frameLine.FindSubmatch(b); m != nil {
		total, _ := strconv.ParseFloat(string(m[1]), 64)
		paint, _ := strconv.ParseFloat(string(m[2]), 64)
		f.mu.Lock()
		f.samples = append(f.samples, frameSample{f.variant.Load(), total, paint, string(m[3])})
		f.mu.Unlock()
	}
	return len(b), nil
}

var variants = []string{"none", "scrim_scale", "blur18", "blur30"}

func runBlur(r *report, frames *frameLog, per time.Duration, outDir string, done chan struct{}) error {
	prog := newProgram()
	term, err := terminal.New(terminal.Options{Conn: prog, Font: terminal.Font{Size: 14}})
	if err != nil {
		return err
	}
	var shown atomic.Int32
	win := mygo.NewWindow(mygo.WindowOptions{Title: "Agent Remote R0 Probe", Width: 1366, Height: 820,
		AutoHideMenuBar: true, Content: ui.View(func(c *ui.Context) { blurView(c, term, variants[shown.Load()]) })})
	stop := make(chan struct{})
	go stream(prog, stop)
	go func() {
		defer close(done)
		defer prog.Close()
		defer close(stop)
		waitFocused(win, 3*time.Second)
		for i := range variants {
			frames.variant.Store(-1)
			shown.Store(int32(i))
			mygo.RunOnMain(win.Invalidate)
			time.Sleep(400 * time.Millisecond) // warm-up: transition and caches
			frames.variant.Store(int32(i))
			start := time.Now()
			time.Sleep(per / 2)
			if outDir != "" {
				if png, err := win.CapturePage(); err == nil {
					name := fmt.Sprintf("%s/blur-%s-%s.png", outDir, r.GOOS, variants[i])
					if os.WriteFile(name, png, 0o644) == nil {
						r.Captures = append(r.Captures, name)
					}
				}
			}
			time.Sleep(per - time.Since(start))
			frames.variant.Store(-1)
			r.Blur = append(r.Blur, summarise(frames, int32(i), time.Since(start)))
		}
		r.Status = "passed"
		for _, v := range r.Blur {
			if v.Frames == 0 {
				r.Status, r.Reason = "failed", "no frames logged for "+v.Name+" (MYGO_FRAME_STATS not active?)"
			}
		}
	}()
	return nil
}

// stream writes Agent-like coloured output so the terminal repaints every frame.
func stream(p *program, stop chan struct{}) {
	t := time.NewTicker(10 * time.Millisecond)
	defer t.Stop()
	for n := 0; ; n++ {
		select {
		case <-stop:
			return
		case <-t.C:
			line := fmt.Sprintf("\x1b[32m●\x1b[0m Update(internal/app/zen.go:%d) \x1b[2m⎿\x1b[0m \x1b[35mfunc\x1b[0m (a *App) \x1b[36mzenView\x1b[0m(c *ui.Context) // %06d\r\n"+
				"  \x1b[2m⎿\x1b[0m  ok  github.com/Agent-Remote/agent-remote-desktop/tests/app  %d.%02ds\r\n"+
				"\x1b[33m›\x1b[0m streaming output line %06d …………………………………………………………\r\n"+
				"\x1b[34m•\x1b[0m Edited locales/zh-Hans.json (+%d -0) 中文与 emoji ✦ 混排\r\n", n, n, n%9, n%100, n, n%40)
			select {
			case p.out <- []byte(line):
			default:
			}
		}
	}
}

func summarise(f *frameLog, v int32, elapsed time.Duration) blurVariant {
	f.mu.Lock()
	var totals, paints []float64
	path := ""
	for _, s := range f.samples {
		if s.variant == v {
			totals, paints = append(totals, s.total), append(paints, s.paint)
			path = s.path
		}
	}
	f.mu.Unlock()
	out := blurVariant{Name: variants[v], Frames: len(totals), Path: path}
	if len(totals) == 0 {
		return out
	}
	out.FPS = round(float64(len(totals)) / elapsed.Seconds())
	out.P50Ms, out.P95Ms, out.MaxMs = pct(totals, 50), pct(totals, 95), pct(totals, 100)
	out.PaintMs = pct(paints, 95)
	return out
}

func pct(v []float64, p int) float64 {
	s := slices.Clone(v)
	slices.Sort(s)
	i := (len(s)*p+99)/100 - 1
	return round(s[max(0, min(i, len(s)-1))])
}

func round(x float64) float64 { return float64(int(x*100+0.5)) / 100 }

// blurView is the Layer 1.2 composition: the workspace pushed back (scale,
// dim, optional blur), a scrim, the 4-sector wheel and a glass HUD.
func blurView(c *ui.Context, term *terminal.Terminal, variant string) {
	ui.Box(c).Fill().Background(ui.Hex("#09090B")).Children(func() {
		ws := ui.Column(c.Key("ws")).Absolute().Fill().Padding(14).Children(func() {
			terminal.View(c.Key("term"), term).Grow(1).FillWidth()
		})
		if variant != "none" {
			ws.Scale(0.965, 0.965).Opacity(0.65)
			if variant == "blur18" {
				ws.Blur(18) // CSS blur(18px) is σ=18, as mygo Blur(sigma)
			}
			if variant == "blur30" {
				ws.Blur(30)
			}
			ui.Box(c.Key("scrim")).Absolute().Fill().Background(ui.RGBA(4, 4, 6, 0.55)).PassThrough()
			ui.Column(c.Key("stage")).Absolute().Fill().Center().PassThrough().Children(func() {
				ui.Box(c.Key("wheel")).Size(490, 490).Draw(drawWheel).Children(func() {
					ui.Column(c.Key("hud")).Absolute().Left(138).Top(138).Size(214, 214).Radius(107).
						Material(glass.Blur{Radius: 25}).Background(ui.RGBA(38, 39, 46, 0.84)).
						Border(1, ui.RGBA(255, 255, 255, 0.11)).
						InsetShadow(0, 1, 0.5, 0, ui.RGBA(255, 255, 255, 0.28)).
						Center().Gap(6).Children(func() {
						ui.Text(c, "⌥C").FontSize(10).Bold().TextColor(ui.Hex("#6E6E73"))
						ui.Text(c, "Codex").FontSize(18).Bold().TextColor(ui.Hex("#34D399"))
						ui.Text(c, variant).FontSize(11).TextColor(ui.Hex("#A1A1A6"))
					})
				})
			})
		}
	})
}

func drawWheel(p *ui.Painter, r ui.Rect) {
	w := chrome.CreateWheel
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	accents := []ui.Color{ui.Hex("#E88267"), ui.Hex("#34D399"), ui.Hex("#47A1FF"), ui.Hex("#B06BF9")}
	for i := 0; i < w.Count; i++ {
		outer, inner := w.SectorOutline(i)
		var path ui.Path
		path.MoveTo(cx+outer[0].P0.X, cy+outer[0].P0.Y)
		for _, s := range outer {
			path.CubeTo(cx+s.C1.X, cy+s.C1.Y, cx+s.C2.X, cy+s.C2.Y, cx+s.P3.X, cy+s.P3.Y)
		}
		path.LineTo(cx+inner[0].P0.X, cy+inner[0].P0.Y)
		for _, s := range inner {
			path.CubeTo(cx+s.C1.X, cy+s.C1.Y, cx+s.C2.X, cy+s.C2.Y, cx+s.P3.X, cy+s.P3.Y)
		}
		path.Close()
		fill, stroke, width := ui.RGBA(30, 31, 37, 0.62), ui.RGBA(255, 255, 255, 0.14), float32(1.35)
		if i == 1 { // hovered sector, as in prototype-v4 02-create-hover-codex
			fill, stroke, width = ui.RGBA(52, 54, 65, 0.85), accents[i], 1.8
		}
		p.FillPath(&path, fill)
		p.StrokePath(&path, width, stroke)
	}
}
