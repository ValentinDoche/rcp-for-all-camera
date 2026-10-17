package widgets

import (
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

const (
	sliderThumbR = 8.0
	sliderTrack  = 4.0
)

// Slider is a thin-track slider with an accent fill, horizontal or vertical
// (vertical: minimum at the bottom). OnChanged only fires for user input;
// Set updates the value silently.
type Slider struct {
	widget.BaseWidget
	Min, Max, Step float64
	Vertical       bool
	OnChanged      func(v float64)

	value    float64
	dragging bool
	focused  bool
	disabled bool
}

func NewSlider(min, max float64) *Slider {
	s := &Slider{Min: min, Max: max}
	s.ExtendBaseWidget(s)
	return s
}

// Value is the current value.
func (s *Slider) Value() float64 { return s.value }

// Set changes the value without calling OnChanged. It is ignored while the
// user drags.
func (s *Slider) Set(v float64) {
	if s.dragging {
		return
	}
	s.value = math.Max(s.Min, math.Min(s.Max, v))
	s.Refresh()
}

func (s *Slider) SetDisabled(d bool) {
	if s.disabled != d {
		s.disabled = d
		s.Refresh()
	}
}

func (s *Slider) ratio() float64 {
	if s.Max <= s.Min {
		return 0
	}
	return (s.value - s.Min) / (s.Max - s.Min)
}

func (s *Slider) fromPosition(p fyne.Position) {
	size := s.Size()
	var r float64
	if s.Vertical {
		span := float64(size.Height) - 2*sliderThumbR
		if span <= 0 {
			return
		}
		r = 1 - (float64(p.Y)-sliderThumbR)/span
	} else {
		span := float64(size.Width) - 2*sliderThumbR
		if span <= 0 {
			return
		}
		r = (float64(p.X) - sliderThumbR) / span
	}
	r = math.Max(0, math.Min(1, r))
	s.change(s.Min + r*(s.Max-s.Min))
}

func (s *Slider) change(v float64) {
	v = Quantize(v, Range{Min: s.Min, Max: s.Max, Step: s.Step})
	if v == s.value {
		return
	}
	s.value = v
	s.Refresh()
	if s.OnChanged != nil {
		s.OnChanged(v)
	}
}

func (s *Slider) Tapped(e *fyne.PointEvent) {
	if !s.disabled {
		s.fromPosition(e.Position)
	}
}

func (s *Slider) Dragged(e *fyne.DragEvent) {
	if s.disabled {
		return
	}
	s.dragging = true
	s.fromPosition(e.Position)
}

func (s *Slider) DragEnd() { s.dragging = false }

func (s *Slider) Cursor() desktop.Cursor { return desktop.PointerCursor }

// keyboard support
func (s *Slider) FocusGained()   { s.focused = true; s.Refresh() }
func (s *Slider) FocusLost()     { s.focused = false; s.Refresh() }
func (s *Slider) TypedRune(rune) {}
func (s *Slider) keyStep() float64 {
	if s.Step > 0 {
		return s.Step
	}
	return (s.Max - s.Min) / 100
}

func (s *Slider) TypedKey(k *fyne.KeyEvent) {
	if s.disabled {
		return
	}
	switch k.Name {
	case fyne.KeyLeft, fyne.KeyDown:
		s.change(s.value - s.keyStep())
	case fyne.KeyRight, fyne.KeyUp:
		s.change(s.value + s.keyStep())
	}
}

func (s *Slider) CreateRenderer() fyne.WidgetRenderer {
	r := &sliderRenderer{s: s}
	r.track = canvas.NewRectangle(style.C(style.Border))
	r.track.CornerRadius = sliderTrack / 2
	r.fill = canvas.NewRectangle(style.C(style.Accent))
	r.fill.CornerRadius = sliderTrack / 2
	r.thumb = canvas.NewCircle(style.C(style.Text))
	r.thumb.StrokeWidth = 2
	r.Refresh()
	return r
}

type sliderRenderer struct {
	s           *Slider
	track, fill *canvas.Rectangle
	thumb       *canvas.Circle
}

func (r *sliderRenderer) MinSize() fyne.Size {
	if r.s.Vertical {
		return fyne.NewSize(2*sliderThumbR+4, 64)
	}
	return fyne.NewSize(64, 2*sliderThumbR+4)
}

func (r *sliderRenderer) Layout(size fyne.Size) {
	ratio := float32(r.s.ratio())
	d := float32(2 * sliderThumbR)
	if r.s.Vertical {
		cx := size.Width / 2
		span := size.Height - d
		ty := sliderThumbR + (1-ratio)*span
		r.track.Move(fyne.NewPos(cx-sliderTrack/2, sliderThumbR))
		r.track.Resize(fyne.NewSize(sliderTrack, span))
		r.fill.Move(fyne.NewPos(cx-sliderTrack/2, ty))
		r.fill.Resize(fyne.NewSize(sliderTrack, size.Height-sliderThumbR-ty))
		r.thumb.Move(fyne.NewPos(cx-sliderThumbR, ty-sliderThumbR))
	} else {
		cy := size.Height / 2
		span := size.Width - d
		tx := sliderThumbR + ratio*span
		r.track.Move(fyne.NewPos(sliderThumbR, cy-sliderTrack/2))
		r.track.Resize(fyne.NewSize(span, sliderTrack))
		r.fill.Move(fyne.NewPos(sliderThumbR, cy-sliderTrack/2))
		r.fill.Resize(fyne.NewSize(tx-sliderThumbR, sliderTrack))
		r.thumb.Move(fyne.NewPos(tx-sliderThumbR, cy-sliderThumbR))
	}
	r.thumb.Resize(fyne.NewSize(d, d))
}

func (r *sliderRenderer) Refresh() {
	r.track.FillColor = withAlpha(style.C(style.BorderStrong), 0x70)
	r.fill.FillColor = style.C(style.Accent)
	r.thumb.FillColor = style.C(style.Text)
	r.thumb.StrokeColor = style.C(style.Inset)
	switch {
	case r.s.disabled:
		r.fill.FillColor = style.C(style.TextFaint)
		r.thumb.FillColor = style.C(style.TextFaint)
	case r.s.focused:
		r.thumb.StrokeColor = style.C(style.Accent)
	}
	r.Layout(r.s.Size())
	r.track.Refresh()
	r.fill.Refresh()
	r.thumb.Refresh()
}

func (r *sliderRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.fill, r.thumb}
}
func (r *sliderRenderer) Destroy() {}
