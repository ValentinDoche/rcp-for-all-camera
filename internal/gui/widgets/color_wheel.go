package widgets

import (
	"image"
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

const (
	wheelRing   = 0.17 // ring thickness as a fraction of the radius
	wheelPuckR  = 7.0  // puck radius in dp
	wheelMargin = 8.0  // puck stays this far inside the rim
)

// ColorWheel is a hue ring with a draggable puck, like the wheels of a color
// corrector. Positions are on the unit disc (y up); mapping to R, G, B values
// is done by PuckToRGB.
type ColorWheel struct {
	widget.BaseWidget
	// OnChanged is called on every drag step with the new puck position.
	OnChanged func(x, y float64)
	// OnEnded is called when the drag or tap finishes.
	OnEnded func()

	diameter float32
	x, y     float64
	dragging bool
	disabled bool
}

// NewColorWheel builds a wheel of the given minimum diameter.
func NewColorWheel(diameter float32) *ColorWheel {
	w := &ColorWheel{diameter: diameter}
	w.ExtendBaseWidget(w)
	return w
}

// SetPuck moves the puck without notifying. Ignored while the user drags.
func (w *ColorWheel) SetPuck(x, y float64) {
	if w.dragging {
		return
	}
	w.x, w.y = x, y
	w.Refresh()
}

// Puck returns the puck position.
func (w *ColorWheel) Puck() (x, y float64) { return w.x, w.y }

// SetDisabled greys the wheel and ignores input.
func (w *ColorWheel) SetDisabled(d bool) {
	if w.disabled != d {
		w.disabled = d
		w.Refresh()
	}
}

func (w *ColorWheel) geometry() (cx, cy, r float64) {
	s := w.Size()
	d := math.Min(float64(s.Width), float64(s.Height))
	return float64(s.Width) / 2, float64(s.Height) / 2, d / 2
}

func (w *ColorWheel) update(p fyne.Position) {
	cx, cy, r := w.geometry()
	x, y := PointToUnit(float64(p.X), float64(p.Y), cx, cy, r-wheelMargin)
	w.x, w.y = x, y
	w.Refresh()
	if w.OnChanged != nil {
		w.OnChanged(x, y)
	}
}

func (w *ColorWheel) Tapped(e *fyne.PointEvent) {
	if w.disabled {
		return
	}
	w.update(e.Position)
	if w.OnEnded != nil {
		w.OnEnded()
	}
}

func (w *ColorWheel) Dragged(e *fyne.DragEvent) {
	if w.disabled {
		return
	}
	w.dragging = true
	w.update(e.Position)
}

func (w *ColorWheel) DragEnd() {
	if !w.dragging {
		return
	}
	w.dragging = false
	if w.OnEnded != nil {
		w.OnEnded()
	}
}

func (w *ColorWheel) Cursor() desktop.Cursor { return desktop.CrosshairCursor }

func (w *ColorWheel) CreateRenderer() fyne.WidgetRenderer {
	r := &wheelRenderer{w: w}
	r.ring = canvas.NewRaster(r.draw)
	r.ring.ScaleMode = canvas.ImageScaleSmooth
	r.hLine = canvas.NewLine(style.C(style.Border))
	r.vLine = canvas.NewLine(style.C(style.Border))
	r.puck = canvas.NewCircle(color.Transparent)
	r.puck.StrokeWidth = 2
	r.dot = canvas.NewCircle(color.Transparent)
	r.Refresh()
	return r
}

type wheelRenderer struct {
	w            *ColorWheel
	ring         *canvas.Raster
	hLine, vLine *canvas.Line
	puck, dot    *canvas.Circle
}

func (r *wheelRenderer) MinSize() fyne.Size {
	return fyne.NewSize(r.w.diameter, r.w.diameter)
}

func (r *wheelRenderer) Layout(s fyne.Size) {
	cx, cy, rad := r.w.geometry()
	d := float32(rad * 2)
	r.ring.Move(fyne.NewPos(float32(cx)-d/2, float32(cy)-d/2))
	r.ring.Resize(fyne.NewSize(d, d))
	inner := float32(rad * (1 - wheelRing))
	r.hLine.Position1 = fyne.NewPos(float32(cx)-inner, float32(cy))
	r.hLine.Position2 = fyne.NewPos(float32(cx)+inner, float32(cy))
	r.vLine.Position1 = fyne.NewPos(float32(cx), float32(cy)-inner)
	r.vLine.Position2 = fyne.NewPos(float32(cx), float32(cy)+inner)
	px := float32(cx + r.w.x*(rad-wheelMargin))
	py := float32(cy - r.w.y*(rad-wheelMargin))
	r.puck.Move(fyne.NewPos(px-wheelPuckR, py-wheelPuckR))
	r.puck.Resize(fyne.NewSize(2*wheelPuckR, 2*wheelPuckR))
	r.dot.Move(fyne.NewPos(px-1.5, py-1.5))
	r.dot.Resize(fyne.NewSize(3, 3))
}

func (r *wheelRenderer) Refresh() {
	line := style.C(style.Border)
	r.hLine.StrokeColor, r.vLine.StrokeColor = line, line
	r.hLine.StrokeWidth, r.vLine.StrokeWidth = 1, 1
	ink := style.C(style.Text)
	if r.w.disabled {
		ink = style.C(style.TextFaint)
	}
	r.puck.StrokeColor = ink
	r.puck.FillColor = withAlpha(style.C(style.Inset), 0xB0)
	r.dot.FillColor = ink
	r.Layout(r.w.Size())
	r.ring.Refresh()
	r.hLine.Refresh()
	r.vLine.Refresh()
	r.puck.Refresh()
	r.dot.Refresh()
}

func (r *wheelRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.ring, r.hLine, r.vLine, r.puck, r.dot}
}
func (r *wheelRenderer) Destroy() {}

func withAlpha(c color.Color, a uint8) color.Color {
	cr, cg, cb, _ := c.RGBA()
	return color.NRGBA{R: uint8(cr >> 8), G: uint8(cg >> 8), B: uint8(cb >> 8), A: a}
}

// draw renders the hue ring with a faintly tinted dark interior.
func (r *wheelRenderer) draw(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	if w < 4 || h < 4 {
		return img
	}
	cx, cy := float64(w)/2, float64(h)/2
	rad := math.Min(cx, cy) - 0.5
	inner := rad * (1 - wheelRing)
	fill := color.NRGBAModel.Convert(style.C(style.Inset)).(color.NRGBA)
	tintK := 0.16
	if style.Luminance(fill) > 0.5 {
		tintK = 0.34 // light theme: the pale interior needs a stronger hue hint
	}
	// greyed: a plain grey ring and an untinted interior, no colour at all
	grey := color.NRGBAModel.Convert(style.C(style.Border)).(color.NRGBA)
	if r.w.disabled {
		tintK = 0
	}
	for py := 0; py < h; py++ {
		for px := 0; px < w; px++ {
			dx, dy := float64(px)+0.5-cx, cy-(float64(py)+0.5)
			d := math.Hypot(dx, dy)
			if d > rad+0.5 {
				continue
			}
			hue := math.Atan2(dy, dx)
			if hue < 0 {
				hue += 2 * math.Pi
			}
			var c color.NRGBA
			switch {
			case d >= inner && r.w.disabled:
				c = grey
			case d >= inner:
				c = hsv(hue, 0.72, 0.92)
			default:
				tint := hsv(hue, 1, 1)
				k := tintK * math.Min(1, d/inner)
				c = mix(fill, tint, k)
			}
			// edge antialiasing on the outer rim
			a := 1.0
			if d > rad-0.5 {
				a = math.Max(0, rad+0.5-d)
			}
			c.A = uint8(float64(c.A) * a)
			img.SetNRGBA(px, py, c)
		}
	}
	return img
}

func mix(a, b color.NRGBA, k float64) color.NRGBA {
	l := func(x, y uint8) uint8 { return uint8(float64(x)*(1-k) + float64(y)*k) }
	return color.NRGBA{R: l(a.R, b.R), G: l(a.G, b.G), B: l(a.B, b.B), A: 0xFF}
}

// hsv converts a hue in radians (counterclockwise from red) to a color.
func hsv(h, s, v float64) color.NRGBA {
	hp := h / (math.Pi / 3)
	c := v * s
	x := c * (1 - math.Abs(math.Mod(hp, 2)-1))
	var rr, gg, bb float64
	switch int(hp) % 6 {
	case 0:
		rr, gg, bb = c, x, 0
	case 1:
		rr, gg, bb = x, c, 0
	case 2:
		rr, gg, bb = 0, c, x
	case 3:
		rr, gg, bb = 0, x, c
	case 4:
		rr, gg, bb = x, 0, c
	default:
		rr, gg, bb = c, 0, x
	}
	m := v - c
	return color.NRGBA{R: uint8((rr + m) * 255), G: uint8((gg + m) * 255), B: uint8((bb + m) * 255), A: 0xFF}
}

// Disabled reports whether the wheel is greyed.
func (w *ColorWheel) Disabled() bool { return w.disabled }
