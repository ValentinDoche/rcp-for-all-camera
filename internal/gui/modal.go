package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"
)

// configOpen reports whether the Settings window is shown.
func (a *App) configOpen() bool { return a.modal != nil && len(a.modal.Objects) > 0 }

// belowLayout gives its objects the whole area under top (the title bar).
type belowLayout struct{ top fyne.CanvasObject }

func (l belowLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

func (l belowLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	h := l.top.Size().Height
	for _, o := range objs {
		o.Move(fyne.NewPos(0, h))
		o.Resize(fyne.NewSize(s.Width, s.Height-h))
	}
}

// centreSizedLayout centres its objects at the size size(area) gives. Its
// MinSize is zero: the window never grows to fit the modal.
type centreSizedLayout struct{ size func(fyne.Size) fyne.Size }

func (l centreSizedLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(0, 0) }

func (l centreSizedLayout) Layout(objs []fyne.CanvasObject, s fyne.Size) {
	sz := l.size(s)
	for _, o := range objs {
		o.Resize(sz)
		o.Move(fyne.NewPos((s.Width-sz.Width)/2, (s.Height-sz.Height)/2))
	}
}

// blocker dims the page under a modal and swallows its clicks, hovers and
// scrolls, so only the modal (and the title bar above it) can be used.
type blocker struct{ widget.BaseWidget }

func newBlocker() *blocker {
	b := &blocker{}
	b.ExtendBaseWidget(b)
	return b
}

func (b *blocker) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(canvas.NewRectangle(color.NRGBA{0, 0, 0, 0x88}))
}

func (b *blocker) Tapped(*fyne.PointEvent)          {}
func (b *blocker) TappedSecondary(*fyne.PointEvent) {}
func (b *blocker) Scrolled(*fyne.ScrollEvent)       {}
func (b *blocker) MouseIn(*desktop.MouseEvent)      {}
func (b *blocker) MouseMoved(*desktop.MouseEvent)   {}
func (b *blocker) MouseOut()                        {}
