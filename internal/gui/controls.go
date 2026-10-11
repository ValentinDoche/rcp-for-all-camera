package gui

import (
	"fmt"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/fr"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// controlRow is the generic widget of one control: the renderer picks the
// widget from the spec (widgets.ChooseKind) so every control a camera
// declares can be driven from the Détails page.
type controlRow struct {
	spec  core.ControlSpec
	kind  widgets.WidgetKind
	obj   fyne.CanvasObject
	apply func(v *core.Value)
}

const labelColWidth = 132

func newControlRow(ctl *Controller, slot int, spec core.ControlSpec) *controlRow {
	r := &controlRow{spec: spec, kind: widgets.ChooseKind(spec)}
	write := func(v core.Value) { ctl.Write(slot, spec.ID, v) }
	label := fr.ControlLabel(spec)
	quiet := false

	name := func(caption string) fyne.CanvasObject {
		items := []fyne.CanvasObject{widgets.NewText(label, style.Text, 13).Clip()}
		if caption != "" {
			items = append(items, widgets.NewText(caption, style.TextMuted, 10).Clip())
		}
		return widgets.Sized(labelColWidth, 0, container.NewVBox(items...))
	}

	if r.kind == widgets.KindSegmented && segmentedWidth(spec) > maxSegmentedWidth {
		r.kind = widgets.KindSelect // labels too long for one row
	}

	switch r.kind {
	case widgets.KindSlider:
		rng := widgets.RangeOf(spec)
		step := stepOf(spec)
		dec := fr.Decimals(spec.Step)
		sl := widgets.NewSlider(spec.Min, spec.Max)
		sl.Step = step
		entry := widget.NewEntry()
		entry.Text = fr.Missing
		unit := ""
		if spec.Unit != "" {
			unit = spec.Unit
		}
		show := func(v float64) { entry.SetText(fr.FormatNumber(v, dec)) }
		sl.OnChanged = func(v float64) {
			v = widgets.Quantize(v, rng)
			show(v)
			write(core.Number(v))
		}
		entry.OnSubmitted = func(s string) {
			v, ok := fr.ParseNumber(s)
			if !ok {
				ctl.SetNotice(fr.Notice{Text: "Valeur numérique attendue pour " + label, Level: fr.LevelError})
				r.apply(ctl.Value(slot, spec.ID))
				return
			}
			v = widgets.Quantize(v, rng)
			sl.Set(v)
			show(v)
			write(core.Number(v))
		}
		caption := fmt.Sprintf("de %s à %s", fr.FormatNumber(spec.Min, dec), fr.FormatNumber(spec.Max, dec))
		if unit != "" {
			caption += " " + unit
		}
		if spec.Step > 0 {
			caption += ", pas " + fr.FormatNumber(spec.Step, dec)
		}
		right := []fyne.CanvasObject{widgets.Sized(76, 0, entry)}
		if unit != "" {
			right = append(right, widgets.Sized(22, 0, widgets.NewText(unit, style.TextMuted, 12)))
		}
		r.obj = container.NewBorder(nil, nil, name(caption), container.NewHBox(right...), sl)
		r.apply = func(v *core.Value) {
			if v != nil && v.Kind == core.KindRange {
				sl.Set(v.Number)
				show(v.Number)
			} else {
				sl.Set(widgets.Neutral(spec.ID, spec.Min, spec.Max))
				entry.SetText(fr.Missing)
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
		sel.PlaceHolder = fr.Missing
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
		chk := widget.NewCheck("Désactivé", nil)
		chk.OnChanged = func(on bool) {
			if quiet {
				return
			}
			if on {
				chk.Text = "Activé"
			} else {
				chk.Text = "Désactivé"
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
				chk.Text = fr.Missing
			} else if on {
				chk.Text = "Activé"
			} else {
				chk.Text = "Désactivé"
			}
			chk.Refresh()
		}

	case widgets.KindButton:
		btn := widget.NewButton("Lancer", func() { write(core.Trigger()) })
		r.obj = container.NewBorder(nil, nil, name("action"), nil, container.NewHBox(btn))
		r.apply = func(*core.Value) {}

	default: // read-only
		val := widgets.NewText(fr.Missing, style.Text, 13).Bold().Mono()
		r.obj = container.NewBorder(nil, nil, name("lecture seule"), nil, val)
		r.apply = func(v *core.Value) { val.SetText(fr.FormatValue(spec, v)) }
	}
	r.apply(ctl.Value(slot, spec.ID))
	return r
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

// buildSections groups every declared control that is not in skip by its
// group, in the fixed display order. Groups without controls are omitted.
func buildSections(ctl *Controller, slot int, skip map[core.ControlID]bool) []*section {
	byGroup := map[core.ControlGroup]*section{}
	norm := func(g core.ControlGroup) core.ControlGroup {
		for _, k := range fr.GroupOrder {
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
			s = &section{group: g, title: fr.GroupTitle(g)}
			byGroup[g] = s
		}
		s.rows = append(s.rows, newControlRow(ctl, slot, sp))
	}
	var out []*section
	for _, g := range fr.GroupOrder {
		if s := byGroup[g]; s != nil {
			out = append(out, s)
		}
	}
	return out
}

// sectionObject lays a section out as a titled panel with two columns of rows.
// Rows sharing a spec.Section are gathered under a small sub-title, in the
// order their section first appears.
func sectionObject(s *section) fyne.CanvasObject {
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
		body.Add(container.NewGridWithColumns(2, bySection[name]...))
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
