package chrome

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

// WheelSize is the side of a wheel in DIPs (prototype-v3's 490 viewBox).
// Callers that need a smaller wheel scale the returned element; pointer
// input follows the transform.
const WheelSize float32 = 490

// Sector is one slot of a wheel as the caller sees it.
type Sector struct {
	Accent  *ui.Color // stroke when hot; nil uses SectorStrokeHover
	Enabled bool      // false: empty slot (dimmed, no hover)
}

// WheelView draws w with the v3 sector look and reports the sector under the
// pointer (-1 for none). hot is the selection to draw highlighted, which the
// caller owns (pointer, arrows, chord cycling). byAngle is true while a chord
// is held, so a flick past the rim still selects (Wheel.Hit). label places
// each sector's label, centred at its LabelCenter, in a 490-DIP box; center
// builds the HUD, laid in a hud×hud box at the centre (pass HUD(c, hud, …)).
func (e Env) WheelView(c *ui.Context, w Wheel, sectors []Sector, hot int, byAngle bool, label func(i int), hud float32, center func()) (ui.Element, int) {
	under := -1
	box := ui.Box(c).Size(WheelSize, WheelSize)
	if x, y, ok := box.PointerPosition(); ok {
		if i := w.Hit(x-WheelSize/2, y-WheelSize/2, byAngle); i >= 0 && i < len(sectors) && sectors[i].Enabled {
			under = i
		}
	}
	scales := make([]float32, len(sectors))
	pressed := box.Pressed()
	for i := range sectors {
		target, a := float32(1), e.Move(theme.Snappy)
		switch {
		case i == hot && pressed:
			target, a = 0.985, e.Move(theme.Press)
		case i == hot && sectors[i].Enabled:
			target = 1.038 // v3 hover scale
		}
		scales[i] = box.AnimateTo(sectorKey(i), target, a)
	}
	box.Children(func() {
		// The ring's soft drop shadow (v3 --wheel-shadow), cast by a circle
		// behind the sectors: SVG drop-shadow has no box equivalent, and the
		// 2–3° gaps between sectors are too thin to show the difference.
		ring := ui.Box(c).Absolute().Left(WheelSize/2-w.Outer).Top(WheelSize/2-w.Outer).
			Size(2*w.Outer, 2*w.Outer).Radius(w.Outer).PassThrough()
		if !e.S.ReduceTransparency {
			shadows(ring, e.P.WheelShadow[:])
		}
		ui.Box(c).Absolute().Top(0).Left(0).Size(WheelSize, WheelSize).PassThrough().
			Draw(func(p *ui.Painter, r ui.Rect) { e.paintSectors(p, r, w, sectors, hot, scales) })
		for i := range sectors {
			label(i)
		}
		if center != nil {
			ui.Box(c).Absolute().Left(WheelSize/2-hud/2).Top(WheelSize/2-hud/2).Size(hud, hud).Children(center)
		}
	})
	return box, under
}

type sectorKey int

func (e Env) paintSectors(p *ui.Painter, r ui.Rect, w Wheel, sectors []Sector, hot int, scales []float32) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	for i := range sectors {
		s := scales[i]
		pt := func(q Point) (float32, float32) { return cx + q.X*s, cy + q.Y*s }
		outer, inner := w.SectorOutline(i)
		var path ui.Path
		path.MoveTo(pt(outer[0].P0))
		for _, seg := range outer {
			x1, y1 := pt(seg.C1)
			x2, y2 := pt(seg.C2)
			x3, y3 := pt(seg.P3)
			path.CubeTo(x1, y1, x2, y2, x3, y3)
		}
		path.LineTo(pt(inner[0].P0))
		for _, seg := range inner {
			x1, y1 := pt(seg.C1)
			x2, y2 := pt(seg.C2)
			x3, y3 := pt(seg.P3)
			path.CubeTo(x1, y1, x2, y2, x3, y3)
		}
		path.Close()
		fill, stroke, width := e.P.SectorFill, e.P.SectorStroke, float32(1.35)
		if e.S.ReduceTransparency {
			fill = e.P.SolidRaised
		}
		if e.S.HighContrast {
			stroke = e.P.TextSecondary
		}
		if i == hot && sectors[i].Enabled {
			fill, stroke, width = e.P.SectorFillHover, e.P.SectorStrokeHover, 1.8
			if e.S.ReduceTransparency {
				fill = e.P.Solid
			}
			if sectors[i].Accent != nil {
				stroke = *sectors[i].Accent
			}
		}
		if !sectors[i].Enabled {
			// An empty slot keeps the ring's geometry but reads as absent:
			// fainter glass, or under reduced transparency an opaque fill
			// with a tertiary outline.
			if e.S.ReduceTransparency {
				fill, stroke = theme.Over(e.P.PillBG, e.P.Solid), e.P.TextTer
			} else {
				fill, stroke = fill.Alpha(0.4), stroke.Alpha(0.4)
			}
		}
		p.FillPath(&path, fill)
		p.StrokePath(&path, width, stroke)
	}
}

// HUD is the wheel's centre puck (v3 .center-hud): a glass circle of the
// given diameter (214 for ⌥C, 240 for ⌥X) whose content the caller builds.
func (e Env) HUD(c *ui.Context, diameter float32, content func()) ui.Element {
	el := ui.Column(c).Size(diameter, diameter).Padding(24).Gap(4).Center()
	return e.Glass(el, HUD, diameter/2, false).Children(content)
}

// HUDTag, HUDTitle and HUDPath are the puck's three text roles.
func (e Env) HUDTag(c *ui.Context, s string) ui.Element {
	return e.Text(c, s, theme.Tag, e.P.TextTer).Margin(0, 0, 2, 0)
}

func (e Env) HUDTitle(c *ui.Context, s string, color *ui.Color) ui.Element {
	col := e.P.TextPrimary
	if color != nil {
		col = *color
	}
	return e.Text(c, s, theme.HUDTitle, col).SingleLine().Ellipsis("…").MaxWidth(170)
}

func (e Env) HUDPath(c *ui.Context, s string) ui.Element {
	return ui.Row(c).Padding(4, 10).Radius(9).Background(e.pillBG()).Border(1, e.P.Hairline).MaxWidth(180).Children(func() {
		e.Text(c, s, theme.MonoSmall, e.P.TextSecondary).SingleLine().Ellipsis("…").Shrink(1).MinWidth(0)
	})
}
