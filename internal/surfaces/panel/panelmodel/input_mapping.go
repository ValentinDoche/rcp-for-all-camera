package panelmodel

import (
	"math"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// StandardAngles are the shutter angles the SHUTTER keys step through.
var StandardAngles = []float64{11.25, 15, 22.5, 45, 72, 90, 144, 172.8, 180, 216, 270, 360}

// Step returns the value one key press away from cur (dir +1 = up, -1 =
// down). ok is false when the value cannot move that way. Up raises the
// number shown: next enum option in declared order, higher range value.
// White balance moves by at least wbMinStep.
func Step(s core.ControlSpec, cur core.Value, dir int) (core.Value, bool) {
	switch s.Kind {
	case core.KindEnum:
		i := OptionIndex(s, cur.Choice)
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
			next = NextAngle(s, cur.Number, dir)
		} else {
			inc := s.Step
			if inc <= 0 {
				inc = (s.Max - s.Min) / 20
			}
			if s.ID == core.WhiteBalance {
				inc = max(inc, WBMinStep)
			}
			next = Quantize(s, cur.Number+float64(dir)*inc)
		}
		next = min(max(next, s.Min), s.Max)
		if next == cur.Number || dir > 0 && next < cur.Number || dir < 0 && next > cur.Number {
			return core.Value{}, false
		}
		return core.Number(next), true
	}
	return core.Value{}, false
}

// WBMinStep is the smallest white balance change of one key press (Kelvin).
const WBMinStep = 100

// NextAngle is the next standard angle beyond cur inside the spec range, or
// cur when there is none.
func NextAngle(s core.ControlSpec, cur float64, dir int) float64 {
	const eps = 1e-6
	if dir > 0 {
		for _, a := range StandardAngles {
			if a > cur+eps && a <= s.Max {
				return a
			}
		}
		return cur
	}
	for i := len(StandardAngles) - 1; i >= 0; i-- {
		if a := StandardAngles[i]; a < cur-eps && a >= s.Min {
			return a
		}
	}
	return cur
}

// CanStep reports whether a key press in direction dir would change the value.
func CanStep(s core.ControlSpec, cur core.Value, dir int) bool {
	_, ok := Step(s, cur, dir)
	return ok
}

// Quantize rounds v to the spec step from Min.
func Quantize(s core.ControlSpec, v float64) float64 {
	if s.Step <= 0 {
		return v
	}
	n := math.Round((v - s.Min) / s.Step)
	// Round away float noise such as 2.8000000000000003.
	return math.Round((s.Min+n*s.Step)*1e9) / 1e9
}

// KnobSpan is the width a full knob travel (micropanel.AnalogMax counts)
// covers, centred on the control's neutral: gain 0.5..1.5, lift -0.5..+0.5.
const KnobSpan = 1.0

// KnobWindow is a color knob's travel: knobSpan around the neutral value
// (1 for gain, 0 for lift), inside the control's range.
func KnobWindow(s core.ControlSpec) Window {
	n := 0.0
	switch s.ID {
	case core.GainR, core.GainG, core.GainB, core.GainY:
		n = 1
	}
	if lo, hi := max(s.Min, n-KnobSpan/2), min(s.Max, n+KnobSpan/2); lo < hi {
		return Window{lo, hi}
	}
	return Window{s.Min, s.Max}
}

// KnobValue applies a color knob movement to cur: a full knob travel spans
// the knob window (knobWindow), and the result stays inside it. It is not
// quantized, so slow turns accumulate. A value set outside the window
// elsewhere is brought back into it by the next movement.
func KnobValue(s core.ControlSpec, cur float64, delta int) float64 {
	w := KnobWindow(s)
	v := cur + float64(delta)/micropanel.AnalogMax*(w.Hi-w.Lo)
	return min(max(v, w.Lo), w.Hi)
}

// The panel counts each knob turn itself and clamps the count at 0 and
// micropanel.AnalogMax: a color knob left near an end could not move further
// that way. The controller recenters their counters to KnobCentre when one
// comes within KnobMargin of an end (and at connect).
const (
	KnobCentre = micropanel.AnalogMax / 2
	KnobMargin = 128
)

func NearKnobEnd(pos int) bool { return pos < KnobMargin || pos > micropanel.AnalogMax-KnobMargin }

// KnobDeadband is the knob movement (in counts) ignored as noise: some
// knobs jitter by one count at rest.
const KnobDeadband = 1

// KnobStep returns the movement to apply for a knob now at pos, measured
// from ref, the position last applied. Moves within the deadband are not
// applied and do not move ref, so jitter writes nothing while a slow turn
// still accumulates.
func KnobStep(ref, pos int) (delta, newRef int) {
	if d := pos - ref; d > KnobDeadband || d < -KnobDeadband {
		return d, pos
	}
	return 0, ref
}

// KnobControls maps the 0x0E knob indexes (micropanel.AnalogNames) to the
// controls they drive. Index 6 (top-right knob) is not mapped; SENS and
// COARSE (7, 8) set the iris lever window (irisWindow).
var KnobControls = [6]core.ControlID{core.GainR, core.LiftR, core.GainG, core.LiftG, core.GainB, core.LiftB}

// LeverPos maps a lever or ring position (-4096..4096) to 0..1.
func LeverPos(raw int32) float64 {
	return Clamp01((float64(raw) + micropanel.JoystickMax) / (2 * micropanel.JoystickMax))
}

// IrisAt maps a lever position (0 = pulled back = closed, 1 = open) to an
// iris value inside the lever window w: logarithmic between w.hi (closed)
// and w.lo (open) for f-stops, linear for a normalized iris (1 = open,
// TODO(verify) per driver). The result is on the spec's step grid.
func IrisAt(s core.ControlSpec, w Window, pos float64) float64 {
	var v float64
	if IsFStop(s) && w.Lo > 0 {
		v = w.Hi * math.Pow(w.Lo/w.Hi, pos)
	} else {
		v = w.Lo + pos*(w.Hi-w.Lo)
	}
	return min(max(Quantize(s, v), s.Min), s.Max)
}

// IrisPos is the lever position matching an iris value (inverse of irisAt).
func IrisPos(s core.ControlSpec, w Window, v float64) float64 {
	if IsFStop(s) && w.Lo > 0 && w.Hi > w.Lo {
		return Clamp01(math.Log(v/w.Hi) / math.Log(w.Lo/w.Hi))
	}
	return RangePos(core.ControlSpec{Min: w.Lo, Max: w.Hi}, v)
}

// IrisKnobs are the COARSE and SENS positions (0..micropanel.AnalogMax).
// Until read is true the lever covers the whole iris range.
type IrisKnobs struct {
	Coarse, Sens int
	Read         bool
}

// Knob indexes in report 0x0E (micropanel.AnalogNames).
const (
	KnobSens   = 7
	KnobCoarse = 8
)

// IrisCentre is the f-stop COARSE points at: logarithmic from Min (knob
// fully counter-clockwise, 0 = most open) to Max (fully clockwise).
func IrisCentre(s core.ControlSpec, k IrisKnobs) float64 {
	t := Clamp01(float64(k.Coarse) / micropanel.AnalogMax)
	return s.Min * math.Pow(s.Max/s.Min, t)
}

// IrisWindowStops is the lever window width in stops set by SENS: 1 stop
// with the knob fully counter-clockwise up to the whole range (full) fully
// clockwise, linear in stops.
func IrisWindowStops(full float64, k IrisKnobs) float64 {
	t := Clamp01(float64(k.Sens) / micropanel.AnalogMax)
	return 1 + (full-1)*t
}

// IrisWindow is the part of the iris range the lever travels over (broadcast
// RCP convention: COARSE sets where, SENS how wide). It is centred on
// irisCentre, W stops wide (one stop = x√2 in f-number, so the window is
// C / 2^(W/4) .. C x 2^(W/4)), and shifted to keep its width when it hits
// an end of the range. The whole range when the knobs are unknown or the
// iris is not in f-stops.
func IrisWindow(s core.ControlSpec, k IrisKnobs) Window {
	whole := Window{s.Min, s.Max}
	if !k.Read || !IsFStop(s) || s.Min <= 0 || s.Max <= s.Min {
		return whole
	}
	full := 2 * math.Log2(s.Max/s.Min) // range width in stops
	if full <= 1 {
		return whole
	}
	c := IrisCentre(s, k)
	half := math.Pow(2, IrisWindowStops(full, k)/4)
	lo, hi := c/half, c*half
	if lo < s.Min {
		hi, lo = hi*s.Min/lo, s.Min
	}
	if hi > s.Max {
		lo, hi = lo*s.Max/hi, s.Max
	}
	return Window{max(lo, s.Min), min(hi, s.Max)}
}

// Window is the part of a control's range a lever or ring travels over.
type Window struct{ Lo, Hi float64 }

// Reaches reports whether v lies inside the window (half a step of slack),
// that is whether the lever can pick it up.
func (w Window) Reaches(s core.ControlSpec, v float64) bool {
	tol := max(s.Step/2, 1e-9)
	return v >= w.Lo-tol && v <= w.Hi+tol
}

// MBWindow is the master black ring's travel: the range limited so the MB
// display stays within ±99 (±0.99 for ranges shown x100).
func MBWindow(s core.ControlSpec) Window {
	lim := MBShown / MBScale(s)
	if lo, hi := max(s.Min, -lim), min(s.Max, lim); lo < hi {
		return Window{lo, hi}
	}
	return Window{s.Min, s.Max}
}

// mbAt and mbPos map the master black ring linearly onto its window
// (mbWindow).
// MBDeadzone is the share of the ring travel, on each side of its centre,
// that stays exactly at 0: without it the ring can hardly rest on 0.
const MBDeadzone = 0.01

// MBAt maps the ring (0..1) onto the master black. When the travel spans 0,
// the centre ± mbDeadzone gives exactly 0, the left part covers lo..0 and
// the right part 0..hi, so both ends are still reached.
func MBAt(s core.ControlSpec, w Window, pos float64) float64 {
	if !(w.Lo < 0 && w.Hi > 0) {
		return min(max(Quantize(s, w.Lo+pos*(w.Hi-w.Lo)), w.Lo), w.Hi)
	}
	var v float64
	switch d := pos - 0.5; {
	case math.Abs(d) <= MBDeadzone:
		return 0
	case d > 0:
		v = (d - MBDeadzone) / (0.5 - MBDeadzone) * w.Hi
	default:
		v = (-d - MBDeadzone) / (0.5 - MBDeadzone) * w.Lo
	}
	return min(max(Quantize(s, v), w.Lo), w.Hi)
}

// MBPos is the ring position of a master black value (inverse of mbAt; 0
// sits at the centre of the dead zone).
func MBPos(_ core.ControlSpec, w Window, v float64) float64 {
	if !(w.Lo < 0 && w.Hi > 0) {
		return RangePos(core.ControlSpec{Min: w.Lo, Max: w.Hi}, v)
	}
	switch {
	case v > 0:
		return Clamp01(0.5 + MBDeadzone + min(v/w.Hi, 1)*(0.5-MBDeadzone))
	case v < 0:
		return Clamp01(0.5 - MBDeadzone - min(v/w.Lo, 1)*(0.5-MBDeadzone))
	}
	return 0.5
}

// PickupTol is how close (in lever travel, 0..1) the control must come to
// the camera value to take over without crossing it.
const PickupTol = 0.01

// Pickup implements soft takeover (SPEC CB-1) for an absolute control the
// software cannot move: it only drives the camera after reaching or
// crossing the camera's current value.
type Pickup struct {
	Picked  bool
	HavePos bool
	Pos     float64
}

// Seed records the control position without taking over (first report
// after a connection).
func (p *Pickup) Seed(pos float64) { p.Pos, p.HavePos, p.Picked = pos, true, false }

// Release gives control back until the next pickup (camera switch, value
// changed elsewhere, value unknown).
func (p *Pickup) Release() { p.Picked = false }

// Move records a new position. target is the camera value as a position,
// known says whether the camera value is known. It reports whether the
// control now drives the camera.
func (p *Pickup) Move(pos, target float64, known bool) bool {
	prev, had := p.Pos, p.HavePos
	p.Pos, p.HavePos = pos, true
	if !known {
		p.Picked = false
		return false
	}
	if !p.Picked && (math.Abs(pos-target) <= PickupTol || had && (prev-target)*(pos-target) < 0) {
		p.Picked = true
	}
	return p.Picked
}
