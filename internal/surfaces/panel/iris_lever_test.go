package panel

import (
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Regression: moved fast, the lever sent 4.0 then 3.2 and the camera
// reported 4.0 afterwards; the controller took that late echo for a change
// made elsewhere and the lever stopped driving.
func TestControllerLateEchoKeepsTheLever(t *testing.T) {
	h := start(t, sonyAPI(), Options{}) // camera at F4
	<-h.status
	s := sonySpecs()[0]
	raw := func(f float64) int32 { return int32(irisPos(s, whole(s), f)*8192 - 4096) }
	h.loaded(t)
	h.input(t, lever(raw(5.6)))
	h.input(t, lever(raw(3.5))) // crosses F4: picked
	h.input(t, lever(raw(2.8)))
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number < 3 })

	// The camera catches up late: F3.5, then the lens snaps 2.8 to 2.9.
	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Iris, Value: core.Number(3.5)}})
	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Iris, Value: core.Number(2.9)}})
	h.input(t, lever(raw(2.4)))
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number > 2.3 && v.Number < 2.5 })

	// The camera reports its own stop for 2.4 (2.5): once it has settled,
	// that value shows and the lever still drives.
	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Iris, Value: core.Number(2.5)}})
	want := digitsOf(map[micropanel.Display]string{micropanel.DispIris: "F2.5"})
	waitFor(t, "F2.5 shown once settled", func() bool { d, _, _ := h.dev.snapshot(); return [3]byte(d[8:11]) == [3]byte(want[8:11]) })
	h.input(t, lever(raw(2)))
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number > 1.9 && v.Number < 2.1 })
}

// irisNormAPI is the Sony-like camera with an IrisNorm control, like a
// Blackmagic camera: the lever must drive it, never the f-stop.
func irisNormAPI(norm float64) *fakeAPI {
	api := sonyAPI()
	api.specs = append(api.specs, rng(core.IrisNorm, 0, 1, 0.001, ""))
	api.values[core.IrisNorm] = core.Number(norm)
	return api
}

func TestControllerLeverDrivesIrisNormOverTheLensRange(t *testing.T) {
	h := start(t, irisNormAPI(0.25), Options{})
	<-h.status
	h.loaded(t)
	h.input(t, lever(-4096)) // seed: pulled back (closed)
	h.input(t, lever(4096))  // forward: crosses the camera (0.25) and drives
	h.input(t, lever(4095))  // a nudge
	h.input(t, lever(4096))  // fully forward = fully open = 0
	h.waitCall(t, "irisNorm", func(v core.Value) bool { return v.Number == 0 })
	for _, c := range h.api.callsCopy() {
		if c.id == string(core.Iris) {
			t.Fatalf("the lever wrote the f-stop (%v) although the camera has IrisNorm", c.v)
		}
	}
	h.input(t, lever(-4096)) // fully back = closed = 1
	h.waitCall(t, "irisNorm", func(v core.Value) bool { return v.Number == 1 })
}

func TestControllerIrisNormPickup(t *testing.T) {
	h := start(t, irisNormAPI(0.5), Options{})
	<-h.status
	h.loaded(t)
	h.input(t, lever(3000)) // seed well forward of the camera (0.5 = centre)
	h.input(t, lever(2000))
	time.Sleep(30 * time.Millisecond)
	if len(h.api.callsCopy()) != 0 {
		t.Fatal("lever wrote before reaching the camera's iris")
	}
	h.input(t, lever(-500)) // crosses the centre: takes over
	h.waitCall(t, "irisNorm", func(v core.Value) bool { return v.Number > 0.5 && v.Number < 0.6 })
}

func TestNormWindow(t *testing.T) {
	if w := normWindow(irisKnobs{}); w != (window{0, 1}) {
		t.Fatalf("unread knobs: %v", w)
	}
	if w := normWindow(irisKnobs{read: true, coarse: 0, sens: micropanel.AnalogMax}); w != (window{0, 1}) {
		t.Fatalf("SENS fully clockwise: %v", w)
	}
	w := normWindow(irisKnobs{read: true, coarse: 0, sens: 0})
	if w.lo != 0 || w.hi < normMinWidth-1e-9 || w.hi > normMinWidth+1e-9 {
		t.Fatalf("narrowest window at the open end: %v", w)
	}
}
