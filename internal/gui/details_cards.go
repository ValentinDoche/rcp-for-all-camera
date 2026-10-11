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

// cardTitle is the icon and title row of a card, with an optional action
// (Auto iris, AF, AWB) on the right.
func cardTitle(icon, title string, action fyne.CanvasObject) fyne.CanvasObject {
	left := container.NewHBox(widgets.NewIcon(icon, style.AccentText, 18), widgets.NewText(title, style.Text, 14).Bold())
	if action == nil {
		return left
	}
	return container.NewBorder(nil, nil, left, action)
}

// card is one panel of the exposure and lens rows.
func card(icon, title string, action fyne.CanvasObject, body ...fyne.CanvasObject) fyne.CanvasObject {
	items := append([]fyne.CanvasObject{cardTitle(icon, title, action)}, body...)
	return widgets.NewPanel(style.Surface, style.Border, 10, container.NewVBox(items...))
}

// caption is the small muted line under a value.
func caption() *widgets.Text {
	return widgets.NewText("", style.TextMuted, 11).Align(fyne.TextAlignCenter)
}

// actionButton is the Auto iris / AF / AWB button of a card, nil when the
// camera does not declare the action.
func (p *detailsPage) actionButton(id core.ControlID, icon, label string) fyne.CanvasObject {
	if !p.ctl.Has(p.slot, id) {
		return nil
	}
	p.cards[id] = true
	return widgets.NewIconButton(icon, label, func() { p.ctl.Write(p.slot, id, core.Trigger()) })
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
type stepper struct {
	obj     fyne.CanvasObject
	readout *widgets.Readout
	spec    core.ControlSpec
}

func (p *detailsPage) newStepper(id core.ControlID, size float32, step float64) *stepper {
	sp, _ := p.ctl.Spec(p.slot, id)
	s := &stepper{spec: sp, readout: widgets.NewReadout(size)}
	p.cards[id] = true
	move := func(dir int) {
		v, ok := p.stepValue(sp, step, dir)
		if !ok {
			return
		}
		p.ctl.Write(p.slot, id, v)
		s.readout.SetText(p.display(sp, &v))
	}
	if sp.Kind == core.KindRange && !sp.ReadOnly {
		s.readout.OnTapped = func() { p.askValue(sp) }
	}
	minus, plus := stepButton(icons.Minus, func() { move(-1) }), stepButton(icons.Plus, func() { move(1) })
	if sp.ReadOnly {
		minus.SetDisabled(true)
		plus.SetDisabled(true)
	}
	s.obj = container.NewBorder(nil, nil, minus, plus, s.readout)
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
		p.ctl.Write(p.slot, sp.ID, core.Number(widgets.Quantize(v, widgets.RangeOf(sp))))
	}, p.win)
	d.Resize(fyne.NewSize(360, 0))
	d.Show()
	p.win.Canvas().Focus(entry)
}

// --- exposure ---------------------------------------------------------------------

// exposureBlock lays the cards out in two halves of equal height: on the
// left the iris, then the lens (focus and zoom) under it; on the right
// compact half-height cards in two columns: gain, shutter, ND, white
// balance, then contrast, saturation and hue. nil when there is none.
func (p *detailsPage) exposureBlock() fyne.CanvasObject {
	var left, right []fyne.CanvasObject
	for _, f := range []func() fyne.CanvasObject{p.irisCard, p.lensCard} {
		if c := f(); c != nil {
			left = append(left, c)
		}
	}
	for _, f := range []func() fyne.CanvasObject{p.gainCard, p.shutterCard, p.ndCard, p.balanceCard} {
		if c := f(); c != nil {
			right = append(right, c)
		}
	}
	for _, id := range imageIDs {
		if c := p.imageCard(id); c != nil {
			right = append(right, c)
		}
	}
	switch {
	case len(left) == 0 && len(right) == 0:
		return nil
	case len(left) == 0:
		return container.NewGridWithColumns(2, right...)
	case len(right) == 0:
		return container.NewGridWithColumns(len(left), left...)
	}
	return container.NewGridWithColumns(2,
		container.NewGridWithRows(len(left), left...),
		container.NewGridWithColumns(2, right...))
}

// irisCard: the f-stop in large, − / + by a third of a stop, « » by a full
// stop, and a bar graduated in full stops (closed on the left, open on the
// right, so + moves it right).
func (p *detailsPage) irisCard() fyne.CanvasObject {
	sp, ok := p.ctl.Spec(p.slot, core.Iris)
	if !ok {
		return nil
	}
	p.cards[core.Iris] = true
	tx := p.ctl.tx
	scale := func() widgets.IrisScale { return widgets.NewIrisScale(sp, p.ctl.Lens(p.slot)) }
	readout := widgets.NewReadout(28)
	bar := widgets.NewSlider(0, 1)
	ticks := widgets.NewTicks()
	write := func(v float64) {
		p.ctl.Write(p.slot, core.Iris, core.Number(v))
		readout.SetText(tx.Explicit(sp, v))
		bar.Set(scale().Open(v))
	}
	move := func(n int) {
		k := scale()
		cur := p.ctl.Target(p.slot, core.Iris)
		if cur == nil {
			return
		}
		if v := k.StepBy(cur.Number, n); v != cur.Number {
			write(v)
		}
	}
	bar.OnChanged = func(open float64) { write(scale().At(open)) }
	if !sp.ReadOnly {
		readout.OnTapped = func() { p.askValue(sp) }
	}
	row := container.NewBorder(nil, nil,
		container.NewHBox(stepButton(icons.ChevronsLeft, func() { move(-3) }), stepButton(icons.Minus, func() { move(-1) })),
		container.NewHBox(stepButton(icons.Plus, func() { move(1) }), stepButton(icons.ChevronsRight, func() { move(3) })),
		readout)
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
	return card(icons.Aperture, tx.T("details.iris"), p.actionButton(core.AutoIris, icons.Wand, tx.T("strip.autoIris")),
		widgets.Sized(0, 40, row), bar, ticks)
}

// gainCard shows the gain the way the camera takes it: in dB when it
// declares a gain, else in ISO. The other one is not shown.
func (p *detailsPage) gainCard() fyne.CanvasObject {
	tx := p.ctl.tx
	if sp, ok := p.ctl.Spec(p.slot, core.Gain); ok {
		p.cards[core.ISO] = true // hidden: the gain says it
		return card(icons.Brightness, tx.T("details.gain"), nil, p.newStepper(core.Gain, 16, stepOf(sp)).obj)
	}
	if _, ok := p.ctl.Spec(p.slot, core.ISO); ok {
		return card(icons.Brightness, "ISO", nil, p.newStepper(core.ISO, 16, 0).obj)
	}
	return nil
}

// shutterCard shows the speed, with the matching angle next to the title,
// or the angle alone.
func (p *detailsPage) shutterCard() fyne.CanvasObject {
	tx := p.ctl.tx
	if _, ok := p.ctl.Spec(p.slot, core.ShutterSpeed); ok {
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
		return card(icons.Stopwatch, tx.T("details.shutter"), angle, st.obj)
	}
	if sp, ok := p.ctl.Spec(p.slot, core.ShutterAngle); ok {
		return card(icons.Stopwatch, tx.T("details.shutter"), nil, p.newStepper(core.ShutterAngle, 16, math.Max(stepOf(sp), 1)).obj)
	}
	return nil
}

// ndCard: one button per filter position.
func (p *detailsPage) ndCard() fyne.CanvasObject {
	sp, ok := p.ctl.Spec(p.slot, core.ND)
	if !ok || sp.Kind != core.KindEnum {
		return nil
	}
	p.cards[core.ND] = true
	tx := p.ctl.tx
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
	return card(icons.Filter, tx.T("details.nd"), nil, seg)
}

// balanceCard: colour temperature and tint side by side, with AWB.
func (p *detailsPage) balanceCard() fyne.CanvasObject {
	tx := p.ctl.tx
	var cells []fyne.CanvasObject
	if sp, ok := p.ctl.Spec(p.slot, core.WhiteBalance); ok {
		cells = append(cells, p.newStepper(core.WhiteBalance, 16, math.Max(stepOf(sp), 100)).obj)
	}
	if sp, ok := p.ctl.Spec(p.slot, core.Tint); ok {
		cells = append(cells, p.newStepper(core.Tint, 14, stepOf(sp)).obj)
	}
	if len(cells) == 0 {
		return nil
	}
	return card(icons.Temperature, tx.T("details.balance"), p.actionButton(core.AutoWhiteBalance, icons.Wand, tx.T("strip.awb")),
		container.NewGridWithColumns(len(cells), cells...))
}

// imageIDs are the image controls shown as compact cards next to the
// exposure.
var imageIDs = []core.ControlID{core.Contrast, core.Saturation, core.Hue}

// imageCard: contrast, saturation or hue with − / +, in %, or degrees for
// the hue, and a reset to neutral.
func (p *detailsPage) imageCard(id core.ControlID) fyne.CanvasObject {
	sp, ok := p.ctl.Spec(p.slot, id)
	if !ok || sp.Kind != core.KindRange {
		return nil
	}
	st := p.newStepper(id, 16, stepOf(sp))
	var reset fyne.CanvasObject
	if n, ok := neutralOf(sp); ok && !sp.ReadOnly {
		reset = widgets.NewIconButton(icons.Reset, "", func() { p.ctl.Write(p.slot, id, core.Number(n)) })
	}
	return card(icons.Palette, p.ctl.tx.ControlLabel(sp), reset, st.obj)
}

// --- lens -------------------------------------------------------------------------

// lensCard is the focus and, with a power zoom, the zoom of the lens, in one
// card under the iris.
func (p *detailsPage) lensCard() fyne.CanvasObject {
	tx := p.ctl.tx
	focus, zoom := p.focusRows(), p.zoomRows()
	af := p.actionButton(core.AutoFocus, icons.FocusAuto, tx.T("details.af"))
	switch {
	case focus == nil && zoom == nil:
		return nil
	case zoom == nil:
		return card(icons.Focus, tx.T("details.focus"), af, focus...)
	case focus == nil:
		return card(icons.ZoomIn, tx.T("details.zoom"), nil, zoom...)
	}
	return card(icons.Camera, tx.GroupTitle(core.GroupLens), af, append(focus, zoom...)...)
}

// travelSlider is a horizontal slider over a range control with step
// buttons on both sides (fine and, when coarse > 0, coarse).
func (p *detailsPage) travelSlider(id core.ControlID, fine, coarse float64, lo, hi []string, ends [2]string) (fyne.CanvasObject, *widgets.Slider) {
	sp, _ := p.ctl.Spec(p.slot, id)
	p.cards[id] = true
	sl := widgets.NewSlider(sp.Min, sp.Max)
	sl.Step = stepOf(sp)
	sl.OnChanged = func(v float64) { p.ctl.Write(p.slot, id, core.Number(widgets.Quantize(v, widgets.RangeOf(sp)))) }
	move := func(d float64) {
		v, ok := p.stepValue(sp, math.Abs(d), int(math.Copysign(1, d)))
		if ok {
			p.ctl.Write(p.slot, id, v)
			sl.Set(v.Number)
		}
	}
	steps := []float64{-coarse, -fine, fine, coarse}
	var left, right []fyne.CanvasObject
	for i, icon := range lo {
		d := steps[i+2-len(lo)]
		left = append(left, stepButton(icon, func() { move(d) }))
	}
	for i, icon := range hi {
		d := steps[2+i]
		right = append(right, stepButton(icon, func() { move(d) }))
	}
	left = append(left, widgets.NewText(ends[0], style.TextMuted, 11))
	right = append([]fyne.CanvasObject{widgets.NewText(ends[1], style.TextMuted, 11)}, right...)
	p.appliers = append(p.appliers, func() {
		if v := p.ctl.Value(p.slot, id); v != nil && v.Kind == core.KindRange {
			sl.Set(v.Number)
		}
	})
	return container.NewBorder(nil, nil, container.NewHBox(left...), container.NewHBox(right...), sl), sl
}

// focusRows: ‹ › fine and « » coarse steps around the focus travel, the
// lens' focus range and, when the camera reports them, the distance and
// the focal length of a lens without power zoom.
func (p *detailsPage) focusRows() []fyne.CanvasObject {
	sp, ok := p.ctl.Spec(p.slot, core.Focus)
	if !ok {
		return nil
	}
	tx := p.ctl.tx
	span := sp.Max - sp.Min
	row, _ := p.travelSlider(core.Focus, span/100, span/20,
		[]string{icons.ChevronsLeft, icons.ChevronLeft}, []string{icons.ChevronRight, icons.ChevronsRight},
		[2]string{tx.T("details.near"), "∞"})
	sub := caption()
	p.appliers = append(p.appliers, func() {
		l := p.ctl.Lens(p.slot)
		parts := []string{p.display(sp, p.ctl.Value(p.slot, core.Focus))}
		if l.FocusDistance > 0 {
			parts[0] = tx.Distance(l.FocusDistance) + " (" + parts[0] + ")"
		}
		if l.FocusNear > 0 && l.FocusFar > 0 {
			parts = append(parts, tx.T("details.focusRange", "near", tx.Distance(l.FocusNear), "far", tx.Distance(l.FocusFar)))
		}
		if !p.ctl.Has(p.slot, core.Zoom) && l.FocalLength > 0 {
			parts = append(parts, tx.T("details.focal", "focal", tx.Focal(l.FocalLength)))
		}
		sub.SetText(strings.Join(parts, " · "))
	})
	return []fyne.CanvasObject{row, sub}
}

// zoomRows: hold the zoom buttons to zoom, or drag; the focal length in mm
// when the camera reports it.
func (p *detailsPage) zoomRows() []fyne.CanvasObject {
	sp, ok := p.ctl.Spec(p.slot, core.Zoom)
	if !ok {
		return nil
	}
	tx := p.ctl.tx
	row, _ := p.travelSlider(core.Zoom, (sp.Max-sp.Min)/50, 0,
		[]string{icons.ZoomOut}, []string{icons.ZoomIn},
		[2]string{tx.T("details.wide"), tx.T("details.tele")})
	sub := caption()
	p.appliers = append(p.appliers, func() {
		l := p.ctl.Lens(p.slot)
		txt := p.display(sp, p.ctl.Value(p.slot, core.Zoom))
		if l.FocalLength > 0 {
			txt = tx.Focal(l.FocalLength)
			if l.FocalMax > l.FocalMin && l.FocalMin > 0 {
				txt += " · " + tx.T("details.range", "min", tx.Focal(l.FocalMin), "max", tx.Focal(l.FocalMax))
			}
		}
		sub.SetText(txt)
	})
	return []fyne.CanvasObject{row, sub}
}
