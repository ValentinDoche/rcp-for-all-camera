package panasonic

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

type opts struct {
	ptz, nd bool
}

func family(o opts) []core.ControlSpec {
	s := []core.ControlSpec{
		speckit.Range(core.Iris, 0, 1, 0.01, ""), // TODO(verify): AW protocol uses a 0..255 position
		speckit.Action(core.AutoIris),
		speckit.Range(core.Focus, 0, 1, 0.001, ""), // TODO(verify)
		speckit.Action(core.AutoFocus),
		speckit.Range(core.Gain, 0, 42, 1, "dB"), // TODO(verify): range per model
		speckit.Enum(core.ShutterSpeed, speckit.Opts(
			// TODO(verify): steps depend on frame rate.
			"1/24", "1/25", "1/30", "1/50", "1/60", "1/100", "1/120", "1/250", "1/500", "1/1000", "1/2000", "1/10000")),
	}
	if o.nd {
		s = append(s, speckit.Enum(core.ND, speckit.Labeled("clear", "Clear", "1/4", "1/4", "1/16", "1/16", "1/64", "1/64"))) // TODO(verify)
	}
	s = append(s,
		speckit.Range(core.WhiteBalance, 2000, 15000, 100, "K"), // TODO(verify)
		speckit.Action(core.AutoWhiteBalance),
		speckit.Range(core.MasterBlack, -150, 150, 1, ""), // TODO(verify): pedestal
		speckit.Range(core.Detail, -31, 31, 1, ""),        // TODO(verify)
		speckit.Toggle(core.ColorBars),
	)
	if o.ptz {
		s = append(s,
			speckit.Range(core.Pan, -1, 1, 0.01, ""),
			speckit.Range(core.Tilt, -1, 1, 0.01, ""),
			speckit.Range(core.Zoom, 0, 1, 0.001, ""),
			speckit.Enum(core.PresetRecall, speckit.Numbered(100)), // TODO(verify): preset count
		)
	}
	return append(s,
		speckit.Toggle(core.Record), // TODO(verify): record only when SD/stream recording is available
		speckit.ReadOnly(speckit.Toggle(core.Tally)),
	)
}
