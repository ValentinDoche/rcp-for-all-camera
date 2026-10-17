package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// detailsPage shows one camera in full: its strip on the left; on the right a
// header with the camera's state, then the blocks the camera's module laid
// out in its ui.go (core.Page): standard cards (iris, gain, shutter, ND,
// white balance, lens, image, presets), color wheels, panels of controls
// with their widgets, and generated sections for every other control.
type detailsPage struct {
	ctl  *Controller
	win  fyne.Window
	slot int
	back func()

	strip  *strip
	obj    *fyne.Container
	body   *fyne.Container
	cancel func()
	head   *detailsHeader

	sig     string
	wheels  []*wheelBlock
	rows    map[core.ControlID]*controlRow
	secs    []*section
	presets *presetsBlock
	// cards lists the controls the exposure and lens cards drive; appliers
	// push the controller's values into those cards.
	cards    map[core.ControlID]bool
	appliers []func()
}

func newDetailsPage(ctl *Controller, win fyne.Window, slot int, back func()) *detailsPage {
	p := &detailsPage{ctl: ctl, win: win, slot: slot, back: back}
	p.strip = newStrip(ctl, slot, stripActions{})
	p.body = container.NewStack()
	p.head = newDetailsHeader(ctl, slot, func() {
		if p.back != nil {
			p.back()
		}
	})
	right := container.NewBorder(p.head.obj, nil, nil, nil, container.NewVScroll(container.New(layout.NewCustomPaddedLayout(8, 8, 0, 8), p.body)))
	p.obj = container.NewBorder(nil, nil, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), p.strip.Object()), nil, right)
	p.cancel = ctl.Subscribe(func(ch Change) {
		switch ch.Kind {
		case ChSlot:
			if ch.Slot == slot {
				p.rebuild()
				p.head.apply()
			}
		case ChValue:
			if ch.Slot == slot {
				p.apply(ch.ID)
				p.head.apply()
			}
		case ChLens:
			if ch.Slot == slot {
				p.applyCards()
			}
		case ChRig:
			p.head.apply()
			if p.presets != nil {
				p.presets.refresh()
			}
		case ChStore:
			if p.presets != nil {
				p.presets.refresh()
			}
		}
	})
	p.rebuild()
	p.head.apply()
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
	p.cards = map[core.ControlID]bool{}
	p.appliers = nil

	p.secs = nil
	p.presets = newPresetsBlock(p.ctl, p.slot)
	page := p.ctl.Page(p.slot)

	// Every block is built in page order, except the ones that show "the
	// rest" (color, remaining): they need to know what the others show, so
	// they are built last and put back in place.
	var deferred []func()
	var rows []fyne.CanvasObject
	for _, r := range page.Rows {
		cells := make([]fyne.CanvasObject, len(r.Blocks))
		for i, b := range r.Blocks {
			i, b := i, b
			if b.Type == core.BlockColor || b.Type == core.BlockRemaining {
				holder := container.NewStack()
				cells[i] = holder
				deferred = append(deferred, func() {
					if o := p.block(b); o != nil {
						holder.Objects = []fyne.CanvasObject{o}
					}
				})
				continue
			}
			cells[i] = p.block(b)
		}
		rows = append(rows, container.NewGridWithColumns(len(cells), cells...))
	}
	// recording is the REC chip of the header
	if sp, ok := p.ctl.Spec(p.slot, core.Record); ok && !sp.ReadOnly {
		p.cards[core.Record] = true
	}
	for _, f := range deferred {
		f()
	}
	if len(p.cards) == 0 && len(p.rows) == 0 && len(p.wheels) == 0 {
		rows = append(rows, container.NewCenter(widgets.NewText(p.ctl.tx.T("details.noControl"), style.TextMuted, 13)))
	}
	p.body.Objects = []fyne.CanvasObject{container.NewVBox(spaced(rows)...)}
	p.body.Refresh()
	p.applyCards()
}

// block builds one block of the module's page.
func (p *detailsPage) block(b core.Block) fyne.CanvasObject {
	switch b.Type {
	case core.BlockExposure:
		return p.exposureBlock()
	case core.BlockColor:
		return p.colorBlock()
	case core.BlockIris:
		return p.irisCard()
	case core.BlockLens:
		return p.lensCard()
	case core.BlockGain:
		return p.gainCard()
	case core.BlockShutter:
		return p.shutterCard()
	case core.BlockND:
		return p.ndCard()
	case core.BlockWhiteBalance:
		return p.balanceCard()
	case core.BlockImage:
		return p.imageCard(b.Control)
	case core.BlockPresets:
		return p.presets.obj
	case core.BlockWheel:
		return p.wheelColumn(wheelIndex(b.Wheel))
	case core.BlockControls:
		s := controlsSection(p.ctl, p.slot, b)
		for _, r := range s.rows {
			p.rows[r.spec.ID] = r
			p.cards[r.spec.ID] = true // shown: the remaining block skips it
		}
		cols := b.Columns
		if cols < 1 {
			cols = 1
		}
		return sectionObject(s, cols)
	case core.BlockGrid:
		cells := make([]fyne.CanvasObject, 0, len(b.Blocks))
		for _, sub := range b.Blocks {
			cells = append(cells, p.block(sub))
		}
		return container.NewGridWithColumns(max(b.Columns, 1), cells...)
	case core.BlockRemaining:
		return p.remainingBlock()
	}
	return nil
}

// wheelColumn is one color wheel in its panel, greyed when the camera has
// no such correction.
func (p *detailsPage) wheelColumn(g int) fyne.CanvasObject {
	wb := newWheelBlock(p.ctl, p.slot, 200)
	wb.setGroup(g)
	p.wheels = append(p.wheels, wb)
	for _, id := range wheelGroups[g].IDs {
		p.cards[id] = true
	}
	tok := style.Text
	if !wheelAvailable(p.ctl, p.slot, g) {
		tok = style.TextFaint
	}
	title := widgets.NewText(p.ctl.tx.T(wheelGroups[g].Key), tok, 14).Bold().Align(fyne.TextAlignCenter)
	return widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(title, wb.obj))
}

func wheelIndex(w core.Wheel) int {
	switch w {
	case core.WheelGamma:
		return 1
	case core.WheelGain:
		return 2
	}
	return 0
}

// remainingSkip is every control another block of the page already shows.
func (p *detailsPage) remainingSkip() map[core.ControlID]bool {
	skip := map[core.ControlID]bool{}
	for id := range p.cards {
		skip[id] = true
	}
	// the master black is the luma of the Lift wheel when there is one
	if p.ctl.Has(p.slot, core.LiftY) && skip[core.LiftY] && wheelAvailable(p.ctl, p.slot, 0) {
		skip[core.MasterBlack] = true
		p.cards[core.MasterBlack] = true
	}
	return skip
}

// remainingBlock shows, in sections by group, every visible control no
// other block of the page shows.
func (p *detailsPage) remainingBlock() fyne.CanvasObject {
	secs := buildSections(p.ctl, p.slot, p.remainingSkip(), false)
	p.secs = append(p.secs, secs...)
	var objs []fyne.CanvasObject
	for _, s := range secs {
		for _, r := range s.rows {
			p.rows[r.spec.ID] = r
		}
		objs = append(objs, sectionObject(s, 1))
	}
	switch len(secs) {
	case 0:
		return nil
	case 1: // a single section takes the full width, two columns of rows
		return sectionObject(secs[0], 2)
	}
	return container.NewGridWithColumns(2, objs...)
}

// colorBlock is the second half of the standard page: the three wheels,
// then the remaining controls. A single remaining section becomes the
// wheels' fourth column, with compact rows, so the standard page fits a
// 1920 x 1080 screen without scrolling.
func (p *detailsPage) colorBlock() fyne.CanvasObject {
	var cols []fyne.CanvasObject
	for g := range wheelGroups {
		cols = append(cols, p.wheelColumn(g))
	}
	skip := p.remainingSkip()
	secs := buildSections(p.ctl, p.slot, skip, false)
	besideWheels := len(secs) <= 1
	if besideWheels {
		secs = buildSections(p.ctl, p.slot, skip, true)
	}
	p.secs = append(p.secs, secs...)
	var lower []fyne.CanvasObject
	for _, s := range secs {
		for _, r := range s.rows {
			p.rows[r.spec.ID] = r
		}
		lower = append(lower, sectionObject(s, 1))
	}
	var out []fyne.CanvasObject
	switch {
	case besideWheels && len(lower) == 0: // keep the fourth column, empty
		cols = append(cols, widgets.NewPanel(style.Surface, style.Border, 12,
			widgets.NewText(p.ctl.tx.GroupTitle(core.GroupColor), style.TextFaint, 14).Bold()))
		out = append(out, container.NewGridWithColumns(len(cols), cols...))
	case besideWheels:
		cols = append(cols, lower[0])
		out = append(out, container.NewGridWithColumns(len(cols), cols...))
	default:
		out = append(out, container.NewGridWithColumns(len(cols), cols...))
		if len(lower) == 1 {
			out = append(out, sectionObject(secs[0], 2))
		} else {
			out = append(out, container.NewGridWithColumns(2, lower...))
		}
	}
	return container.NewVBox(spaced(out)...)
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
	if p.cards[id] {
		p.applyCards()
	}
}

func (p *detailsPage) applyCards() {
	for _, f := range p.appliers {
		f()
	}
}

func (p *detailsPage) applyAll() {
	for _, w := range p.wheels {
		w.apply()
	}
	for id, r := range p.rows {
		r.apply(p.ctl.Value(p.slot, id))
	}
	p.applyCards()
}

// --- header -----------------------------------------------------------------------

// detailsHeader is the top bar: back button, title, and the camera's state as
// chips (on air, recording, connection). The REC chip toggles recording;
// recording is a guarded control, so the operator lock still asks first.
type detailsHeader struct {
	ctl  *Controller
	slot int
	obj  fyne.CanvasObject

	title, sub *widgets.Text
	tally      *widgets.IconButton
	rec        *widgets.IconButton
	connIcon   *widgets.Icon
	connTxt    *widgets.Text
}

func newDetailsHeader(ctl *Controller, slot int, back func()) *detailsHeader {
	h := &detailsHeader{ctl: ctl, slot: slot}
	backBtn := widgets.NewIconButton(icons.Back, ctl.tx.T("details.back"), back)
	h.title = widgets.NewText("", style.Text, 16).Bold()
	h.sub = widgets.NewText("", style.TextMuted, 12)
	h.tally = widgets.NewIconButton(icons.Broadcast, ctl.tx.T("details.onAir"), nil)
	h.tally.SetDanger(true)
	h.rec = widgets.NewIconButton(icons.Record, ctl.tx.T("details.rec"), func() {
		on := false
		if v := ctl.Value(slot, core.Record); v != nil {
			on = v.On
		}
		ctl.Write(slot, core.Record, core.Bool(!on))
	})
	h.connIcon = widgets.NewIcon(icons.Wifi, style.TextMuted, 16)
	h.connTxt = widgets.NewText("", style.TextMuted, 12)
	left := container.NewHBox(backBtn,
		container.New(layout.NewCustomPaddedLayout(0, 0, 8, 0), widgets.NewIcon(icons.Video, style.AccentText, 18)),
		h.title, h.sub)
	right := container.NewHBox(h.tally, h.rec, h.connIcon, h.connTxt)
	h.obj = container.New(layout.NewCustomPaddedLayout(0, 4, 0, 8), container.NewBorder(nil, nil, left, right))
	return h
}

func (h *detailsHeader) apply() {
	tx := h.ctl.tx
	h.title.SetText(tx.T("details.title", "slot", h.slot))
	cam := h.ctl.Camera(h.slot)
	if cam == nil {
		h.sub.SetText("")
		h.connTxt.SetText("")
		h.tally.Hide()
		h.rec.Hide()
		return
	}
	sub := fmt.Sprintf("%s %s", cam.Brand, cam.ModelName)
	if cam.Host != "" {
		sub += " · " + cam.Host
	}
	h.sub.SetText(sub)
	if v := h.ctl.Value(h.slot, core.Tally); v != nil && v.On {
		h.tally.Show()
	} else {
		h.tally.Hide()
	}
	if sp, ok := h.ctl.Spec(h.slot, core.Record); ok && !sp.ReadOnly {
		h.rec.Show()
		v := h.ctl.Value(h.slot, core.Record)
		h.rec.SetDanger(v != nil && v.On)
		h.rec.SetDisabled(cam.Status.State != core.StateConnected)
	} else {
		h.rec.Hide()
	}
	tok, icon := style.TextMuted, icons.WifiOff
	switch cam.Status.State {
	case core.StateConnected:
		tok, icon = style.OK, icons.Wifi
	case core.StateError:
		tok = style.Danger
	}
	h.connIcon.SetName(icon)
	h.connIcon.SetToken(tok)
	h.connTxt.SetToken(tok)
	h.connTxt.SetText(tx.ConnLabel(cam.Status.State))
}
