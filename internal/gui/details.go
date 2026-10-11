package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// imageSliderIDs get a dedicated block under the wheels.
var imageSliderIDs = []core.ControlID{core.Contrast, core.Saturation, core.Hue}

// detailsPage shows one camera in full: its strip on the left, three large
// wheels, the image sliders and a generated section for every other control.
type detailsPage struct {
	ctl  *Controller
	win  fyne.Window
	slot int
	back func()

	strip  *strip
	obj    *fyne.Container
	body   *fyne.Container
	cancel func()

	sig     string
	wheels  []*wheelBlock
	rows    map[core.ControlID]*controlRow
	secs    []*section
	sliders []*controlRow
	presets *presetsBlock
}

func newDetailsPage(ctl *Controller, win fyne.Window, slot int, back func()) *detailsPage {
	p := &detailsPage{ctl: ctl, win: win, slot: slot, back: back}
	p.strip = newStrip(ctl, slot, stripActions{})
	p.body = container.NewStack()
	backBtn := widget.NewButtonWithIcon(ctl.tx.T("details.back"), theme.NavigateBackIcon(), func() {
		if p.back != nil {
			p.back()
		}
	})
	title := widgets.NewText("", style.Text, 16).Bold()
	sub := widgets.NewText("", style.TextMuted, 12)
	setTitle := func() {
		title.SetText(ctl.tx.T("details.title", "slot", slot))
		if cam := ctl.Camera(slot); cam != nil {
			sub.SetText(fmt.Sprintf("%s %s", cam.Brand, cam.ModelName))
		}
	}
	setTitle()
	head := container.NewBorder(nil, nil, backBtn, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 8, 0), container.NewHBox(title, sub)))
	right := container.NewBorder(head, nil, nil, nil, container.NewVScroll(container.New(layout.NewCustomPaddedLayout(8, 8, 0, 8), p.body)))
	p.obj = container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), p.strip.Object()), nil, right)
	p.cancel = ctl.Subscribe(func(ch Change) {
		switch ch.Kind {
		case ChSlot:
			if ch.Slot == slot {
				p.rebuild()
			}
		case ChValue:
			if ch.Slot == slot {
				p.apply(ch.ID)
			}
		case ChRig:
			if p.presets != nil {
				p.presets.refresh()
			}
		}
	})
	p.rebuild()
	return p
}

// Close stops listening and closes the embedded strip.
func (p *detailsPage) Close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
	p.strip.Close()
}

func (p *detailsPage) rebuild() {
	if !p.ctl.Loaded(p.slot) {
		p.body.Objects = []fyne.CanvasObject{
			container.NewCenter(widgets.NewText(p.ctl.tx.T("strip.reading"), style.TextMuted, 13)),
		}
		p.body.Refresh()
		p.sig = ""
		return
	}
	var sb []byte
	for _, sp := range p.ctl.Specs(p.slot) {
		sb = append(sb, sp.ID...)
		sb = append(sb, ',')
	}
	if string(sb) == p.sig && len(p.body.Objects) > 0 {
		p.applyAll()
		return
	}
	p.sig = string(sb)
	p.rows = map[core.ControlID]*controlRow{}
	p.wheels = nil
	p.sliders = nil

	var blocks []fyne.CanvasObject

	// three wheels
	var cols []fyne.CanvasObject
	for g := range wheelGroups {
		if !wheelAvailable(p.ctl, p.slot, g) {
			continue
		}
		wb := newWheelBlock(p.ctl, p.slot, 200)
		wb.setGroup(g)
		p.wheels = append(p.wheels, wb)
		title := widgets.NewText(p.ctl.tx.T(wheelGroups[g].Key), style.Text, 14).Bold().Align(fyne.TextAlignCenter)
		cols = append(cols, widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(title, wb.obj)))
	}
	if len(cols) > 0 {
		blocks = append(blocks, container.NewGridWithColumns(len(cols), cols...))
	}

	// contrast, saturation, hue
	skip := wheelControlIDs()
	var sl []fyne.CanvasObject
	for _, id := range imageSliderIDs {
		if sp, ok := p.ctl.Spec(p.slot, id); ok {
			row := newControlRow(p.ctl, p.slot, sp)
			p.sliders = append(p.sliders, row)
			p.rows[id] = row
			sl = append(sl, container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4), row.obj))
			skip[id] = true
		}
	}
	if len(sl) > 0 {
		blocks = append(blocks, widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(
			widgets.NewText(p.ctl.tx.T("details.image"), style.Text, 14).Bold(),
			widgets.Sized(0, 1, widgets.NewHairline()),
			container.NewGridWithColumns(len(sl), sl...),
		)))
	}

	// every remaining control
	p.secs = buildSections(p.ctl, p.slot, skip)
	for _, s := range p.secs {
		for _, r := range s.rows {
			p.rows[r.spec.ID] = r
		}
		blocks = append(blocks, sectionObject(s))
	}
	if len(blocks) == 0 {
		blocks = append(blocks, container.NewCenter(widgets.NewText(p.ctl.tx.T("details.noControl"), style.TextMuted, 13)))
	}
	// local presets come first, so they are reachable without scrolling
	p.presets = newPresetsBlock(p.ctl, p.win, p.slot)
	blocks = append([]fyne.CanvasObject{p.presets.obj}, blocks...)
	p.body.Objects = []fyne.CanvasObject{container.NewVBox(spaced(blocks)...)}
	p.body.Refresh()
}

// spaced inserts a gap between blocks.
func spaced(blocks []fyne.CanvasObject) []fyne.CanvasObject {
	out := make([]fyne.CanvasObject, len(blocks))
	for i, b := range blocks {
		out[i] = container.New(layout.NewCustomPaddedLayout(0, 6, 0, 0), b)
	}
	return out
}

func (p *detailsPage) apply(id core.ControlID) {
	for _, w := range p.wheels {
		w.apply()
	}
	if r := p.rows[id]; r != nil {
		r.apply(p.ctl.Value(p.slot, id))
	}
}

func (p *detailsPage) applyAll() {
	for _, w := range p.wheels {
		w.apply()
	}
	for id, r := range p.rows {
		r.apply(p.ctl.Value(p.slot, id))
	}
}
