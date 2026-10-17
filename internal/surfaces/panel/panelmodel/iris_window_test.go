package panelmodel_test

import (
	"math"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

func TestIrisWindow(t *testing.T) {
	s := rng(core.Iris, 1.4, 22, 0.1, "f")
	if w := panelmodel.IrisWindow(s, panelmodel.IrisKnobs{Coarse: 0, Sens: 0}); w != whole(s) {
		t.Errorf("knobs not read: window = %+v, want the whole range", w)
	}
	// SENS fully clockwise: the whole range, wherever COARSE is.
	for _, c := range []int{0, 300, 1024} {
		w := panelmodel.IrisWindow(s, panelmodel.IrisKnobs{Coarse: c, Sens: micropanel.AnalogMax, Read: true})
		if !near(w.Lo, 1.4, 1e-9) || !near(w.Hi, 22, 1e-9) {
			t.Errorf("coarse %d, full sens: window = %+v, want 1.4..22", c, w)
		}
	}
	// COARSE: knob 0 = most open, 1024 = most closed, logarithmic.
	if c := panelmodel.IrisCentre(s, panelmodel.IrisKnobs{Coarse: 0}); c != 1.4 {
		t.Errorf("centre at 0 = %v", c)
	}
	if c := panelmodel.IrisCentre(s, panelmodel.IrisKnobs{Coarse: micropanel.AnalogMax}); !near(c, 22, 1e-9) {
		t.Errorf("centre at 1024 = %v", c)
	}
	// One stop centred on 5.6: 5.6 / 2^(1/4) .. 5.6 x 2^(1/4).
	k := panelmodel.IrisKnobs{Coarse: coarseFor(s, 5.6), Sens: 0, Read: true}
	w := panelmodel.IrisWindow(s, k)
	c := panelmodel.IrisCentre(s, k)
	if !near(c, 5.6, 0.02) || !near(w.Lo, c/math.Pow(2, 0.25), 1e-9) || !near(w.Hi, c*math.Pow(2, 0.25), 1e-9) {
		t.Errorf("1 stop at 5.6: window = %+v (centre %v)", w, c)
	}
	// Clamped at an end: shifted to keep the width.
	w = panelmodel.IrisWindow(s, panelmodel.IrisKnobs{Coarse: 0, Sens: 0, Read: true})
	if w.Lo != 1.4 || !near(w.Hi, 1.4*math.Sqrt2, 1e-9) {
		t.Errorf("1 stop at the open end = %+v, want 1.4..1.98", w)
	}
	w = panelmodel.IrisWindow(s, panelmodel.IrisKnobs{Coarse: micropanel.AnalogMax, Sens: 0, Read: true})
	if w.Hi != 22 || !near(w.Lo, 22/math.Sqrt2, 1e-9) {
		t.Errorf("1 stop at the closed end = %+v, want 15.6..22", w)
	}
	// The lever maps over the window: fully open = its open end.
	k = panelmodel.IrisKnobs{Coarse: coarseFor(s, 4), Sens: sensFor(s, 2), Read: true}
	w = panelmodel.IrisWindow(s, k)
	if got := panelmodel.IrisAt(s, w, 1); !near(got, w.Lo, 0.05) {
		t.Errorf("lever open = %v, want %v", got, w.Lo)
	}
	if got := panelmodel.IrisAt(s, w, 0); !near(got, w.Hi, 0.05) {
		t.Errorf("lever closed = %v, want %v", got, w.Hi)
	}
	if got := panelmodel.IrisAt(s, w, panelmodel.IrisPos(s, w, 4)); got != 4 {
		t.Errorf("round trip F4 = %v", got)
	}
	if !w.Reaches(s, 4) || w.Reaches(s, 11) {
		t.Error("only values inside the window can be picked up")
	}
	// A normalized iris has no window.
	n := rng(core.Iris, 0, 1, 0.01, "")
	if w := panelmodel.IrisWindow(n, panelmodel.IrisKnobs{Coarse: 500, Sens: 0, Read: true}); w != whole(n) {
		t.Errorf("normalized iris window = %+v", w)
	}
}

// irisBar returns the 16 iris scale segments of a frame.
func irisBar(f panelmodel.Frame) (b [micropanel.LEDIrisBarCount]bool) {
	for i := range b {
		b[i] = panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisBar0+i)
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
	v := baseView()                                                                            // camera at F4 (segment 5)
	v.Knobs = panelmodel.IrisKnobs{Coarse: coarseFor(s, 5.6), Sens: sensFor(s, 2), Read: true} // ~4..8: segments 5..9
	if got := irisBar(panelmodel.Render(v)); got != segs(5, 9) {
		t.Errorf("masked bar = %v, want segments 5..9", got)
	}
	// SENS view: the window itself, whatever the iris.
	v.Cam.Values[core.Iris] = core.Number(11)
	v.Sens = true
	if got := irisBar(panelmodel.Render(v)); got != segs(5, 9) {
		t.Errorf("SENS view = %v, want segments 5..9", got)
	}
	// Window reaching the closed end: lit down to CLS.
	v.Sens = false
	v.Knobs = panelmodel.IrisKnobs{Coarse: micropanel.AnalogMax, Sens: sensFor(s, 2), Read: true} // 11..22
	if got := irisBar(panelmodel.Render(v)); got != segs(11, 15) {
		t.Errorf("bar = %v, want segments 11..15", got)
	}
	v.Sens = true
	if got := irisBar(panelmodel.Render(v)); got != segs(11, 15) {
		t.Errorf("SENS view = %v, want segments 11..15", got)
	}
}

func TestRenderCoarsePreview(t *testing.T) {
	s := sonySpecs()[0]
	v := baseView()
	v.Knobs = panelmodel.IrisKnobs{Coarse: coarseFor(s, 5.6), Sens: 0, Read: true}
	v.Coarse, v.Blink = true, true
	f := panelmodel.Render(v)
	want := digitsOf(map[micropanel.Display]string{micropanel.DispShutter: "F5.6"})
	if [4]byte(f.Digits[4:8]) != [4]byte(want[4:8]) {
		t.Errorf("SHUTTER = % x, want F5.6", f.Digits[4:8])
	}
	if panelmodel.LEDOn(&f.LEDs, micropanel.LEDShutterUp) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDShutterDown) {
		t.Error("SHUTTER arrows must be off during the COARSE preview")
	}
	v.Blink = false
	if f := panelmodel.Render(v); [4]byte(f.Digits[4:8]) != [4]byte{} {
		t.Error("the COARSE preview must blink")
	}
	for f, want := range map[float64]string{11: "F11", 22: "F22", 1.4: "F1.4"} {
		if got := panelmodel.FStopText(f); got != want {
			t.Errorf("fstopText(%v) = %q, want %q", f, got, want)
		}
	}
}
