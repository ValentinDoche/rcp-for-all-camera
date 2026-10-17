package gui

import (
	"math"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// Every camera gets the same cards in the same places. A control the camera
// does not declare is greyed, never removed, so the operator always finds
// things where they expect them.

// cardTitle is the icon and title row of a card, with an optional action
// (Auto iris, AF, AWB) on the right. A card whose controls are all missing
// has a faint title.
func cardTitle(icon, title string, on bool, action fyne.CanvasObject) fyne.CanvasObject {
	iconTok, textTok := style.AccentText, style.Text
	if !on {
		iconTok, textTok = style.TextFaint, style.TextFaint
	}
	left := container.NewHBox(widgets.NewIcon(icon, iconTok, 18), widgets.NewText(title, textTok, 14).Bold())
	if action == nil {
		return left
	}
	return container.NewBorder(nil, nil, left, action)
}

// card is one panel of the exposure and lens rows.
func card(icon, title string, on bool, action fyne.CanvasObject, body ...fyne.CanvasObject) fyne.CanvasObject {
	items := append([]fyne.CanvasObject{cardTitle(icon, title, on, action)}, body...)
	return widgets.NewPanel(style.Surface, style.Border, 10, container.NewVBox(items...))
}

// caption is the small muted line under a value.
func caption() *widgets.Text {
	return widgets.NewText("", style.TextMuted, 11).Align(fyne.TextAlignCenter)
}

// has reports whether the camera declares id.
func (p *detailsPage) has(id core.ControlID) bool { return p.ctl.Has(p.slot, id) }

// actionButton is the Auto iris / AF / AWB button of a card, greyed when the
// camera does not declare the action.
func (p *detailsPage) actionButton(id core.ControlID, icon, label string) fyne.CanvasObject {
	b := widgets.NewIconButton(icon, label, func() { p.ctl.Write(p.slot, id, core.Trigger()) })
	if p.has(id) {
		p.cards[id] = true
	} else {
		b.SetDisabled(true)
	}
	return b
}

// stepButton is a − / + style button that repeats while held.
func stepButton(icon string, f func()) *widgets.IconButton {
	b := widgets.NewIconButton(icon, "", f)
	b.Repeat = true
	return b
}

// --- steppers ---------------------------------------------------------------------

// stepper is a [−] value [+] row for one range or enum control. Each tap moves
// the value by one step (one option for an enum) from where the control is
// heading, so quick taps add up. A range value can be typed by clicking it.
// A control the camera does not declare gives a greyed stepper.
type stepper struct {
	obj     fyne.CanvasObject
	readout *widgets.Readout
	spec    core.ControlSpec
}

func (p *detailsPage) newStepper(id core.ControlID, size float32, step float64) *stepper {
	sp, ok := p.ctl.Spec(p.slot, id)
	s := &stepper{spec: sp, readout: widgets.NewReadout(size)}
	move := func(dir int) {
		v, ok := p.stepValue(sp, step, dir)
		if !ok {
			return
		}
		p.ctl.Write(p.slot, id, v)
		s.readout.SetText(p.display(sp, &v))
	}
	minus, plus := stepButton(icons.Minus, func() { move(-1) }), stepButton(icons.Plus, func() { move(1) })
	s.obj = container.NewBorder(nil, nil, minus, plus, s.readout)
	if !ok {
		minus.SetDisabled(true)
		plus.SetDisabled(true)
		s.readout.SetToken(style.TextFaint)
		return s
	}
	p.cards[id] = true
	if sp.Kind == core.KindRange && !sp.ReadOnly {
		s.readout.OnTapped = func() { p.askValue(sp) }
	}
	if sp.ReadOnly {
		minus.SetDisabled(true)
		plus.SetDisabled(true)
	}
	p.appliers = append(p.appliers, func() { s.readout.SetText(p.display(sp, p.ctl.Value(p.slot, id))) })
	return s
}

// stepValue is the value one step away (dir -1 or +1) from the control's
// target; ok is false when it is unknown or already at the end.
func (p *detailsPage) stepValue(sp core.ControlSpec, step float64, dir int) (core.Value, bool) {
	cur := p.ctl.Target(p.slot, sp.ID)
	switch sp.Kind {
	case core.KindRange:
		base := (sp.Min + sp.Max) / 2
		if cur != nil && cur.Kind == core.KindRange {
			base = cur.Number
		}
		v := widgets.Quantize(base+float64(dir)*step, widgets.RangeOf(sp))
		return core.Number(v), cur == nil || v != base
	case core.KindEnum:
		i := optionIndex(sp, cur)
		if i < 0 {
			i = len(sp.Options) / 2
		} else {
			i += dir
		}
		if i < 0 || i >= len(sp.Options) {
			return core.Value{}, false
		}
		return core.Choice(sp.Options[i].Value), true
	}
	return core.Value{}, false
}

// display is the explicit text of a value: "f/4.0", "+12 dB", "1/50 s".
func (p *detailsPage) display(sp core.ControlSpec, v *core.Value) string {
	if v == nil || v.Kind != sp.Kind {
		return texts.Missing
	}
	if sp.Kind == core.KindRange {
		return p.ctl.tx.Explicit(sp, v.Number)
	}
	txt := texts.ShortLabel(p.ctl.tx.FormatValue(sp, v))
	if sp.ID == core.ShutterSpeed && strings.HasPrefix(txt, "1/") {
		txt += " s"
	}
	return txt
}

// askValue lets the operator type an exact value, in the unit shown.
func (p *detailsPage) askValue(sp core.ControlSpec) {
	if p.win == nil {
		return
	}
	tx := p.ctl.tx
	label := tx.ControlLabel(sp)
	entry := widget.NewEntry()
	entry.SetText(p.display(sp, p.ctl.Value(p.slot, sp.ID)))
	hint := tx.T("details.typeRange", "min", tx.Explicit(sp, sp.Min), "max", tx.Explicit(sp, sp.Max))
	items := []*widget.FormItem{{Text: label, Widget: entry, HintText: hint}}
	d := dialog.NewForm(tx.T("details.typeTitle", "label", label), tx.T("details.typeApply"), tx.T("confirm.no"), items, func(ok bool) {
		if !ok {
			return
		}
		v, good := texts.ParseExplicit(sp, entry.Text)
		if !good {
			p.ctl.SetNotice(texts.Notice{Text: tx.T("controlrow.numberExpected", "label", label), Level: texts.LevelError})
			return
		}
		r := widgets.RangeOf(sp)
		if sp.ID == core.Iris { // inside the lens, not just the control's range
			k := widgets.NewIrisScale(sp, p.ctl.Lens(p.slot))
			r.Min, r.Max = k.Min, k.Max
		}
		p.ctl.Write(p.slot, sp.ID, core.Number(widgets.Quantize(v, r)))
	}, p.win)
	d.Resize(fyne.NewSize(360, 0))
	d.Show()
	p.win.Canvas().Focus(entry)
}

// --- exposure ---------------------------------------------------------------------

// exposureBlock lays the cards out in two halves of equal height: on the
// left the iris, then the lens (zoom and focus) under it; on the right
// compact half-height cards in two columns: gain, shutter, ND, white
// balance, contrast, saturation, hue and the presets.
func (p *detailsPage) exposureBlock() fyne.CanvasObject {
	left := []fyne.CanvasObject{p.irisCard(), p.lensCard()}
	right := []fyne.CanvasObject{p.gainCard(), p.shutterCard(), p.ndCard(), p.balanceCard()}
	for _, id := range imageIDs {
		right = append(right, p.imageCard(id))
	}
	right = append(right, p.presets.obj)
	return container.NewGridWithColumns(2,
		container.NewGridWithRows(len(left), left...),
		container.NewGridWithColumns(2, right...))
}

// irisCard: the f-stop in large, − / + by a third of a stop, « » by a full
// stop, and a bar graduated in full stops (closed on the left, open on the
// right, so + moves it right).
func (p *detailsPage) irisCard() fyne.CanvasObject {
	tx := p.ctl.tx
	sp, ok := p.ctl.Spec(p.slot, core.Iris)
	readout := widgets.NewReadout(28)
	bar := widgets.NewSlider(0, 1)
	ticks := widgets.NewTicks()
	scale := func() widgets.IrisScale { return widgets.NewIrisScale(sp, p.ctl.Lens(p.slot)) }
	write := func(v float64) {
		p.ctl.Write(p.slot, core.Iris, core.Number(v))
		readout.SetText(tx.Explicit(sp, v))
		bar.Set(scale().Open(v))
	}
	move := func(n int) {
		cur := p.ctl.Target(p.slot, core.Iris)
		if cur == nil {
			return
		}
		if v := scale().StepBy(cur.Number, n); v != cur.Number {
			write(v)
		}
	}
	btns := []*widgets.IconButton{
		stepButton(icons.ChevronsLeft, func() { move(-3) }), stepButton(icons.Minus, func() { move(-1) }),
		stepButton(icons.Plus, func() { move(1) }), stepButton(icons.ChevronsRight, func() { move(3) }),
	}
	row := container.NewBorder(nil, nil, container.NewHBox(btns[0], btns[1]), container.NewHBox(btns[2], btns[3]), readout)
	auto := p.actionButton(core.AutoIris, icons.Wand, tx.T("strip.autoIris"))
	body := []fyne.CanvasObject{widgets.Sized(0, 40, row), bar, ticks}
	if !ok {
		for _, b := range btns {
			b.SetDisabled(true)
		}
		bar.SetDisabled(true)
		readout.SetToken(style.TextFaint)
		return card(icons.Aperture, tx.T("details.iris"), false, auto, body...)
	}
	p.cards[core.Iris] = true
	bar.OnChanged = func(open float64) { write(scale().At(open)) }
	if !sp.ReadOnly {
		readout.OnTapped = func() { p.askValue(sp) }
	}
	p.appliers = append(p.appliers, func() {
		k := scale()
		v := p.ctl.Value(p.slot, core.Iris)
		readout.SetText(p.display(sp, v))
		if v != nil {
			bar.Set(k.Open(v.Number))
		}
		var ts []widgets.Tick
		for _, f := range k.Labels() {
			ts = append(ts, widgets.Tick{Pos: k.Open(f), Text: texts.Iris(f)})
		}
		ticks.SetTicks(ts)
	})
	return card(icons.Aperture, tx.T("details.iris"), true, auto, body...)
}

// gainCard shows the gain the way the camera takes it: in dB when it
// declares a gain, else in ISO. The other one is not shown.
func (p *detailsPage) gainCard() fyne.CanvasObject {
	tx := p.ctl.tx
	switch {
	case p.has(core.Gain):
		p.cards[core.ISO] = true // hidden: the gain says it
		sp, _ := p.ctl.Spec(p.slot, core.Gain)
		return card(icons.Brightness, tx.T("details.gain"), true, nil, p.newStepper(core.Gain, 16, stepOf(sp)).obj)
	case p.has(core.ISO):
		return card(icons.Brightness, "ISO", true, nil, p.newStepper(core.ISO, 16, 0).obj)
	}
	return card(icons.Brightness, tx.T("details.gain"), false, nil, p.newStepper(core.Gain, 16, 0).obj)
}

// shutterCard shows the speed, with the matching angle next to the title,
// or the angle alone.
func (p *detailsPage) shutterCard() fyne.CanvasObject {
	tx := p.ctl.tx
	switch {
	case p.has(core.ShutterSpeed):
		st := p.newStepper(core.ShutterSpeed, 16, 0)
		p.cards[core.ShutterAngle] = true // shown next to the title
		angle := widgets.NewText("", style.TextMuted, 11)
		p.appliers = append(p.appliers, func() {
			angle.SetText("")
			if a, ok := p.ctl.Spec(p.slot, core.ShutterAngle); ok {
				if v := p.ctl.Value(p.slot, core.ShutterAngle); v != nil {
					angle.SetText(tx.FormatValue(a, v))
				}
			}
		})
		return card(icons.Stopwatch, tx.T("details.shutter"), true, angle, st.obj)
	case p.has(core.ShutterAngle):
		sp, _ := p.ctl.Spec(p.slot, core.ShutterAngle)
		return card(icons.Stopwatch, tx.T("details.shutter"), true, nil, p.newStepper(core.ShutterAngle, 16, math.Max(stepOf(sp), 1)).obj)
	}
	return card(icons.Stopwatch, tx.T("details.shutter"), false, nil, p.newStepper(core.ShutterSpeed, 16, 0).obj)
}

// ndCard: one button per filter position.
func (p *detailsPage) ndCard() fyne.CanvasObject {
	tx := p.ctl.tx
	sp, ok := p.ctl.Spec(p.slot, core.ND)
	if !ok || sp.Kind != core.KindEnum {
		seg := widgets.NewSegmented([]string{texts.Missing}, nil)
		seg.SetDisabled(true)
		return card(icons.Filter, tx.T("details.nd"), false, nil, seg)
	}
	p.cards[core.ND] = true
	labels := make([]string, len(sp.Options))
	for i, o := range sp.Options {
		labels[i] = strings.TrimPrefix(texts.ShortLabel(o.Label), "ND ")
		if o.Value == "0" {
			labels[i] = tx.T("details.ndClear")
		}
	}
	quiet := false
	seg := widgets.NewSegmented(labels, func(i int) {
		if !quiet {
			p.ctl.Write(p.slot, core.ND, core.Choice(sp.Options[i].Value))
		}
	})
	seg.SetDisabled(sp.ReadOnly)
	p.appliers = append(p.appliers, func() {
		quiet = true
		seg.SetSelected(optionIndex(sp, p.ctl.Value(p.slot, core.ND)))
		quiet = false
	})
	return card(icons.Filter, tx.T("details.nd"), true, nil, seg)
}

// balanceCard: colour temperature and tint side by side, with AWB.
func (p *detailsPage) balanceCard() fyne.CanvasObject {
	tx := p.ctl.tx
	wbStep, tintStep := 100.0, 1.0
	if sp, ok := p.ctl.Spec(p.slot, core.WhiteBalance); ok {
		wbStep = math.Max(stepOf(sp), 100)
	}
	if sp, ok := p.ctl.Spec(p.slot, core.Tint); ok {
		tintStep = stepOf(sp)
	}
	on := p.has(core.WhiteBalance) || p.has(core.Tint)
	return card(icons.Temperature, tx.T("details.balance"), on, p.actionButton(core.AutoWhiteBalance, icons.Wand, tx.T("strip.awb")),
		container.NewGridWithColumns(2, p.newStepper(core.WhiteBalance, 16, wbStep).obj, p.newStepper(core.Tint, 14, tintStep).obj))
}

// imageIDs are the image controls shown as compact cards next to the
// exposure.
var imageIDs = []core.ControlID{core.Contrast, core.Saturation, core.Hue}

// imageCard: contrast, saturation or hue with − / +, in %, or degrees for
// the hue, and a reset to neutral.
func (p *detailsPage) imageCard(id core.ControlID) fyne.CanvasObject {
	sp, ok := p.ctl.Spec(p.slot, id)
	reset := widgets.NewIconButton(icons.Reset, "", nil)
	if !ok || sp.Kind != core.KindRange {
		reset.SetDisabled(true)
		return card(icons.Palette, p.ctl.tx.ControlLabel(core.Std(id)), false, reset, p.newStepper(id, 16, 0).obj)
	}
	if n, ok := neutralOf(sp); ok && !sp.ReadOnly {
		reset.OnTapped = func() { p.ctl.Write(p.slot, id, core.Number(n)) }
	} else {
		reset.SetDisabled(true)
	}
	return card(icons.Palette, p.ctl.tx.ControlLabel(sp), true, reset, p.newStepper(id, 16, stepOf(sp)).obj)
}

// --- lens -------------------------------------------------------------------------

// lensCard is the lens: the zoom lever above the focus, with AF.
func (p *detailsPage) lensCard() fyne.CanvasObject {
	tx := p.ctl.tx
	on := p.has(core.Zoom) || p.has(core.Focus)
	af := p.actionButton(core.AutoFocus, icons.FocusAuto, tx.T("details.af"))
	body := append(p.zoomRows(), p.focusRows()...)
	return card(icons.Camera, tx.GroupTitle(core.GroupLens), on, af, body...)
}

// zoomRate is how far the zoom moves per lever tick at full deflection, as a
// share of its travel (a full sweep in about 2.5 s).
const zoomRate = 0.02

// zoomHold moves a position zoom while its lever is held: the further the
// lever is pushed (d, -1..1), the faster it goes. Shared by the strip and
// the details page.
func zoomHold(ctl *Controller, slot int, sp core.ControlSpec, d float64) {
	cur := ctl.Target(slot, core.Zoom)
	if cur == nil {
		return
	}
	v := widgets.Quantize(cur.Number+d*zoomRate*(sp.Max-sp.Min), widgets.RangeOf(sp))
	if v != cur.Number {
		ctl.Write(slot, core.Zoom, core.Number(v))
	}
}

// zoomRows: a spring-loaded lever, like the zoom rocker of a lens: push it
// towards Tele or Wide to zoom, faster the further it goes; released, it
// springs back to the centre and the zoom stops. The focal length in mm
// when the camera reports it.
func (p *detailsPage) zoomRows() []fyne.CanvasObject {
	tx := p.ctl.tx
	sp, ok := p.ctl.Spec(p.slot, core.Zoom)
	lever := widgets.NewLever(false)
	row := container.NewBorder(nil, nil,
		container.NewHBox(widgets.NewIcon(icons.ZoomOut, style.TextMuted, 16), widgets.NewText(tx.T("details.wide"), style.TextMuted, 11)),
		container.NewHBox(widgets.NewText(tx.T("details.tele"), style.TextMuted, 11), widgets.NewIcon(icons.ZoomIn, style.TextMuted, 16)),
		lever)
	sub := caption()
	if !ok || sp.Kind != core.KindRange || sp.ReadOnly {
		lever.SetDisabled(true)
		sub.SetText(tx.T("details.zoom") + " " + texts.Missing)
		return []fyne.CanvasObject{row, sub}
	}
	p.cards[core.Zoom] = true
	lever.OnHold = func(d float64) { zoomHold(p.ctl, p.slot, sp, d) }
	p.appliers = append(p.appliers, func() {
		l := p.ctl.Lens(p.slot)
		txt := tx.T("details.zoom") + " " + p.display(sp, p.ctl.Value(p.slot, core.Zoom))
		if l.FocalLength > 0 {
			txt = tx.T("details.zoom") + " " + tx.Focal(l.FocalLength)
			if l.FocalMax > l.FocalMin && l.FocalMin > 0 {
				txt += " · " + tx.T("details.range", "min", tx.Focal(l.FocalMin), "max", tx.Focal(l.FocalMax))
			}
		}
		sub.SetText(txt)
	})
	return []fyne.CanvasObject{row, sub}
}

// focusRows: ‹ › fine and « » coarse steps around the focus travel, the
// lens' focus range and, when the camera reports them, the distance and
// the focal length of a lens without power zoom.
func (p *detailsPage) focusRows() []fyne.CanvasObject {
	tx := p.ctl.tx
	sp, ok := p.ctl.Spec(p.slot, core.Focus)
	sl := widgets.NewSlider(0, 1)
	if ok {
		sl.Min, sl.Max, sl.Step = sp.Min, sp.Max, stepOf(sp)
	}
	span := sp.Max - sp.Min
	move := func(d float64) {
		v, ok := p.stepValue(sp, math.Abs(d), int(math.Copysign(1, d)))
		if ok {
			p.ctl.Write(p.slot, core.Focus, v)
			sl.Set(v.Number)
		}
	}
	btns := []*widgets.IconButton{
		stepButton(icons.ChevronsLeft, func() { move(-span / 20) }), stepButton(icons.ChevronLeft, func() { move(-span / 100) }),
		stepButton(icons.ChevronRight, func() { move(span / 100) }), stepButton(icons.ChevronsRight, func() { move(span / 20) }),
	}
	row := container.NewBorder(nil, nil,
		container.NewHBox(btns[0], btns[1], widgets.NewText(tx.T("details.near"), style.TextMuted, 11)),
		container.NewHBox(widgets.NewText("∞", style.TextMuted, 11), btns[2], btns[3]), sl)
	sub := caption()
	if !ok || sp.Kind != core.KindRange || sp.ReadOnly {
		for _, b := range btns {
			b.SetDisabled(true)
		}
		sl.SetDisabled(true)
		sub.SetText(tx.T("details.focus") + " " + texts.Missing)
		return []fyne.CanvasObject{row, sub}
	}
	p.cards[core.Focus] = true
	sl.OnChanged = func(v float64) {
		p.ctl.Write(p.slot, core.Focus, core.Number(widgets.Quantize(v, widgets.RangeOf(sp))))
	}
	p.appliers = append(p.appliers, func() {
		v := p.ctl.Value(p.slot, core.Focus)
		if v != nil && v.Kind == core.KindRange {
			sl.Set(v.Number)
		}
		l := p.ctl.Lens(p.slot)
		first := p.display(sp, v)
		if l.FocusDistance > 0 {
			first = tx.Distance(l.FocusDistance) + " (" + first + ")"
		}
		parts := []string{tx.T("details.focus") + " " + first}
		if l.FocusNear > 0 && l.FocusFar > 0 {
			parts = append(parts, tx.T("details.focusRange", "near", tx.Distance(l.FocusNear), "far", tx.Distance(l.FocusFar)))
		}
		if !p.has(core.Zoom) && l.FocalLength > 0 {
			parts = append(parts, tx.T("details.focal", "focal", tx.Focal(l.FocalLength)))
		}
		sub.SetText(strings.Join(parts, " · "))
	})
	return []fyne.CanvasObject{row, sub}
}
