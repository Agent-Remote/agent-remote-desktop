package chrome

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// Glass styles e as a v3 glass surface: frosted backdrop and tint, 1-DIP
// hairline, the specular inset (top highlight, bottom shade) and the
// elevated shadow. Under reduced transparency it is a solid surface of the
// same shape; under high contrast the hairline becomes secondary text and
// the specular highlight goes (apple-design §12, §14).
func (e Env) Glass(el ui.Element, m Material, radius float32, hover bool) ui.Element {
	el = el.Radius(radius)
	if e.S.ReduceTransparency {
		el = el.Background(e.solidOf(m))
	} else {
		el = el.Material(tinted{blur: glass.Blur{Radius: e.blurOf(m)}, tint: e.tintOf(m, hover).Gradient()})
	}
	if e.S.HighContrast {
		el = el.Border(1, e.P.TextSecondary)
	} else {
		w := e.P.SpecularWidth
		el = el.Border(1, e.P.Hairline).
			InsetShadow(0, w, w/2, 0, e.P.SpecularTop).
			InsetShadow(0, -w, w, 0, e.P.SpecularBottom)
	}
	if m != Chip {
		el = shadows(el, e.P.Elevated[:])
	}
	return el
}

func shadows(el ui.Element, s []theme.Shadow) ui.Element {
	for _, l := range s {
		el = el.Shadow(l.X, l.Y, l.Blur, l.Spread, l.Color)
	}
	return el
}

// Text is a text element in a type-scale style and colour.
func (e Env) Text(c *ui.Context, s string, style theme.TextStyle, color ui.Color) ui.Element {
	return style.Apply(c, ui.Text(c, s)).TextColor(color)
}

// Scrim is the dimming layer of a modal task (doc §4.7). Without
// transparency it is heavier, since the backdrop no longer blurs.
func (e Env) Scrim(c *ui.Context) ui.Element {
	color := e.P.Scrim
	if e.S.ReduceTransparency {
		color = color.Alpha(2)
	}
	return ui.Box(c).Absolute().Top(0).Right(0).Bottom(0).Left(0).Background(color)
}
