package widgets

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

// --- Text ----------------------------------------------------------------------

// Text is a single line of text whose color is a style token, so it follows
// the theme (canvas.Text alone does not).
type Text struct {
	widget.BaseWidget
	text  string
	token fyne.ThemeColorName
	size  float32
	bold  bool
	mono  bool
	clip  bool
	align fyne.TextAlign
}

// NewText builds a text in the given token color and size.
func NewText(s string, token fyne.ThemeColorName, size float32) *Text {
	t := &Text{text: s, token: token, size: size, align: fyne.TextAlignLeading}
	t.ExtendBaseWidget(t)
	return t
}

// Bold draws the text in bold.
func (t *Text) Bold() *Text { t.bold = true; return t }

// Mono draws the text in the monospace font.
func (t *Text) Mono() *Text { t.mono = true; return t }

// Clip cuts the text that does not fit, with an ellipsis.
func (t *Text) Clip() *Text { t.clip = true; return t }

// Align sets the horizontal alignment.
func (t *Text) Align(a fyne.TextAlign) *Text { t.align = a; return t }

// SetText changes the text.
func (t *Text) SetText(s string) {
	if t.text != s {
		t.text = s
		t.Refresh()
	}
}

// SetToken changes the colour, a theme colour name.
func (t *Text) SetToken(tok fyne.ThemeColorName) {
	if t.token != tok {
		t.token = tok
		t.Refresh()
	}
}

// Text returns the text shown.
func (t *Text) Text() string { return t.text }

func (t *Text) style() fyne.TextStyle { return fyne.TextStyle{Bold: t.bold, Monospace: t.mono} }

func (t *Text) CreateRenderer() fyne.WidgetRenderer {
	c := canvas.NewText(t.text, style.C(t.token))
	return &textRenderer{t: t, c: c}
}

type textRenderer struct {
	t *Text
	c *canvas.Text
}

func (r *textRenderer) MinSize() fyne.Size {
	m := fyne.MeasureText(r.t.text, r.t.size, r.t.style())
	if r.t.clip {
		m.Width = 8
	}
	return fyne.NewSize(m.Width, m.Height)
}

func (r *textRenderer) Layout(s fyne.Size) {
	txt := r.t.text
	m := fyne.MeasureText(txt, r.t.size, r.t.style())
	if r.t.clip && m.Width > s.Width {
		rs := []rune(txt)
		for len(rs) > 0 {
			rs = rs[:len(rs)-1]
			cand := string(rs) + "…"
			if fyne.MeasureText(cand, r.t.size, r.t.style()).Width <= s.Width {
				txt = cand
				break
			}
		}
		if len(rs) == 0 {
			txt = ""
		}
	}
	r.c.Text = txt
	r.c.Resize(fyne.NewSize(s.Width, m.Height))
	r.c.Move(fyne.NewPos(0, (s.Height-m.Height)/2))
}

func (r *textRenderer) Refresh() {
	r.c.Color = style.C(r.t.token)
	r.c.TextSize = r.t.size
	r.c.TextStyle = r.t.style()
	r.c.Alignment = r.t.align
	r.c.Text = r.t.text
	r.Layout(r.t.Size())
	r.c.Refresh()
}

func (r *textRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.c} }
func (r *textRenderer) Destroy()                     {}

// --- Panel ----------------------------------------------------------------------

// Panel draws a rounded surface (fill and optional outline tokens) behind its
// content, with a uniform inner padding.
type Panel struct {
	widget.BaseWidget
	fill, stroke fyne.ThemeColorName
	content      fyne.CanvasObject
	pad          float32
	flat         bool
}

// NewPanel wraps content in a surface. stroke may be empty for no outline.
func NewPanel(fill, stroke fyne.ThemeColorName, pad float32, content fyne.CanvasObject) *Panel {
	p := &Panel{fill: fill, stroke: stroke, content: content, pad: pad}
	p.ExtendBaseWidget(p)
	return p
}

// Flat removes the corner radius (full-bleed bars).
func (p *Panel) Flat() *Panel { p.flat = true; return p }

// SetFill changes the background colour.
func (p *Panel) SetFill(tok fyne.ThemeColorName) {
	if p.fill != tok {
		p.fill = tok
		p.Refresh()
	}
}

// SetContent replaces what the panel holds.
func (p *Panel) SetContent(o fyne.CanvasObject) {
	p.content = o
	p.Refresh()
}

func (p *Panel) CreateRenderer() fyne.WidgetRenderer {
	r := canvas.NewRectangle(style.C(p.fill))
	r.CornerRadius = style.Radius
	return &panelRenderer{p: p, bg: r}
}

type panelRenderer struct {
	p  *Panel
	bg *canvas.Rectangle
}

func (r *panelRenderer) MinSize() fyne.Size {
	if r.p.content == nil {
		return fyne.NewSize(2*r.p.pad, 2*r.p.pad)
	}
	m := r.p.content.MinSize()
	return fyne.NewSize(m.Width+2*r.p.pad, m.Height+2*r.p.pad)
}

func (r *panelRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	if r.p.content != nil {
		r.p.content.Move(fyne.NewPos(r.p.pad, r.p.pad))
		r.p.content.Resize(fyne.NewSize(s.Width-2*r.p.pad, s.Height-2*r.p.pad))
	}
}

func (r *panelRenderer) Refresh() {
	r.bg.CornerRadius = style.Radius
	if r.p.flat {
		r.bg.CornerRadius = 0
	}
	r.bg.FillColor = style.C(r.p.fill)
	if r.p.stroke != "" {
		r.bg.StrokeColor = style.C(r.p.stroke)
		r.bg.StrokeWidth = 1
	} else {
		r.bg.StrokeColor = color.Transparent
		r.bg.StrokeWidth = 0
	}
	r.Layout(r.p.Size())
	r.bg.Refresh()
	if r.p.content != nil {
		r.p.content.Refresh()
	}
}

func (r *panelRenderer) Objects() []fyne.CanvasObject {
	if r.p.content == nil {
		return []fyne.CanvasObject{r.bg}
	}
	return []fyne.CanvasObject{r.bg, r.p.content}
}
func (r *panelRenderer) Destroy() {}

// --- Hairline and sizing helpers ------------------------------------------------

// Hairline is a 1 px horizontal rule in the border token.
type Hairline struct {
	widget.BaseWidget
	token fyne.ThemeColorName
}

// NewHairline returns a one-pixel separator line.
func NewHairline() *Hairline {
	h := &Hairline{token: style.Border}
	h.ExtendBaseWidget(h)
	return h
}

func (h *Hairline) CreateRenderer() fyne.WidgetRenderer {
	return &hairRenderer{h: h, r: canvas.NewRectangle(style.C(h.token))}
}

type hairRenderer struct {
	h *Hairline
	r *canvas.Rectangle
}

func (r *hairRenderer) MinSize() fyne.Size           { return fyne.NewSize(1, 1) }
func (r *hairRenderer) Layout(s fyne.Size)           { r.r.Resize(s) }
func (r *hairRenderer) Refresh()                     { r.r.FillColor = style.C(r.h.token); r.r.Refresh() }
func (r *hairRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.r} }
func (r *hairRenderer) Destroy()                     {}

type sizedLayout struct{ w, h float32 }

func (l sizedLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	m := fyne.NewSize(0, 0)
	if len(objs) > 0 {
		m = objs[0].MinSize()
	}
	if l.w > 0 {
		m.Width = l.w
	}
	if l.h > 0 {
		m.Height = l.h
	}
	return m
}

func (l sizedLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	for _, o := range objs {
		o.Move(fyne.NewPos(0, 0))
		o.Resize(s)
	}
}

// Sized forces the minimum size of o (a zero side keeps the natural one).
func Sized(w, h float32, o fyne.CanvasObject) fyne.CanvasObject {
	return fyne.NewContainerWithLayout(sizedLayout{w, h}, o)
}

// Dot is a small status dot in a token color. Use it only for a real
// semantic state (connected, on air), never as decoration.
type Dot struct {
	widget.BaseWidget
	token fyne.ThemeColorName
}

// NewDot returns a status dot of colour token.
func NewDot(token fyne.ThemeColorName) *Dot {
	d := &Dot{token: token}
	d.ExtendBaseWidget(d)
	return d
}

// SetToken changes the colour of the dot.
func (d *Dot) SetToken(t fyne.ThemeColorName) {
	if d.token != t {
		d.token = t
		d.Refresh()
	}
}

func (d *Dot) CreateRenderer() fyne.WidgetRenderer {
	return &dotRenderer{d: d, c: canvas.NewCircle(style.C(d.token))}
}

type dotRenderer struct {
	d *Dot
	c *canvas.Circle
}

func (r *dotRenderer) MinSize() fyne.Size { return fyne.NewSize(10, 10) }
func (r *dotRenderer) Layout(s fyne.Size) {
	r.c.Resize(fyne.NewSize(10, 10))
	r.c.Move(fyne.NewPos((s.Width-10)/2, (s.Height-10)/2))
}
func (r *dotRenderer) Refresh()                     { r.c.FillColor = style.C(r.d.token); r.c.Refresh() }
func (r *dotRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.c} }
func (r *dotRenderer) Destroy()                     {}
