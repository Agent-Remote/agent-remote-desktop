// Package theme defines the workbench's coordinated native themes and light
// decorative artwork. Terminal colors stay deliberately restrained for long
// sessions even when the surrounding workbench uses an anime accent palette.
package theme

import (
	"github.com/egoist/mygo/plugins/terminal"
	"github.com/egoist/mygo/ui"
)

// ThemeNames is the stable order used by the toolbar's quick theme switcher.
// Settings still exposes each name directly; keeping the cycle here avoids a
// second implementation that could accidentally skip the anime theme.
var ThemeNames = [...]string{"midnight", "sakura", "daybreak"}

// PortraitMinWindowWidth is the minimum window width at which decorative
// character art can coexist with the workbench navigation and project rows.
// Keeping this policy in the theme package makes the responsive rule explicit
// and testable rather than coupling it to one sidebar layout.
const PortraitMinWindowWidth float32 = 1100

// PortraitVisible reports whether the optional character art should be shown.
// Narrow windows and split panes reserve every available pixel for navigation
// and terminal content; focus mode also removes all decorative artwork.
func PortraitVisible(windowWidth float32, focusMode, splitMode bool) bool {
	return windowWidth >= PortraitMinWindowWidth && !focusMode && !splitMode
}

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

func UI(name string) ui.Theme {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#111721")
	t.Surface = ui.Hex("#1c2635")
	t.SurfaceHover = ui.Hex("#273549")
	t.SurfacePressed = ui.Hex("#32455e")
	t.Text = ui.Hex("#e8eef8")
	t.TextMuted = ui.Hex("#9dacc2")
	t.Border = ui.Hex("#2b394c")
	t.Accent = ui.Hex("#a3caed")
	t.AccentText = ui.Hex("#142239")
	t.AccentHover = ui.Hex("#bcddf7")
	t.AccentPressed = ui.Hex("#8cb5db")
	t.Selection = ui.Hex("#364e6b")
	t.Focus = t.Accent
	t.Danger = ui.Hex("#f5a6b2")
	if name == "daybreak" {
		t = *ui.LightTheme()
		t.Background = ui.Hex("#f5f2ed")
		t.Surface = ui.Hex("#eae7e2")
		t.SurfaceHover = ui.Hex("#dddfe4")
		t.SurfacePressed = ui.Hex("#d3d9e1")
		t.Text = ui.Hex("#25354a")
		t.TextMuted = ui.Hex("#596b80")
		t.Border = ui.Hex("#d6dce3")
		t.Accent = ui.Hex("#355e85")
		t.AccentText = ui.Hex("#ffffff")
		t.AccentHover = ui.Hex("#416f99")
		t.AccentPressed = ui.Hex("#294c6e")
		t.Selection = ui.Hex("#ceddeb")
		t.Focus = t.Accent
	}
	if name == "sakura" {
		t = *ui.DarkTheme()
		t.Background = ui.Hex("#1c1725")
		t.Surface = ui.Hex("#2a2035")
		t.SurfaceHover = ui.Hex("#3a2a48")
		t.SurfacePressed = ui.Hex("#49345a")
		t.Text = ui.Hex("#f8eef7")
		t.TextMuted = ui.Hex("#c4abc2")
		t.Border = ui.Hex("#543d5b")
		t.Accent = ui.Hex("#f0a8c7")
		t.AccentText = ui.Hex("#321b32")
		t.AccentHover = ui.Hex("#f6bfd7")
		t.AccentPressed = ui.Hex("#d987ae")
		t.Selection = ui.Hex("#5d3b62")
		t.Focus = t.Accent
		t.Danger = ui.Hex("#ff9eaa")
	}
	t.FontSize = 14
	t.Radius = 9
	t.Spacing = 4
	return t
}

func Terminal(name string) *terminal.Theme {
	t := terminal.DarkTheme()
	if name == "daybreak" {
		t = terminal.LightTheme()
	}
	u := UI(name)
	t.Background = u.Background
	t.Foreground = u.Text
	// Keep the surrounding anime palette out of terminal glyph colors while
	// making cursor and selection state visible in every built-in theme. The
	// foreground is used for selected text to preserve contrast in Daybreak's
	// pale selection field.
	t.Cursor = u.Accent
	t.CursorText = u.AccentText
	t.Selection = u.Selection
	t.SelectionText = u.Text
	return t
}

var sakuraPortrait = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 180 120">
<defs><linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop stop-color="#f0a8c7" stop-opacity=".35"/><stop offset="1" stop-color="#8f7ade" stop-opacity=".15"/></linearGradient></defs>
<path d="M0 0h180v120H0z" fill="url(#bg)"/><circle cx="142" cy="32" r="22" fill="#f9d7e8" fill-opacity=".9"/>
<path d="M53 108c2-28 15-45 38-45s36 17 38 45" fill="#6c4a91"/><path d="M68 61c-5-28 6-46 27-46s31 18 26 46c-6 10-14 15-26 15S74 71 68 61z" fill="#f6c4c9"/>
<path d="M63 43c0-25 11-38 31-38 19 0 31 13 31 38-8-8-14-11-23-11-14 0-24 5-39 11z" fill="#32223f"/><circle cx="83" cy="52" r="3" fill="#32223f"/><circle cx="106" cy="52" r="3" fill="#32223f"/>
<path d="M89 63c5 4 10 4 15 0" fill="none" stroke="#b25f87" stroke-width="2" stroke-linecap="round"/><path d="M20 22c8-7 13-6 19 1-7 7-13 7-19-1zm129 73c8-7 13-6 19 1-7 7-13 7-19-1z" fill="#f0a8c7"/>
</svg>`))

// Portrait returns the optional decorative character art for the selected
// theme. Keeping this as a vector keeps the binary portable across all three
// release platforms and avoids reducing terminal text contrast.
func Portrait(name string) *ui.SVG {
	if name == "sakura" {
		return sakuraPortrait
	}
	return nil
}
