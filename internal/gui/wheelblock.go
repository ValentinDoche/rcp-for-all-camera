package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// Color groups, in the order of the Lift / Gamma / Gain selector. Each row is
// Y, R, G, B.
var wheelGroups = []struct {
	Key string
	IDs [4]core.ControlID
}{
	{"wheel.lift", [4]core.ControlID{core.LiftY, core.LiftR, core.LiftG, core.LiftB}},
	{"wheel.gamma", [4]core.ControlID{core.GammaY, core.GammaR, core.GammaG, core.GammaB}},
	{"wheel.gain", [4]core.ControlID{core.GainY, core.GainR, core.GainG, core.GainB}},
}

// wheelControlIDs are the controls the wheel blocks own; the generic renderer
// skips them.
func wheelControlIDs() map[core.ControlID]bool {
	m := map[core.ControlID]bool{}
	for _, g := range wheelGroups {
		for _, id := range g.IDs {
			m[id] = true
		}
	}
	return m
}

// wheelAvailable reports whether the camera declares R, G and B of group g.
func wheelAvailable(ctl *Controller, slot, g int) bool {
	ids := wheelGroups[g].IDs
	return ctl.Has(slot, ids[1]) && ctl.Has(slot, ids[2]) && ctl.Has(slot, ids[3])
}

// wheelBlock is the color wheel with its master slider, four readouts and the
// reset button, for one of Lift / Gamma / Gain.
type wheelBlock struct {
	ctl   *Controller
	slot  int
	group int

	wheel *widgets.ColorWheel
	y     *widgets.Slider
	cells [4]*widgets.Text
	reset *widget.Button
	obj   fyne.CanvasObject
	quiet bool
}

func newWheelBlock(ctl *Controller, slot int, diameter float32) *wheelBlock {
	b := &wheelBlock{ctl: ctl, slot: slot}
	b.wheel = widgets.NewColorWheel(diameter)
	b.wheel.OnChanged = b.onWheel
	b.y = widgets.NewSlider(0, 1)
	b.y.OnChanged = b.onY

	tokens := [4]fyne.ThemeColorName{style.Text, style.ChanR, style.ChanG, style.ChanB}
	var cellObjs []fyne.CanvasObject
	for i := range b.cells {
		b.cells[i] = widgets.NewText(texts.Missing, style.Text, 12).Mono().Align(fyne.TextAlignCenter)
		bar := widgets.Sized(0, 2, widgets.NewPanel(tokens[i], "", 0, nil))
		cellObjs = append(cellObjs, container.NewVBox(b.cells[i], bar))
	}
	b.reset = widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), b.onReset)
	b.reset.Importance = widget.LowImportance

	readouts := container.NewBorder(nil, nil, nil, b.reset, container.NewGridWithColumns(4, cellObjs...))
	b.obj = container.NewVBox(
		container.NewCenter(b.wheel),
		container.New(layout.NewCustomPaddedLayout(2, 2, 10, 10), b.y),
		readouts,
	)
	return b
}

// setGroup switches the block to Lift (0), Gamma (1) or Gain (2).
func (b *wheelBlock) setGroup(g int) {
	b.group = g
	ids := wheelGroups[g].IDs
	if sp, ok := b.ctl.Spec(b.slot, ids[0]); ok {
		b.y.Min, b.y.Max = sp.Min, sp.Max
		b.y.Step = stepOf(sp)
	}
	b.apply()
}

func stepOf(sp core.ControlSpec) float64 {
	if sp.Step > 0 {
		return sp.Step
	}
	return (sp.Max - sp.Min) / 1000
}

func (b *wheelBlock) ids() [4]core.ControlID { return wheelGroups[b.group].IDs }

// rgbRange returns the range shared by the R, G, B controls of the group.
func (b *wheelBlock) rgbSpec(i int) (core.ControlSpec, bool) { return b.ctl.Spec(b.slot, b.ids()[i]) }

func (b *wheelBlock) masterValue() float64 {
	ids := b.ids()
	if v := b.ctl.Value(b.slot, ids[0]); v != nil {
		return v.Number
	}
	if sp, ok := b.ctl.Spec(b.slot, ids[0]); ok {
		return widgets.Neutral(ids[0], sp.Min, sp.Max)
	}
	return 0
}

// apply shows the controller's current values.
func (b *wheelBlock) apply() {
	b.quiet = true
	defer func() { b.quiet = false }()
	ids := b.ids()
	var vals [4]*core.Value
	for i, id := range ids {
		vals[i] = b.ctl.Value(b.slot, id)
		sp, ok := b.ctl.Spec(b.slot, id)
		if !ok {
			b.cells[i].SetText(texts.Missing)
			continue
		}
		b.cells[i].SetText(b.ctl.tx.FormatValue(sp, vals[i]))
	}
	if sp, ok := b.ctl.Spec(b.slot, ids[0]); ok {
		b.y.Min, b.y.Max, b.y.Step = sp.Min, sp.Max, stepOf(sp)
		if vals[0] != nil {
			b.y.Set(vals[0].Number)
		} else {
			b.y.Set(widgets.Neutral(ids[0], sp.Min, sp.Max))
		}
		b.y.SetDisabled(false)
	} else {
		// not declared: greyed, never hidden, so every camera looks the same
		b.y.Set((b.y.Min + b.y.Max) / 2)
		b.y.SetDisabled(true)
	}
	sp, ok := b.rgbSpec(1)
	if ok && vals[1] != nil && vals[2] != nil && vals[3] != nil {
		span := widgets.WheelSpan(sp.Min, sp.Max)
		x, y := widgets.RGBToPuck(vals[1].Number, vals[2].Number, vals[3].Number, b.masterValue(), span)
		b.wheel.SetPuck(x, y)
	} else {
		b.wheel.SetPuck(0, 0)
	}
	avail := wheelAvailable(b.ctl, b.slot, b.group)
	b.wheel.SetDisabled(!avail)
	if avail {
		b.reset.Enable()
	} else {
		b.reset.Disable()
	}
}

func (b *wheelBlock) onWheel(x, y float64) {
	sp, ok := b.rgbSpec(1)
	if !ok {
		return
	}
	ids := b.ids()
	span := widgets.WheelSpan(sp.Min, sp.Max)
	r, g, bl := widgets.PuckToRGB(x, y, b.masterValue(), span, sp.Min, sp.Max)
	for i, v := range [3]float64{r, g, bl} {
		spec, ok := b.rgbSpec(i + 1)
		if !ok {
			continue
		}
		v = widgets.Quantize(v, widgets.RangeOf(spec))
		b.cells[i+1].SetText(b.ctl.tx.FormatValue(spec, &core.Value{Kind: core.KindRange, Number: v}))
		b.ctl.Write(b.slot, ids[i+1], core.Number(v))
	}
}

func (b *wheelBlock) onY(v float64) {
	if b.quiet {
		return
	}
	id := b.ids()[0]
	sp, ok := b.ctl.Spec(b.slot, id)
	if !ok {
		return
	}
	v = widgets.Quantize(v, widgets.RangeOf(sp))
	b.cells[0].SetText(b.ctl.tx.FormatValue(sp, &core.Value{Kind: core.KindRange, Number: v}))
	b.ctl.Write(b.slot, id, core.Number(v))
}

func (b *wheelBlock) onReset() {
	for _, id := range b.ids() {
		sp, ok := b.ctl.Spec(b.slot, id)
		if !ok {
			continue
		}
		b.ctl.Write(b.slot, id, core.Number(widgets.Neutral(id, sp.Min, sp.Max)))
	}
	b.wheel.SetPuck(0, 0)
}
