package widgets

import (
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

// --- Icon ------------------------------------------------------------------------

// Icon is a line icon (internal/gui/icons) drawn in a style token, so it
// follows the theme.
type Icon struct {
	widget.BaseWidget
	name  string
	token fyne.ThemeColorName
	size  float32
}

// NewIcon builds an icon of the given name, color token and side in pixels.
func NewIcon(name string, token fyne.ThemeColorName, size float32) *Icon {
	i := &Icon{name: name, token: token, size: size}
	i.ExtendBaseWidget(i)
	return i
}

// Name is the icon's name.
func (i *Icon) Name() string { return i.name }

// SetToken changes the colour of the icon.
func (i *Icon) SetToken(t fyne.ThemeColorName) {
	if i.token != t {
		i.token = t
		i.Refresh()
	}
}

// SetName changes the icon (a name of the icons package).
func (i *Icon) SetName(n string) {
	if i.name != n {
		i.name = n
		i.Refresh()
	}
}

func (i *Icon) CreateRenderer() fyne.WidgetRenderer {
	img := canvas.NewImageFromResource(icons.Res(i.name, style.C(i.token)))
	img.FillMode = canvas.ImageFillContain
	return &iconRenderer{i: i, img: img}
}

type iconRenderer struct {
	i   *Icon
	img *canvas.Image
}

func (r *iconRenderer) MinSize() fyne.Size { return fyne.NewSize(r.i.size, r.i.size) }
func (r *iconRenderer) Layout(s fyne.Size) {
	r.img.Resize(fyne.NewSize(r.i.size, r.i.size))
	r.img.Move(fyne.NewPos((s.Width-r.i.size)/2, (s.Height-r.i.size)/2))
}
func (r *iconRenderer) Refresh() {
	r.img.Resource = icons.Res(r.i.name, style.C(r.i.token))
	r.Layout(r.i.Size())
	r.img.Refresh()
}
func (r *iconRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.img} }
func (r *iconRenderer) Destroy()                     {}

// --- IconButton --------------------------------------------------------------------

// Hold-to-repeat timings of an IconButton.
var (
	RepeatDelay = 400 * time.Millisecond
	RepeatEvery = 90 * time.Millisecond
)

const iconButtonH = 30

// IconButton is a compact button with an icon and an optional label. With
// Repeat set, holding it down repeats OnTapped until the mouse is released.
// Selected draws it in the accent (an active mode); Danger in the tally red.
type IconButton struct {
	widget.BaseWidget
	OnTapped func()
	Repeat   bool

	icon     string
	label    string
	selected bool
	danger   bool
	disabled bool
	hovered  bool
	pressed  bool

	mu       sync.Mutex
	stop     chan struct{}
	repeated bool
}

// NewIconButton builds a button; label may be empty for an icon-only button.
func NewIconButton(icon, label string, tapped func()) *IconButton {
	b := &IconButton{icon: icon, label: label, OnTapped: tapped}
	b.ExtendBaseWidget(b)
	return b
}

// Label returns the text of the button.
func (b *IconButton) Label() string { return b.label }
func (b *IconButton) Icon() string  { return b.icon }

// Disabled reports whether the button ignores taps.
func (b *IconButton) Disabled() bool {
	return b.disabled
}

// SetLabel changes the text of the button.
func (b *IconButton) SetLabel(s string) {
	if b.label != s {
		b.label = s
		b.Refresh()
	}
}

// SetIcon changes the icon of the button.
func (b *IconButton) SetIcon(s string) {
	if b.icon != s {
		b.icon = s
		b.Refresh()
	}
}

// Selected reports whether the button is shown as selected (armed).
func (b *IconButton) Selected() bool { return b.selected }

// SetSelected lights the button (a toggle that is on).
func (b *IconButton) SetSelected(on bool) {
	if b.selected != on {
		b.selected = on
		b.Refresh()
	}
}

// SetDanger draws the button in the danger colour.
func (b *IconButton) SetDanger(on bool) {
	if b.danger != on {
		b.danger = on
		b.Refresh()
	}
}

// SetDisabled greys the button out and makes it ignore taps.
func (b *IconButton) SetDisabled(d bool) {
	if b.disabled != d {
		b.disabled = d
		if d {
			b.stopRepeat()
		}
		b.Refresh()
	}
}

func (b *IconButton) fire() {
	if !b.disabled && b.OnTapped != nil {
		b.OnTapped()
	}
}

// Tapped fires once, unless a held press already repeated.
func (b *IconButton) Tapped(*fyne.PointEvent) {
	b.mu.Lock()
	repeated := b.repeated
	b.repeated = false
	b.mu.Unlock()
	if !repeated {
		b.fire()
	}
}

func (b *IconButton) MouseDown(e *desktop.MouseEvent) {
	if b.disabled || !b.Repeat || e.Button != desktop.MouseButtonPrimary {
		return
	}
	b.pressed = true
	b.Refresh()
	b.mu.Lock()
	if b.stop != nil {
		close(b.stop)
	}
	stop := make(chan struct{})
	b.stop, b.repeated = stop, false
	b.mu.Unlock()
	go func() {
		t := time.NewTimer(RepeatDelay)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				b.mu.Lock()
				b.repeated = true
				b.mu.Unlock()
				fyne.Do(b.fire)
				t.Reset(RepeatEvery)
			}
		}
	}()
}

func (b *IconButton) MouseUp(*desktop.MouseEvent) {
	b.pressed = false
	b.stopRepeat()
	b.Refresh()
}

func (b *IconButton) stopRepeat() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.stop != nil {
		close(b.stop)
		b.stop = nil
	}
}

func (b *IconButton) MouseIn(*desktop.MouseEvent)    { b.hovered = true; b.Refresh() }
func (b *IconButton) MouseMoved(*desktop.MouseEvent) {}
func (b *IconButton) MouseOut() {
	b.hovered = false
	b.pressed = false
	b.stopRepeat()
	b.Refresh()
}

func (b *IconButton) Cursor() desktop.Cursor {
	if b.disabled {
		return desktop.DefaultCursor
	}
	return desktop.PointerCursor
}

func (b *IconButton) CreateRenderer() fyne.WidgetRenderer {
	r := &iconButtonRenderer{b: b}
	r.bg = canvas.NewRectangle(style.C(style.Raised))
	r.bg.CornerRadius = style.Radius
	r.bg.StrokeWidth = 1
	r.icon = NewIcon(b.icon, style.Text, 16)
	r.text = canvas.NewText(b.label, style.C(style.Text))
	r.text.TextSize = 12
	r.text.TextStyle = fyne.TextStyle{Bold: true}
	r.Refresh()
	return r
}

type iconButtonRenderer struct {
	b    *IconButton
	bg   *canvas.Rectangle
	icon *Icon
	text *canvas.Text
}

func (r *iconButtonRenderer) MinSize() fyne.Size {
	w := float32(iconButtonH)
	if r.b.label != "" {
		w = 10 + 16 + 6 + fyne.MeasureText(r.b.label, 12, fyne.TextStyle{Bold: true}).Width + 10
	}
	return fyne.NewSize(w, iconButtonH)
}

func (r *iconButtonRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	if r.b.label == "" {
		r.icon.Resize(fyne.NewSize(16, 16))
		r.icon.Move(fyne.NewPos((s.Width-16)/2, (s.Height-16)/2))
		return
	}
	tm := fyne.MeasureText(r.b.label, 12, fyne.TextStyle{Bold: true})
	x := (s.Width - (16 + 6 + tm.Width)) / 2
	r.icon.Resize(fyne.NewSize(16, 16))
	r.icon.Move(fyne.NewPos(x, (s.Height-16)/2))
	r.text.Move(fyne.NewPos(x+22, (s.Height-tm.Height)/2))
	r.text.Resize(tm)
}

func (r *iconButtonRenderer) Refresh() {
	b := r.b
	fill, fg, stroke := style.C(style.Raised), style.Text, style.C(style.Border)
	switch {
	case b.disabled:
		fill, fg = style.C(style.Surface), style.TextFaint
	case b.danger:
		fill, fg, stroke = style.C(style.Tally), style.OnTally, style.C(style.Tally)
	case b.selected:
		fill, fg, stroke = style.C(style.Accent), style.OnAccent, style.C(style.Accent)
	}
	if !b.disabled && (b.hovered || b.pressed) {
		a := uint8(0x14)
		if b.pressed {
			a = 0x30
		}
		fill = blend(fill, style.C(style.Text), a)
	}
	r.bg.FillColor = fill
	r.bg.StrokeColor = stroke
	r.icon.name = b.icon
	r.icon.SetToken(fg)
	r.icon.Refresh()
	r.text.Text = b.label
	r.text.Color = style.C(fg)
	r.Layout(b.Size())
	r.bg.Refresh()
	r.text.Refresh()
}

func (r *iconButtonRenderer) Objects() []fyne.CanvasObject {
	if r.b.label == "" {
		return []fyne.CanvasObject{r.bg, r.icon}
	}
	return []fyne.CanvasObject{r.bg, r.icon, r.text}
}
func (r *iconButtonRenderer) Destroy() { r.b.stopRepeat() }

// blend mixes over into base at alpha a (0..255).
func blend(base, over color.Color, a uint8) color.Color {
	br, bg, bb, _ := base.RGBA()
	or, og, ob, _ := over.RGBA()
	k := uint32(a)
	mix := func(x, y uint32) uint8 { return uint8(((x>>8)*(255-k) + (y>>8)*k) / 255) }
	return color.NRGBA{R: mix(br, or), G: mix(bg, og), B: mix(bb, ob), A: 0xFF}
}

// --- Readout ----------------------------------------------------------------------

// Readout is a large monospaced value. When OnTapped is set it is clickable
// (to type an exact value).
type Readout struct {
	Text
	OnTapped func()
}

// NewReadout builds a centred bold monospaced readout.
func NewReadout(size float32) *Readout {
	r := &Readout{Text: Text{text: "--", token: style.Text, size: size, bold: true, mono: true, align: fyne.TextAlignCenter}}
	r.ExtendBaseWidget(r)
	return r
}

func (r *Readout) Tapped(*fyne.PointEvent) {
	if r.OnTapped != nil {
		r.OnTapped()
	}
}

func (r *Readout) Cursor() desktop.Cursor {
	if r.OnTapped != nil {
		return desktop.TextCursor
	}
	return desktop.DefaultCursor
}

// --- Ticks ------------------------------------------------------------------------

// Tick is one label of a Ticks row, at a position 0..1 along it.
type Tick struct {
	Pos  float64
	Text string
}

// Ticks draws small labels at given positions, under a horizontal slider
// (inset by the slider's thumb radius so they line up with the track).
type Ticks struct {
	widget.BaseWidget
	ticks []Tick
}

// NewTicks returns an empty scale.
func NewTicks() *Ticks {
	t := &Ticks{}
	t.ExtendBaseWidget(t)
	return t
}

// SetTicks replaces the marks of the scale.
func (t *Ticks) SetTicks(ts []Tick) {
	t.ticks = ts
	t.Refresh()
}

func (t *Ticks) CreateRenderer() fyne.WidgetRenderer { return &ticksRenderer{t: t} }

type ticksRenderer struct {
	t     *Ticks
	texts []*canvas.Text
}

const ticksSize = 10

func (r *ticksRenderer) sync() {
	for len(r.texts) < len(r.t.ticks) {
		r.texts = append(r.texts, canvas.NewText("", style.C(style.TextMuted)))
	}
	r.texts = r.texts[:len(r.t.ticks)]
}

func (r *ticksRenderer) MinSize() fyne.Size {
	return fyne.NewSize(40, fyne.MeasureText("8", ticksSize, fyne.TextStyle{}).Height)
}

func (r *ticksRenderer) Layout(s fyne.Size) {
	r.sync()
	span := s.Width - 2*sliderThumbR
	for i, tx := range r.texts {
		tk := r.t.ticks[i]
		m := fyne.MeasureText(tk.Text, ticksSize, fyne.TextStyle{})
		x := sliderThumbR + float32(tk.Pos)*span - m.Width/2
		x = max(0, min(s.Width-m.Width, x))
		tx.Move(fyne.NewPos(x, 0))
		tx.Resize(m)
	}
}

func (r *ticksRenderer) Refresh() {
	r.sync()
	for i, tx := range r.texts {
		tx.Text = r.t.ticks[i].Text
		tx.TextSize = ticksSize
		tx.Color = style.C(style.TextMuted)
		tx.Refresh()
	}
	r.Layout(r.t.Size())
}

func (r *ticksRenderer) Objects() []fyne.CanvasObject {
	r.sync()
	o := make([]fyne.CanvasObject, len(r.texts))
	for i, t := range r.texts {
		o[i] = t
	}
	return o
}
func (r *ticksRenderer) Destroy() {}
