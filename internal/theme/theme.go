// Package theme holds the redesign's design tokens (light / dark / kawaii,
// stored as daybreak / midnight / sakura), the motion policy and the mapping
// onto mygo's ui.Theme and the terminal. Terminal colours stay restrained for
// long sessions even when the surrounding workbench uses the kawaii palette.
package theme

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// ThemeNames is the stable order used by the toolbar's quick theme switcher.
// Settings still exposes each name directly; keeping the cycle here avoids a
// second implementation that could accidentally skip the anime theme.
var ThemeNames = [...]string{"midnight", "sakura", "daybreak"}

// Next returns the next built-in theme in the user-facing cycle. Unknown or
// legacy values recover to the first theme so a malformed preference never
// leaves the workbench without a predictable quick-switch result.
func Next(name string) string {
	if len(ThemeNames) == 0 {
		return "midnight"
	}
	for i, candidate := range ThemeNames {
		if candidate == name {
			return ThemeNames[(i+1)%len(ThemeNames)]
		}
	}
	return ThemeNames[0]
}

// UI returns the mygo theme built from a palette, so mygo's own widgets
// (buttons, inputs, checkboxes, scroll bars) follow the redesign tokens.
// Surface tokens are the v3 glass colours flattened onto Canvas: widgets
// draw them opaque, and true translucency is left to chrome surfaces.
func UI(name string) ui.Theme {
	p := PaletteFor(name)
	t := *ui.LightTheme()
	if p.Dark {
		t = *ui.DarkTheme()
	}
	t.Background = p.Canvas
	t.Surface = Over(p.GlassCard, p.Canvas)
	t.SurfaceHover = Over(p.GlassCardHover, p.Canvas)
	t.SurfacePressed = t.Surface.Mix(p.TextPrimary, 0.08)
	t.Text = p.TextPrimary
	t.TextMuted = p.TextSecondary
	t.Border = Over(p.Hairline, p.Canvas)
	t.Accent = p.AccentFill
	t.AccentText = p.AccentText
	if p.Dark {
		t.AccentHover, t.AccentPressed = p.AccentFill.Mix(ui.Hex("#FFFFFF"), 0.12), p.AccentFill.Mix(ui.Hex("#000000"), 0.15)
	} else {
		t.AccentHover, t.AccentPressed = p.AccentFill.Mix(ui.Hex("#000000"), 0.1), p.AccentFill.Mix(ui.Hex("#000000"), 0.22)
	}
	t.Selection = p.Accent.Alpha(0.28)
	t.Focus = p.Accent
	t.Danger, t.Warning, t.Success = p.Danger, p.Warning, p.Success
	t.FontSize = 14
	t.Radius = 10
	t.Spacing = 4
	return t
}

// Terminal returns the terminal colours for a palette. The pane background
// is the opaque TerminalBG; ANSI colours stay mygo's defaults so programs'
// semantic colours are not re-tinted by the decorative palette.
func Terminal(name string) *terminal.Theme {
	p := PaletteFor(name)
	t := terminal.LightTheme()
	if p.Dark {
		t = terminal.DarkTheme()
	}
	t.Background = p.TerminalBG
	t.Foreground = p.TextPrimary
	t.Cursor = p.AccentFill
	t.CursorText = p.AccentText
	t.Selection = Over(p.Accent.Alpha(0.28), p.TerminalBG)
	t.SelectionText = p.TextPrimary
	return t
}

// TerminalPair returns the themes for Terminal.SetTheme: following the
// system it hands the terminal both appearances, so it switches with the
// system even while no view is built; otherwise the stored choice alone.
func TerminalPair(stored string, followSystem bool) (base, darkVariant *terminal.Theme) {
	if followSystem {
		return Terminal(light.ID), Terminal(dark.ID)
	}
	return Terminal(stored), nil
}
