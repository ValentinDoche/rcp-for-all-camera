// Package widgets holds the custom Fyne widgets of the GUI (color wheel,
// iris pad, meters, segmented control...) and the pure logic behind them.
package widgets

import (
	"math"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Range is the numeric domain of a range control.
type Range struct{ Min, Max, Step float64 }

// RangeOf extracts the Range of a range spec.
func RangeOf(s core.ControlSpec) Range { return Range{Min: s.Min, Max: s.Max, Step: s.Step} }

// Quantize clamps v into the range and snaps it to the step (counted from
// Min). A zero step only clamps.
func Quantize(v float64, r Range) float64 {
	if math.IsNaN(v) {
		return r.Min
	}
	v = math.Max(r.Min, math.Min(r.Max, v))
	if r.Step > 0 {
		v = r.Min + math.Round((v-r.Min)/r.Step)*r.Step
		v = math.Max(r.Min, math.Min(r.Max, v))
		v = math.Round(v*1e9) / 1e9
	}
	return v
}

// Norm maps v to 0..1 inside the range.
func Norm(v float64, r Range) float64 {
	if r.Max <= r.Min {
		return 0
	}
	return math.Max(0, math.Min(1, (v-r.Min)/(r.Max-r.Min)))
}

// --- color wheel -----------------------------------------------------------

// channelAngle is the hue angle of R, G, B on the wheel (counterclockwise from
// the right), the same as an HSV hue ring.
var channelAngle = [3]float64{0, 2 * math.Pi / 3, 4 * math.Pi / 3}

// PointToUnit converts a pointer position (screen coordinates, y down) to the
// unit disc (y up), clamped to the disc.
func PointToUnit(px, py, cx, cy, radius float64) (x, y float64) {
	if radius <= 0 {
		return 0, 0
	}
	x, y = (px-cx)/radius, (cy-py)/radius
	if n := math.Hypot(x, y); n > 1 {
		x, y = x/n, y/n
	}
	return x, y
}

// WheelSpan is how far a channel moves from the master value when the puck is
// at the rim: an eighth of the control range, at most 1.
func WheelSpan(min, max float64) float64 {
	s := (max - min) / 8
	if s <= 0 {
		return 1
	}
	return math.Min(s, 1)
}

// PuckToRGB gives the R, G, B values for a puck at (x, y) on the unit disc:
// each channel is the master value y0 plus span times the projection of the
// puck on the channel's hue, clamped to [min, max].
func PuckToRGB(x, y, y0, span, min, max float64) (r, g, b float64) {
	var out [3]float64
	for i, a := range channelAngle {
		v := y0 + span*(x*math.Cos(a)+y*math.Sin(a))
		out[i] = math.Max(min, math.Min(max, v))
	}
	return out[0], out[1], out[2]
}

// RGBToPuck is the inverse of PuckToRGB (least squares, exact when no
// channel was clamped). The result is clamped to the disc.
func RGBToPuck(r, g, b, y0, span float64) (x, y float64) {
	if span <= 0 {
		return 0, 0
	}
	for i, v := range [3]float64{r, g, b} {
		o := v - y0
		x += o * math.Cos(channelAngle[i])
		y += o * math.Sin(channelAngle[i])
	}
	x, y = x*2/3/span, y*2/3/span
	if n := math.Hypot(x, y); n > 1 {
		x, y = x/n, y/n
	}
	return x, y
}

// Neutral is the "no correction" value of a color control, clamped into its
// range: lift and gamma at 0, gain at 1.
func Neutral(id core.ControlID, min, max float64) float64 {
	v := 0.0
	switch id {
	case core.GainR, core.GainG, core.GainB, core.GainY:
		v = 1
	}
	return math.Max(min, math.Min(max, v))
}

// --- iris pad ----------------------------------------------------------------

// PadToValues maps a pad position in [-1, 1] (y up) to the iris (vertical,
// up = open) and master black (horizontal) values. A nil range gives a nil
// result: that axis is not driven.
func PadToValues(x, y float64, iris, masterBlack *Range) (irisV, mbV *float64) {
	at := func(r *Range, u float64) *float64 {
		if r == nil {
			return nil
		}
		v := Quantize(r.Min+(u+1)/2*(r.Max-r.Min), *r)
		return &v
	}
	return at(iris, y), at(masterBlack, x)
}

// ValuesToPad is the inverse of PadToValues. A missing axis stays centred.
func ValuesToPad(masterBlack, iris *Range, mbV, irisV float64) (x, y float64) {
	if masterBlack != nil {
		x = Norm(mbV, *masterBlack)*2 - 1
	}
	if iris != nil {
		y = Norm(irisV, *iris)*2 - 1
	}
	return x, y
}

// --- generic renderer choice -----------------------------------------------------

// WidgetKind is the kind of widget the generic renderer builds for a control.
type WidgetKind string

const (
	KindSlider    WidgetKind = "slider"
	KindSegmented WidgetKind = "segmented"
	KindSelect    WidgetKind = "select"
	KindCheck     WidgetKind = "check"
	KindButton    WidgetKind = "button"
	KindLabel     WidgetKind = "label"
	KindStepper   WidgetKind = "stepper" // − and + around the value, no slider
	KindText      WidgetKind = "text"    // a text field
	KindReadout   WidgetKind = "readout" // a read-only value, large
)

// MaxSegmented is the largest enum shown as a segmented control.
const MaxSegmented = 5

// ChooseKind decides how a control is shown in the details page: the
// widget the module's page asks for (w), else the default widget of its
// kind.
func ChooseKind(s core.ControlSpec, w core.Widget) WidgetKind {
	if w != "" && w.Fits(s.Kind, s.ReadOnly) {
		switch w {
		case core.WidgetSlider:
			return KindSlider
		case core.WidgetStepper:
			return KindStepper
		case core.WidgetButtons:
			return KindSegmented
		case core.WidgetSelect:
			return KindSelect
		case core.WidgetSwitch:
			return KindCheck
		case core.WidgetButton:
			return KindButton
		case core.WidgetReadout:
			return KindReadout
		case core.WidgetText:
			if !s.ReadOnly {
				return KindText
			}
		}
	}
	if s.ReadOnly {
		return KindLabel
	}
	switch s.Kind {
	case core.KindRange:
		return KindSlider
	case core.KindEnum:
		if len(s.Options) <= MaxSegmented {
			return KindSegmented
		}
		return KindSelect
	case core.KindToggle:
		return KindCheck
	case core.KindAction:
		return KindButton
	case core.KindText:
		return KindText
	}
	return KindLabel
}

// --- throttle -------------------------------------------------------------------

// Throttler rate-limits writes per key. Calls with the same key run one at a
// time, never closer than the interval; while one runs or waits, newer calls
// replace the pending one, so the final value is always sent and stale
// intermediate values are dropped.
type Throttler struct {
	interval time.Duration
	mu       sync.Mutex
	keys     map[string]*throttleKey
	wg       sync.WaitGroup
}

type throttleKey struct{ pending func() }

func NewThrottler(interval time.Duration) *Throttler {
	return &Throttler{interval: interval, keys: map[string]*throttleKey{}}
}

// Do schedules fn for key. fn runs on a Throttler goroutine.
func (t *Throttler) Do(key string, fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if k, ok := t.keys[key]; ok {
		k.pending = fn
		return
	}
	k := &throttleKey{pending: fn}
	t.keys[key] = k
	t.wg.Add(1)
	go t.run(key, k)
}

func (t *Throttler) run(key string, k *throttleKey) {
	defer t.wg.Done()
	for {
		t.mu.Lock()
		fn := k.pending
		k.pending = nil
		if fn == nil {
			delete(t.keys, key)
			t.mu.Unlock()
			return
		}
		t.mu.Unlock()
		start := time.Now()
		fn()
		if d := t.interval - time.Since(start); d > 0 {
			time.Sleep(d)
		}
	}
}

// Wait blocks until every scheduled call has run (used by tests).
func (t *Throttler) Wait() { t.wg.Wait() }
