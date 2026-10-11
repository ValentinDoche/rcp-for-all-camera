package blackmagic

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Pocket cameras: no color bars output, ISO instead of gain.
func pocketSpecs(hasND bool) []core.ControlSpec {
	s := lensSpecs()
	s = append(s, speckit.Enum(core.ISO, speckit.Opts(
		// TODO(verify): ISO steps differ per model (dual native ISO) and per firmware.
		"100", "125", "160", "200", "250", "320", "400", "500", "640", "800",
		"1000", "1250", "1600", "2000", "2500", "3200", "4000", "5000", "6400", "8000",
		"10000", "12800", "16000", "20000", "25600")))
	s = append(s, shutterSpecs()...)
	if hasND {
		s = append(s, ndSpec())
	}
	s = append(s, whiteSpecs()...)
	s = append(s, colorSpecs()...)
	return append(s, statusSpecs()...)
}

// Studio cameras: gain in dB, color bars, optional built-in ND.
func studioSpecs(hasND bool) []core.ControlSpec {
	s := lensSpecs()
	// TODO(verify): gain range and step; recent firmware also exposes ISO on Studio cameras.
	s = append(s, speckit.Range(core.Gain, -12, 36, 2, "dB"))
	s = append(s, shutterSpecs()...)
	if hasND {
		s = append(s, ndSpec())
	}
	s = append(s, whiteSpecs()...)
	s = append(s, colorSpecs()...)
	s = append(s, speckit.Toggle(core.ColorBars)) // TODO(verify): /colorBars endpoint availability
	return append(s, statusSpecs()...)
}

func lensSpecs() []core.ControlSpec {
	return []core.ControlSpec{
		// TODO(verify): lens-dependent; the API reports normalised 0..1 and apertureStop.
		speckit.Range(core.Iris, 0, 1, 0.01, ""),
		speckit.Action(core.AutoIris),
		speckit.Range(core.Focus, 0, 1, 0.001, ""),
		speckit.Action(core.AutoFocus),
		// TODO(verify): zoom only works with motorised lenses.
		speckit.Range(core.Zoom, 0, 1, 0.001, ""),
	}
}

func shutterSpecs() []core.ControlSpec {
	return []core.ControlSpec{
		// TODO(verify): minimum angle depends on frame rate.
		speckit.Range(core.ShutterAngle, 11.2, 360, 0.1, "°"),
		speckit.Enum(core.ShutterSpeed, speckit.Opts(
			"1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/250", "1/500", "1/1000", "1/2000")), // TODO(verify)
	}
}

func ndSpec() core.ControlSpec {
	// TODO(verify): positions in stops as exposed by /video/ndFilter.
	return speckit.Enum(core.ND, speckit.Labeled("0", "Clear", "2", "ND 0.6 (2 stops)", "4", "ND 1.2 (4 stops)", "6", "ND 1.8 (6 stops)"))
}

func whiteSpecs() []core.ControlSpec {
	return []core.ControlSpec{
		speckit.Range(core.WhiteBalance, 2500, 10000, 50, "K"), // TODO(verify)
		speckit.Range(core.Tint, -50, 50, 1, ""),               // TODO(verify)
		speckit.Action(core.AutoWhiteBalance),
	}
}

func colorSpecs() []core.ControlSpec {
	var s []core.ControlSpec
	// TODO(verify): Camera Control color correction ranges (lift 0..2, gamma -4..4, gain 0..16 in the API).
	for _, id := range []core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY} {
		s = append(s, speckit.Range(id, -2, 2, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GammaR, core.GammaG, core.GammaB, core.GammaY} {
		s = append(s, speckit.Range(id, -4, 4, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		s = append(s, speckit.Range(id, 0, 16, 0.01, ""))
	}
	return append(s,
		speckit.Range(core.Contrast, 0, 2, 0.01, ""),                                                               // TODO(verify)
		speckit.Range(core.Saturation, 0, 2, 0.01, ""),                                                             // TODO(verify)
		speckit.Range(core.Hue, -1, 1, 0.01, ""),                                                                   // TODO(verify)
		speckit.Enum(core.Detail, speckit.Labeled("off", "Off", "low", "Low", "medium", "Medium", "high", "High")), // TODO(verify): sharpening levels
	)
}

func statusSpecs() []core.ControlSpec {
	return []core.ControlSpec{
		speckit.Toggle(core.Record),
		speckit.ReadOnly(speckit.Toggle(core.Tally)),
	}
}
