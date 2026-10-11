package widgets

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

const padPuckR = 11.0

// IrisPad is the joystick pad of a camera strip: dragging up or down changes
// the iris, left or right changes the master black. Each axis can be switched
// off when the camera has no such control.
type IrisPad struct {
	widget.BaseWidget
	// OnChanged is called on each drag step with the pad position in [-1, 1]
	// (y up) and which axes are driven.
	OnChanged func(x, y float64, hasX, hasY bool)
	OnEnded   func()

	x, y       float64
	hasX, hasY bool
	active     bool
	dragging   bool
	minSize    fyne.Size
}

func NewIrisPad(min fyne.Size) *IrisPad {
	p := &IrisPad{minSize: min}
	p.ExtendBaseWidget(p)
	return p
}

// SetAxes enables the horizontal (master black) and vertical (iris) axes.
func (p *IrisPad) SetAxes(hasX, hasY bool) {
	if p.hasX != hasX || p.hasY != hasY {
		p.hasX, p.hasY = hasX, hasY
		p.Refresh()
	}
}

// SetActive colors the puck in the accent (selected camera).
func (p *IrisPad) SetActive(a bool) {
	if p.active != a {
		p.active = a
		p.Refresh()
	}
}

// SetPos moves the puck without notifying. Ignored while dragging.
func (p *IrisPad) SetPos(x, y float64) {
	if p.dragging {
		return
	}
	p.x, p.y = clamp1(x), clamp1(y)
	p.Refresh()
}

func (p *IrisPad) Pos() (x, y float64) { return p.x, p.y }

func clamp1(v float64) float64 { return math.Max(-1, math.Min(1, v)) }

func (p *IrisPad) update(pos fyne.Position) {
	s := p.Size()
	hw, hh := float64(s.Width)/2-padPuckR, float64(s.Height)/2-padPuckR
	if hw <= 0 || hh <= 0 {
		return
	}
	if p.hasX {
		p.x = clamp1((float64(pos.X) - float64(s.Width)/2) / hw)
	}
	if p.hasY {
		p.y = clamp1((float64(s.Height)/2 - float64(pos.Y)) / hh)
	}
	p.Refresh()
	if p.OnChanged != nil {
		p.OnChanged(p.x, p.y, p.hasX, p.hasY)
	}
}

func (p *IrisPad) Tapped(e *fyne.PointEvent) {
	if !p.hasX && !p.hasY {
		return
	}
	p.update(e.Position)
	if p.OnEnded != nil {
		p.OnEnded()
	}
}

func (p *IrisPad) Dragged(e *fyne.DragEvent) {
	if !p.hasX && !p.hasY {
		return
	}
	p.dragging = true
	p.update(e.Position)
}

func (p *IrisPad) DragEnd() {
	if !p.dragging {
		return
	}
	p.dragging = false
	if p.OnEnded != nil {
		p.OnEnded()
	}
}

func (p *IrisPad) Cursor() desktop.Cursor { return desktop.CrosshairCursor }

func (p *IrisPad) CreateRenderer() fyne.WidgetRenderer {
	r := &padRenderer{p: p}
	r.bg = canvas.NewRectangle(style.C(style.Inset))
	r.bg.CornerRadius = style.Radius
	r.bg.StrokeWidth = 1
	r.h = canvas.NewLine(style.C(style.Border))
	r.v = canvas.NewLine(style.C(style.Border))
	r.puck = canvas.NewCircle(color.Transparent)
	r.puck.StrokeWidth = 2
	r.dot = canvas.NewCircle(color.Transparent)
	r.Refresh()
	return r
}

type padRenderer struct {
	p         *IrisPad
	bg        *canvas.Rectangle
	h, v      *canvas.Line
	puck, dot *canvas.Circle
}

func (r *padRenderer) MinSize() fyne.Size { return r.p.minSize }

func (r *padRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	cx, cy := s.Width/2, s.Height/2
	hw, hh := float64(s.Width)/2-padPuckR, float64(s.Height)/2-padPuckR
	px := float32(float64(cx) + r.p.x*hw)
	py := float32(float64(cy) - r.p.y*hh)
	// the cross goes through the puck's centre and follows it
	r.h.Position1, r.h.Position2 = fyne.NewPos(6, py), fyne.NewPos(s.Width-6, py)
	r.v.Position1, r.v.Position2 = fyne.NewPos(px, 6), fyne.NewPos(px, s.Height-6)
	r.puck.Move(fyne.NewPos(px-padPuckR, py-padPuckR))
	r.puck.Resize(fyne.NewSize(2*padPuckR, 2*padPuckR))
	r.dot.Move(fyne.NewPos(px-3, py-3))
	r.dot.Resize(fyne.NewSize(6, 6))
}

func (r *padRenderer) Refresh() {
	r.bg.FillColor = style.C(style.Inset)
	r.bg.StrokeColor = style.C(style.Border)
	enabled := r.p.hasX || r.p.hasY
	line := style.C(style.Border)
	r.h.StrokeColor, r.v.StrokeColor = line, line
	r.h.StrokeWidth, r.v.StrokeWidth = 1, 1
	if r.p.hasX {
		r.h.StrokeColor = style.C(style.BorderStrong)
	}
	if r.p.hasY {
		r.v.StrokeColor = style.C(style.BorderStrong)
	}
	ink := style.C(style.Text)
	switch {
	case !enabled:
		ink = style.C(style.TextFaint)
	case r.p.active:
		ink = style.C(style.Accent)
	}
	r.puck.StrokeColor = ink
	r.puck.FillColor = withAlpha(style.C(style.Raised), 0xE0)
	r.dot.FillColor = ink
	r.Layout(r.p.Size())
	r.bg.Refresh()
	r.h.Refresh()
	r.v.Refresh()
	r.puck.Refresh()
	r.dot.Refresh()
}

func (r *padRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.bg, r.h, r.v, r.puck, r.dot}
}
func (r *padRenderer) Destroy() {}

// --- LevelMeter ----------------------------------------------------------------

// LevelMeter is a vertical segmented bar, like the iris level meter of a
// camera panel. Level is 0 (empty) to 1 (full).
type LevelMeter struct {
	widget.BaseWidget
	level    float64
	known    bool
	segments int
	minSize  fyne.Size
}

func NewLevelMeter(segments int, min fyne.Size) *LevelMeter {
	m := &LevelMeter{segments: segments, minSize: min}
	m.ExtendBaseWidget(m)
	return m
}

// SetLevel sets the lit fraction; known=false shows an empty meter.
func (m *LevelMeter) SetLevel(l float64, known bool) {
	l = math.Max(0, math.Min(1, l))
	if m.level != l || m.known != known {
		m.level, m.known = l, known
		m.Refresh()
	}
}

// Lit returns how many segments are on.
func (m *LevelMeter) Lit() int {
	if !m.known {
		return 0
	}
	return int(math.Round(m.level * float64(m.segments)))
}

func (m *LevelMeter) CreateRenderer() fyne.WidgetRenderer {
	r := &meterRenderer{m: m}
	for i := 0; i < m.segments; i++ {
		s := canvas.NewRectangle(color.Transparent)
		s.CornerRadius = 1
		r.segs = append(r.segs, s)
	}
	r.Refresh()
	return r
}

type meterRenderer struct {
	m    *LevelMeter
	segs []*canvas.Rectangle
}

func (r *meterRenderer) MinSize() fyne.Size { return r.m.minSize }

func (r *meterRenderer) Layout(s fyne.Size) {
	n := float32(len(r.segs))
	gap := float32(2)
	h := (s.Height - gap*(n-1)) / n
	for i, seg := range r.segs {
		// segment 0 is the bottom one
		y := s.Height - float32(i+1)*h - float32(i)*gap
		seg.Move(fyne.NewPos(0, y))
		seg.Resize(fyne.NewSize(s.Width, h))
	}
}

func (r *meterRenderer) Refresh() {
	lit := r.m.Lit()
	for i, seg := range r.segs {
		if i < lit {
			seg.FillColor = style.C(style.OK)
		} else {
			seg.FillColor = style.C(style.Border)
		}
		seg.Refresh()
	}
	r.Layout(r.m.Size())
}

func (r *meterRenderer) Objects() []fyne.CanvasObject {
	o := make([]fyne.CanvasObject, len(r.segs))
	for i, s := range r.segs {
		o[i] = s
	}
	return o
}
func (r *meterRenderer) Destroy() {}
