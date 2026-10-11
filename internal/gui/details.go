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
// header with the camera's state, the exposure cards (iris, gain, shutter,
// ND, white balance), the lens cards (focus, zoom), the presets, the three
// wheels with the image sliders, and a generated section for every other
// control.
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

	var blocks []fyne.CanvasObject

	// exposure, lens, image and presets cards
	p.presets = newPresetsBlock(p.ctl, p.slot)
	if b := p.exposureBlock(); b != nil {
		blocks = append(blocks, b)
	}
	// recording is the REC chip of the header
	if sp, ok := p.ctl.Spec(p.slot, core.Record); ok && !sp.ReadOnly {
		p.cards[core.Record] = true
	}

	// three wheels, greyed when the camera has no such correction
	var cols []fyne.CanvasObject
	for g := range wheelGroups {
		wb := newWheelBlock(p.ctl, p.slot, 200)
		wb.setGroup(g)
		p.wheels = append(p.wheels, wb)
		tok := style.Text
		if !wheelAvailable(p.ctl, p.slot, g) {
			tok = style.TextFaint
		}
		title := widgets.NewText(p.ctl.tx.T(wheelGroups[g].Key), tok, 14).Bold().Align(fyne.TextAlignCenter)
		cols = append(cols, widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(title, wb.obj)))
	}

	// the other controls: half-width panels side by side, or one full-width
	// panel when there is a single section
	skip := wheelControlIDs()
	for id := range p.cards {
		skip[id] = true
	}
	// the master black is the luma of the Lift wheel when there is one
	if p.ctl.Has(p.slot, core.LiftY) && wheelAvailable(p.ctl, p.slot, 0) {
		skip[core.MasterBlack] = true
		p.cards[core.MasterBlack] = true
	}
	var lower []fyne.CanvasObject

	// every remaining control. A single section next to the wheels becomes
	// their fourth column, with compact rows, so the whole page fits the
	// screen (1920 x 1080) without scrolling.
	p.secs = buildSections(p.ctl, p.slot, skip, false)
	besideWheels := len(p.secs) <= 1
	if besideWheels {
		p.secs = buildSections(p.ctl, p.slot, skip, true)
	}
	for _, s := range p.secs {
		for _, r := range s.rows {
			p.rows[r.spec.ID] = r
		}
		lower = append(lower, sectionObject(s, 1))
	}
	switch {
	case besideWheels && len(lower) == 0: // keep the fourth column, empty
		cols = append(cols, widgets.NewPanel(style.Surface, style.Border, 12,
			widgets.NewText(p.ctl.tx.GroupTitle(core.GroupColor), style.TextFaint, 14).Bold()))
		blocks = append(blocks, container.NewGridWithColumns(len(cols), cols...))
	case besideWheels:
		cols = append(cols, lower[0])
		blocks = append(blocks, container.NewGridWithColumns(len(cols), cols...))
	case len(cols) > 0:
		blocks = append(blocks, container.NewGridWithColumns(len(cols), cols...))
	}
	switch {
	case besideWheels || len(lower) == 0:
	case len(lower) == 1: // a single section takes the full width, two columns of rows
		blocks = append(blocks, sectionObject(p.secs[0], 2))
	default:
		blocks = append(blocks, container.NewGridWithColumns(2, lower...))
	}
	if len(p.cards) == 0 && len(p.rows) == 0 && len(p.wheels) == 0 {
		blocks = append(blocks, container.NewCenter(widgets.NewText(p.ctl.tx.T("details.noControl"), style.TextMuted, 13)))
	}
	p.body.Objects = []fyne.CanvasObject{container.NewVBox(spaced(blocks)...)}
	p.body.Refresh()
	p.applyCards()
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
