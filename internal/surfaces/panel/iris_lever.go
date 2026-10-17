package panel

import (
	"math"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// When the camera offers core.IrisNorm (the iris over the mounted lens'
// range, 0 = open, 1 = closed), the lever drives it instead of the f-stop,
// as ATEM Software Control does (setApertureNormalised(1 - lever)): the
// whole travel is the lens' range, so the lever can never ask for a stop
// the lens lacks, and a f/2.8 lens tops out at 2.8 with the lever forward.
// The displays and the iris scale keep showing the f-stop (core.Iris).

// normMinWidth is the narrowest lever window SENS gives, as a fraction of
// the lens' range.
const normMinWidth = 0.1

// irisLeverID is the control the iris lever drives on c: IrisNorm when the
// camera declares it and reports its value, else the f-stop.
func irisLeverID(c camState) core.ControlID {
	if _, _, declared, known := c.get(core.IrisNorm); declared && known {
		return core.IrisNorm
	}
	return core.Iris
}

// normWindow is the lever window over the lens' range: COARSE sets where
// (fully counter-clockwise = most open), SENS how wide (fully clockwise =
// the whole range). The whole range until the knobs are read.
func normWindow(k irisKnobs) window {
	if !k.read {
		return window{0, 1}
	}
	w := normMinWidth + (1-normMinWidth)*clamp01(float64(k.sens)/micropanel.AnalogMax)
	c := clamp01(float64(k.coarse) / micropanel.AnalogMax)
	lo, hi := c-w/2, c+w/2
	if lo < 0 {
		lo, hi = 0, w
	}
	if hi > 1 {
		lo, hi = 1-w, 1
	}
	return window{max(lo, 0), min(hi, 1)}
}

// normAt is the IrisNorm value under the lever (forward = the window's open
// end), normPos its inverse.
func normAt(s core.ControlSpec, w window, pos float64) float64 {
	return min(max(quantize(s, w.hi-pos*(w.hi-w.lo)), s.Min), s.Max)
}

func normPos(_ core.ControlSpec, w window, v float64) float64 {
	if w.hi <= w.lo {
		return 0
	}
	return clamp01((w.hi - v) / (w.hi - w.lo))
}

// irisView is the lever window and the COARSE centre in f-stops, for the
// iris scale and the previews. With IrisNorm the window is mapped onto the
// lens' f-stops, logarithmically between its ends (the lens limits when the
// camera reports them, else the control's range): an estimate, the camera
// alone knows its lens' curve.
func irisView(c camState, k irisKnobs) (w window, centre float64) {
	s, _, _, _ := c.get(core.Iris)
	if irisLeverID(c) != core.IrisNorm || !isFStop(s) || s.Min <= 0 || s.Max <= s.Min {
		return irisWindow(s, k), irisCentre(s, k)
	}
	f := func(n float64) float64 { return s.Min * math.Pow(s.Max/s.Min, n) }
	nw := normWindow(k)
	return window{f(nw.lo), f(nw.hi)}, f((nw.lo + nw.hi) / 2)
}
