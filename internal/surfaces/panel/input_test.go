package panel

import (
	"math"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestStepEnumAndRange(t *testing.T) {
	nd := enum(core.ND, "clear", "1/4", "1/8")
	if v, ok := step(nd, core.Choice("clear"), +1); !ok || v.Choice != "1/4" {
		t.Errorf("nd up = %v %v", v, ok)
	}
	if _, ok := step(nd, core.Choice("clear"), -1); ok {
		t.Error("nd down from first option must not move")
	}
	if _, ok := step(nd, core.Choice("1/8"), +1); ok {
		t.Error("nd up from last option must not move")
	}
	gain := rng(core.Gain, -12, 36, 2, "dB")
	if v, ok := step(gain, core.Number(6), +1); !ok || v.Number != 8 {
		t.Errorf("gain up = %v", v)
	}
	if v, ok := step(gain, core.Number(35), +1); !ok || v.Number != 36 {
		t.Errorf("gain up near max = %v %v, want clamp to 36", v, ok)
	}
	if _, ok := step(gain, core.Number(36), +1); ok {
		t.Error("gain up at max must not move")
	}
	noStep := rng(core.Gain, 0, 20, 0, "dB")
	if v, _ := step(noStep, core.Number(0), +1); v.Number != 1 {
		t.Errorf("step 0 uses span/20, got %v", v.Number)
	}
	angle := rng(core.ShutterAngle, 11.2, 360, 0.1, "°")
	if v, ok := step(angle, core.Number(180), -1); !ok || v.Number != 172.8 {
		t.Errorf("angle down = %v", v)
	}
	if v, ok := step(angle, core.Number(100), +1); !ok || v.Number != 144 {
		t.Errorf("angle up from non-standard = %v", v)
	}
	if _, ok := step(angle, core.Number(360), +1); ok {
		t.Error("angle up at 360 must not move")
	}
	if _, ok := step(kind(core.AutoIris, core.KindAction), core.Trigger(), +1); ok {
		t.Error("action is not steppable")
	}
}

func TestKnobValueIsRelativeAndClamped(t *testing.T) {
	s := rng(core.LiftR, -2, 2, 0.01, "")
	if got := knobValue(s, 0, 256); math.Abs(got-1) > 1e-9 {
		t.Errorf("quarter turn over span 4 = %v, want 1", got)
	}
	if got := knobValue(s, 1.9, 1024); got != 2 {
		t.Errorf("clamp = %v", got)
	}
	if got := knobValue(s, 0, 1); got == 0 {
		t.Error("one count must still move the value (no quantization)")
	}
}

func TestIrisLeverMapping(t *testing.T) {
	s := rng(core.Iris, 1.4, 22, 0.1, "f")
	if got := irisAt(s, 0); got != 22 {
		t.Errorf("lever closed = %v, want 22", got)
	}
	if got := irisAt(s, 1); got != 1.4 {
		t.Errorf("lever open = %v, want 1.4", got)
	}
	for _, f := range []float64{2.8, 4, 11} {
		if got := irisAt(s, irisPos(s, f)); math.Abs(got-f) > 0.05 {
			t.Errorf("round trip %v = %v", f, got)
		}
	}
	if leverPos(-4096) != 0 || leverPos(4096) != 1 || leverPos(0) != 0.5 || leverPos(9999) != 1 {
		t.Error("leverPos")
	}
	n := rng(core.Iris, 0, 1, 0.01, "")
	if irisAt(n, 0.5) != 0.5 || irisPos(n, 0.25) != 0.25 {
		t.Error("normalized iris must be linear")
	}
	mb := rng(core.MasterBlack, -15, 15, 1, "")
	if mbAt(mb, 0.5) != 0 || mbAt(mb, 1) != 15 || mbPos(mb, -15) != 0 {
		t.Error("master black mapping")
	}
}

func TestPickupWaitsUntilCrossing(t *testing.T) {
	// CB-1 example: camera at F10, lever at F4.
	s := rng(core.Iris, 1.4, 22, 0.1, "f")
	target := irisPos(s, 10)
	var p pickup
	p.seed(irisPos(s, 4))
	if p.move(irisPos(s, 4.5), target, true) {
		t.Fatal("took over while moving around F4")
	}
	if p.move(irisPos(s, 5.6), target, true) {
		t.Fatal("took over before reaching F10")
	}
	if !p.move(irisPos(s, 11), target, true) {
		t.Fatal("did not take over after crossing F10")
	}
	if !p.move(irisPos(s, 4), target, true) {
		t.Fatal("lost control after pickup")
	}
	p.release()
	if p.move(irisPos(s, 4.2), target, true) {
		t.Fatal("still driving after release")
	}
}

func TestPickupReachWithinToleranceAndUnknown(t *testing.T) {
	var p pickup
	p.seed(0.2)
	if !p.move(0.5+pickupTol/2, 0.5, true) {
		t.Error("reaching the value within tolerance must take over")
	}
	var q pickup
	q.seed(0.2)
	if q.move(0.9, 0.5, false) {
		t.Error("unknown value must never take over")
	}
	if q.picked {
		t.Error("unknown value must release")
	}
	var r pickup // no seed: the first move cannot detect a crossing
	if r.move(0.9, 0.5, true) {
		t.Error("first move without a previous position must not take over far from the value")
	}
}
