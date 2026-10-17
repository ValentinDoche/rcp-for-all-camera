package panel

import (
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

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
	st.Analog[panelmodel.KnobSens] = micropanel.AnalogMax
	c.onInput(st) // connect: positions only, no preview
	shutter := func(at time.Time) [4]byte { f := c.frame(at); return [4]byte(f.Digits[4:8]) }
	shown := func(text string) [4]byte {
		d := digitsOf(map[micropanel.Display]string{micropanel.DispShutter: text})
		return [4]byte(d[4:8])
	}
	fifty, f56 := shown("50"), shown("F5.6")
	if shutter(t0) != fifty {
		t.Fatal("no preview at connect")
	}
	st.Analog[panelmodel.KnobCoarse] = uint16(coarseFor(s, 5.6))
	c.onInput(st)
	if shutter(t0) != f56 || shutter(t0.Add(250*time.Millisecond)) != [4]byte{} {
		t.Error("COARSE must blink its centre on SHUTTER")
	}
	now = t0.Add(3 * time.Second)
	st.Analog[panelmodel.KnobCoarse]++ // within the deadband: not a movement
	c.onInput(st)
	if shutter(t0.Add(5*time.Second)) != fifty {
		t.Error("the preview must end 5 s after the last COARSE movement")
	}
	st.Analog[panelmodel.KnobCoarse] += 10
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
	k := panelmodel.IrisKnobs{Coarse: coarseFor(s, 4), Sens: 0, Read: true} // one stop around F4
	w := panelmodel.IrisWindow(s, k)
	st := micropanel.State{AnalogRead: true}
	st.Analog[panelmodel.KnobCoarse], st.Analog[panelmodel.KnobSens] = uint16(k.Coarse), uint16(k.Sens)
	st.Lever = -4096
	h.input(t, st)
	for r := int32(-4096); r <= 4096; r += 256 {
		st.Lever = r
		h.input(t, st)
	}
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == panelmodel.Quantize(s, w.Lo) })
	for _, c := range h.api.callsCopy() {
		if c.id == string(core.Iris) && !w.Reaches(s, c.v.Number) {
			t.Errorf("lever asked for f/%v outside the window %+v", c.v.Number, w)
		}
	}

	// Window moved around F11: the camera iris (~F3.4) is left outside, so
	// it is set to the value under the lever (fully open = the window's most
	// open stop) and the lever keeps control instead of being stuck.
	k2 := panelmodel.IrisKnobs{Coarse: coarseFor(s, 11), Sens: 0, Read: true}
	w2 := panelmodel.IrisWindow(s, k2)
	before := len(h.api.callsCopy())
	st.Analog[panelmodel.KnobCoarse] = uint16(k2.Coarse)
	h.input(t, st)
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == panelmodel.Quantize(s, w2.Lo) })
	for r := int32(4096); r >= -4096; r -= 256 {
		st.Lever = r
		h.input(t, st)
	}
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == panelmodel.Quantize(s, w2.Hi) })
	for _, c := range h.api.callsCopy()[before:] {
		if c.id == string(core.Iris) && !w2.Reaches(s, c.v.Number) {
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
	st.Analog[panelmodel.KnobCoarse], st.Analog[panelmodel.KnobSens] = uint16(coarseFor(s, 4)), 1024 // full range
	h.input(t, st)
	before := len(h.api.callsCopy())
	st.Analog[panelmodel.KnobCoarse] = uint16(coarseFor(s, 5.6))
	h.input(t, st)
	time.Sleep(30 * time.Millisecond)
	if got := h.api.callsCopy(); len(got) != before {
		t.Fatalf("turning COARSE wrote to the camera: %+v", got[before:])
	}
}
