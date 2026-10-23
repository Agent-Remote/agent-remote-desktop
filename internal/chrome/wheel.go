// Package chrome holds the workbench's business-independent native surfaces.
// Wheel geometry follows prototype-v3/v4 exactly: 0° is 12 o'clock and angles
// grow clockwise, so a sector index means the same place in every theme.
package chrome

import "math"

// Wheel describes a ring of equal annular sectors around a centre HUD.
type Wheel struct {
	Count  int
	Inner  float32 // inner radius of the sectors, in DIPs
	Outer  float32 // outer radius of the sectors, in DIPs
	Gap    float32 // degrees trimmed from each side of a sector
	DeadR  float32 // pointer inside this radius selects nothing (the HUD)
	LabelR float32 // radius at which sector labels are centred
}

// CreateWheel and ProjectWheel are the two wheels of the redesign, in the
// 490-DIP coordinate space of prototype-v3.
var (
	CreateWheel  = Wheel{Count: 4, Inner: 115, Outer: 220, Gap: 2, DeadR: 107, LabelR: 168}
	ProjectWheel = Wheel{Count: 8, Inner: 126, Outer: 232, Gap: 1.5, DeadR: 120, LabelR: 180}
)

// Point is a position relative to the wheel's centre.
type Point struct{ X, Y float32 }

// Cubic is one cubic Bézier segment.
type Cubic struct{ P0, C1, C2, P3 Point }

// Polar returns the point at radius r and angle deg (0° at 12 o'clock,
// clockwise), relative to the centre.
func Polar(r, deg float32) Point {
	rad := float64(deg-90) * math.Pi / 180
	return Point{r * float32(math.Cos(rad)), r * float32(math.Sin(rad))}
}

// SectorSpan returns the start and end angle of sector i, centred on its slot.
func (w Wheel) SectorSpan(i int) (start, end float32) {
	step := 360 / float32(w.Count)
	return float32(i)*step - step/2 + w.Gap, float32(i+1)*step - step/2 - w.Gap
}

// LabelCenter returns where sector i's label is centred.
func (w Wheel) LabelCenter(i int) Point {
	start, end := w.SectorSpan(i)
	return Polar(w.LabelR, (start+end)/2)
}

// Arc approximates the arc of radius r from angle a to angle b with cubic
// segments of at most 90° each. A 90° segment keeps the radial error below
// 2.7e-4·r, under 0.07 DIP at the project wheel's 232-DIP rim.
func Arc(r, a, b float32) []Cubic {
	n := int(math.Ceil(math.Abs(float64(b-a)) / 90))
	if n < 1 {
		n = 1
	}
	step := (b - a) / float32(n)
	k := float32(4.0 / 3.0 * math.Tan(float64(step)*math.Pi/720))
	out := make([]Cubic, 0, n)
	for i := 0; i < n; i++ {
		s := a + float32(i)*step
		e := s + step
		p0, p3 := Polar(r, s), Polar(r, e)
		// Tangents of a clockwise arc are the radius vector turned +90°.
		t0, t3 := Point{-p0.Y, p0.X}, Point{-p3.Y, p3.X}
		out = append(out, Cubic{
			P0: p0,
			C1: Point{p0.X + k*t0.X, p0.Y + k*t0.Y},
			C2: Point{p3.X - k*t3.X, p3.Y - k*t3.Y},
			P3: p3,
		})
	}
	return out
}

// SectorOutline returns the closed outline of sector i: outer arc clockwise,
// then inner arc counter-clockwise. Callers join the segments with line-tos.
func (w Wheel) SectorOutline(i int) (outer, inner []Cubic) {
	start, end := w.SectorSpan(i)
	return Arc(w.Outer, start, end), Arc(w.Inner, end, start)
}

// Hit maps a pointer offset from the centre to a sector index. Inside DeadR
// it reports -1 (the HUD owns that area). With byAngle (a chord is held) any
// distance outside the HUD selects by angle so a flick reaches a sector;
// otherwise points more than 12% beyond the rim report -1.
func (w Wheel) Hit(dx, dy float32, byAngle bool) int {
	d := float32(math.Hypot(float64(dx), float64(dy)))
	if d < w.DeadR || (!byAngle && d > w.Outer*1.12) {
		return -1
	}
	angle := float32(math.Mod(math.Atan2(float64(dx), float64(-dy))*180/math.Pi+360, 360))
	step := 360 / float32(w.Count)
	i := int(float32(math.Mod(float64(angle+step/2), 360)) / step)
	return min(i, w.Count-1)
}
