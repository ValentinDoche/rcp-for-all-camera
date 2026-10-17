package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
)

// PanelAPI switches where the panel comes from (*app.Service implements it).
// It is optional: without it the panel status in the top bar is not a menu.
type PanelAPI interface {
	PanelSources() ([]app.PanelSource, string)
	SelectedPanelSource() string
	SelectPanelSource(id string) error
}

var _ PanelAPI = (*app.Service)(nil)

// tapArea makes its content clickable (the panel status in the top bar).
type tapArea struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func(*fyne.PointEvent)
}

func newTapArea(content fyne.CanvasObject, onTap func(*fyne.PointEvent)) *tapArea {
	t := &tapArea{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tapArea) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }

func (t *tapArea) Tapped(e *fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap(e)
	}
}

func (t *tapArea) Cursor() desktop.Cursor { return desktop.PointerCursor }

// panelMenuItems builds the panel source menu: automatic USB, each USB
// panel when there are several (or one is chosen), and the simulator. The
// selected source is checked; choosing one calls pick with its id.
func (a *App) panelMenuItems(list []app.PanelSource, sel string, pick func(id string)) []*fyne.MenuItem {
	tx := a.ctl.Tx()
	var serials []string
	for _, p := range list {
		if p.Serial != "" {
			serials = append(serials, p.Serial)
		}
	}
	selSerial := ""
	if len(sel) > len(app.PanelSourceSerialPrefix) && sel[:len(app.PanelSourceSerialPrefix)] == app.PanelSourceSerialPrefix {
		selSerial = sel[len(app.PanelSourceSerialPrefix):]
	}
	item := func(label, id string) *fyne.MenuItem {
		m := fyne.NewMenuItem(label, func() { pick(id) })
		m.Checked = id == sel
		return m
	}

	auto := tx.T("app.panel.source.usbAuto")
	if len(serials) == 1 {
		auto = tx.T("app.panel.source.usbOne", "serial", serials[0])
	}
	items := []*fyne.MenuItem{item(auto, app.PanelSourceUSB)}
	if len(serials) > 1 || selSerial != "" {
		found := false
		for _, sn := range serials {
			found = found || sn == selSerial
			items = append(items, item(tx.T("app.panel.source.usb", "serial", sn), app.PanelSourceSerialPrefix+sn))
		}
		if selSerial != "" && !found {
			items = append(items, item(tx.T("app.panel.source.usbAbsent", "serial", selSerial), sel))
		}
	}
	return append(items, fyne.NewMenuItemSeparator(), item(tx.T("app.panel.source.sim"), app.PanelSourceSim))
}

// simSelected reports whether the simulator is the selected panel.
func (a *App) simSelected() bool {
	p, ok := a.api.(PanelAPI)
	return ok && p.SelectedPanelSource() == app.PanelSourceSim
}

// showPanelMenu opens the panel source menu under the status.
func (a *App) showPanelMenu(e *fyne.PointEvent) {
	p, ok := a.api.(PanelAPI)
	if !ok {
		return
	}
	list, sel := p.PanelSources()
	if len(list) == 0 {
		return
	}
	items := a.panelMenuItems(list, sel, func(id string) {
		if err := p.SelectPanelSource(id); err != nil {
			a.ctl.SetNotice(a.ctl.Tx().NoticeFor(err))
		}
		a.syncChrome() // the status text names the simulator
	})
	widget.ShowPopUpMenuAtPosition(fyne.NewMenu("", items...), a.win.Canvas(), e.AbsolutePosition)
}
