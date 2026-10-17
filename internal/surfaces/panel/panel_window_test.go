package panel_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// COARSE blinks its centre on SHUTTER for 5 s after the last movement,
// moves nothing on the camera, then SHUTTER shows the shutter again.
func TestControllerCoarsePreviewTimesOut(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := sonySpecs()[0]
		h := start(t, sonyAPI(), panel.Options{}) // camera at F4
		<-h.status
		h.loaded(t)
		shutter := func() [4]byte { d, _, _ := h.dev.snapshot(); return [4]byte(d[4:8]) }
		fifty, f56 := shutterOf("50"), shutterOf("F5.6")
		st := micropanel.State{AnalogRead: true}
		st.Analog[panelmodel.KnobSens] = micropanel.AnalogMax
		h.input(t, st) // connect: positions only, no preview
		settle(10 * time.Millisecond)
		if shutter() != fifty {
			t.Fatal("no preview at connect")
		}
		st.Analog[panelmodel.KnobCoarse] = uint16(coarseFor(s, 5.6))
		h.input(t, st)
		seenF56, seenBlank := false, false
		for i := 0; i < 20; i++ { // one second of blinking
			settle(50 * time.Millisecond)
			seenF56 = seenF56 || shutter() == f56
			seenBlank = seenBlank || shutter() == [4]byte{}
		}
		if !seenF56 || !seenBlank {
			t.Error("COARSE must blink its centre on SHUTTER")
		}
		settle(2 * time.Second)            // 3 s after the movement
		st.Analog[panelmodel.KnobCoarse]++ // within the deadband: not a movement
		h.input(t, st)
		settle(panel.PreviewHold - 3*time.Second + 100*time.Millisecond)
		if shutter() != fifty {
			t.Error("the preview must end PreviewHold after the last COARSE movement")
		}
		st.Analog[panelmodel.KnobCoarse] += 10
		h.input(t, st) // a real movement restarts the preview
		settle(panel.PreviewHold - time.Second)
		if shutter() == fifty {
			// the blink may show blank: sample a little to see the preview
			seen := false
			for i := 0; i < 10 && !seen; i++ {
				settle(50 * time.Millisecond)
				seen = shutter() != fifty
			}
			if !seen {
				t.Error("the preview must last PreviewHold from the last movement")
			}
		}
		settle(2 * time.Second)
		if shutter() != fifty {
			t.Error("the preview must end after PreviewHold")
		}
		for _, c := range h.api.callsCopy() {
			if c.id == string(core.Iris) || c.id == string(core.IrisNorm) {
				t.Errorf("COARSE wrote the iris: %+v", c)
			}
		}
	})
}

// The lever works inside the COARSE/SENS window; turning COARSE writes
// nothing and gives control back until the lever picks the iris up again.
func TestControllerIrisLeverWindow(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := sonySpecs()[0]
		h := start(t, sonyAPI(), panel.Options{}) // camera at F4
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
	})
}

// A COARSE move that keeps the camera iris inside the window writes
// nothing: the lever simply has to pick the value up again.
func TestControllerCoarseInsideTheWindowWritesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := sonySpecs()[0]
		h := start(t, sonyAPI(), panel.Options{}) // camera at F4
		<-h.status
		h.loaded(t)
		st := micropanel.State{AnalogRead: true, Lever: -4096}
		st.Analog[panelmodel.KnobCoarse], st.Analog[panelmodel.KnobSens] = uint16(coarseFor(s, 4)), 1024 // full range
		h.input(t, st)
		before := len(h.api.callsCopy())
		st.Analog[panelmodel.KnobCoarse] = uint16(coarseFor(s, 5.6))
		h.input(t, st)
		settle(30 * time.Millisecond)
		if got := h.api.callsCopy(); len(got) != before {
			t.Fatalf("turning COARSE wrote to the camera: %+v", got[before:])
		}
	})
}
