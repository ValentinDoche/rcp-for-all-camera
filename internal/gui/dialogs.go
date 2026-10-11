package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// showConfirm asks a yes/no question; cb receives the answer (false on
// cancel or dismissal). Nothing is ever confirmed by default.
func showConfirm(win fyne.Window, title, body, yes, no string, cb func(bool)) {
	lbl := widget.NewLabel(body)
	lbl.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm(title, yes, no, widgets.Sized(380, 0, lbl), cb, win)
	d.Show()
}
