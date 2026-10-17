//go:build windows

package gui

import (
	"sync"
	"syscall"
	"unsafe"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

// The OS title bar is removed by subclassing the window procedure:
// WM_NCCALCSIZE gives the whole window to the client area (the window keeps
// its styles, so Windows still animates, snaps and draws the shadow), and
// WM_NCHITTEST hands the resize borders and the free part of the app's own
// title bar back to Windows, which then moves, snaps and maximizes the
// window on double-click by itself.

var (
	user32                = syscall.NewLazyDLL("user32.dll")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procCallWindowProcW   = user32.NewProc("CallWindowProcW")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procIsZoomed          = user32.NewProc("IsZoomed")
	procShowWindow        = user32.NewProc("ShowWindow")
	procGetWindowRect     = user32.NewProc("GetWindowRect")
	procScreenToClient    = user32.NewProc("ScreenToClient")
	procGetSystemMetrics  = user32.NewProc("GetSystemMetrics")
	procGetDpiForWindow   = user32.NewProc("GetDpiForWindow")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procAdjustRectForDpi  = user32.NewProc("AdjustWindowRectExForDpi")
	procAdjustRect        = user32.NewProc("AdjustWindowRectEx")
)

const (
	gwlpWndProc = ^uintptr(3)  // -4
	gwlStyle    = ^uintptr(15) // -16
	gwlExStyle  = ^uintptr(19) // -20

	wmGetMinMaxInfo     = 0x0024
	wmWindowPosChanging = 0x0046
	wmEnterSizeMove     = 0x0231
	wmExitSizeMove      = 0x0232
	wmNCCalcSize        = 0x0083
	wmNCHitTest         = 0x0084

	htClient      = 1
	htCaption     = 2
	htLeft        = 10
	htRight       = 11
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	htBottom      = 15
	htBottomLeft  = 16
	htBottomRight = 17

	swMinimize = 6
	swMaximize = 3
	swRestore  = 9

	swpNoSize       = 0x0001
	swpNoMove       = 0x0002
	swpNoZOrder     = 0x0004
	swpFrameChanged = 0x0020

	smCxSizeFrame    = 32
	smCySizeFrame    = 33
	smCxPaddedBorder = 92
)

type rect struct{ Left, Top, Right, Bottom int32 }

type point struct{ X, Y int32 }

// windowPos is WINDOWPOS.
type windowPos struct {
	hwnd, insertAfter uintptr
	X, Y, CX, CY      int32
	Flags             uint32
}

// minMaxInfo is MINMAXINFO.
type minMaxInfo struct {
	Reserved, MaxSize, MaxPosition, MinTrackSize, MaxTrackSize point
}

type winChrome struct {
	mu     sync.Mutex
	hwnd   uintptr
	prev   uintptr
	zones  *captionZones
	sizing bool // the user is moving or resizing the window (modal loop)
}

// The window procedure is a C callback: one per process, routed by hwnd.
var (
	chromeMu   sync.Mutex
	chromes    = map[uintptr]*winChrome{}
	wndProcPtr = syscall.NewCallback(chromeWndProc)
)

func newWindowChrome() windowChrome { return &winChrome{} }

func (c *winChrome) Install(w fyne.Window, zones *captionZones) bool {
	nw, ok := w.(driver.NativeWindow)
	if !ok {
		return false
	}
	var hwnd uintptr
	nw.RunNative(func(ctx any) {
		if wc, ok := ctx.(driver.WindowsWindowContext); ok {
			hwnd = wc.HWND
		}
	})
	if hwnd == 0 {
		return false
	}
	c.mu.Lock()
	c.hwnd, c.zones = hwnd, zones
	c.mu.Unlock()
	chromeMu.Lock()
	chromes[hwnd] = c
	chromeMu.Unlock()
	prev, _, _ := procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, wndProcPtr)
	if prev == 0 {
		chromeMu.Lock()
		delete(chromes, hwnd)
		chromeMu.Unlock()
		return false
	}
	c.mu.Lock()
	c.prev = prev
	c.mu.Unlock()
	// Recompute the frame now that WM_NCCALCSIZE is ours.
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoZOrder|swpFrameChanged)
	return true
}

func (c *winChrome) handle() uintptr {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hwnd
}

func (c *winChrome) Minimize() {
	if h := c.handle(); h != 0 {
		procShowWindow.Call(h, swMinimize)
	}
}

func (c *winChrome) ToggleMaximize() {
	h := c.handle()
	if h == 0 {
		return
	}
	if c.Maximized() {
		procShowWindow.Call(h, swRestore)
	} else {
		procShowWindow.Call(h, swMaximize)
	}
}

func (c *winChrome) Maximized() bool {
	h := c.handle()
	if h == 0 {
		return false
	}
	z, _, _ := procIsZoomed.Call(h)
	return z != 0
}

func dpiOf(hwnd uintptr) int32 {
	if procGetDpiForWindow.Find() == nil {
		if d, _, _ := procGetDpiForWindow.Call(hwnd); d != 0 {
			return int32(d)
		}
	}
	return 96
}

// frameSize is how much a captioned frame adds to the client size of hwnd.
func frameSize(hwnd uintptr) (w, h int32) {
	style, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlStyle)
	ex, _, _ := procGetWindowLongPtrW.Call(hwnd, gwlExStyle)
	var r rect
	if procAdjustRectForDpi.Find() == nil {
		procAdjustRectForDpi.Call(uintptr(unsafe.Pointer(&r)), style, 0, ex, uintptr(dpiOf(hwnd)))
	} else {
		procAdjustRect.Call(uintptr(unsafe.Pointer(&r)), style, 0, ex)
	}
	return r.Right - r.Left, r.Bottom - r.Top
}

func metric(i int) int32 {
	v, _, _ := procGetSystemMetrics.Call(uintptr(i))
	return int32(v)
}

func chromeWndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	chromeMu.Lock()
	c := chromes[hwnd]
	chromeMu.Unlock()
	if c == nil {
		return 0
	}
	c.mu.Lock()
	prev, zones, sizing := c.prev, c.zones, c.sizing
	c.mu.Unlock()

	switch msg {
	case wmEnterSizeMove, wmExitSizeMove:
		c.mu.Lock()
		c.sizing = msg == wmEnterSizeMove
		c.mu.Unlock()
	case wmWindowPosChanging:
		// Fyne asks GLFW for the window's current size again whenever the
		// content changes, and GLFW adds the frame of a captioned window,
		// which this window does not have: it grew by that frame at each
		// page change. Such a request (outside a user move or resize, not
		// maximized) is brought back to the size the content asked for.
		wp := *(**windowPos)(unsafe.Pointer(&lparam)) // lparam is a pointer from Windows
		if !sizing && wp.Flags&swpNoSize == 0 {
			if z, _, _ := procIsZoomed.Call(hwnd); z == 0 {
				var wr rect
				procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
				fw, fh := frameSize(hwnd)
				if wp.CX == wr.Right-wr.Left+fw && wp.CY == wr.Bottom-wr.Top+fh {
					wp.CX -= fw
					wp.CY -= fh
				}
			}
		}
	case wmGetMinMaxInfo:
		// GLFW turns Fyne's minimum content size into a minimum window size
		// by adding the frame of a captioned window, but here the client
		// area is the whole window: without taking that frame back, every
		// content change (Fyne resets the limits) grew the window.
		r, _, _ := procCallWindowProcW.Call(prev, hwnd, msg, wparam, lparam)
		mmi := *(**minMaxInfo)(unsafe.Pointer(&lparam)) // lparam is a pointer from Windows
		fw, fh := frameSize(hwnd)
		if mmi.MinTrackSize.X > fw {
			mmi.MinTrackSize.X -= fw
		}
		if mmi.MinTrackSize.Y > fh {
			mmi.MinTrackSize.Y -= fh
		}
		return r
	case wmNCCalcSize:
		if wparam == 0 {
			break
		}
		// lparam points to NCCALCSIZE_PARAMS, whose first field is the
		// proposed window rectangle: keeping it makes the client area the
		// whole window. A maximized window overhangs the screen by its
		// frame, which is taken back so nothing is cut off.
		if z, _, _ := procIsZoomed.Call(hwnd); z != 0 {
			r := *(**rect)(unsafe.Pointer(&lparam)) // lparam is a pointer from Windows
			fx := metric(smCxSizeFrame) + metric(smCxPaddedBorder)
			fy := metric(smCySizeFrame) + metric(smCxPaddedBorder)
			r.Left += fx
			r.Right -= fx
			r.Top += fy
			r.Bottom -= fy
		}
		return 0
	case wmNCHitTest:
		x, y := int32(int16(lparam&0xFFFF)), int32(int16((lparam>>16)&0xFFFF))
		var wr rect
		procGetWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&wr)))
		zoomed, _, _ := procIsZoomed.Call(hwnd)
		if zoomed == 0 {
			b := 6 * dpiOf(hwnd) / 96
			left, right := x < wr.Left+b, x >= wr.Right-b
			top, bottom := y < wr.Top+b, y >= wr.Bottom-b
			switch {
			case top && left:
				return htTopLeft
			case top && right:
				return htTopRight
			case bottom && left:
				return htBottomLeft
			case bottom && right:
				return htBottomRight
			case left:
				return htLeft
			case right:
				return htRight
			case top:
				return htTop
			case bottom:
				return htBottom
			}
		}
		if zones != nil {
			p := point{x, y}
			procScreenToClient.Call(hwnd, uintptr(unsafe.Pointer(&p)))
			if zones.atPixel(p.X, p.Y) {
				return htCaption
			}
		}
		return htClient
	}
	r, _, _ := procCallWindowProcW.Call(prev, hwnd, msg, wparam, lparam)
	return r
}
