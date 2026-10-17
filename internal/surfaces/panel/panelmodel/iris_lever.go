package panelmodel

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

// NormMinWidth is the narrowest lever window SENS gives, as a fraction of
// the lens' range.
const NormMinWidth = 0.1

// IrisLeverID is the control the iris lever drives on c: IrisNorm when the
// camera declares it and reports its value, else the f-stop.
func IrisLeverID(c Camera) core.ControlID {
	if _, _, declared, known := c.Get(core.IrisNorm); declared && known {
		return core.IrisNorm
	}
	return core.Iris
}

// NormWindow is the lever window over the lens' range: COARSE sets where
// (fully counter-clockwise = most open), SENS how wide (fully clockwise =
// the whole range). The whole range until the knobs are read.
func NormWindow(k IrisKnobs) Window {
	if !k.Read {
		return Window{0, 1}
	}
	w := NormMinWidth + (1-NormMinWidth)*Clamp01(float64(k.Sens)/micropanel.AnalogMax)
	c := Clamp01(float64(k.Coarse) / micropanel.AnalogMax)
	lo, hi := c-w/2, c+w/2
	if lo < 0 {
		lo, hi = 0, w
	}
	if hi > 1 {
		lo, hi = 1-w, 1
	}
	return Window{max(lo, 0), min(hi, 1)}
}

// NormAt is the IrisNorm value under the lever (forward = the window's open
// end), normPos its inverse.
func NormAt(s core.ControlSpec, w Window, pos float64) float64 {
	return min(max(Quantize(s, w.Hi-pos*(w.Hi-w.Lo)), s.Min), s.Max)
}

func NormPos(_ core.ControlSpec, w Window, v float64) float64 {
	if w.Hi <= w.Lo {
		return 0
	}
	return Clamp01((w.Hi - v) / (w.Hi - w.Lo))
}

// IrisView is the lever window and the COARSE centre in f-stops, for the
// iris scale and the previews. With IrisNorm the window is mapped onto the
// lens' f-stops, logarithmically between its ends (the lens limits when the
// camera reports them, else the control's range): an estimate, the camera
// alone knows its lens' curve.
func IrisView(c Camera, k IrisKnobs) (w Window, centre float64) {
	s, _, _, _ := c.Get(core.Iris)
	if IrisLeverID(c) != core.IrisNorm || !IsFStop(s) || s.Min <= 0 || s.Max <= s.Min {
		return IrisWindow(s, k), IrisCentre(s, k)
	}
	f := func(n float64) float64 { return s.Min * math.Pow(s.Max/s.Min, n) }
	nw := NormWindow(k)
	return Window{f(nw.Lo), f(nw.Hi)}, f((nw.Lo + nw.Hi) / 2)
}
