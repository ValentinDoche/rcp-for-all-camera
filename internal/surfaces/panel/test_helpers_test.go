package panel

import (
	"math"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"
)

func rng(id core.ControlID, lo, hi, st float64, unit string) core.ControlSpec {
	s := core.Std(id)
	s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, lo, hi, st, unit
	return s
}

func enum(id core.ControlID, opts ...string) core.ControlSpec {
	s := core.Std(id)
	s.Kind = core.KindEnum
	for _, o := range opts {
		s.Options = append(s.Options, core.Option{Value: o, Label: o})
	}
	return s
}

func kind(id core.ControlID, k core.ControlKind) core.ControlSpec {
	s := core.Std(id)
	s.Kind = k
	return s
}

// sonySpecs looks like the Sony FX6 declaration.
func sonySpecs() []core.ControlSpec {
	return []core.ControlSpec{
		rng(core.Iris, 1.4, 22, 0.1, "f"),
		kind(core.AutoIris, core.KindAction),
		rng(core.Gain, -3, 33, 1, "dB"),
		enum(core.ShutterSpeed, "1/24", "1/25", "1/30", "1/50", "1/60", "1/100", "1/2000"),
		rng(core.ShutterAngle, 11.25, 360, 0.25, "°"),
		enum(core.ND, "clear", "1/4", "1/8", "1/16"),
		rng(core.MasterBlack, -15, 15, 1, ""),
		kind(core.AutoWhiteBalance, core.KindAction),
		kind(core.ColorBars, core.KindToggle),
		rng(core.GainR, 0, 16, 0.01, ""), rng(core.GainG, 0, 16, 0.01, ""), rng(core.GainB, 0, 16, 0.01, ""),
		rng(core.LiftR, -2, 2, 0.01, ""), rng(core.LiftG, -2, 2, 0.01, ""), rng(core.LiftB, -2, 2, 0.01, ""),
		kind(core.Tally, core.KindToggle),
		rng(core.WhiteBalance, 2000, 15000, 50, "K"), rng(core.Tint, -7, 7, 1, ""),
	}
}

func views(specs []core.ControlSpec, vals map[core.ControlID]core.Value) []app.ControlView {
	var out []app.ControlView
	for _, s := range specs {
		cv := app.ControlView{Spec: s}
		if v, ok := vals[s.ID]; ok {
			cv.Value = &v
		}
		out = append(out, cv)
	}
	return out
}

func cam(slot int, specs []core.ControlSpec, vals map[core.ControlID]core.Value) panelmodel.Camera {
	return panelmodel.NewCamera(slot, views(specs, vals))
}

func sonyValues() map[core.ControlID]core.Value {
	return map[core.ControlID]core.Value{
		core.Iris: core.Number(4), core.Gain: core.Number(6), core.ShutterSpeed: core.Choice("1/50"),
		core.ND: core.Choice("1/8"), core.MasterBlack: core.Number(3), core.ColorBars: core.Bool(false),
		core.GainR: core.Number(1), core.LiftR: core.Number(0),
		core.WhiteBalance: core.Number(5600), core.Tint: core.Number(0),
	}
}

// rigWith assigns cameras to the given slots.
func rigWith(active int, slots ...int) app.Rig {
	r := app.Rig{Slots: make([]app.Slot, app.SlotCount), Active: active}
	for i := range r.Slots {
		r.Slots[i].Slot = i + 1
	}
	for _, s := range slots {
		r.Slots[s-1].Camera = &app.CameraView{Model: "sony-fx6", Name: "Cam", Status: core.Status{State: core.StateDisconnected}}
	}
	return r
}

// whole is the window covering a control's whole range.
func whole(s core.ControlSpec) panelmodel.Window { return panelmodel.Window{Lo: s.Min, Hi: s.Max} }

// coarseFor is the COARSE knob position whose window centre is f.
func coarseFor(s core.ControlSpec, f float64) int {
	return int(math.Round(math.Log(f/s.Min) / math.Log(s.Max/s.Min) * micropanel.AnalogMax))
}

func digitsOf(texts map[micropanel.Display]string) [14]byte {
	var d [14]byte
	for disp, s := range texts {
		micropanel.EncodeText(&d, disp, s)
	}
	return d
}

// shutterOf is the SHUTTER display (digits 4..7) showing text.
func shutterOf(text string) [4]byte {
	d := digitsOf(map[micropanel.Display]string{micropanel.DispShutter: text})
	return [4]byte(d[4:8])
}

func cameraColour(l micropanel.LEDBits, slot int) micropanel.Colour {
	base := 16 + 3*(slot-1)
	var c micropanel.Colour
	for ch := 0; ch < 3; ch++ {
		if panelmodel.LEDOn(&l, base+ch) {
			c |= 1 << ch
		}
	}
	return c
}

// sensFor is the SENS knob position giving a window of w stops.
func sensFor(s core.ControlSpec, w float64) int {
	full := 2 * math.Log2(s.Max/s.Min)
	return int(math.Round((w - 1) / (full - 1) * micropanel.AnalogMax))
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }
