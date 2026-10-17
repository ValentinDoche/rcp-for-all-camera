package gui

import (
	"sync"

	"fyne.io/fyne/v2"
)

// windowChrome removes the operating system's title bar so the app draws its
// own (icon, name, panel status, minimize/maximize/close), and gives those
// buttons and window dragging to the native window. chrome_windows.go is the
// only implementation; elsewhere the OS title bar stays (Install = false).
type windowChrome interface {
	// Install hides the OS title bar of w. zones tells whether a point
	// belongs to the title bar's free area (dragging it moves the window).
	// It returns false when not supported.
	Install(w fyne.Window, zones *captionZones) bool
	Minimize()
	ToggleMaximize()
	Maximized() bool
}

// captionZones is the title bar geometry the native window asks about. It
// is written on layout and read by the window procedure (both on the main
// thread, the lock is only a safety net).
type captionZones struct {
	mu     sync.Mutex
	scale  float32 // canvas scale: window pixels per Fyne unit
	height float32
	holes  []zone // interactive parts of the title bar (buttons, status)
}

type zone struct {
	pos  fyne.Position
	size fyne.Size
}

func (z *captionZones) set(scale, height float32, holes []zone) {
	if scale <= 0 {
		scale = 1
	}
	z.mu.Lock()
	z.scale, z.height, z.holes = scale, height, holes
	z.mu.Unlock()
}

// atPixel is at for a point in window pixels (the native window's unit).
func (z *captionZones) atPixel(x, y int32) bool {
	z.mu.Lock()
	s := z.scale
	z.mu.Unlock()
	if s <= 0 {
		s = 1
	}
	return z.at(fyne.NewPos(float32(x)/s, float32(y)/s))
}

// at reports whether p is in the title bar but not on one of its controls.
func (z *captionZones) at(p fyne.Position) bool {
	z.mu.Lock()
	defer z.mu.Unlock()
	if p.Y < 0 || p.Y >= z.height {
		return false
	}
	for _, h := range z.holes {
		if p.X >= h.pos.X && p.X < h.pos.X+h.size.Width &&
			p.Y >= h.pos.Y && p.Y < h.pos.Y+h.size.Height {
			return false
		}
	}
	return true
}
