package bm

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// The spec builders below are the controls the Blackmagic REST API knows.
// Each module composes the ones its camera has; ranges marked TODO(verify)
// are assumptions to check on hardware.

// LensSpecs are the lens controls: iris (f-stops), the hidden iris over the
// lens' range, focus, zoom and the auto functions. A module drops Zoom when
// the lens has no power zoom.
func LensSpecs() []core.ControlSpec {
	return []core.ControlSpec{
		// f-stops (apertureStop). The camera snaps to the lens' stops and
		// reports the lens' own limits in /lens/iris/description (core.Lens).
		// TODO(verify): f/1.0 to f/22 covers the lenses seen so far.
		speckit.Range(core.Iris, 1, 22, 0.1, "f"),
		irisNormSpec(),
		speckit.Action(core.AutoIris),
		speckit.Range(core.Focus, 0, 1, 0.001, ""),
		speckit.Action(core.AutoFocus),
		speckit.Range(core.Zoom, 0, 1, 0.001, ""),
	}
}

// irisNormSpec is the iris over the lens' range ("normalised" of
// /lens/iris), for the panel's lever: the camera maps it to the mounted
// lens, so the lever never asks for a stop the lens lacks. 0 = open,
// 1 = closed, as ATEM Software Control sends it (1 - lever position).
// TODO(verify) the direction on a live camera.
func irisNormSpec() core.ControlSpec {
	s := speckit.Range(core.IrisNorm, 0, 1, 0.001, "")
	s.Hidden = true
	return s
}

// GainSpec is the gain in dB (Studio cameras).
// TODO(verify): the API accepts any dB value; the usable range is the camera's.
func GainSpec() core.ControlSpec { return speckit.Range(core.Gain, -12, 36, 2, "dB") }

// ISOSpec is the ISO, as the menu steps.
// TODO(verify): steps differ per model (dual native ISO) and per firmware;
// the API takes any integer.
func ISOSpec() core.ControlSpec {
	return speckit.Enum(core.ISO, speckit.Opts(
		"100", "125", "160", "200", "250", "320", "400", "500", "640", "800",
		"1000", "1250", "1600", "2000", "2500", "3200", "4000", "5000", "6400", "8000",
		"10000", "12800", "16000", "20000", "25600"))
}

// ShutterSpecs are the shutter angle and the given shutter speeds ("1/50").
// The base converts one into the other with the frame rate.
func ShutterSpecs(speeds ...string) []core.ControlSpec {
	return []core.ControlSpec{
		// TODO(verify): the minimum angle depends on the frame rate.
		speckit.Range(core.ShutterAngle, 11.2, 360, 0.1, "°"),
		speckit.Enum(core.ShutterSpeed, speckit.Opts(speeds...)),
	}
}

// NDSpec is the built-in ND filter, in stops.
// TODO(verify): positions in stops as exposed by /video/ndFilter.
func NDSpec() core.ControlSpec {
	return speckit.Enum(core.ND, speckit.Labeled("0", "Clear", "2", "ND 0.6 (2 stops)", "4", "ND 1.2 (4 stops)", "6", "ND 1.8 (6 stops)"))
}

// WhiteSpecs are the white balance, the tint and auto white balance.
func WhiteSpecs() []core.ControlSpec {
	return []core.ControlSpec{
		speckit.Range(core.WhiteBalance, 2500, 10000, 50, "K"), // TODO(verify)
		speckit.Range(core.Tint, -50, 50, 1, ""),               // TODO(verify)
		speckit.Action(core.AutoWhiteBalance),
	}
}

// WheelSpecs are lift, gamma and gain for red, green, blue and luma, as the
// API ranges them (lift -2..2, gamma -4..4, gain 0..16). withMasterBlack
// adds the master black (the lift luma, driven by the panel's ring).
func WheelSpecs(withMasterBlack bool) []core.ControlSpec {
	var s []core.ControlSpec
	lift := []core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY}
	if withMasterBlack {
		lift = append(lift, core.MasterBlack)
	}
	for _, id := range lift {
		s = append(s, speckit.Range(id, -2, 2, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GammaR, core.GammaG, core.GammaB, core.GammaY} {
		s = append(s, speckit.Range(id, -4, 4, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		s = append(s, speckit.Range(id, 0, 16, 0.01, ""))
	}
	return s
}

// ColorCorrectionSpecs are every color correction control of the REST API:
// the wheels with the master black, the offsets, contrast and its pivot,
// saturation, hue and the luma mix.
func ColorCorrectionSpecs() []core.ControlSpec {
	s := WheelSpecs(true)
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
	)
}

// RecordSpec starts and stops recording (transport 0).
func RecordSpec() core.ControlSpec { return speckit.Toggle(core.Record) }
