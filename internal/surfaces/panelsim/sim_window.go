package panelsim

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// WindowTitle is the title of the simulator window.
const WindowTitle = "ATEM Micro Camera Panel simulator"

// help explains the mouse mapping.
const help = "Click: button (held while the mouse button is down)\n" +
	"Knobs: drag up/down or use the mouse wheel\n" +
	"Lever: drag up/down = iris, left/right or wheel = master black"

// NewWindow returns the simulator window: the panel drawing, a "plugged in"
// check box and a short help. Closing the window hides it and unplugs the
// panel; Show brings it back (plug it in again with Sim.SetPlugged).
func NewWindow(a fyne.App, s *Sim) fyne.Window {
	w := a.NewWindow(WindowTitle)
	plugged := widget.NewCheck("Panel plugged in", s.SetPlugged)
	plugged.SetChecked(s.Plugged())
	hint := widget.NewLabel(help)
	hint.Importance = widget.LowImportance
	v := NewView(s)
	v.afterRefresh = func() { plugged.SetChecked(s.Plugged()) }
	w.SetContent(container.NewBorder(nil, container.NewVBox(plugged, hint), nil, nil,
		container.NewCenter(v)))
	w.SetFixedSize(true)
	w.SetCloseIntercept(func() {
		s.SetPlugged(false)
		w.Hide()
	})
	return w
}
