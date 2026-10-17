package demo

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Demo-specific control ids: no standard id fits these color controls.
const (
	OffsetR          core.ControlID = "demo.offsetR"
	OffsetG          core.ControlID = "demo.offsetG"
	OffsetB          core.ControlID = "demo.offsetB"
	OffsetY          core.ControlID = "demo.offsetY"
	ContrastPivot    core.ControlID = "demo.contrastPivot"
	LumaContribution core.ControlID = "demo.lumaContribution"
)

// specs are the demo camera's controls: those of a studio camera with an
// interchangeable lens, so the GUI and the panel show a full interface.
func specs() []core.ControlSpec {
	irisNorm := speckit.Range(core.IrisNorm, 0, 1, 0.001, "")
	irisNorm.Hidden = true // the panel's lever drives it; people see Iris
	s := []core.ControlSpec{
		speckit.Range(core.Iris, 1, 22, 0.1, "f"),
		irisNorm,
		speckit.Action(core.AutoIris),
		speckit.Range(core.Focus, 0, 1, 0.001, ""),
		speckit.Action(core.AutoFocus),
		speckit.Range(core.Zoom, 0, 1, 0.001, ""), // dropped without a power zoom
		speckit.Range(core.Gain, -12, 36, 2, "dB"),
		speckit.Enum(core.ISO, speckit.Opts(
			"100", "125", "160", "200", "250", "320", "400", "500", "640", "800",
			"1000", "1250", "1600", "2000", "2500", "3200", "4000", "5000", "6400", "8000",
			"10000", "12800", "16000", "20000", "25600")),
		speckit.Range(core.ShutterAngle, 11.2, 360, 0.1, "°"),
		speckit.Enum(core.ShutterSpeed, speckit.Opts(
			"1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/125",
			"1/200", "1/250", "1/500", "1/1000", "1/2000")),
		speckit.Enum(core.ND, speckit.Labeled("0", "Clear", "2", "ND 0.6 (2 stops)", "4", "ND 1.2 (4 stops)", "6", "ND 1.8 (6 stops)")),
		speckit.Range(core.WhiteBalance, 2500, 10000, 50, "K"),
		speckit.Range(core.Tint, -50, 50, 1, ""),
		speckit.Action(core.AutoWhiteBalance),
	}
	for _, id := range []core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY, core.MasterBlack} {
		s = append(s, speckit.Range(id, -2, 2, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GammaR, core.GammaG, core.GammaB, core.GammaY} {
		s = append(s, speckit.Range(id, -4, 4, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		s = append(s, speckit.Range(id, 0, 16, 0.01, ""))
	}
	for _, o := range []struct {
		id    core.ControlID
		label string
	}{{OffsetR, "Offset R"}, {OffsetG, "Offset G"}, {OffsetB, "Offset B"}, {OffsetY, "Offset Y"}} {
		s = append(s, speckit.Color(speckit.Range(o.id, -8, 8, 0.01, ""), o.label))
	}
	return append(s,
		speckit.Range(core.Contrast, 0, 2, 0.01, ""),
		speckit.Color(speckit.Range(ContrastPivot, 0, 1, 0.01, ""), "Contrast pivot"),
		speckit.Range(core.Saturation, 0, 2, 0.01, ""),
		speckit.Range(core.Hue, -1, 1, 0.01, ""),
		speckit.Color(speckit.Range(LumaContribution, 0, 1, 0.01, ""), "Luma mix"),
		speckit.Toggle(core.Record),
	)
}
