package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// PadCross returns, for the external tests, where the pad draws its puck
// centre and its cross lines.
func PadCross(p *IrisPad) (centre fyne.Position, vx1, vx2, hy1, hy2 float32) {
	r := test.WidgetRenderer(p).(*padRenderer)
	centre = fyne.NewPos(r.puck.Position().X+padPuckR, r.puck.Position().Y+padPuckR)
	return centre, r.v.Position1.X, r.v.Position2.X, r.h.Position1.Y, r.h.Position2.Y
}
