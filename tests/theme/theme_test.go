package theme_test

import (
	"math"
	"testing"

	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

func luminance(c ui.Color) float64 {
	ch := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(c.R) + 0.7152*ch(c.G) + 0.0722*ch(c.B)
}

// contrast is the WCAG 2 contrast ratio of two opaque colours.
func contrast(a, b ui.Color) float64 {
	x, y := luminance(a), luminance(b)
	return (math.Max(x, y) + 0.05) / (math.Min(x, y) + 0.05)
}

func TestThemeCycleIncludesAllBuiltIns(t *testing.T) {
	for from, want := range map[string]string{"midnight": "sakura", "sakura": "daybreak", "daybreak": "midnight", "legacy-theme": "midnight"} {
		if got := theme.Next(from); got != want {
			t.Fatalf("Next(%q) = %q, want %q", from, got, want)
		}
	}
}

// The v3 baseline must stay verbatim (prototype-v3.html / v4 tokens.css §1).
func TestPalettesKeepPrototypeV3Values(t *testing.T) {
	for id, want := range map[string][3]string{
		"daybreak": {"#F5F5F7", "#1D1D1F", "#6E6E73"},
		"midnight": {"#09090B", "#F5F5F7", "#A1A1A6"},
		"sakura":   {"#FFF8FA", "#3B2838", "#8A6D83"},
	} {
		p := theme.PaletteFor(id)
		if p.Canvas != ui.Hex(want[0]) || p.TextPrimary != ui.Hex(want[1]) || p.TextSecondary != ui.Hex(want[2]) {
			t.Fatalf("%s drifted from prototype-v3: canvas %v text %v secondary %v", id, p.Canvas, p.TextPrimary, p.TextSecondary)
		}
	}
	if !theme.PaletteFor("sakura").Decorations || theme.PaletteFor("midnight").Decorations || theme.PaletteFor("daybreak").Decorations {
		t.Fatal("kawaii decorations must be on for sakura only")
	}
}

// Doc §5.2: every text/background pair the tokens promise as readable.
func TestPaletteTextContrast(t *testing.T) {
	white := ui.Hex("#FFFFFF")
	for _, id := range theme.ThemeNames {
		p := theme.PaletteFor(id)
		for name, pair := range map[string][2]ui.Color{
			"text/canvas":          {p.TextPrimary, p.Canvas},
			"text/terminal":        {p.TextPrimary, p.TerminalBG},
			"ink/canvas":           {p.AccentInk, p.Canvas},
			"ink/selected":         {p.AccentInk, theme.Over(p.Selected(), p.Canvas)},
			"text/selected":        {p.TextPrimary, theme.Over(p.Selected(), p.Canvas)},
			"accentText/fill":      {p.AccentText, p.AccentFill},
			"white/dangerFill":     {white, p.DangerFill},
			"warning/canvas":       {p.Warning, p.Canvas},
			"text/solid":           {p.TextPrimary, p.Solid},
			"text/solidRaised":     {p.TextPrimary, p.SolidRaised},
			"cursorText/cursor":    {theme.Terminal(id).CursorText, theme.Terminal(id).Cursor},
			"selectionText/select": {theme.Terminal(id).SelectionText, theme.Terminal(id).Selection},
		} {
			if r := contrast(pair[0], pair[1]); r < 4.5 {
				t.Errorf("%s %s: %.2f:1 < 4.5", id, name, r)
			}
		}
		// Secondary text: kawaii's v3 value is 4.36 (doc §5.2 documents the
		// one permitted adjustment); the others must clear 4.5.
		min := 4.5
		if id == "sakura" {
			min = 4.3
		}
		if r := contrast(p.TextSecondary, p.Canvas); r < min {
			t.Errorf("%s secondary/canvas %.2f:1 < %.1f", id, r, min)
		}
		// Focus rings and outlines are non-text: 3:1 (WCAG 1.4.11).
		if r := contrast(p.Accent, p.Canvas); r < 3 {
			t.Errorf("%s accent/canvas %.2f:1 < 3", id, r)
		}
	}
}

func TestResolveFollowsSystemButKeepsKawaiiExplicit(t *testing.T) {
	for _, tc := range []struct {
		stored             string
		follow, systemDark bool
		want               string
	}{
		{"sakura", true, true, "midnight"},
		{"sakura", true, false, "daybreak"},
		{"sakura", false, true, "sakura"},
		{"midnight", false, false, "midnight"},
		{"unknown", false, false, "midnight"},
	} {
		if got := theme.Resolve(tc.stored, tc.follow, tc.systemDark); got != tc.want {
			t.Fatalf("Resolve(%q, %v, %v) = %q, want %q", tc.stored, tc.follow, tc.systemDark, got, tc.want)
		}
	}
}

func TestTerminalPairHandsBothAppearancesWhileFollowing(t *testing.T) {
	base, darkVariant := theme.TerminalPair("sakura", true)
	if base == nil || darkVariant == nil || base.Background != theme.PaletteFor("daybreak").TerminalBG || darkVariant.Background != theme.PaletteFor("midnight").TerminalBG {
		t.Fatalf("following pair = %+v / %+v", base, darkVariant)
	}
	base, darkVariant = theme.TerminalPair("sakura", false)
	if darkVariant != nil || base.Background != theme.PaletteFor("sakura").TerminalBG {
		t.Fatalf("explicit pair = %+v / %+v", base, darkVariant)
	}
}

func TestEveryBuiltInTerminalIsOpaqueAndMatchesItsPane(t *testing.T) {
	for _, name := range theme.ThemeNames {
		p, term, u := theme.PaletteFor(name), theme.Terminal(name), theme.UI(name)
		if term.Background != p.TerminalBG || term.Background.A != 255 || term.Foreground != u.Text {
			t.Fatalf("%s terminal base colours: %+v", name, term)
		}
		if u.Background != p.Canvas || u.Accent != p.AccentFill || u.Danger != p.Danger || u.Dark != p.Dark {
			t.Fatalf("%s mygo theme does not follow the palette", name)
		}
	}
}

func TestToolAccentsMapRealProviders(t *testing.T) {
	p := theme.PaletteFor("midnight")
	if p.ToolAccent("claude") != p.AccentClaude || p.ToolAccent("codex") != p.AccentCodex ||
		p.ToolAccent("fclaude") != p.AccentGemini || p.ToolAccent("other") != p.AccentCustom {
		t.Fatal("tool accents do not follow doc §5.2")
	}
}
