package widgets

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

// SideNav is the category list on the left of the settings window: one row
// per category, the selected one raised with an accent bar.
type SideNav struct {
	widget.BaseWidget
	OnSelect func(i int)

	items    []string
	selected int
	hover    int
}

const sideNavRowH = 36

// NewSideNav returns a vertical list of items; onSelect gets the index tapped.
func NewSideNav(items []string, onSelect func(i int)) *SideNav {
	n := &SideNav{items: items, OnSelect: onSelect, selected: -1, hover: -1}
	n.ExtendBaseWidget(n)
	return n
}

// Selected returns the index of the selected item.
func (n *SideNav) Selected() int { return n.selected }

// SetSelected selects item i without calling the callback.
func (n *SideNav) SetSelected(i int) {
	if n.selected != i {
		n.selected = i
		n.Refresh()
	}
}

func (n *SideNav) rowAt(y float32) int {
	i := int((y - 8) / sideNavRowH)
	if y < 8 || i < 0 || i >= len(n.items) {
		return -1
	}
	return i
}

func (n *SideNav) Tapped(e *fyne.PointEvent) {
	i := n.rowAt(e.Position.Y)
	if i < 0 {
		return
	}
	n.SetSelected(i)
	if n.OnSelect != nil {
		n.OnSelect(i)
	}
}

func (n *SideNav) MouseIn(e *desktop.MouseEvent) { n.MouseMoved(e) }
func (n *SideNav) MouseMoved(e *desktop.MouseEvent) {
	if i := n.rowAt(e.Position.Y); i != n.hover {
		n.hover = i
		n.Refresh()
	}
}
func (n *SideNav) MouseOut() { n.hover = -1; n.Refresh() }

func (n *SideNav) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (n *SideNav) CreateRenderer() fyne.WidgetRenderer {
	r := &sideNavRenderer{n: n, bg: canvas.NewRectangle(style.C(style.Inset)),
		sel: canvas.NewRectangle(style.C(style.Raised)), hov: canvas.NewRectangle(style.C(style.Surface)),
		bar: canvas.NewRectangle(style.C(style.Accent)), edge: canvas.NewRectangle(style.C(style.Border))}
	r.sel.CornerRadius, r.hov.CornerRadius = style.Radius, style.Radius
	r.Refresh()
	return r
}

type sideNavRenderer struct {
	n                       *SideNav
	bg, sel, hov, bar, edge *canvas.Rectangle
	texts                   []*canvas.Text
}

func (r *sideNavRenderer) sync() {
	for len(r.texts) < len(r.n.items) {
		r.texts = append(r.texts, canvas.NewText("", style.C(style.Text)))
	}
	r.texts = r.texts[:len(r.n.items)]
}

func (r *sideNavRenderer) MinSize() fyne.Size {
	w := float32(160)
	for _, it := range r.n.items {
		if m := fyne.MeasureText(it, 13, fyne.TextStyle{Bold: true}).Width + 48; m > w {
			w = m
		}
	}
	return fyne.NewSize(w, 16+sideNavRowH*float32(len(r.n.items)))
}

func (r *sideNavRenderer) row(i int) (fyne.Position, fyne.Size) {
	return fyne.NewPos(8, 8+sideNavRowH*float32(i)+2), fyne.NewSize(r.n.Size().Width-16, sideNavRowH-4)
}

func (r *sideNavRenderer) Layout(s fyne.Size) {
	r.sync()
	r.bg.Resize(s)
	r.edge.Move(fyne.NewPos(s.Width-1, 0))
	r.edge.Resize(fyne.NewSize(1, s.Height))
	for i, t := range r.texts {
		t.Text = r.n.items[i]
		t.TextSize = 13
		p, sz := r.row(i)
		m := fyne.MeasureText(t.Text, 13, fyne.TextStyle{Bold: true})
		t.Move(fyne.NewPos(p.X+16, p.Y+(sz.Height-m.Height)/2))
		t.Resize(fyne.NewSize(sz.Width-20, m.Height))
	}
	if i := r.n.selected; i >= 0 && i < len(r.n.items) {
		p, sz := r.row(i)
		r.sel.Move(p)
		r.sel.Resize(sz)
		r.bar.Move(fyne.NewPos(p.X, p.Y+7))
		r.bar.Resize(fyne.NewSize(3, sz.Height-14))
	}
	if i := r.n.hover; i >= 0 && i < len(r.n.items) {
		p, sz := r.row(i)
		r.hov.Move(p)
		r.hov.Resize(sz)
	}
}

func (r *sideNavRenderer) Refresh() {
	r.sync()
	n := r.n
	r.bg.FillColor = style.C(style.Inset)
	r.edge.FillColor = style.C(style.Border)
	r.sel.FillColor = style.C(style.Raised)
	r.hov.FillColor = style.C(style.Surface)
	r.bar.FillColor = style.C(style.Accent)
	r.sel.Hidden = n.selected < 0 || n.selected >= len(n.items)
	r.bar.Hidden = r.sel.Hidden
	r.hov.Hidden = n.hover < 0 || n.hover >= len(n.items) || n.hover == n.selected
	for i, t := range r.texts {
		if i == n.selected {
			t.Color, t.TextStyle = style.C(style.Text), fyne.TextStyle{Bold: true}
		} else {
			t.Color, t.TextStyle = style.C(style.TextMuted), fyne.TextStyle{}
		}
		t.Refresh()
	}
	r.Layout(n.Size())
	for _, o := range []*canvas.Rectangle{r.bg, r.edge, r.hov, r.sel, r.bar} {
		o.Refresh()
	}
}

func (r *sideNavRenderer) Objects() []fyne.CanvasObject {
	r.sync()
	o := []fyne.CanvasObject{r.bg, r.edge, r.hov, r.sel, r.bar}
	for _, t := range r.texts {
		o = append(o, t)
	}
	return o
}

func (r *sideNavRenderer) Destroy() {}
