package chrome_test

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/Agent-Remote/agent-remote-desktop/internal/chrome"
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

// gallery lays every R1 component out once, over coloured stripes so glass
// translucency (and its absence under reduced transparency) is visible.
func gallery(env chrome.Env) func(c *ui.Context) {
	choice := 1
	return func(c *ui.Context) {
		t := theme.UI(env.P.ID)
		c.SetTheme(&t)
		p := env.P
		ui.Box(c).Fill().Background(p.Canvas).Children(func() {
			for i, col := range []ui.Color{p.AccentClaude, p.AccentCodex, p.AccentGemini, p.AccentCustom} {
				ui.Box(c.Key(fmt.Sprint("stripe", i))).Absolute().Top(float32(40+i*130)).Left(0).Right(0).Height(46).Background(col.Alpha(0.55))
			}
			ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(24).Gap(24).Children(func() {
				ui.Column(c).Width(520).Gap(16).Children(func() {
					env.Text(c, "Agent Remote", theme.SheetTitle, p.TextPrimary)
					ui.Row(c).Gap(8).Children(func() {
						env.Button(c, "Cancel", chrome.Secondary)
						env.Button(c, "Confirm pause", chrome.Primary)
						env.Button(c, "Stop session", chrome.Danger)
					})
					env.Segmented(c, &choice, "Theme", "System", "Light", "Dark", "Kawaii")
					ui.Row(c).Gap(6).Wrap().Children(func() {
						for i, s := range []chrome.Status{chrome.StatusOutput, chrome.StatusIdle, chrome.StatusConnected, chrome.StatusDisconnected} {
							st := s
							env.Pill(c.Key(i), &st, []string{"Output", "Idle", "Connected", "Disconnected"}[i])
						}
					})
					ui.Row(c).Gap(6).Wrap().Children(func() {
						env.ContextPill(c, "server", "dev.agent-remote.test")
						env.ContextPill(c, "device", "mac-rem-01")
					})
					env.Panel(c, "Stages", false, func() {
						env.Stages(c, []chrome.Stage{
							{Name: "Server record", Note: "done", State: chrome.StageDone},
							{Name: "Local Mutagen", Note: "running", State: chrome.StageRunning},
							{Name: "Shared SQLite", Note: "unknown", State: chrome.StageUnknown},
							{Name: "Receipt", Note: "waiting", State: chrome.StageWaiting},
						})
					})
					env.Panel(c, "Review · pause sync", true, func() {
						env.KV(c, [][2]string{{"sync_session_id", "sync_7f2c3e1b9a0d4c6e8f2a1b3c5d7e9f01"}, {"fingerprint", "fp_3b8d02c4"}})
					})
					env.Banner(c, "Result unknown: the local stage returned no receipt.", func() { env.Button(c, "Query original", chrome.Secondary) })
				})
				accent := p.AccentCodex
				sectors := []chrome.Sector{{Enabled: true, Accent: &p.AccentClaude}, {Enabled: true, Accent: &accent}, {Enabled: true, Accent: &p.AccentGemini}, {Enabled: false}}
				env.WheelView(c, chrome.CreateWheel, sectors, 1, false, func(i int) {
					pt := chrome.CreateWheel.LabelCenter(i)
					ui.Box(c.Key(fmt.Sprint("label", i))).Absolute().Left(chrome.WheelSize/2+pt.X-40).Top(chrome.WheelSize/2+pt.Y-10).Width(80).PassThrough().Children(func() {
						env.Text(c, []string{"Claude", "Codex", "fclaude", "—"}[i], theme.BodyStrong, p.TextPrimary).TextAlign(ui.Center).FillWidth()
					})
				}, 214, func() {
					env.HUD(c, 214, func() {
						env.HUDTag(c, "⌥C")
						env.HUDTitle(c, "Codex", &accent)
						env.HUDPath(c, "~/Documents/Git/agent-remote")
					})
				})
			})
		})
	}
}

var variants = map[string]theme.Surfaces{
	"default":              {},
	"reduced-transparency": {ReduceTransparency: true},
	"high-contrast":        {ReduceTransparency: true, HighContrast: true},
}

// TestGalleryBaselines compares the gallery with reviewed PNGs per OS
// (font rasterizers differ between platforms). DESKTOP_UPDATE_BASELINES=1
// writes them; a missing baseline for this OS skips rather than passes.
func TestGalleryBaselines(t *testing.T) {
	for _, id := range theme.ThemeNames {
		for name, s := range variants {
			t.Run(id+"-"+name, func(t *testing.T) {
				env := chrome.Env{P: theme.PaletteFor(id), S: s}
				tt := ui.NewTester(gallery(env), 1100, 620)
				tt.SetDark(env.P.Dark)
				tt.Frame()
				compareBaseline(t, fmt.Sprintf("gallery-%s-%s", id, name), tt.Image())
			})
		}
	}
}

// Reduced motion is visible at a fixed instant: 60 ms after asking a sheet
// to open, the spring has it part-way up; under reduced motion it is there.
func TestSheetMotionBaselines(t *testing.T) {
	for name, reduced := range map[string]bool{"normal": false, "reduced-motion": true} {
		t.Run(name, func(t *testing.T) {
			env := chrome.Env{P: theme.PaletteFor("midnight"), S: theme.Surfaces{ReduceMotion: reduced}}
			var sheet chrome.SheetState
			view := func(c *ui.Context) {
				t := theme.UI(env.P.ID)
				c.SetTheme(&t)
				ui.Box(c).Fill().Background(env.P.Canvas).Children(func() {
					env.Sheet(c, &sheet, 600, 360, nil, func() {
						ui.Column(c).Padding(0, 28).Children(func() { env.Text(c, "Control Center", theme.SheetTitle, env.P.TextPrimary) })
					})
				})
			}
			tt := ui.NewTester(view, 800, 500)
			tt.SetDark(true)
			tt.Frame()
			sheet.Open()
			tt.Frame()
			tt.Advance(60 * time.Millisecond)
			compareBaseline(t, "sheet-60ms-"+name, tt.Image())
		})
	}
}

func compareBaseline(t *testing.T, name string, got *image.RGBA) {
	t.Helper()
	path := filepath.Join("testdata", runtime.GOOS, name+".png")
	if os.Getenv("DESKTOP_UPDATE_BASELINES") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		f, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, got); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		t.Skipf("no %s baseline for %s; review and record with DESKTOP_UPDATE_BASELINES=1", runtime.GOOS, name)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	want, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if want.Bounds() != got.Bounds() {
		t.Fatalf("%s: size %v, baseline %v", name, got.Bounds(), want.Bounds())
	}
	// Tolerate antialiasing noise: at most 0.5% of pixels may differ, and by
	// no more than 24/255 in any channel, so a changed colour or layout fails.
	differ, worst := 0, 0
	b := got.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r1, g1, b1, _ := got.At(x, y).RGBA()
			r2, g2, b2, _ := want.At(x, y).RGBA()
			d := max(absDiff(r1, r2), absDiff(g1, g2), absDiff(b1, b2)) >> 8
			if d > 2 {
				differ++
				worst = max(worst, int(d))
			}
		}
	}
	if total := b.Dx() * b.Dy(); differ*200 > total || worst > 24 && differ*1000 > total {
		t.Fatalf("%s differs from baseline: %d of %d pixels, worst %d/255", name, differ, total, worst)
	}
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
