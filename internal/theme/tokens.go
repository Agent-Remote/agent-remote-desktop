package theme

import "github.com/egoist/mygo/ui"

// Palette is one complete set of design tokens of the redesign (agent-remote
// docs/agent-remote-desktop-redesign.zh-CN.md §5). Values in the first group
// copy prototype-v3 verbatim; the second group are the v4 supplements, which
// were contrast-checked against Canvas. Change a v3 value only together with
// prototype-v3.html, and never introduce a colour here that is not in v4's
// tokens.css.
type Palette struct {
	ID   string // stable storage identifier: daybreak | midnight | sakura
	Dark bool

	// ---- v3 baseline ----
	Canvas                              ui.Color
	Mesh1, Mesh2, Mesh3                 ui.Color
	TextPrimary, TextSecondary, TextTer ui.Color
	GlassDock, GlassHUD                 Fill
	GlassCard, GlassCardHover           ui.Color
	SectorFill, SectorFillHover         ui.Color
	SectorStroke, SectorStrokeHover     ui.Color
	Hairline                            ui.Color
	SpecularTop, SpecularBottom         ui.Color // --specular-inset: 1px top highlight, bottom shade
	SpecularWidth                       float32  // kawaii uses 1.5
	Elevated                            [2]Shadow
	WheelShadow                         [2]Shadow
	CodeBG, CodePreviewBG               ui.Color
	PillBG                              ui.Color
	VignetteCenter, VignetteEdge        ui.Color
	SynKeyword, SynFunc, SynString      ui.Color
	SynComment                          ui.Color
	AccentClaude, AccentCodex           ui.Color
	AccentGemini, AccentCustom          ui.Color
	Decorations                         bool // --kawaii-deco-opacity

	// ---- v4 supplements ----
	Accent      ui.Color // focus rings, borders, glows: non-text only
	AccentFill  ui.Color // primary button fill, with AccentText
	AccentInk   ui.Color // accent-coloured text on Canvas
	AccentText  ui.Color
	Danger      ui.Color // danger text and icons on Canvas
	DangerFill  ui.Color // destructive button fill, with white text (≥4.5:1)
	Warning     ui.Color
	Success     ui.Color
	Solid       ui.Color // reduced-transparency surface
	SolidRaised ui.Color
	Scrim       ui.Color
	TerminalBG  ui.Color // opaque; never blurred (doc §4.1)
}

// Fill is a flat colour or a linear gradient, as v3's glass backgrounds.
type Fill struct {
	Angle float32
	Stops []ui.GradientStop
}

func flat(c ui.Color) Fill {
	return Fill{Stops: []ui.GradientStop{{Color: c, At: 0}, {Color: c, At: 1}}}
}

// Gradient returns the fill as a mygo gradient.
func (f Fill) Gradient() ui.LinearGradient { return ui.LinearGradient{Angle: f.Angle, Stops: f.Stops} }

// Average is the fill's mean colour, for surfaces that cannot take a gradient.
func (f Fill) Average() ui.Color {
	if len(f.Stops) == 0 {
		return ui.Transparent
	}
	var r, g, b, a float32
	for _, s := range f.Stops {
		r, g, b, a = r+float32(s.Color.R), g+float32(s.Color.G), b+float32(s.Color.B), a+float32(s.Color.A)
	}
	n := float32(len(f.Stops))
	return ui.Color{R: uint8(r/n + 0.5), G: uint8(g/n + 0.5), B: uint8(b/n + 0.5), A: uint8(a/n + 0.5)}
}

// Shadow is one CSS box-shadow layer.
type Shadow struct {
	X, Y, Blur, Spread float32
	Color              ui.Color
}

// Over composites a translucent colour onto an opaque one, as the screen
// shows it, for tokens that must be opaque (terminal, text backgrounds).
func Over(top, bottom ui.Color) ui.Color {
	a := float32(top.A) / 255
	m := func(t, b uint8) uint8 { return uint8(float32(t)*a + float32(b)*(1-a) + 0.5) }
	return ui.Color{R: m(top.R, bottom.R), G: m(top.G, bottom.G), B: m(top.B, bottom.B), A: 255}
}

var light = Palette{
	ID: "daybreak", Dark: false,
	Canvas: ui.Hex("#F5F5F7"),
	Mesh1:  ui.RGBA(0, 113, 227, 0.045), Mesh2: ui.RGBA(217, 101, 72, 0.04), Mesh3: ui.Transparent,
	TextPrimary: ui.Hex("#1D1D1F"), TextSecondary: ui.Hex("#6E6E73"), TextTer: ui.Hex("#86868B"),
	GlassDock: flat(ui.RGBA(255, 255, 255, 0.72)),
	GlassCard: ui.RGBA(255, 255, 255, 0.65), GlassCardHover: ui.RGBA(255, 255, 255, 0.92),
	GlassHUD:   Fill{Angle: 155, Stops: []ui.GradientStop{{Color: ui.RGBA(255, 255, 255, 0.92), At: 0}, {Color: ui.RGBA(246, 246, 249, 0.85), At: 1}}},
	SectorFill: ui.RGBA(255, 255, 255, 0.68), SectorFillHover: ui.RGBA(255, 255, 255, 0.96),
	SectorStroke: ui.RGBA(0, 0, 0, 0.08), SectorStrokeHover: ui.RGBA(0, 0, 0, 0.18),
	Hairline:    ui.RGBA(0, 0, 0, 0.075),
	SpecularTop: ui.RGBA(255, 255, 255, 1), SpecularBottom: ui.RGBA(0, 0, 0, 0.03), SpecularWidth: 1,
	Elevated:    [2]Shadow{{0, 20, 50, 0, ui.RGBA(15, 23, 42, 0.08)}, {0, 2, 8, 0, ui.RGBA(15, 23, 42, 0.04)}},
	WheelShadow: [2]Shadow{{0, 28, 60, 0, ui.RGBA(15, 23, 42, 0.12)}, {0, 4, 12, 0, ui.RGBA(15, 23, 42, 0.06)}},
	CodeBG:      ui.RGBA(255, 255, 255, 0.85), CodePreviewBG: ui.RGBA(242, 242, 247, 0.8),
	PillBG:         ui.RGBA(0, 0, 0, 0.045),
	VignetteCenter: ui.RGBA(245, 245, 247, 0.35), VignetteEdge: ui.RGBA(229, 229, 234, 0.78),
	SynKeyword: ui.Hex("#AD3DA4"), SynFunc: ui.Hex("#326D74"), SynString: ui.Hex("#D12F1B"), SynComment: ui.Hex("#707F8D"),
	AccentClaude: ui.Hex("#D96548"), AccentCodex: ui.Hex("#179A66"), AccentGemini: ui.Hex("#0066CC"), AccentCustom: ui.Hex("#7839EE"),

	Accent: ui.Hex("#0071E3"), AccentFill: ui.Hex("#0071E3"), AccentInk: ui.Hex("#0066CC"), AccentText: ui.Hex("#FFFFFF"),
	Danger: ui.Hex("#D70015"), DangerFill: ui.Hex("#D70015"), Warning: ui.Hex("#B25000"), Success: ui.Hex("#179A66"),
	Solid: ui.Hex("#FFFFFF"), SolidRaised: ui.Hex("#F2F2F5"),
	Scrim: ui.RGBA(15, 23, 42, 0.18), TerminalBG: ui.Hex("#FCFCFD"),
}

var dark = Palette{
	ID: "midnight", Dark: true,
	Canvas: ui.Hex("#09090B"),
	Mesh1:  ui.RGBA(56, 152, 255, 0.055), Mesh2: ui.RGBA(232, 130, 103, 0.045), Mesh3: ui.Transparent,
	TextPrimary: ui.Hex("#F5F5F7"), TextSecondary: ui.Hex("#A1A1A6"), TextTer: ui.Hex("#6E6E73"),
	GlassDock: flat(ui.RGBA(22, 22, 26, 0.68)),
	GlassCard: ui.RGBA(32, 32, 37, 0.58), GlassCardHover: ui.RGBA(46, 46, 54, 0.78),
	GlassHUD:   Fill{Angle: 155, Stops: []ui.GradientStop{{Color: ui.RGBA(38, 39, 46, 0.84), At: 0}, {Color: ui.RGBA(20, 20, 25, 0.90), At: 1}}},
	SectorFill: ui.RGBA(30, 31, 37, 0.62), SectorFillHover: ui.RGBA(52, 54, 65, 0.85),
	SectorStroke: ui.RGBA(255, 255, 255, 0.14), SectorStrokeHover: ui.RGBA(255, 255, 255, 0.48),
	Hairline:    ui.RGBA(255, 255, 255, 0.11),
	SpecularTop: ui.RGBA(255, 255, 255, 0.28), SpecularBottom: ui.RGBA(0, 0, 0, 0.55), SpecularWidth: 1,
	Elevated:    [2]Shadow{{0, 24, 60, 0, ui.RGBA(0, 0, 0, 0.65)}, {0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.4)}},
	WheelShadow: [2]Shadow{{0, 30, 70, 0, ui.RGBA(0, 0, 0, 0.85)}, {0, 4, 16, 0, ui.RGBA(0, 0, 0, 0.5)}},
	CodeBG:      ui.RGBA(16, 16, 20, 0.88), CodePreviewBG: ui.RGBA(9, 9, 12, 0.65),
	PillBG:         ui.RGBA(255, 255, 255, 0.08),
	VignetteCenter: ui.RGBA(15, 16, 22, 0.45), VignetteEdge: ui.RGBA(4, 4, 6, 0.86),
	SynKeyword: ui.Hex("#FF7AB2"), SynFunc: ui.Hex("#6BDFFF"), SynString: ui.Hex("#FF8170"), SynComment: ui.Hex("#7F8C98"),
	AccentClaude: ui.Hex("#E88267"), AccentCodex: ui.Hex("#34D399"), AccentGemini: ui.Hex("#47A1FF"), AccentCustom: ui.Hex("#B06BF9"),

	Accent: ui.Hex("#3898FF"), AccentFill: ui.Hex("#0A6CD9"), AccentInk: ui.Hex("#3898FF"), AccentText: ui.Hex("#FFFFFF"),
	Danger: ui.Hex("#FF6961"), DangerFill: ui.Hex("#C62828"), Warning: ui.Hex("#FFB340"), Success: ui.Hex("#34D399"),
	Solid: ui.Hex("#1C1C1F"), SolidRaised: ui.Hex("#26262B"),
	Scrim: ui.RGBA(0, 0, 0, 0.5), TerminalBG: ui.Hex("#0F0F13"),
}

var kawaii = Palette{
	ID: "sakura", Dark: false,
	Canvas: ui.Hex("#FFF8FA"),
	Mesh1:  ui.RGBA(255, 120, 175, 0.16), Mesh2: ui.RGBA(96, 197, 255, 0.15), Mesh3: ui.RGBA(184, 136, 255, 0.13),
	TextPrimary: ui.Hex("#3B2838"), TextSecondary: ui.Hex("#8A6D83"), TextTer: ui.Hex("#B599AE"),
	GlassDock: Fill{Angle: 180, Stops: []ui.GradientStop{{Color: ui.RGBA(255, 250, 253, 0.82), At: 0}, {Color: ui.RGBA(255, 240, 247, 0.68), At: 1}}},
	GlassCard: ui.RGBA(255, 255, 255, 0.72), GlassCardHover: ui.RGBA(255, 255, 255, 0.95),
	GlassHUD: Fill{Angle: 145, Stops: []ui.GradientStop{
		{Color: ui.RGBA(255, 255, 255, 0.95), At: 0}, {Color: ui.RGBA(255, 240, 248, 0.88), At: 0.55}, {Color: ui.RGBA(240, 249, 255, 0.90), At: 1}}},
	SectorFill: ui.RGBA(255, 255, 255, 0.74), SectorFillHover: ui.RGBA(255, 255, 255, 0.98),
	SectorStroke: ui.RGBA(255, 145, 190, 0.32), SectorStrokeHover: ui.RGBA(255, 105, 165, 0.75),
	Hairline:    ui.RGBA(255, 140, 185, 0.24),
	SpecularTop: ui.RGBA(255, 255, 255, 1), SpecularBottom: ui.RGBA(255, 140, 185, 0.18), SpecularWidth: 1.5,
	Elevated:    [2]Shadow{{0, 20, 50, 0, ui.RGBA(236, 72, 153, 0.11)}, {0, 4, 14, 0, ui.RGBA(139, 92, 246, 0.06)}},
	WheelShadow: [2]Shadow{{0, 26, 55, 0, ui.RGBA(236, 72, 153, 0.16)}, {0, 6, 18, 0, ui.RGBA(96, 165, 250, 0.12)}},
	CodeBG:      ui.RGBA(255, 255, 255, 0.88), CodePreviewBG: ui.RGBA(255, 242, 248, 0.75),
	PillBG:         ui.RGBA(255, 120, 175, 0.10),
	VignetteCenter: ui.RGBA(255, 248, 250, 0.3), VignetteEdge: ui.RGBA(255, 228, 240, 0.72),
	SynKeyword: ui.Hex("#EC4899"), SynFunc: ui.Hex("#0EA5E9"), SynString: ui.Hex("#F97316"), SynComment: ui.Hex("#B599AE"),
	AccentClaude: ui.Hex("#FF6B9E"), AccentCodex: ui.Hex("#10B981"), AccentGemini: ui.Hex("#38BDF8"), AccentCustom: ui.Hex("#A855F7"),
	Decorations: true,

	Accent: ui.Hex("#EC4899"), AccentFill: ui.Hex("#C81E6F"), AccentInk: ui.Hex("#C81E6F"), AccentText: ui.Hex("#FFFFFF"),
	// Success is emerald 600, not v3's 500: a done-stage mark keeps 3:1 on
	// the light pink sheet. Solid is whiter than v3's tint so secondary
	// text stays at the kawaii floor under reduced transparency.
	Danger: ui.Hex("#E11D48"), DangerFill: ui.Hex("#E11D48"), Warning: ui.Hex("#C2410C"), Success: ui.Hex("#059669"),
	Solid: ui.Hex("#FFF8FB"), SolidRaised: ui.Hex("#FFFFFF"),
	Scrim: ui.RGBA(120, 40, 90, 0.14), TerminalBG: ui.Hex("#FFFDFE"),
}

// PaletteFor returns the palette of a stored theme ID; unknown IDs recover
// to the dark palette, as the storage layer rejects them anyway.
func PaletteFor(id string) Palette {
	switch id {
	case "daybreak":
		return light
	case "sakura":
		return kawaii
	}
	return dark
}

// Resolve returns the stored theme ID to show: with followSystem the
// system's light or dark appearance chooses daybreak or midnight, otherwise
// the stored choice stands. Kawaii is only ever an explicit choice (D4).
func Resolve(stored string, followSystem, systemDark bool) string {
	if !followSystem {
		return PaletteFor(stored).ID
	}
	if systemDark {
		return dark.ID
	}
	return light.ID
}

// SelectedTintAlpha is the strongest accent tint behind AccentInk or Text
// that keeps 4.5:1 in every palette (light 4.58, kawaii 4.59 at 0.08).
const SelectedTintAlpha = 0.08

// Selected is the background of a selected row, tab or segment.
func (p Palette) Selected() ui.Color { return p.AccentFill.Alpha(SelectedTintAlpha) }

// PillText is the label colour of a capsule (status, context, account).
// The v3 capsule tint over a sheet costs secondary text up to 0.5:1, so
// labels move a quarter of the way to primary text: 4.5:1 everywhere
// (tests/theme TestControlCenterSurfaceContrast) without changing either
// v3 value.
func (p Palette) PillText() ui.Color { return p.TextSecondary.Mix(p.TextPrimary, 0.25) }

// ToolAccent maps a session tool to its v3 accent (doc §5.2).
func (p Palette) ToolAccent(tool string) ui.Color {
	switch tool {
	case "claude":
		return p.AccentClaude
	case "codex":
		return p.AccentCodex
	case "fclaude", "account-binding":
		return p.AccentGemini
	}
	return p.AccentCustom
}
