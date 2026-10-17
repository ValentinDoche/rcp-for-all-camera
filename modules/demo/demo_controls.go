package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

// Demo-specific controls: no standard id fits them.
const (
	offsetR          = "demo.offsetR"
	offsetG          = "demo.offsetG"
	offsetB          = "demo.offsetB"
	offsetY          = "demo.offsetY"
	contrastPivot    = "demo.contrastPivot"
	lumaContribution = "demo.lumaContribution"
	// A custom block at the end of the page, to show the feature.
	tallyLamp = "demo.tallyLamp"
	tallyTest = "demo.tallyTest"
	firmware  = "demo.firmware"
	cameraID  = "demo.cameraId"
)

// specs are the demo camera's controls: those of a studio camera with an
// interchangeable lens, plus a custom "Demo" block.
func specs() []rcpmod.ControlSpec {
	irisNorm := rcpmod.Range(rcpmod.IrisNorm, 0, 1, 0.001, "")
	irisNorm.Hidden = true // the panel's lever drives it; people see Iris
	s := []rcpmod.ControlSpec{
		rcpmod.Range(rcpmod.Iris, 1, 22, 0.1, "f"),
		irisNorm,
		rcpmod.Action(rcpmod.AutoIris),
		rcpmod.Range(rcpmod.Focus, 0, 1, 0.001, ""),
		rcpmod.Action(rcpmod.AutoFocus),
		rcpmod.Range(rcpmod.Zoom, 0, 1, 0.001, ""), // dropped without a power zoom
		rcpmod.Range(rcpmod.Gain, -12, 36, 2, "dB"),
		rcpmod.Enum(rcpmod.ISO, rcpmod.Opts(
			"100", "125", "160", "200", "250", "320", "400", "500", "640", "800",
			"1000", "1250", "1600", "2000", "2500", "3200", "4000", "5000", "6400", "8000",
			"10000", "12800", "16000", "20000", "25600")),
		rcpmod.Range(rcpmod.ShutterAngle, 11.2, 360, 0.1, "°"),
		rcpmod.Enum(rcpmod.ShutterSpeed, rcpmod.Opts(
			"1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/125",
			"1/200", "1/250", "1/500", "1/1000", "1/2000")),
		rcpmod.Enum(rcpmod.ND, rcpmod.Pairs("0", "Clear", "2", "ND 0.6 (2 stops)", "4", "ND 1.2 (4 stops)", "6", "ND 1.8 (6 stops)")),
		rcpmod.Range(rcpmod.WhiteBalance, 2500, 10000, 50, "K"),
		rcpmod.Range(rcpmod.Tint, -50, 50, 1, ""),
		rcpmod.Action(rcpmod.AutoWhiteBalance),
	}
	for _, id := range []string{rcpmod.LiftR, rcpmod.LiftG, rcpmod.LiftB, rcpmod.LiftY, rcpmod.MasterBlack} {
		s = append(s, rcpmod.Range(id, -2, 2, 0.01, ""))
	}
	for _, id := range []string{rcpmod.GammaR, rcpmod.GammaG, rcpmod.GammaB, rcpmod.GammaY} {
		s = append(s, rcpmod.Range(id, -4, 4, 0.01, ""))
	}
	for _, id := range []string{rcpmod.GainR, rcpmod.GainG, rcpmod.GainB, rcpmod.GainY} {
		s = append(s, rcpmod.Range(id, 0, 16, 0.01, ""))
	}
	for _, o := range []struct{ id, label string }{
		{offsetR, "Offset R"}, {offsetG, "Offset G"}, {offsetB, "Offset B"}, {offsetY, "Offset Y"},
	} {
		s = append(s, rcpmod.Labeled(rcpmod.Range(o.id, -8, 8, 0.01, ""), o.label, rcpmod.GroupColor))
	}
	s = append(s,
		rcpmod.Range(rcpmod.Contrast, 0, 2, 0.01, ""),
		rcpmod.Labeled(rcpmod.Range(contrastPivot, 0, 1, 0.01, ""), "Contrast pivot", rcpmod.GroupColor),
		rcpmod.Range(rcpmod.Saturation, 0, 2, 0.01, ""),
		rcpmod.Range(rcpmod.Hue, -1, 1, 0.01, ""),
		rcpmod.Labeled(rcpmod.Range(lumaContribution, 0, 1, 0.01, ""), "Luma mix", rcpmod.GroupColor),
		rcpmod.Toggle(rcpmod.Record),
	)
	// The custom block: a switch, a button, a read-only value and a text.
	return append(s,
		rcpmod.Custom(rcpmod.Labeled(rcpmod.Toggle(tallyLamp), "Tally lamp", rcpmod.GroupMisc), "Demo", rcpmod.WidgetSwitch),
		rcpmod.Custom(rcpmod.Labeled(rcpmod.Action(tallyTest), "Flash the tally", rcpmod.GroupMisc), "Demo", rcpmod.WidgetButton),
		rcpmod.Custom(rcpmod.Labeled(rcpmod.ReadOnly(rcpmod.ControlSpec{ID: firmware, Kind: rcpmod.KindText}), "Firmware", rcpmod.GroupMisc), "Demo", rcpmod.WidgetReadout),
		rcpmod.Custom(rcpmod.Labeled(rcpmod.ControlSpec{ID: cameraID, Kind: rcpmod.KindText}, "Camera name", rcpmod.GroupMisc), "Demo", rcpmod.WidgetText),
	)
}

// initial is the state the camera starts in.
func initial() map[string]rcpmod.Value {
	v := map[string]rcpmod.Value{
		rcpmod.Iris: rcpmod.Number(4), rcpmod.IrisNorm: rcpmod.Number(irisNormOf(4)),
		rcpmod.Focus: rcpmod.Number(0.6), rcpmod.Zoom: rcpmod.Number(0.25),
		rcpmod.Gain: rcpmod.Number(0), rcpmod.ISO: rcpmod.Choice("400"),
		rcpmod.ShutterSpeed: rcpmod.Choice("1/100"), rcpmod.ShutterAngle: rcpmod.Number(180),
		rcpmod.ND: rcpmod.Choice("2"), rcpmod.WhiteBalance: rcpmod.Number(5600), rcpmod.Tint: rcpmod.Number(0),
		rcpmod.Contrast: rcpmod.Number(1), rcpmod.Saturation: rcpmod.Number(1), rcpmod.Hue: rcpmod.Number(0),
		contrastPivot: rcpmod.Number(0.5), lumaContribution: rcpmod.Number(1),
		rcpmod.Record: rcpmod.Bool(false), rcpmod.MasterBlack: rcpmod.Number(0),
		tallyLamp: rcpmod.Bool(false), firmware: rcpmod.Text("demo 0.3.0"), cameraID: rcpmod.Text("Demo camera"),
	}
	for _, id := range []string{rcpmod.LiftR, rcpmod.LiftG, rcpmod.LiftB, rcpmod.LiftY,
		rcpmod.GammaR, rcpmod.GammaG, rcpmod.GammaB, rcpmod.GammaY, offsetR, offsetG, offsetB, offsetY} {
		v[id] = rcpmod.Number(0)
	}
	for _, id := range []string{rcpmod.GainR, rcpmod.GainG, rcpmod.GainB, rcpmod.GainY} {
		v[id] = rcpmod.Number(1)
	}
	return v
}
