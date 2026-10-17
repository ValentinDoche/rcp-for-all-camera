package gui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/brand"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// Title bar geometry.
const (
	titleBarHeight = 38
	winButtonW     = 46
)

// closeHover is the background of the close button under the mouse: the
// Windows convention, kept identical in both themes.
var closeHover = color.NRGBA{0xC4, 0x2B, 0x1C, 0xFF}

// titleBar replaces the OS title bar: logo and app name on the left, panel
// status (a menu, see panelmenu.go) and the window buttons on the right.
// Its free area moves the window (Windows asks captionZones).
type titleBar struct {
	widget.BaseWidget
	chrome windowChrome
	zones  *captionZones
	native bool // the OS title bar is gone: window buttons shown

	logo     *canvas.Image
	name     *widgets.Text
	status   fyne.CanvasObject
	min, max *winButton
	close    *winButton
}

func newTitleBar(chrome windowChrome, zones *captionZones, status fyne.CanvasObject, onClose func()) *titleBar {
	t := &titleBar{chrome: chrome, zones: zones, status: status}
	t.logo = canvas.NewImageFromResource(brand.Logo)
	t.logo.FillMode = canvas.ImageFillContain
	t.name = widgets.NewText(WindowTitle, style.Text, 14).Bold()
	t.min = newWinButton(icons.Minus, false, chrome.Minimize)
	t.max = newWinButton(icons.Maximize, false, chrome.ToggleMaximize)
	t.close = newWinButton(icons.Close, true, onClose)
	t.ExtendBaseWidget(t)
	return t
}

// setNative shows the window buttons once the OS title bar is removed.
func (t *titleBar) setNative(on bool) {
	t.native = on
	t.Refresh()
}

func (t *titleBar) CreateRenderer() fyne.WidgetRenderer {
	r := &titleBarRenderer{t: t, bg: canvas.NewRectangle(style.C(style.Surface)),
		line: canvas.NewRectangle(style.C(style.Border))}
	return r
}

type titleBarRenderer struct {
	t        *titleBar
	bg, line *canvas.Rectangle
}

func (r *titleBarRenderer) MinSize() fyne.Size {
	w := 40 + r.t.name.MinSize().Width + r.t.status.MinSize().Width + 3*winButtonW + 40
	return fyne.NewSize(w, titleBarHeight)
}

func (r *titleBarRenderer) Layout(s fyne.Size) {
	t := r.t
	r.bg.Resize(s)
	r.line.Move(fyne.NewPos(0, s.Height-1))
	r.line.Resize(fyne.NewSize(s.Width, 1))
	t.logo.Move(fyne.NewPos(12, (s.Height-22)/2))
	t.logo.Resize(fyne.NewSize(22, 22))
	ns := t.name.MinSize()
	t.name.Move(fyne.NewPos(42, (s.Height-ns.Height)/2))
	t.name.Resize(ns)

	right := s.Width - 12
	var holes []zone
	if t.native {
		if t.chrome.Maximized() {
			t.max.setIcon(icons.Restore)
		} else {
			t.max.setIcon(icons.Maximize)
		}
		bs := fyne.NewSize(winButtonW, s.Height-1)
		for i, b := range []*winButton{t.close, t.max, t.min} {
			b.Show()
			p := fyne.NewPos(s.Width-float32(i+1)*winButtonW, 0)
			b.Move(p)
			b.Resize(bs)
			holes = append(holes, zone{p, bs})
		}
		right = s.Width - 3*winButtonW - 12
	} else {
		t.min.Hide()
		t.max.Hide()
		t.close.Hide()
	}
	ss := t.status.MinSize()
	sp := fyne.NewPos(right-ss.Width, (s.Height-ss.Height)/2)
	t.status.Move(sp)
	t.status.Resize(ss)
	holes = append(holes, zone{sp, ss})

	scale := float32(1)
	if c := fyne.CurrentApp().Driver().CanvasForObject(t); c != nil {
		scale = c.Scale()
	}
	t.zones.set(scale, s.Height, holes)
}

func (r *titleBarRenderer) Refresh() {
	r.bg.FillColor = style.C(style.Surface)
	r.line.FillColor = style.C(style.Border)
	r.bg.Refresh()
	r.line.Refresh()
	r.Layout(r.t.Size())
	for _, o := range r.Objects()[2:] {
		o.Refresh()
	}
}

func (r *titleBarRenderer) Objects() []fyne.CanvasObject {
	t := r.t
	return []fyne.CanvasObject{r.bg, r.line, t.logo, t.name, t.status, t.min, t.max, t.close}
}

func (r *titleBarRenderer) Destroy() {}

// installChrome removes the OS title bar once the native window exists
// (it is created when the window is first shown), retrying for a moment.
func (a *App) installChrome(tries int) {
	if a.titleBar.chrome.Install(a.win, a.titleBar.zones) {
		a.titleBar.setNative(true)
		return
	}
	if tries > 0 {
		time.AfterFunc(50*time.Millisecond, func() { fyne.Do(func() { a.installChrome(tries - 1) }) })
	}
}

// --- window buttons ----------------------------------------------------------

// winButton is a minimize, maximize/restore or close button.
type winButton struct {
	widget.BaseWidget
	icon   string
	danger bool // close: red under the mouse
	hover  bool
	onTap  func()
}

func newWinButton(icon string, danger bool, onTap func()) *winButton {
	b := &winButton{icon: icon, danger: danger, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *winButton) setIcon(name string) {
	if b.icon != name {
		b.icon = name
		b.Refresh()
	}
}

func (b *winButton) Tapped(*fyne.PointEvent) {
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *winButton) MouseIn(*desktop.MouseEvent)    { b.hover = true; b.Refresh() }
func (b *winButton) MouseMoved(*desktop.MouseEvent) {}
func (b *winButton) MouseOut()                      { b.hover = false; b.Refresh() }

func (b *winButton) CreateRenderer() fyne.WidgetRenderer {
	r := &winButtonRenderer{b: b, bg: canvas.NewRectangle(color.Transparent), img: canvas.NewImageFromResource(nil)}
	r.img.FillMode = canvas.ImageFillContain
	r.Refresh()
	return r
}

type winButtonRenderer struct {
	b   *winButton
	bg  *canvas.Rectangle
	img *canvas.Image
}

func (r *winButtonRenderer) MinSize() fyne.Size { return fyne.NewSize(winButtonW, titleBarHeight-1) }

func (r *winButtonRenderer) Layout(s fyne.Size) {
	r.bg.Resize(s)
	const ic = 14
	r.img.Move(fyne.NewPos((s.Width-ic)/2, (s.Height-ic)/2))
	r.img.Resize(fyne.NewSize(ic, ic))
}

func (r *winButtonRenderer) Refresh() {
	b := r.b
	fg := style.C(style.TextMuted)
	var bg color.Color = color.Transparent
	switch {
	case b.hover && b.danger:
		bg, fg = closeHover, color.White
	case b.hover:
		bg, fg = style.C(style.Raised), style.C(style.Text)
	}
	r.bg.FillColor = bg
	r.img.Resource = icons.Res(b.icon, fg)
	r.bg.Refresh()
	r.img.Refresh()
}

func (r *winButtonRenderer) Objects() []fyne.CanvasObject { return []fyne.CanvasObject{r.bg, r.img} }
func (r *winButtonRenderer) Destroy()                     {}
