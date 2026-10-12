package panel

import (
	"math"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// coarseFor is the COARSE knob position whose window centre is f.
func coarseFor(s core.ControlSpec, f float64) int {
	return int(math.Round(math.Log(f/s.Min) / math.Log(s.Max/s.Min) * micropanel.AnalogMax))
}

// sensFor is the SENS knob position giving a window of w stops.
func sensFor(s core.ControlSpec, w float64) int {
	full := 2 * math.Log2(s.Max/s.Min)
	return int(math.Round((w - 1) / (full - 1) * micropanel.AnalogMax))
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

func TestIrisWindow(t *testing.T) {
	s := rng(core.Iris, 1.4, 22, 0.1, "f")
	if w := irisWindow(s, irisKnobs{coarse: 0, sens: 0}); w != whole(s) {
		t.Errorf("knobs not read: window = %+v, want the whole range", w)
	}
	// SENS fully clockwise: the whole range, wherever COARSE is.
	for _, c := range []int{0, 300, 1024} {
		w := irisWindow(s, irisKnobs{coarse: c, sens: micropanel.AnalogMax, read: true})
		if !near(w.lo, 1.4, 1e-9) || !near(w.hi, 22, 1e-9) {
			t.Errorf("coarse %d, full sens: window = %+v, want 1.4..22", c, w)
		}
	}
	// COARSE: knob 0 = most open, 1024 = most closed, logarithmic.
	if c := irisCentre(s, irisKnobs{coarse: 0}); c != 1.4 {
		t.Errorf("centre at 0 = %v", c)
	}
	if c := irisCentre(s, irisKnobs{coarse: micropanel.AnalogMax}); !near(c, 22, 1e-9) {
		t.Errorf("centre at 1024 = %v", c)
	}
	// One stop centred on 5.6: 5.6 / 2^(1/4) .. 5.6 x 2^(1/4).
	k := irisKnobs{coarse: coarseFor(s, 5.6), sens: 0, read: true}
	w := irisWindow(s, k)
	c := irisCentre(s, k)
	if !near(c, 5.6, 0.02) || !near(w.lo, c/math.Pow(2, 0.25), 1e-9) || !near(w.hi, c*math.Pow(2, 0.25), 1e-9) {
		t.Errorf("1 stop at 5.6: window = %+v (centre %v)", w, c)
	}
	// Clamped at an end: shifted to keep the width.
	w = irisWindow(s, irisKnobs{coarse: 0, sens: 0, read: true})
	if w.lo != 1.4 || !near(w.hi, 1.4*math.Sqrt2, 1e-9) {
		t.Errorf("1 stop at the open end = %+v, want 1.4..1.98", w)
	}
	w = irisWindow(s, irisKnobs{coarse: micropanel.AnalogMax, sens: 0, read: true})
	if w.hi != 22 || !near(w.lo, 22/math.Sqrt2, 1e-9) {
		t.Errorf("1 stop at the closed end = %+v, want 15.6..22", w)
	}
	// The lever maps over the window: fully open = its open end.
	k = irisKnobs{coarse: coarseFor(s, 4), sens: sensFor(s, 2), read: true}
	w = irisWindow(s, k)
	if got := irisAt(s, w, 1); !near(got, w.lo, 0.05) {
		t.Errorf("lever open = %v, want %v", got, w.lo)
	}
	if got := irisAt(s, w, 0); !near(got, w.hi, 0.05) {
		t.Errorf("lever closed = %v, want %v", got, w.hi)
	}
	if got := irisAt(s, w, irisPos(s, w, 4)); got != 4 {
		t.Errorf("round trip F4 = %v", got)
	}
	if !w.reaches(s, 4) || w.reaches(s, 11) {
		t.Error("only values inside the window can be picked up")
	}
	// A normalized iris has no window.
	n := rng(core.Iris, 0, 1, 0.01, "")
	if w := irisWindow(n, irisKnobs{coarse: 500, sens: 0, read: true}); w != whole(n) {
		t.Errorf("normalized iris window = %+v", w)
	}
}

// irisBar returns the 16 iris scale segments of a frame.
func irisBar(f frame) (b [micropanel.LEDIrisBarCount]bool) {
	for i := range b {
		b[i] = ledOn(&f.leds, micropanel.LEDIrisBar0+i)
	}
	return
}

func segs(from, to int) (b [micropanel.LEDIrisBarCount]bool) {
	for i := from; i <= to; i++ {
		b[i] = true
	}
	return
}

// Owner's example: centred on 5.6 with a narrow sensitivity, 2.8 and the
// segments from 16 to CLS go dark.
func TestRenderIrisBarMaskedToTheWindow(t *testing.T) {
	s := sonySpecs()[0]
	v := baseView()                                                                 // camera at F4 (segment 5)
	v.knobs = irisKnobs{coarse: coarseFor(s, 5.6), sens: sensFor(s, 2), read: true} // ~4..8: segments 5..9
	if got := irisBar(render(v)); got != segs(5, 9) {
		t.Errorf("masked bar = %v, want segments 5..9", got)
	}
	// SENS view: the window itself, whatever the iris.
	v.cam.values[core.Iris] = core.Number(11)
	v.sens = true
	if got := irisBar(render(v)); got != segs(5, 9) {
		t.Errorf("SENS view = %v, want segments 5..9", got)
	}
	// Window reaching the closed end: lit down to CLS.
	v.sens = false
	v.knobs = irisKnobs{coarse: micropanel.AnalogMax, sens: sensFor(s, 2), read: true} // 11..22
	if got := irisBar(render(v)); got != segs(11, 15) {
		t.Errorf("bar = %v, want segments 11..15", got)
	}
	v.sens = true
	if got := irisBar(render(v)); got != segs(11, 15) {
		t.Errorf("SENS view = %v, want segments 11..15", got)
	}
}

func TestRenderCoarsePreview(t *testing.T) {
	s := sonySpecs()[0]
	v := baseView()
	v.knobs = irisKnobs{coarse: coarseFor(s, 5.6), sens: 0, read: true}
	v.coarse, v.blink = true, true
	f := render(v)
	want := digitsOf(map[micropanel.Display]string{micropanel.DispShutter: "F5.6"})
	if [4]byte(f.digits[4:8]) != [4]byte(want[4:8]) {
		t.Errorf("SHUTTER = % x, want F5.6", f.digits[4:8])
	}
	if ledOn(&f.leds, micropanel.LEDShutterUp) || ledOn(&f.leds, micropanel.LEDShutterDown) {
		t.Error("SHUTTER arrows must be off during the COARSE preview")
	}
	v.blink = false
	if f := render(v); [4]byte(f.digits[4:8]) != [4]byte{} {
		t.Error("the COARSE preview must blink")
	}
	for f, want := range map[float64]string{11: "F11", 22: "F22", 1.4: "F1.4"} {
		if got := fstopText(f); got != want {
			t.Errorf("fstopText(%v) = %q, want %q", f, got, want)
		}
	}
}

// COARSE blinks its centre on SHUTTER for 5 s after the last movement,
// moves nothing on the camera, then SHUTTER shows the shutter again.
func TestControllerCoarsePreviewTimesOut(t *testing.T) {
	s := sonySpecs()[0]
	c := New(Options{Logger: quietLog()}) // no writer: any camera write would panic
	t0 := time.Unix(1000, 0)              // blink phase on
	now := t0
	c.now = func() time.Time { return now }
	c.rig = rigWith(2, 1, 2, 4)
	c.cam = cam(2, sonySpecs(), sonyValues())
	st := micropanel.State{AnalogRead: true}
	st.Analog[knobSens] = micropanel.AnalogMax
	c.onInput(st) // connect: positions only, no preview
	shutter := func(at time.Time) [4]byte { f := c.frame(at); return [4]byte(f.digits[4:8]) }
	shown := func(text string) [4]byte {
		d := digitsOf(map[micropanel.Display]string{micropanel.DispShutter: text})
		return [4]byte(d[4:8])
	}
	fifty, f56 := shown("50"), shown("F5.6")
	if shutter(t0) != fifty {
		t.Fatal("no preview at connect")
	}
	st.Analog[knobCoarse] = uint16(coarseFor(s, 5.6))
	c.onInput(st)
	if shutter(t0) != f56 || shutter(t0.Add(250*time.Millisecond)) != [4]byte{} {
		t.Error("COARSE must blink its centre on SHUTTER")
	}
	now = t0.Add(3 * time.Second)
	st.Analog[knobCoarse]++ // within the deadband: not a movement
	c.onInput(st)
	if shutter(t0.Add(5*time.Second)) != fifty {
		t.Error("the preview must end 5 s after the last COARSE movement")
	}
	st.Analog[knobCoarse] += 10
	c.onInput(st) // a real movement at t0+3s restarts the 5 s
	if shutter(t0.Add(7*time.Second)) == fifty || shutter(t0.Add(8*time.Second)) != fifty {
		t.Error("the preview must last 5 s from the last movement")
	}
}

// The lever works inside the COARSE/SENS window; turning COARSE writes
// nothing and gives control back until the lever picks the iris up again.
func TestControllerIrisLeverWindow(t *testing.T) {
	s := sonySpecs()[0]
	h := start(t, sonyAPI(), Options{}) // camera at F4
	<-h.status
	h.loaded(t)
	k := irisKnobs{coarse: coarseFor(s, 4), sens: 0, read: true} // one stop around F4
	w := irisWindow(s, k)
	st := micropanel.State{AnalogRead: true}
	st.Analog[knobCoarse], st.Analog[knobSens] = uint16(k.coarse), uint16(k.sens)
	st.Lever = -4096
	h.input(t, st)
	for r := int32(-4096); r <= 4096; r += 256 {
		st.Lever = r
		h.input(t, st)
	}
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == quantize(s, w.lo) })
	for _, c := range h.api.callsCopy() {
		if c.id == string(core.Iris) && !w.reaches(s, c.v.Number) {
			t.Errorf("lever asked for f/%v outside the window %+v", c.v.Number, w)
		}
	}

	// Window moved around F11: the camera iris (~F3.4) is left outside, so
	// it is set to the value under the lever (fully open = the window's most
	// open stop) and the lever keeps control instead of being stuck.
	k2 := irisKnobs{coarse: coarseFor(s, 11), sens: 0, read: true}
	w2 := irisWindow(s, k2)
	before := len(h.api.callsCopy())
	st.Analog[knobCoarse] = uint16(k2.coarse)
	h.input(t, st)
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == quantize(s, w2.lo) })
	for r := int32(4096); r >= -4096; r -= 256 {
		st.Lever = r
		h.input(t, st)
	}
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == quantize(s, w2.hi) })
	for _, c := range h.api.callsCopy()[before:] {
		if c.id == string(core.Iris) && !w2.reaches(s, c.v.Number) {
			t.Errorf("lever asked for f/%v outside the new window %+v", c.v.Number, w2)
		}
	}
}

// A COARSE move that keeps the camera iris inside the window writes
// nothing: the lever simply has to pick the value up again.
func TestControllerCoarseInsideTheWindowWritesNothing(t *testing.T) {
	s := sonySpecs()[0]
	h := start(t, sonyAPI(), Options{}) // camera at F4
	<-h.status
	h.loaded(t)
	st := micropanel.State{AnalogRead: true, Lever: -4096}
	st.Analog[knobCoarse], st.Analog[knobSens] = uint16(coarseFor(s, 4)), 1024 // full range
	h.input(t, st)
	before := len(h.api.callsCopy())
	st.Analog[knobCoarse] = uint16(coarseFor(s, 5.6))
	h.input(t, st)
	time.Sleep(30 * time.Millisecond)
	if got := h.api.callsCopy(); len(got) != before {
		t.Fatalf("turning COARSE wrote to the camera: %+v", got[before:])
	}
}
