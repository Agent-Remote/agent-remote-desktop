package theme

import "github.com/egoist/mygo/ui"

// TextStyle is one level of the type scale (doc §5.3): size, weight and
// leading together, with size-specific tracking — negative as text grows,
// slightly positive for small labels (apple-design §15). Tracking is in em.
type TextStyle struct {
	Size     float32
	Weight   int
	Tracking float32
	Leading  float32 // line-height multiple; 0 keeps mygo's default
	Mono     bool
}

var (
	Display    = TextStyle{Size: 38, Weight: 700, Tracking: -0.02, Leading: 1.05}
	SheetTitle = TextStyle{Size: 22, Weight: 700, Tracking: -0.015, Leading: 1.15}
	PageTitle  = TextStyle{Size: 20, Weight: 700, Tracking: -0.012, Leading: 1.2}
	HUDTitle   = TextStyle{Size: 18, Weight: 700, Tracking: -0.01, Leading: 1.2}
	Body       = TextStyle{Size: 13.5, Weight: 500, Leading: 1.5}
	BodyStrong = TextStyle{Size: 13.5, Weight: 600, Leading: 1.5}
	Caption    = TextStyle{Size: 11.5, Weight: 500, Tracking: 0.01, Leading: 1.4}
	Tag        = TextStyle{Size: 10, Weight: 700, Tracking: 0.12, Leading: 1.2}
	MonoSmall  = TextStyle{Size: 11, Weight: 400, Leading: 1.4, Mono: true}
)

// MonoFont is the fallback chain of prototype-v4 for paths, IDs and code.
const MonoFont = "SF Mono, JetBrains Mono, Cascadia Mono, Menlo, Consolas, monospace"

// Apply styles a text element. Sizes scale with the theme's font size, so
// the desktop's text-size setting (TextScale) scales the whole scale.
func (s TextStyle) Apply(c *ui.Context, e ui.Element) ui.Element {
	size := s.Size * c.Theme().FontSize / 14
	e = e.FontSize(size).FontWeight(s.Weight)
	if s.Tracking != 0 {
		e = e.LetterSpacing(s.Tracking * size)
	}
	if s.Leading != 0 {
		e = e.LineHeight(s.Leading)
	}
	if s.Mono {
		e = e.Font(MonoFont)
	}
	return e
}
