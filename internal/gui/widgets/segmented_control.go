package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

const (
	segHeight = 28
	segText   = 12
)

// Segmented is a row of mutually exclusive options. The selected one is
// raised and its text uses the accent.
type Segmented struct {
	widget.BaseWidget
	Options  []string
	OnChange func(i int)

	selected int
	disabled bool
}

func NewSegmented(options []string, onChange func(i int)) *Segmented {
	s := &Segmented{Options: options, OnChange: onChange, selected: -1}
	s.ExtendBaseWidget(s)
	return s
}

func (s *Segmented) Selected() int { return s.selected }

// SetSelected changes the selection without calling OnChange. -1 clears it.
func (s *Segmented) SetSelected(i int) {
	if s.selected != i {
		s.selected = i
		s.Refresh()
	}
}

func (s *Segmented) SetDisabled(d bool) {
	if s.disabled != d {
		s.disabled = d
		s.Refresh()
	}
}

// SetOptions replaces the options and clears the selection.
func (s *Segmented) SetOptions(o []string) {
	s.Options = o
	s.selected = -1
	s.Refresh()
}

func (s *Segmented) Tapped(e *fyne.PointEvent) {
	n := len(s.Options)
	if s.disabled || n == 0 || s.Size().Width <= 0 {
		return
	}
	i := int(e.Position.X / (s.Size().Width / float32(n)))
	if i < 0 {
		i = 0
	}
	if i >= n {
		i = n - 1
	}
	if i != s.selected {
		s.selected = i
		s.Refresh()
		if s.OnChange != nil {
			s.OnChange(i)
		}
	}
}

func (s *Segmented) CreateRenderer() fyne.WidgetRenderer {
	r := &segRenderer{s: s}
	r.bg = canvas.NewRectangle(style.C(style.Inset))
	r.bg.CornerRadius = style.Radius
	r.bg.StrokeWidth = 1
	r.sel = canvas.NewRectangle(style.C(style.Raised))
	r.sel.CornerRadius = style.Radius - 1
	r.sel.StrokeWidth = 1
	r.Refresh()
	return r
}

type segRenderer struct {
	s     *Segmented
	bg    *canvas.Rectangle
	sel   *canvas.Rectangle
	texts []*canvas.Text
}

func (r *segRenderer) sync() {
	for len(r.texts) < len(r.s.Options) {
		r.texts = append(r.texts, canvas.NewText("", style.C(style.Text)))
	}
	r.texts = r.texts[:len(r.s.Options)]
}

func (r *segRenderer) MinSize() fyne.Size {
	var w float32
	for _, o := range r.s.Options {
		tw := fyne.MeasureText(o, segText, fyne.TextStyle{Bold: true}).Width + 20
		if tw > w {
			w = tw
		}
	}
	return fyne.NewSize(w*float32(len(r.s.Options)), segHeight)
}

func (r *segRenderer) Layout(s fyne.Size) {
	r.sync()
	r.bg.Resize(s)
	n := float32(len(r.s.Options))
	if n == 0 {
		return
	}
	w := s.Width / n
	for i, t := range r.texts {
		t.Text = r.s.Options[i]
		t.TextSize = segText
		t.Alignment = fyne.TextAlignCenter
		m := fyne.MeasureText(t.Text, segText, fyne.TextStyle{Bold: true})
		t.Move(fyne.NewPos(w*float32(i), (s.Height-m.Height)/2))
		t.Resize(fyne.NewSize(w, m.Height))
	}
	if r.s.selected >= 0 && r.s.selected < len(r.s.Options) {
		r.sel.Move(fyne.NewPos(w*float32(r.s.selected)+2, 2))
		r.sel.Resize(fyne.NewSize(w-4, s.Height-4))
	}
}

func (r *segRenderer) Refresh() {
	r.sync()
	r.bg.FillColor = style.C(style.Inset)
	r.bg.StrokeColor = style.C(style.BorderStrong)
	r.sel.FillColor = style.C(style.Raised)
	r.sel.StrokeColor = style.C(style.BorderStrong)
	r.sel.Hidden = r.s.selected < 0 || r.s.selected >= len(r.s.Options)
	for i, t := range r.texts {
		t.TextStyle = fyne.TextStyle{Bold: i == r.s.selected}
		switch {
		case r.s.disabled:
			t.Color = style.C(style.TextFaint)
		case i == r.s.selected:
			t.Color = style.C(style.AccentText)
		default:
			t.Color = style.C(style.TextMuted)
		}
		t.Refresh()
	}
	r.Layout(r.s.Size())
	r.bg.Refresh()
	r.sel.Refresh()
}

func (r *segRenderer) Objects() []fyne.CanvasObject {
	r.sync()
	o := []fyne.CanvasObject{r.bg, r.sel}
	for _, t := range r.texts {
		o = append(o, t)
	}
	return o
}
func (r *segRenderer) Destroy() {}

// --- Tabs ----------------------------------------------------------------------

// TabItem is one tab of a Tabs strip.
type TabItem struct {
	Text string
	Dim  bool // empty slot: shown muted
	Live bool // on air: red marker before the label
	// Disabled: not selectable (empty slot); drawn in the faint text color.
	Disabled bool
}

// Tabs is the slot strip (Cam1 to Cam8): text tabs, the selected one in the
// accent color with an underline.
type Tabs struct {
	widget.BaseWidget
	OnSelect func(i int)

	items    []TabItem
	selected int
}

func NewTabs(items []TabItem, onSelect func(i int)) *Tabs {
	t := &Tabs{items: items, OnSelect: onSelect, selected: -1}
	t.ExtendBaseWidget(t)
	return t
}

func (t *Tabs) Selected() int { return t.selected }

// Disabled reports whether tab i cannot be selected.
func (t *Tabs) Disabled(i int) bool { return i >= 0 && i < len(t.items) && t.items[i].Disabled }

func (t *Tabs) SetSelected(i int) {
	if t.selected != i {
		t.selected = i
		t.Refresh()
	}
}

func (t *Tabs) SetItems(items []TabItem) {
	t.items = items
	t.Refresh()
}

const tabW = 104

func (t *Tabs) originX() float32 {
	total := tabW * float32(len(t.items))
	if x := (t.Size().Width - total) / 2; x > 0 {
		return x
	}
	return 0
}

func (t *Tabs) Tapped(e *fyne.PointEvent) {
	i := int((e.Position.X - t.originX()) / tabW)
	if e.Position.X < t.originX() || i < 0 || i >= len(t.items) || t.items[i].Disabled {
		return
	}
	if i != t.selected {
		t.selected = i
		t.Refresh()
	}
	if t.OnSelect != nil {
		t.OnSelect(i)
	}
}

func (t *Tabs) CreateRenderer() fyne.WidgetRenderer {
	r := &tabsRenderer{t: t}
	r.line = canvas.NewRectangle(style.C(style.Border))
	r.under = canvas.NewRectangle(style.C(style.Accent))
	r.Refresh()
	return r
}

type tabsRenderer struct {
	t     *Tabs
	line  *canvas.Rectangle
	under *canvas.Rectangle
	texts []*canvas.Text
	dots  []*canvas.Circle
}

func (r *tabsRenderer) sync() {
	for len(r.texts) < len(r.t.items) {
		r.texts = append(r.texts, canvas.NewText("", style.C(style.Text)))
		r.dots = append(r.dots, canvas.NewCircle(style.C(style.Tally)))
	}
	r.texts = r.texts[:len(r.t.items)]
	r.dots = r.dots[:len(r.t.items)]
}

func (r *tabsRenderer) MinSize() fyne.Size { return fyne.NewSize(tabW*float32(len(r.t.items)), 34) }

func (r *tabsRenderer) Layout(s fyne.Size) {
	r.sync()
	x0 := r.t.originX()
	r.line.Move(fyne.NewPos(0, s.Height-1))
	r.line.Resize(fyne.NewSize(s.Width, 1))
	for i, tx := range r.texts {
		it := r.t.items[i]
		tx.Text = it.Text
		tx.TextSize = 13
		m := fyne.MeasureText(it.Text, 13, tx.TextStyle)
		tx.Move(fyne.NewPos(x0+tabW*float32(i), (s.Height-m.Height)/2-1))
		tx.Resize(fyne.NewSize(tabW, m.Height))
		tx.Alignment = fyne.TextAlignCenter
		r.dots[i].Move(fyne.NewPos(x0+tabW*float32(i)+tabW/2-m.Width/2-12, s.Height/2-4))
		r.dots[i].Resize(fyne.NewSize(7, 7))
	}
	if i := r.t.selected; i >= 0 && i < len(r.t.items) {
		r.under.Move(fyne.NewPos(x0+tabW*float32(i)+14, s.Height-2))
		r.under.Resize(fyne.NewSize(tabW-28, 2))
	}
}

func (r *tabsRenderer) Refresh() {
	r.sync()
	r.line.FillColor = style.C(style.Border)
	r.under.FillColor = style.C(style.Accent)
	r.under.Hidden = r.t.selected < 0 || r.t.selected >= len(r.t.items)
	for i, tx := range r.texts {
		it := r.t.items[i]
		switch {
		case i == r.t.selected:
			tx.Color = style.C(style.AccentText)
			tx.TextStyle = fyne.TextStyle{Bold: true}
		case it.Disabled:
			tx.Color = style.C(style.TextFaint)
			tx.TextStyle = fyne.TextStyle{}
		case it.Dim:
			tx.Color = style.C(style.TextMuted)
			tx.TextStyle = fyne.TextStyle{}
		default:
			tx.Color = style.C(style.Text)
			tx.TextStyle = fyne.TextStyle{}
		}
		r.dots[i].FillColor = style.C(style.Tally)
		r.dots[i].Hidden = !it.Live
		tx.Refresh()
		r.dots[i].Refresh()
	}
	r.Layout(r.t.Size())
	r.line.Refresh()
	r.under.Refresh()
}

func (r *tabsRenderer) Objects() []fyne.CanvasObject {
	r.sync()
	o := []fyne.CanvasObject{r.line, r.under}
	for i := range r.texts {
		o = append(o, r.texts[i], r.dots[i])
	}
	return o
}
func (r *tabsRenderer) Destroy() {}
