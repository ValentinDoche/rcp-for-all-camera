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

// knobControls maps the 0x0E knob indexes (micropanel.AnalogNames) to the
// controls they drive. Indexes 6..8 (top-right knob, SENS, COARSE) are not
// mapped.
var knobControls = [6]core.ControlID{core.GainR, core.LiftR, core.GainG, core.LiftG, core.GainB, core.LiftB}

// leverPos maps a lever or ring position (-4096..4096) to 0..1.
func leverPos(raw int32) float64 {
	return clamp01((float64(raw) + micropanel.JoystickMax) / (2 * micropanel.JoystickMax))
}

// irisAt maps a lever position (0 = pulled back = closed, 1 = open) to an
// iris value: logarithmic between Max (closed) and Min (open) for f-stops,
// linear for a normalized iris (1 = open, TODO(verify) per driver).
func irisAt(s core.ControlSpec, pos float64) float64 {
	if isFStop(s) && s.Min > 0 {
		return quantize(s, s.Max*math.Pow(s.Min/s.Max, pos))
	}
	return quantize(s, s.Min+pos*(s.Max-s.Min))
}

// irisPos is the lever position matching an iris value (inverse of irisAt).
func irisPos(s core.ControlSpec, v float64) float64 {
	if isFStop(s) && s.Min > 0 && s.Max > s.Min {
		return clamp01(math.Log(v/s.Max) / math.Log(s.Min/s.Max))
	}
	return rangePos(s, v)
}

// mbAt and mbPos map the master black ring linearly onto the range.
func mbAt(s core.ControlSpec, pos float64) float64 { return quantize(s, s.Min+pos*(s.Max-s.Min)) }
func mbPos(s core.ControlSpec, v float64) float64  { return rangePos(s, v) }

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
