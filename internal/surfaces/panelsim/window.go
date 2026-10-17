package panelsim

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// WindowTitle is the title of the simulator window.
const WindowTitle = "Simulateur ATEM Micro Camera Panel"

// help explains the mouse mapping (French, like the owner's other tools).
const help = "Clic : bouton (maintenu tant que la souris est enfoncée)\n" +
	"Molettes : glisser haut/bas ou molette de la souris\n" +
	"Levier : glisser haut/bas = iris, gauche/droite ou molette = master black"

// NewWindow returns the simulator window: the panel drawing, a "plugged in"
// check box and a short help. Closing the window unplugs the panel.
func NewWindow(a fyne.App, s *Sim) fyne.Window {
	w := a.NewWindow(WindowTitle)
	plugged := widget.NewCheck("Panneau branché", s.SetPlugged)
	plugged.SetChecked(s.Plugged())
	hint := widget.NewLabel(help)
	hint.Importance = widget.LowImportance
	w.SetContent(container.NewBorder(nil, container.NewVBox(plugged, hint), nil, nil,
		container.NewCenter(NewView(s))))
	w.SetFixedSize(true)
	w.SetOnClosed(func() { s.SetPlugged(false) })
	return w
}
