package sony

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// opts selects which optional controls a model exposes.
type opts struct {
	iso, gain, shutterAngle, nd, masterBlack, bars, zoom, ptz bool
}

func family(o opts) []core.ControlSpec {
	// TODO(verify): iris is lens dependent; Sony reports f-numbers, range here is generic.
	s := []core.ControlSpec{
		speckit.Range(core.Iris, 1.4, 22, 0.1, "f"),
		speckit.Action(core.AutoIris),
		speckit.Range(core.Focus, 0, 1, 0.001, ""),
		speckit.Action(core.AutoFocus),
	}
	if o.zoom {
		s = append(s, speckit.Range(core.Zoom, 0, 1, 0.001, "")) // TODO(verify): power-zoom lens or PTZ only
	}
	if o.iso {
		s = append(s, speckit.Enum(core.ISO, speckit.Opts(
			// TODO(verify): steps depend on base ISO and Cine EI mode.
			"100", "125", "160", "200", "250", "320", "400", "500", "640", "800",
			"1000", "1250", "1600", "2000", "2500", "3200", "4000", "5000", "6400",
			"8000", "10000", "12800", "16000", "20000", "25600", "32000", "40000", "51200")))
	}
	if o.gain {
		s = append(s, speckit.Range(core.Gain, -3, 33, 1, "dB")) // TODO(verify): range per model
	}
	s = append(s, speckit.Enum(core.ShutterSpeed, speckit.Opts(
		// TODO(verify): steps depend on frame rate.
		"1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/125", "1/250", "1/500", "1/1000", "1/2000")))
	if o.shutterAngle {
		s = append(s, speckit.Range(core.ShutterAngle, 11.25, 360, 0.25, "°")) // TODO(verify)
	}
	if o.nd {
		// Electronic variable ND.
		// TODO(verify): FX6/FX9 variable ND steps (1/4 to 1/128 in the camera menu).
		s = append(s, speckit.Enum(core.ND, speckit.Labeled(
			"clear", "Clear", "1/4", "1/4", "1/8", "1/8", "1/16", "1/16", "1/32", "1/32", "1/64", "1/64", "1/128", "1/128")))
	}
	if o.masterBlack {
		s = append(s, speckit.Range(core.MasterBlack, -15, 15, 1, "")) // TODO(verify)
	}
	s = append(s,
		speckit.Range(core.WhiteBalance, 2000, 15000, 100, "K"), // TODO(verify)
		speckit.Range(core.Tint, -7, 7, 1, ""),                  // TODO(verify): white-balance shift
		speckit.Action(core.AutoWhiteBalance),
	)
	if o.bars {
		s = append(s, speckit.Toggle(core.ColorBars))
	}
	if o.ptz {
		s = append(s,
			speckit.Range(core.Pan, -1, 1, 0.01, ""),
			speckit.Range(core.Tilt, -1, 1, 0.01, ""),
			speckit.Enum(core.PresetRecall, speckit.Numbered(16)), // TODO(verify): preset count
		)
	}
	return append(s,
		speckit.Toggle(core.Record),
		speckit.ReadOnly(speckit.Toggle(core.Tally)),
	)
}
