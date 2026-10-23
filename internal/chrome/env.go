package chrome

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// Env is what every chrome component draws with: the palette on screen and
// the frame's surface policy. Components carry no text of their own; callers
// pass localized labels, so chrome stays outside the i18n catalog.
type Env struct {
	P theme.Palette
	S theme.Surfaces
}

// Move and Fade return a motion token adjusted for reduced motion.
func (e Env) Move(a ui.Animation) ui.Animation { return theme.Move(a, e.S.ReduceMotion) }
func (e Env) Fade(a ui.Animation) ui.Animation { return theme.Fade(a, e.S.ReduceMotion) }

// Material is how glass is filled: frosted backdrop plus the v3 tint, or a
// solid colour under reduced transparency.
type Material uint8

const (
	Dock Material = iota // large structural sheets: dock, control center
	Card                 // cards and tiles
	HUD                  // wheel centre puck, dialogs
	Chip                 // pane chips, toasts
)

// blur radii (σ) and v3 tint per material; bigger surfaces read as thicker.
func (e Env) blurOf(m Material) float32 {
	switch m {
	case Dock:
		return 30
	case HUD:
		return 25
	case Card:
		return 20
	}
	return 10
}

func (e Env) tintOf(m Material, hover bool) theme.Fill {
	switch m {
	case Dock:
		return e.P.GlassDock
	case HUD:
		return e.P.GlassHUD
	}
	if hover {
		return flatFill(e.P.GlassCardHover)
	}
	return flatFill(e.P.GlassCard)
}

func (e Env) solidOf(m Material) ui.Color {
	if m == Card || m == Chip {
		return e.P.SolidRaised
	}
	return e.P.Solid
}

func flatFill(c ui.Color) theme.Fill {
	return theme.Fill{Stops: []ui.GradientStop{{Color: c, At: 0}, {Color: c, At: 1}}}
}

// tinted is a frosted backdrop followed by a gradient tint, as one material,
// so a glass surface stays a single element whose padding holds its content.
type tinted struct {
	blur glass.Blur
	tint ui.LinearGradient
}

func (t tinted) PaintMaterial(p *ui.Painter, box ui.Rect, radii [4]float32) {
	t.blur.PaintMaterial(p, box, radii)
	if radii[0] == radii[1] && radii[1] == radii[2] && radii[2] == radii[3] {
		p.FillGradient(box, t.tint, radii[0])
		return
	}
	p.FillPathGradient(roundedRect(box, radii), t.tint)
}

// roundedRect is box with its own radius per corner (top-left, top-right,
// bottom-right, bottom-left), each quarter a cubic of κ = 0.5523.
func roundedRect(b ui.Rect, r [4]float32) *ui.Path {
	const k = 0.5523
	x0, y0, x1, y1 := b.X, b.Y, b.X+b.W, b.Y+b.H
	var p ui.Path
	p.MoveTo(x0+r[0], y0)
	p.LineTo(x1-r[1], y0)
	p.CubeTo(x1-r[1]+k*r[1], y0, x1, y0+r[1]-k*r[1], x1, y0+r[1])
	p.LineTo(x1, y1-r[2])
	p.CubeTo(x1, y1-r[2]+k*r[2], x1-r[2]+k*r[2], y1, x1-r[2], y1)
	p.LineTo(x0+r[3], y1)
	p.CubeTo(x0+r[3]-k*r[3], y1, x0, y1-r[3]+k*r[3], x0, y1-r[3])
	p.LineTo(x0, y0+r[0])
	p.CubeTo(x0, y0+r[0]-k*r[0], x0+r[0]-k*r[0], y0, x0+r[0], y0)
	p.Close()
	return &p
}
