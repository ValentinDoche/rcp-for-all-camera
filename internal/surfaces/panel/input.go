package panel

import (
	"math"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// standardAngles are the shutter angles the SHUTTER keys step through.
var standardAngles = []float64{11.25, 15, 22.5, 45, 72, 90, 144, 172.8, 180, 216, 270, 360}

// step returns the value one key press away from cur (dir +1 = up, -1 =
// down). ok is false when the value cannot move that way. Up raises the
// number shown: next enum option in declared order, higher range value.
func step(s core.ControlSpec, cur core.Value, dir int) (core.Value, bool) {
	switch s.Kind {
	case core.KindEnum:
		i := optionIndex(s, cur.Choice)
		if i < 0 {
			return core.Value{}, false
		}
		j := i + dir
		if j < 0 || j >= len(s.Options) {
			return core.Value{}, false
		}
		return core.Choice(s.Options[j].Value), true
	case core.KindRange:
		var next float64
		if s.ID == core.ShutterAngle {
			next = nextAngle(s, cur.Number, dir)
		} else {
			inc := s.Step
			if inc <= 0 {
				inc = (s.Max - s.Min) / 20
			}
			next = quantize(s, cur.Number+float64(dir)*inc)
		}
		next = min(max(next, s.Min), s.Max)
		if next == cur.Number || dir > 0 && next < cur.Number || dir < 0 && next > cur.Number {
			return core.Value{}, false
		}
		return core.Number(next), true
	}
	return core.Value{}, false
}

// nextAngle is the next standard angle beyond cur inside the spec range, or
// cur when there is none.
func nextAngle(s core.ControlSpec, cur float64, dir int) float64 {
	const eps = 1e-6
	if dir > 0 {
		for _, a := range standardAngles {
			if a > cur+eps && a <= s.Max {
				return a
			}
		}
		return cur
	}
	for i := len(standardAngles) - 1; i >= 0; i-- {
		if a := standardAngles[i]; a < cur-eps && a >= s.Min {
			return a
		}
	}
	return cur
}

// canStep reports whether a key press in direction dir would change the value.
func canStep(s core.ControlSpec, cur core.Value, dir int) bool {
	_, ok := step(s, cur, dir)
	return ok
}

// quantize rounds v to the spec step from Min.
func quantize(s core.ControlSpec, v float64) float64 {
	if s.Step <= 0 {
		return v
	}
	n := math.Round((v - s.Min) / s.Step)
	// Round away float noise such as 2.8000000000000003.
	return math.Round((s.Min+n*s.Step)*1e9) / 1e9
}

// knobValue applies a color knob movement to cur: a full knob travel
// (micropanel.AnalogMax counts) spans the whole control range. The result
// is clamped, not quantized, so slow turns accumulate.
func knobValue(s core.ControlSpec, cur float64, delta int) float64 {
	v := cur + float64(delta)/micropanel.AnalogMax*(s.Max-s.Min)
	return min(max(v, s.Min), s.Max)
}

// knobDeadband is the knob movement (in counts) ignored as noise: some
// knobs jitter by one count at rest.
const knobDeadband = 1

// knobStep returns the movement to apply for a knob now at pos, measured
// from ref, the position last applied. Moves within the deadband are not
// applied and do not move ref, so jitter writes nothing while a slow turn
// still accumulates.
func knobStep(ref, pos int) (delta, newRef int) {
	if d := pos - ref; d > knobDeadband || d < -knobDeadband {
		return d, pos
	}
	return 0, ref
}

// knobControls maps the 0x0E knob indexes (micropanel.AnalogNames) to the
// controls they drive. Index 6 (top-right knob) is not mapped; SENS and
// COARSE (7, 8) set the iris lever window (irisWindow).
var knobControls = [6]core.ControlID{core.GainR, core.LiftR, core.GainG, core.LiftG, core.GainB, core.LiftB}

// leverPos maps a lever or ring position (-4096..4096) to 0..1.
func leverPos(raw int32) float64 {
	return clamp01((float64(raw) + micropanel.JoystickMax) / (2 * micropanel.JoystickMax))
}

// irisAt maps a lever position (0 = pulled back = closed, 1 = open) to an
// iris value inside the lever window w: logarithmic between w.hi (closed)
// and w.lo (open) for f-stops, linear for a normalized iris (1 = open,
// TODO(verify) per driver). The result is on the spec's step grid.
func irisAt(s core.ControlSpec, w window, pos float64) float64 {
	var v float64
	if isFStop(s) && w.lo > 0 {
		v = w.hi * math.Pow(w.lo/w.hi, pos)
	} else {
		v = w.lo + pos*(w.hi-w.lo)
	}
	return min(max(quantize(s, v), s.Min), s.Max)
}

// irisPos is the lever position matching an iris value (inverse of irisAt).
func irisPos(s core.ControlSpec, w window, v float64) float64 {
	if isFStop(s) && w.lo > 0 && w.hi > w.lo {
		return clamp01(math.Log(v/w.hi) / math.Log(w.lo/w.hi))
	}
	return rangePos(core.ControlSpec{Min: w.lo, Max: w.hi}, v)
}

// irisKnobs are the COARSE and SENS positions (0..micropanel.AnalogMax).
// Until read is true the lever covers the whole iris range.
type irisKnobs struct {
	coarse, sens int
	read         bool
}

// Knob indexes in report 0x0E (micropanel.AnalogNames).
const (
	knobSens   = 7
	knobCoarse = 8
)

// irisCentre is the f-stop COARSE points at: logarithmic from Min (knob
// fully counter-clockwise, 0 = most open) to Max (fully clockwise).
func irisCentre(s core.ControlSpec, k irisKnobs) float64 {
	t := clamp01(float64(k.coarse) / micropanel.AnalogMax)
	return s.Min * math.Pow(s.Max/s.Min, t)
}

// irisWindowStops is the lever window width in stops set by SENS: 1 stop
// with the knob fully counter-clockwise up to the whole range (full) fully
// clockwise, linear in stops.
func irisWindowStops(full float64, k irisKnobs) float64 {
	t := clamp01(float64(k.sens) / micropanel.AnalogMax)
	return 1 + (full-1)*t
}

// irisWindow is the part of the iris range the lever travels over (broadcast
// RCP convention: COARSE sets where, SENS how wide). It is centred on
// irisCentre, W stops wide (one stop = x√2 in f-number, so the window is
// C / 2^(W/4) .. C x 2^(W/4)), and shifted to keep its width when it hits
// an end of the range. The whole range when the knobs are unknown or the
// iris is not in f-stops.
func irisWindow(s core.ControlSpec, k irisKnobs) window {
	whole := window{s.Min, s.Max}
	if !k.read || !isFStop(s) || s.Min <= 0 || s.Max <= s.Min {
		return whole
	}
	full := 2 * math.Log2(s.Max/s.Min) // range width in stops
	if full <= 1 {
		return whole
	}
	c := irisCentre(s, k)
	half := math.Pow(2, irisWindowStops(full, k)/4)
	lo, hi := c/half, c*half
	if lo < s.Min {
		hi, lo = hi*s.Min/lo, s.Min
	}
	if hi > s.Max {
		lo, hi = lo*s.Max/hi, s.Max
	}
	return window{max(lo, s.Min), min(hi, s.Max)}
}

// window is the part of a control's range a lever or ring travels over.
type window struct{ lo, hi float64 }

// reaches reports whether v lies inside the window (half a step of slack),
// that is whether the lever can pick it up.
func (w window) reaches(s core.ControlSpec, v float64) bool {
	tol := max(s.Step/2, 1e-9)
	return v >= w.lo-tol && v <= w.hi+tol
}

// mbWindow is the master black ring's travel: the range limited so the MB
// display stays within ±99 (±0.99 for ranges shown x100).
func mbWindow(s core.ControlSpec) window {
	lim := mbShown / mbScale(s)
	if lo, hi := max(s.Min, -lim), min(s.Max, lim); lo < hi {
		return window{lo, hi}
	}
	return window{s.Min, s.Max}
}

// mbAt and mbPos map the master black ring linearly onto its window
// (mbWindow).
func mbAt(s core.ControlSpec, w window, pos float64) float64 {
	return min(max(quantize(s, w.lo+pos*(w.hi-w.lo)), w.lo), w.hi)
}

func mbPos(_ core.ControlSpec, w window, v float64) float64 {
	return rangePos(core.ControlSpec{Min: w.lo, Max: w.hi}, v)
}

// pickupTol is how close (in lever travel, 0..1) the control must come to
// the camera value to take over without crossing it.
const pickupTol = 0.01

// pickup implements soft takeover (SPEC CB-1) for an absolute control the
// software cannot move: it only drives the camera after reaching or
// crossing the camera's current value.
type pickup struct {
	picked  bool
	havePos bool
	pos     float64
}

// seed records the control position without taking over (first report
// after a connection).
func (p *pickup) seed(pos float64) { p.pos, p.havePos, p.picked = pos, true, false }

// release gives control back until the next pickup (camera switch, value
// changed elsewhere, value unknown).
func (p *pickup) release() { p.picked = false }

// move records a new position. target is the camera value as a position,
// known says whether the camera value is known. It reports whether the
// control now drives the camera.
func (p *pickup) move(pos, target float64, known bool) bool {
	prev, had := p.pos, p.havePos
	p.pos, p.havePos = pos, true
	if !known {
		p.picked = false
		return false
	}
	if !p.picked && (math.Abs(pos-target) <= pickupTol || had && (prev-target)*(pos-target) < 0) {
		p.picked = true
	}
	return p.picked
}
