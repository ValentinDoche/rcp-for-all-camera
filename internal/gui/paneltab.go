package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// panelTab is Settings > Panel: the panel source, the same choice as the
// title bar's panel menu.
type panelTab struct {
	tx      texts.Texts
	api     PanelAPI // nil: no choice available
	obj     fyne.CanvasObject
	radio   *widget.RadioGroup
	ids     map[string]string // label -> source id
	quiet   bool
	changed func() // after a switch (title bar status)
	notice  func(error)
}

func newPanelTab(tx texts.Texts, api PanelAPI, changed func(), notice func(error)) *panelTab {
	p := &panelTab{tx: tx, api: api, changed: changed, notice: notice}
	if api == nil {
		p.obj = centred(680, widgets.NewPanel(style.Surface, style.Border, 12,
			widgets.NewText(tx.T("config.panel.unavailable"), style.TextMuted, 13)))
		return p
	}
	p.radio = widget.NewRadioGroup(nil, p.onPick)
	refresh := widget.NewButton(tx.T("config.panel.refresh"), p.Refresh)
	row := newSettingRow(tx.T("config.panel.source"), p.radio, tx.T("config.panel.help"))
	p.obj = centred(680, widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(
		container.NewBorder(nil, nil, nil, refresh, widgets.NewText(tx.T("config.tab.panel"), style.Text, 14).Bold()),
		widgets.Sized(0, 1, widgets.NewHairline()),
		row.obj,
	)))
	p.Refresh()
	return p
}

// Refresh lists the sources again (USB panels plugged in since).
func (p *panelTab) Refresh() {
	if p.api == nil {
		return
	}
	list, sel := p.api.PanelSources()
	p.ids = map[string]string{}
	var labels []string
	selLabel := ""
	for _, c := range panelChoices(p.tx, list, sel) {
		labels = append(labels, c.label)
		p.ids[c.label] = c.id
		if c.id == sel {
			selLabel = c.label
		}
	}
	p.quiet = true
	p.radio.Options = labels
	p.radio.SetSelected(selLabel)
	p.radio.Refresh()
	p.quiet = false
}

func (p *panelTab) onPick(label string) {
	id, ok := p.ids[label]
	if p.quiet || !ok {
		return
	}
	if err := p.api.SelectPanelSource(id); err != nil && p.notice != nil {
		p.notice(err)
	}
	p.Refresh()
	if p.changed != nil {
		p.changed()
	}
}
