package gui

import (
	"strconv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// presetButtonWidth keeps the 1..5 buttons compact and aligned across rows.
const presetButtonWidth = 44

// presetsBlock is the "Presets" panel of the details page: three rows of
// buttons 1..PresetCount to recall, store and delete the presets the app keeps
// for the slot's camera.
type presetsBlock struct {
	ctl  *Controller
	win  fyne.Window
	slot int

	obj    fyne.CanvasObject
	recall [app.PresetCount]*widget.Button
	store  [app.PresetCount]*widget.Button
	del    [app.PresetCount]*widget.Button
}

func newPresetsBlock(ctl *Controller, win fyne.Window, slot int) *presetsBlock {
	b := &presetsBlock{ctl: ctl, win: win, slot: slot}
	tx := ctl.tx
	row := func(key string, btns *[app.PresetCount]*widget.Button, onTap func(n int)) fyne.CanvasObject {
		cells := make([]fyne.CanvasObject, app.PresetCount)
		for i := range btns {
			n := i + 1
			btn := widget.NewButton(strconv.Itoa(n), func() { onTap(n) })
			btns[i] = btn
			cells[i] = widgets.Sized(presetButtonWidth, 0, btn)
		}
		name := widgets.Sized(labelColWidth, 0, widgets.NewText(tx.T(key), style.Text, 13).Clip())
		return container.NewBorder(nil, nil, name, nil, container.NewHBox(cells...))
	}
	rows := container.NewVBox(
		widgets.NewText(tx.T("preset.title"), style.Text, 14).Bold(),
		widgets.Sized(0, 1, widgets.NewHairline()),
		row("preset.recall", &b.recall, func(n int) { ctl.RecallPreset(slot, n) }),
		row("preset.store", &b.store, b.onStore),
		row("preset.delete", &b.del, b.onDelete),
	)
	b.obj = widgets.NewPanel(style.Surface, style.Border, 12, rows)
	b.refresh()
	return b
}

func (b *presetsBlock) has(n int) bool {
	for _, p := range b.ctl.Presets(b.slot) {
		if p == n {
			return true
		}
	}
	return false
}

// refresh enables the buttons that make sense for the stored presets.
func (b *presetsBlock) refresh() {
	for i := range b.recall {
		on := b.has(i + 1)
		setEnabled(b.recall[i], on)
		setEnabled(b.del[i], on)
		setEnabled(b.store[i], true)
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

// ask shows a confirmation dialog, or runs yes at once without a window.
func (b *presetsBlock) ask(title, body, yes string, run func()) {
	if b.win == nil {
		run()
		return
	}
	tx := b.ctl.tx
	showConfirm(b.win, title, body, yes, tx.T("confirm.no"), func(ok bool) {
		if ok {
			run()
		}
	})
}

func (b *presetsBlock) onStore(n int) {
	if !b.has(n) {
		b.ctl.StorePreset(b.slot, n)
		return
	}
	tx := b.ctl.tx
	b.ask(tx.T("preset.replaceTitle"), tx.T("preset.replaceBody", "n", n), tx.T("preset.replaceYes"),
		func() { b.ctl.StorePreset(b.slot, n) })
}

func (b *presetsBlock) onDelete(n int) {
	tx := b.ctl.tx
	b.ask(tx.T("preset.deleteTitle"), tx.T("preset.deleteBody", "n", n), tx.T("preset.deleteYes"),
		func() { b.ctl.DeletePreset(b.slot, n) })
}
