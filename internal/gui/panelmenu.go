package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
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

// panelChoice is one panel source as offered to the user.
type panelChoice struct {
	label, id string
}

// panelChoices lists the panel sources to offer: automatic USB, each USB
// panel when there are several (or one is chosen, even unplugged), and the
// simulator last.
func panelChoices(tx texts.Texts, list []app.PanelSource, sel string) []panelChoice {
	var serials []string
	for _, p := range list {
		if p.Serial != "" {
			serials = append(serials, p.Serial)
		}
	}
	selSerial, _ := strings.CutPrefix(sel, app.PanelSourceSerialPrefix)
	if selSerial == sel {
		selSerial = ""
	}
	auto := tx.T("app.panel.source.usbAuto")
	if len(serials) == 1 {
		auto = tx.T("app.panel.source.usbOne", "serial", serials[0])
	}
	out := []panelChoice{{auto, app.PanelSourceUSB}}
	if len(serials) > 1 || selSerial != "" {
		found := false
		for _, sn := range serials {
			found = found || sn == selSerial
			out = append(out, panelChoice{tx.T("app.panel.source.usb", "serial", sn), app.PanelSourceSerialPrefix + sn})
		}
		if selSerial != "" && !found {
			out = append(out, panelChoice{tx.T("app.panel.source.usbAbsent", "serial", selSerial), sel})
		}
	}
	return append(out, panelChoice{tx.T("app.panel.source.sim"), app.PanelSourceSim})
}

// panelMenuItems builds the panel source menu; the selected source is
// checked and choosing one calls pick with its id.
func (a *App) panelMenuItems(list []app.PanelSource, sel string, pick func(id string)) []*fyne.MenuItem {
	var items []*fyne.MenuItem
	for _, c := range panelChoices(a.ctl.Tx(), list, sel) {
		if c.id == app.PanelSourceSim {
			items = append(items, fyne.NewMenuItemSeparator())
		}
		m := fyne.NewMenuItem(c.label, func() { pick(c.id) })
		m.Checked = c.id == sel
		items = append(items, m)
	}
	return items
}

// panelAPIOf returns api's panel selection, or nil when it has none.
func panelAPIOf(api any) PanelAPI {
	if p, ok := api.(PanelAPI); ok {
		return p
	}
	return nil
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
