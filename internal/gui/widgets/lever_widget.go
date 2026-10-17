package widgets

import (
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

// leverEvery is how often a held Lever calls OnHold.
const leverEvery = 50 * time.Millisecond

// Lever is a spring-loaded control, like a zoom rocker: it follows the
// pointer while dragged and springs back to the centre when released. While
// it is off centre, OnHold is called every leverEvery with the deflection,
// -1..1 (right or up is positive), so the further it is pushed, the faster
// the control moves. A tap gives one OnHold at the tapped deflection.
type Lever struct {
	widget.BaseWidget
	Vertical bool
	OnHold   func(d float64)

	d        float64
	disabled bool

	mu   sync.Mutex
	stop chan struct{}
}

// NewLever returns a spring-loaded lever, vertical or horizontal.
func NewLever(vertical bool) *Lever {
	l := &Lever{Vertical: vertical}
	l.ExtendBaseWidget(l)
	return l
}

// Deflection is the current deflection, -1..1 (0 at rest).
func (l *Lever) Deflection() float64 { return l.d }

// Disabled reports whether the lever is greyed out.
func (l *Lever) Disabled() bool { return l.disabled }

// SetDisabled greys the lever out and makes it ignore drags.
func (l *Lever) SetDisabled(d bool) {
	if l.disabled != d {
		l.disabled = d
		if d {
			l.release()
		}
		l.Refresh()
	}
}

func (l *Lever) deflection(p fyne.Position) float64 {
	s := l.Size()
	var d float64
	if l.Vertical {
		half := float64(s.Height)/2 - sliderThumbR
		if half > 0 {
			d = (float64(s.Height)/2 - float64(p.Y)) / half
		}
	} else {
		half := float64(s.Width)/2 - sliderThumbR
		if half > 0 {
			d = (float64(p.X) - float64(s.Width)/2) / half
		}
	}
	return math.Max(-1, math.Min(1, d))
}

func (l *Lever) Tapped(e *fyne.PointEvent) {
	if l.disabled || l.OnHold == nil {
		return
	}
	if d := l.deflection(e.Position); d != 0 {
		l.OnHold(d)
	}
}

func (l *Lever) Dragged(e *fyne.DragEvent) {
	if l.disabled {
		return
	}
	l.d = l.deflection(e.Position)
	l.Refresh()
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.stop != nil {
		return
	}
	stop := make(chan struct{})
	l.stop = stop
	go func() {
		t := time.NewTicker(leverEvery)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				fyne.Do(func() {
					if l.d != 0 && l.OnHold != nil {
						l.OnHold(l.d)
					}
				})
			}
		}
	}()
}

// DragEnd springs the lever back to the centre.
func (l *Lever) DragEnd() { l.release() }

func (l *Lever) release() {
	l.mu.Lock()
	if l.stop != nil {
		close(l.stop)
		l.stop = nil
	}
	l.mu.Unlock()
	if l.d != 0 {
		l.d = 0
		l.Refresh()
	}
}

func (l *Lever) Cursor() desktop.Cursor {
	if l.disabled {
		return desktop.DefaultCursor
	}
	if l.Vertical {
		return desktop.VResizeCursor
	}
	return desktop.HResizeCursor
}

func (l *Lever) CreateRenderer() fyne.WidgetRenderer {
	r := &leverRenderer{l: l}
	r.track = canvas.NewRectangle(style.C(style.Border))
	r.track.CornerRadius = sliderTrack / 2
	r.fill = canvas.NewRectangle(style.C(style.Accent))
	r.mark = canvas.NewRectangle(style.C(style.TextMuted))
	r.thumb = canvas.NewCircle(style.C(style.Text))
	r.thumb.StrokeWidth = 2
	r.Refresh()
	return r
}

type leverRenderer struct {
	l                 *Lever
	track, fill, mark *canvas.Rectangle
	thumb             *canvas.Circle
}

func (r *leverRenderer) MinSize() fyne.Size {
	if r.l.Vertical {
		return fyne.NewSize(2*sliderThumbR+4, 64)
	}
	return fyne.NewSize(64, 2*sliderThumbR+4)
}

func (r *leverRenderer) Layout(s fyne.Size) {
	d := float32(r.l.d)
	if r.l.Vertical {
		cx, cy := s.Width/2, s.Height/2
		half := cy - sliderThumbR
		ty := cy - d*half
		r.track.Move(fyne.NewPos(cx-sliderTrack/2, sliderThumbR))
		r.track.Resize(fyne.NewSize(sliderTrack, s.Height-2*sliderThumbR))
		r.mark.Move(fyne.NewPos(cx-7, cy-1))
		r.mark.Resize(fyne.NewSize(14, 2))
		r.fill.Move(fyne.NewPos(cx-sliderTrack/2, min(cy, ty)))
		r.fill.Resize(fyne.NewSize(sliderTrack, abs32(cy-ty)))
		r.thumb.Move(fyne.NewPos(cx-sliderThumbR, ty-sliderThumbR))
	} else {
		cx, cy := s.Width/2, s.Height/2
		half := cx - sliderThumbR
		tx := cx + d*half
		r.track.Move(fyne.NewPos(sliderThumbR, cy-sliderTrack/2))
		r.track.Resize(fyne.NewSize(s.Width-2*sliderThumbR, sliderTrack))
		r.mark.Move(fyne.NewPos(cx-1, cy-7))
		r.mark.Resize(fyne.NewSize(2, 14))
		r.fill.Move(fyne.NewPos(min(cx, tx), cy-sliderTrack/2))
		r.fill.Resize(fyne.NewSize(abs32(tx-cx), sliderTrack))
		r.thumb.Move(fyne.NewPos(tx-sliderThumbR, cy-sliderThumbR))
	}
	r.thumb.Resize(fyne.NewSize(2*sliderThumbR, 2*sliderThumbR))
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

func (r *leverRenderer) Refresh() {
	r.track.FillColor = withAlpha(style.C(style.BorderStrong), 0x70)
	r.fill.FillColor = style.C(style.Accent)
	r.mark.FillColor = style.C(style.TextMuted)
	r.thumb.FillColor = style.C(style.Text)
	r.thumb.StrokeColor = style.C(style.Inset)
	if r.l.disabled {
		r.thumb.FillColor = style.C(style.TextFaint)
		r.mark.FillColor = style.C(style.TextFaint)
	}
	r.Layout(r.l.Size())
	r.track.Refresh()
	r.fill.Refresh()
	r.mark.Refresh()
	r.thumb.Refresh()
}

func (r *leverRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.track, r.mark, r.fill, r.thumb}
}
func (r *leverRenderer) Destroy() { r.l.release() }
