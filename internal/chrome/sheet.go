package chrome

import (
	"github.com/Agent-Remote/agent-remote-desktop/internal/theme"
	"github.com/egoist/mygo/ui"
)

// SheetState is a bottom sheet that rises from the breathing bar and is
// dragged 1:1: rubber-banded past its bounds, thrown toward the projected
// end of a flick and released with the pointer's velocity (apple-design
// §2, §5, §6, §9). Offset.Y is 0 when open and Height when closed.
type SheetState struct {
	Offset  ui.Point
	Height  float32
	pending bool // Open before the first build: rise once Height is known
}

// Open and Close move the sheet as a key or button does; a drag moves it
// directly. Both re-target the spring from where the sheet shows.
func (s *SheetState) Open() {
	if s.Height == 0 {
		s.pending = true
		return
	}
	s.Offset.Y = 0
}

func (s *SheetState) Close() { s.pending, s.Offset.Y = false, s.Height }

// IsOpen reports the target, not the position on screen.
func (s *SheetState) IsOpen() bool { return s.pending || s.Height > 0 && s.Offset.Y < s.Height/2 }

// Shown reports whether the sheet must still be built: open, or on its way
// down after a close or a throw (a closed sheet at rest is not built).
func (s *SheetState) Shown(el ui.Element) bool { return s.IsOpen() || el.Transitioning() }

// Sheet builds the sheet at the bottom of its parent, height h. Callers
// keep building it while it is open or moving (Element.Transitioning) and
// read Progress, 0 closed to 1 open, to push the workspace back in step.
func (e Env) Sheet(c *ui.Context, s *SheetState, width, h float32, grab func(), content func()) (ui.Element, float32) {
	if s.Height != h {
		closed := s.Height == 0 || s.Offset.Y >= s.Height
		s.Height = h
		if closed {
			s.Offset.Y = h
		}
	} else if s.pending {
		// The first frame placed the sheet below the window; now rise from there.
		s.pending, s.Offset.Y = false, 0
		c.AnimationFrame()
	}
	el := ui.Column(c).Absolute().Bottom(0).Left(0).Right(0).MarginX(ui.Auto).Width(width).Height(h)
	el = e.Glass(el, Dock, 28, false).Radius(28, 28, 0, 0).BorderWidth(1, 1, 0, 1).Clip()
	el = el.Draggable(&s.Offset, ui.DraggableOptions{
		Axis:      ui.Vertical,
		Bounds:    &ui.Rect{H: h},
		Snap:      ui.Nearest(ui.Point{}, ui.Point{Y: h}),
		Animation: e.Move(theme.Sheet),
	})
	progress := float32(1)
	if h > 0 {
		progress = 1 - el.DragOffset().Y/h
	}
	el.Children(func() {
		ui.Row(c).Height(22).FillWidth().Center().Children(func() {
			ui.Box(c).Size(40, 5).Radius(2.5).Background(e.P.TextTer.Alpha(0.45))
			if grab != nil {
				grab()
			}
		})
		content()
	})
	return el, max(0, min(1, progress))
}
