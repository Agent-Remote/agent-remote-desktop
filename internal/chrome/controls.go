package chrome

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

// Status is what the product can actually observe about a session from its
// PTY or connection — never "thinking" or "awaiting approval" (doc §4.2).
type Status uint8

const (
	StatusIdle Status = iota
	StatusOutput
	StatusConnected
	StatusDisconnected
	StatusExited
)

// Dot is the status indicator; the caller always puts a text label next to
// it so state is never conveyed by colour alone.
func (e Env) Dot(c *ui.Context, s Status) ui.Element {
	d := ui.Box(c).Size(7, 7).Radius(3.5)
	switch s {
	case StatusOutput:
		return d.Background(e.P.AccentCodex).Shadow(0, 0, 9, 0, e.P.AccentCodex.Alpha(0.8))
	case StatusConnected:
		return d.Background(e.P.AccentGemini)
	case StatusDisconnected:
		return d.Border(1.5, e.P.TextTer)
	}
	return d.Background(e.P.TextTer)
}

// pillBG is the capsule fill: the v3 translucent pill, or opaque without
// transparency so nothing behind shows through.
func (e Env) pillBG() ui.Color {
	if e.S.ReduceTransparency {
		return theme.Over(e.P.PillBG, e.P.SolidRaised)
	}
	return e.P.PillBG
}

// Pill is a capsule label (status pills, wheel badges).
func (e Env) Pill(c *ui.Context, dot *Status, label string) ui.Element {
	return ui.Row(c).Gap(6).Padding(3, 10).Radius(999).Background(e.pillBG()).Border(1, e.P.Hairline).
		Children(func() {
			if dot != nil {
				e.Dot(c, *dot)
			}
			e.Text(c, label, theme.Caption, e.P.PillText()).FontWeight(600).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		})
}

// ContextPill shows one entry of the operation context strip (home, server,
// device, project): a short label and the exact value in monospace.
func (e Env) ContextPill(c *ui.Context, label, value string) ui.Element {
	return ui.Row(c).Gap(5).Padding(3, 10).Radius(999).Background(e.pillBG()).Border(1, e.P.Hairline).
		Children(func() {
			// The label names the value, so it is secondary text, not tertiary.
			e.Text(c, label, theme.Caption, e.P.PillText()).FontWeight(600)
			e.Text(c, value, theme.MonoSmall, e.P.PillText()).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
		})
}

// Kind is a button's role. Danger is only for irreversible actions.
type Kind uint8

const (
	Secondary Kind = iota
	Primary
	Danger
)

// Button is a capsule button. Feedback starts on pointer-down: it presses to
// 0.97 on the Press spring and returns on Snappy (apple-design §1), which
// only paints. Callers chain OnClick, Label and Tooltip as on any element.
func (e Env) Button(c *ui.Context, label string, kind Kind) ui.Element {
	b := ui.ButtonBase(c).Padding(6, 14).Radius(999).Cursor(ui.CursorPointer)
	text := e.P.TextPrimary
	switch kind {
	case Primary:
		b, text = b.Background(e.P.AccentFill), e.P.AccentText
	case Danger:
		b, text = b.Background(e.P.DangerFill), ui.Hex("#FFFFFF")
	default:
		bg := e.pillBG()
		if b.Hovered() {
			bg = theme.Over(e.P.GlassCardHover, e.P.Canvas)
		}
		b = b.Background(b.AnimateColor("bg", bg, e.Fade(ui.Animation{Duration: 100e6}))).Border(1, e.P.Hairline)
		if !e.S.HighContrast {
			b = b.InsetShadow(0, 1, 0.5, 0, e.P.SpecularTop)
		}
	}
	b.Children(func() { e.Text(c, label, theme.BodyStrong, text).FontSize(12 * c.Theme().FontSize / 14) })
	return Pressable(b, e)
}

// Pressable gives an element the press feedback of Button.
func Pressable(el ui.Element, e Env) ui.Element {
	pose, a := ui.Pose{}, e.Move(theme.Snappy)
	if el.Pressed() {
		pose, a = ui.Pose{Scale: 0.97}, e.Move(theme.Press)
	}
	return el.AnimatePose(pose, a)
}

// Segmented is the appearance switch of v3 (theme toggle bar): segments in
// a capsule, the chosen one raised. It is mygo's SegmentedBase, so it is a
// radio group to assistive technology, one Tab stop, and the arrows move
// the choice. name labels the group. It reports whether the choice changed.
func (e Env) Segmented(c *ui.Context, selected *int, name string, labels ...string) bool {
	seg := ui.SegmentedBase(c, selected, len(labels))
	seg.Track.Label(name).Gap(2).Padding(3).Radius(999).Background(e.pillBG()).Border(1, e.P.Hairline).Children(func() {
		for i, l := range labels {
			s := seg.Segment(i).Padding(4, 11).Radius(999).Cursor(ui.CursorPointer).Label(l)
			color, weight := e.P.TextSecondary, 500
			if i == *selected {
				s = s.Background(e.P.Canvas).Shadow(0, 2, 10, 0, ui.RGBA(0, 0, 0, 0.08))
				if !e.S.HighContrast {
					s = s.InsetShadow(0, 1, 0.5, 0, e.P.SpecularTop)
				}
				color, weight = e.P.TextPrimary, 600
				if e.P.Decorations {
					color = e.P.AccentInk
				}
			}
			Pressable(s, e).Children(func() { e.Text(c, l, theme.Caption, color).FontWeight(weight) })
		}
	})
	return seg.Track.Changed()
}
