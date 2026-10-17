package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// controlRow is the generic widget of one control: the renderer picks the
// widget from the page item and the spec (widgets.ChooseKind) so every
// control a camera declares can be driven from the Details page.
type controlRow struct {
	spec  core.ControlSpec
	kind  widgets.WidgetKind
	obj   fyne.CanvasObject
	apply func(v *core.Value)
}

const labelColWidth = 132

func newControlRow(ctl *Controller, slot int, spec core.ControlSpec) *controlRow {
	return buildControlRow(ctl, slot, spec, "", false)
}

// buildControlRow builds a row; compact is for a narrow column: a range
// control puts a small name on a first line and − / slider / + / value on a
// second.
func buildControlRow(ctl *Controller, slot int, spec core.ControlSpec, w core.Widget, compact bool) *controlRow {
	r := &controlRow{spec: spec, kind: widgets.ChooseKind(spec, w)}
	write := func(v core.Value) { ctl.Write(slot, spec.ID, v) }
	label := ctl.tx.ControlLabel(spec)
	quiet := false

	name := func(caption string) fyne.CanvasObject {
		items := []fyne.CanvasObject{widgets.NewText(label, style.Text, 13).Clip()}
		if caption != "" {
			items = append(items, widgets.NewText(caption, style.TextMuted, 10).Clip())
		}
		return widgets.Sized(labelColWidth, 0, container.NewVBox(items...))
	}

	if r.kind == widgets.KindSegmented && w == "" && segmentedWidth(spec) > maxSegmentedWidth {
		r.kind = widgets.KindSelect // labels too long for one row
	}

	switch r.kind {
	case widgets.KindSlider:
		rng := widgets.RangeOf(spec)
		step := stepOf(spec)
		sl := widgets.NewSlider(spec.Min, spec.Max)
		sl.Step = step
		entry := widget.NewEntry()
		entry.Text = texts.Missing
		show := func(v float64) { entry.SetText(ctl.tx.Explicit(spec, v)) }
		set := func(v float64) {
			v = widgets.Quantize(v, rng)
			sl.Set(v)
			show(v)
			write(core.Number(v))
		}
		sl.OnChanged = func(v float64) {
			v = widgets.Quantize(v, rng)
			show(v)
			write(core.Number(v))
		}
		entry.OnSubmitted = func(s string) {
			v, ok := texts.ParseExplicit(spec, s)
			if !ok {
				ctl.SetNotice(texts.Notice{Text: ctl.tx.T("controlrow.numberExpected", "label", label), Level: texts.LevelError})
				r.apply(ctl.Value(slot, spec.ID))
				return
			}
			set(v)
		}
		// − / + move by one step (ten for fine ranges), from where the
		// control is heading so quick taps add up.
		nudge := step
		if (spec.Max-spec.Min)/step > 200 {
			nudge = step * 10
		}
		bump := func(dir float64) {
			base := (spec.Min + spec.Max) / 2
			if v := ctl.Target(slot, spec.ID); v != nil && v.Kind == core.KindRange {
				base = v.Number
			}
			set(base + dir*nudge)
		}
		caption := ctl.tx.T("controlrow.range", "min", ctl.tx.Explicit(spec, spec.Min), "max", ctl.tx.Explicit(spec, spec.Max))
		right := []fyne.CanvasObject{stepButton(icons.Plus, func() { bump(1) }), widgets.Sized(84, 0, entry)}
		if n, ok := neutralOf(spec); ok {
			right = append(right, widgets.NewIconButton(icons.Reset, "", func() { set(n) }))
		}
		if compact {
			var reset []fyne.CanvasObject
			if len(right) > 2 {
				reset = right[2:]
			}
			r.obj = container.NewVBox(
				widgets.NewText(label, style.TextMuted, 11).Clip(),
				container.NewBorder(nil, nil, stepButton(icons.Minus, func() { bump(-1) }),
					container.NewHBox(append([]fyne.CanvasObject{right[0], widgets.Sized(72, 0, entry)}, reset...)...), sl))
		} else {
			r.obj = container.NewBorder(nil, nil,
				container.NewHBox(name(caption), stepButton(icons.Minus, func() { bump(-1) })),
				container.NewHBox(right...), sl)
		}
		r.apply = func(v *core.Value) {
			if v != nil && v.Kind == core.KindRange {
				sl.Set(v.Number)
				show(v.Number)
			} else {
				sl.Set(widgets.Neutral(spec.ID, spec.Min, spec.Max))
				entry.SetText(texts.Missing)
			}
		}

	case widgets.KindSegmented:
		labels := make([]string, len(spec.Options))
		for i, o := range spec.Options {
			labels[i] = o.Label
		}
		seg := widgets.NewSegmented(labels, func(i int) {
			if !quiet {
				write(core.Choice(spec.Options[i].Value))
			}
		})
		r.obj = container.NewBorder(nil, nil, name(""), nil, container.NewHBox(seg))
		r.apply = func(v *core.Value) {
			seg.SetSelected(optionIndex(spec, v))
		}

	case widgets.KindSelect:
		labels := make([]string, len(spec.Options))
		for i, o := range spec.Options {
			labels[i] = o.Label
		}
		sel := widget.NewSelect(labels, nil)
		sel.PlaceHolder = texts.Missing
		sel.OnChanged = func(string) {
			if quiet {
				return
			}
			if i := sel.SelectedIndex(); i >= 0 {
				write(core.Choice(spec.Options[i].Value))
			}
		}
		r.obj = container.NewBorder(nil, nil, name(""), nil, container.NewHBox(widgets.Sized(160, 0, sel)))
		r.apply = func(v *core.Value) {
			quiet = true
			defer func() { quiet = false }()
			if i := optionIndex(spec, v); i >= 0 {
				sel.SetSelectedIndex(i)
			} else {
				sel.ClearSelected()
			}
		}

	case widgets.KindCheck:
		chk := widget.NewCheck(ctl.tx.T("value.off"), nil)
		chk.OnChanged = func(on bool) {
			if quiet {
				return
			}
			if on {
				chk.Text = ctl.tx.T("value.on")
			} else {
				chk.Text = ctl.tx.T("value.off")
			}
			chk.Refresh()
			write(core.Bool(on))
		}
		r.obj = container.NewBorder(nil, nil, name(""), nil, container.NewHBox(chk))
		r.apply = func(v *core.Value) {
			quiet = true
			defer func() { quiet = false }()
			on := v != nil && v.Kind == core.KindToggle && v.On
			chk.SetChecked(on)
			if v == nil {
				chk.Text = texts.Missing
			} else if on {
				chk.Text = ctl.tx.T("value.on")
			} else {
				chk.Text = ctl.tx.T("value.off")
			}
			chk.Refresh()
		}

	case widgets.KindStepper:
		step := stepOf(spec)
		rng := widgets.RangeOf(spec)
		val := widgets.NewText(texts.Missing, style.Text, 14).Bold().Mono()
		bump := func(dir float64) {
			base := (spec.Min + spec.Max) / 2
			if v := ctl.Target(slot, spec.ID); v != nil && v.Kind == core.KindRange {
				base = v.Number
			}
			v := widgets.Quantize(base+dir*step, rng)
			val.SetText(ctl.tx.Explicit(spec, v))
			write(core.Number(v))
		}
		r.obj = container.NewBorder(nil, nil, name(""), nil, container.NewHBox(
			stepButton(icons.Minus, func() { bump(-1) }), widgets.Sized(84, 0, container.NewCenter(val)),
			stepButton(icons.Plus, func() { bump(1) })))
		r.apply = func(v *core.Value) {
			if v != nil && v.Kind == core.KindRange {
				val.SetText(ctl.tx.Explicit(spec, v.Number))
			} else {
				val.SetText(texts.Missing)
			}
		}

	case widgets.KindText:
		entry := widget.NewEntry()
		entry.OnSubmitted = func(s string) {
			if len(s) > core.MaxTextLen {
				ctl.SetNotice(texts.Notice{Text: ctl.tx.T("controlrow.textTooLong", "label", label, "max", core.MaxTextLen), Level: texts.LevelError})
				r.apply(ctl.Value(slot, spec.ID))
				return
			}
			write(core.Text(s))
		}
		r.obj = container.NewBorder(nil, nil, name(""), nil, widgets.Sized(220, 0, entry))
		r.apply = func(v *core.Value) {
			if v != nil && v.Kind == core.KindText {
				entry.SetText(v.Text)
			}
		}

	case widgets.KindReadout:
		val := widgets.NewText(texts.Missing, style.Text, 18).Bold().Mono()
		r.obj = container.NewBorder(nil, nil, name(""), nil, val)
		r.apply = func(v *core.Value) { val.SetText(ctl.tx.FormatValue(spec, v)) }

	case widgets.KindButton:
		btn := widget.NewButton(ctl.tx.T("controlrow.run"), func() { write(core.Trigger()) })
		r.obj = container.NewBorder(nil, nil, name(ctl.tx.T("controlrow.action")), nil, container.NewHBox(btn))
		r.apply = func(*core.Value) {}

	default: // read-only
		val := widgets.NewText(texts.Missing, style.Text, 13).Bold().Mono()
		r.obj = container.NewBorder(nil, nil, name(ctl.tx.T("controlrow.readOnly")), nil, val)
		r.apply = func(v *core.Value) { val.SetText(ctl.tx.FormatValue(spec, v)) }
	}
	r.apply(ctl.Value(slot, spec.ID))
	return r
}

// neutralOf is the "no correction" value of an image control, for its reset
// button: contrast and saturation 1, hue 0.
func neutralOf(spec core.ControlSpec) (float64, bool) {
	n := 0.0
	switch spec.ID {
	case core.Contrast, core.Saturation:
		n = 1
	case core.Hue:
	default:
		return 0, false
	}
	if n < spec.Min || n > spec.Max {
		return 0, false
	}
	return n, true
}

func optionIndex(spec core.ControlSpec, v *core.Value) int {
	if v == nil || v.Kind != core.KindEnum {
		return -1
	}
	for i, o := range spec.Options {
		if o.Value == v.Choice {
			return i
		}
	}
	return -1
}

// section is one titled group of control rows.
type section struct {
	group core.ControlGroup
	title string
	rows  []*controlRow
}

// controlsSection builds a BlockControls panel: its items, in order, each
// with the widget the module asked for. Items the camera does not declare
// (or hides) are skipped.
func controlsSection(ctl *Controller, slot int, b core.Block) *section {
	s := &section{group: core.GroupMisc, title: b.Title}
	for _, it := range b.Items {
		sp, ok := ctl.Spec(slot, it.Control)
		if !ok || sp.Hidden {
			continue
		}
		s.rows = append(s.rows, buildControlRow(ctl, slot, sp, it.Widget, false))
	}
	return s
}

// buildSections groups every declared control that is not in skip by its
// group, in the fixed display order. Groups without controls are omitted.
func buildSections(ctl *Controller, slot int, skip map[core.ControlID]bool, compact bool) []*section {
	byGroup := map[core.ControlGroup]*section{}
	norm := func(g core.ControlGroup) core.ControlGroup {
		for _, k := range texts.GroupOrder {
			if g == k {
				return g
			}
		}
		return core.GroupMisc
	}
	for _, sp := range ctl.Specs(slot) {
		if skip[sp.ID] {
			continue
		}
		g := norm(sp.Group)
		s := byGroup[g]
		if s == nil {
			s = &section{group: g, title: ctl.tx.GroupTitle(g)}
			byGroup[g] = s
		}
		s.rows = append(s.rows, buildControlRow(ctl, slot, sp, "", compact))
	}
	var out []*section
	for _, g := range texts.GroupOrder {
		if s := byGroup[g]; s != nil {
			out = append(out, s)
		}
	}
	return out
}

// sectionObject lays a section out as a titled panel with cols columns of rows.
// Rows sharing a spec.Section are gathered under a small sub-title, in the
// order their section first appears.
func sectionObject(s *section, cols int) fyne.CanvasObject {
	var order []string
	bySection := map[string][]fyne.CanvasObject{}
	for _, r := range s.rows {
		if _, seen := bySection[r.spec.Section]; !seen {
			order = append(order, r.spec.Section)
		}
		bySection[r.spec.Section] = append(bySection[r.spec.Section],
			container.New(layout.NewCustomPaddedLayout(2, 2, 4, 4), r.obj))
	}
	body := container.NewVBox(
		widgets.NewText(s.title, style.Text, 14).Bold(),
		widgets.Sized(0, 1, widgets.NewHairline()),
	)
	for _, name := range order {
		if name != "" {
			body.Add(container.New(layout.NewCustomPaddedLayout(8, 2, 4, 4),
				widgets.NewText(name, style.TextMuted, 12).Bold()))
		}
		body.Add(container.NewGridWithColumns(cols, bySection[name]...))
	}
	return widgets.NewPanel(style.Surface, style.Border, 12, body)
}

// maxSegmentedWidth is the widest segmented control a row accepts; longer
// option lists are shown as a select so rows keep a bounded minimum width.
const maxSegmentedWidth = 260

func segmentedWidth(spec core.ControlSpec) float32 {
	var w float32
	for _, o := range spec.Options {
		if tw := fyne.MeasureText(o.Label, 12, fyne.TextStyle{Bold: true}).Width + 20; tw > w {
			w = tw
		}
	}
	return w * float32(len(spec.Options))
}
