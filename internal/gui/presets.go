package gui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// presetsBlock is the "Presets" card of the details page. It works like the
// panel's scene buttons: 1..PresetCount recall a stored preset; to store,
// press Store first (it lights up), then the number. Arming Store is the
// confirmation, so replacing a preset asks nothing more.
type presetsBlock struct {
	ctl  *Controller
	slot int

	obj   fyne.CanvasObject
	nums  [app.PresetCount]*widget.Button
	store *widgets.IconButton
	armed bool
}

func newPresetsBlock(ctl *Controller, slot int) *presetsBlock {
	b := &presetsBlock{ctl: ctl, slot: slot}
	cells := make([]fyne.CanvasObject, app.PresetCount)
	for i := range b.nums {
		n := i + 1
		b.nums[i] = widget.NewButton(strconv.Itoa(n), func() { b.press(n) })
		cells[i] = b.nums[i]
	}
	b.store = widgets.NewIconButton(icons.Save, ctl.tx.T("preset.store"), func() { b.arm(!b.armed) })
	b.obj = card(icons.Bookmarks, ctl.tx.T("preset.title"), true, b.store, container.NewGridWithColumns(app.PresetCount, cells...))
	b.refresh()
	return b
}

// press recalls preset n, or stores it when Store is armed.
func (b *presetsBlock) press(n int) {
	if b.armed {
		b.arm(false)
		b.ctl.StorePreset(b.slot, n)
		return
	}
	b.ctl.RecallPreset(b.slot, n)
}

func (b *presetsBlock) arm(on bool) {
	b.armed = on
	b.refresh()
}

func (b *presetsBlock) has(n int) bool {
	for _, p := range b.ctl.Presets(b.slot) {
		if p == n {
			return true
		}
	}
	return false
}

// refresh: a stored preset can be recalled; while Store is armed every
// number can be pressed and stands out.
func (b *presetsBlock) refresh() {
	b.store.SetSelected(b.armed)
	for i, btn := range b.nums {
		imp := widget.MediumImportance
		if b.armed {
			imp = widget.HighImportance
		}
		if btn.Importance != imp {
			btn.Importance = imp
			btn.Refresh()
		}
		setEnabled(btn, b.armed || b.has(i+1))
	}
}

func setEnabled(btn *widget.Button, on bool) {
	if on == !btn.Disabled() {
		return
	}
	if on {
		btn.Enable()
	} else {
		btn.Disable()
	}
}
