package panel_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// --- fakes -------------------------------------------------------------------

type call struct {
	select_ bool
	slot    int
	id      string
	v       core.Value
	preset  string // "store", "recall" or "arm" (n = preset, v.On = armed)
	n       int
}

type fakeAPI struct {
	mu     sync.Mutex
	rig    app.Rig
	specs  []core.ControlSpec
	values map[core.ControlID]core.Value
	calls  []call
	setErr error
	lens   core.Lens
	armed  bool
	ctl    *panel.Controller // receives rig:changed like app.Host would
}

func (a *fakeAPI) Rig() app.Rig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rig
}

func (a *fakeAPI) Controls(slot int) ([]app.ControlView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return views(a.specs, a.values), nil
}

func (a *fakeAPI) Lens(int) (core.Lens, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.lens, nil
}

func (a *fakeAPI) SetControl(slot int, id string, v core.Value) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, call{slot: slot, id: id, v: v})
	return a.setErr
}

func (a *fakeAPI) SelectSlot(slot int) error {
	a.mu.Lock()
	a.calls = append(a.calls, call{select_: true, slot: slot})
	a.rig.Active = slot
	rig, ctl := a.rig, a.ctl
	a.mu.Unlock()
	if ctl != nil {
		ctl.Emit(app.EventRigChanged, rig)
	}
	return nil
}

// StorePreset adds n to the slot's presets and disarms, like the app.
func (a *fakeAPI) StorePreset(slot, n int) error {
	a.mu.Lock()
	a.calls = append(a.calls, call{preset: "store", slot: slot, n: n})
	a.rig.Slots = slices.Clone(a.rig.Slots)
	sl := &a.rig.Slots[slot-1]
	if !slices.Contains(sl.Presets, n) {
		sl.Presets = append(slices.Clone(sl.Presets), n)
		slices.Sort(sl.Presets)
	}
	rig, ctl := a.rig, a.ctl
	a.mu.Unlock()
	a.SetStoreArmed(false)
	if ctl != nil {
		ctl.Emit(app.EventRigChanged, rig)
	}
	return nil
}

func (a *fakeAPI) RecallPreset(slot, n int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.calls = append(a.calls, call{preset: "recall", slot: slot, n: n})
	return nil
}

// SetStoreArmed emits preset:storeArmed on a change, like the app.
func (a *fakeAPI) SetStoreArmed(on bool) {
	a.mu.Lock()
	if a.armed == on {
		a.mu.Unlock()
		return
	}
	a.armed = on
	a.calls = append(a.calls, call{preset: "arm", v: core.Bool(on)})
	ctl := a.ctl
	a.mu.Unlock()
	if ctl != nil {
		ctl.Emit(app.EventStoreArmed, app.StoreArmedState{Armed: on})
	}
}

func (a *fakeAPI) StoreArmed() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.armed
}

// presetCalls returns the preset calls of kind ("store", "recall", "arm").
func (a *fakeAPI) presetCalls(kind string) []call {
	var out []call
	for _, c := range a.callsCopy() {
		if c.preset == kind {
			out = append(out, c)
		}
	}
	return out
}

func (a *fakeAPI) callsCopy() []call {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]call(nil), a.calls...)
}

type fakeDevice struct {
	in      chan micropanel.State
	fail    chan error
	mu      sync.Mutex
	digits  [14]byte
	matrix  [32]byte
	leds    micropanel.LEDBits
	writes  int
	clears  int
	closed  atomic.Bool
	running atomic.Bool

	features    [][]byte
	knobs       [9]uint16
	ignoreKnobs bool
}

func newFakeDevice() *fakeDevice {
	return &fakeDevice{in: make(chan micropanel.State), fail: make(chan error, 1)}
}

func (d *fakeDevice) Serial() string { return "SN1" }
func (d *fakeDevice) Run(ctx context.Context, fn func(micropanel.State), _ func([]byte)) error {
	d.running.Store(true)
	defer d.running.Store(false)
	defer d.Close()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-d.fail:
			return err
		case s := <-d.in:
			fn(s)
		}
	}
}
func (d *fakeDevice) SetDigitsRaw(seg [14]byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.digits, d.writes = seg, d.writes+1
	return nil
}
func (d *fakeDevice) SetMatrixRaw(px [32]byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.matrix, d.writes = px, d.writes+1
	return nil
}
func (d *fakeDevice) WriteReport(b []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if b[0] == 0x02 && b[31] == 0 {
		copy(d.leds[:], b[1:31])
	}
	d.writes++
	return nil
}
func (d *fakeDevice) ClearLEDs(int) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.leds = micropanel.LEDBits{}
	d.clears++
	return nil
}
func (d *fakeDevice) Close() error { d.closed.Store(true); return nil }

// SetFeature and GetFeature emulate the 0x0E knob counters; with
// ignoreKnobs the panel accepts the write but keeps its positions.
func (d *fakeDevice) SetFeature(b []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.features = append(d.features, append([]byte(nil), b...))
	if pos, ok := micropanel.ParseAnalog(b); ok && !d.ignoreKnobs {
		d.knobs = pos
	}
	return nil
}
func (d *fakeDevice) GetFeature(id byte, size int) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return micropanel.AnalogReport(d.knobs)[:size], nil
}
func (d *fakeDevice) featureWrites() [][]byte {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([][]byte(nil), d.features...)
}

func (d *fakeDevice) snapshot() ([14]byte, [32]byte, micropanel.LEDBits) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.digits, d.matrix, d.leds
}

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %s", what)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

type harness struct {
	api    *fakeAPI
	dev    *fakeDevice
	ctl    *panel.Controller
	status chan app.SurfaceStatus
	cancel context.CancelFunc
	done   chan struct{}
}

func start(t *testing.T, api *fakeAPI, opts panel.Options) *harness {
	t.Helper()
	h := &harness{api: api, dev: newFakeDevice(), status: make(chan app.SurfaceStatus, 16), done: make(chan struct{})}
	if opts.Open == nil {
		opts.Open = func() (panel.Device, error) { return h.dev, nil }
	}
	if opts.Busy == nil {
		opts.Busy = func() bool { return false }
	}
	opts.Logger = quietLog()
	opts.Throttle, opts.FrameEvery, opts.RetryMin = 5*time.Millisecond, 5*time.Millisecond, 5*time.Millisecond
	h.ctl = panel.New(opts)
	api.ctl = h.ctl
	ctx, cancel := context.WithCancel(context.Background())
	h.cancel = cancel
	go func() {
		defer close(h.done)
		h.ctl.Run(ctx, api, func(s app.SurfaceStatus) { h.status <- s })
	}()
	t.Cleanup(h.stop)
	return h
}

func (h *harness) stop() {
	h.cancel()
	<-h.done
}

// input sends a panel state and waits until the controller handled it.
func (h *harness) input(t *testing.T, s micropanel.State) {
	t.Helper()
	select {
	case h.dev.in <- s:
	case <-time.After(2 * time.Second):
		t.Fatal("device not reading")
	}
}

func sonyAPI() *fakeAPI {
	return &fakeAPI{rig: rigWith(2, 1, 2, 4), specs: sonySpecs(), values: sonyValues()}
}

// loaded waits until the active camera's values are rendered (the
// controller loads them asynchronously).
func (h *harness) loaded(t *testing.T) {
	t.Helper()
	waitFor(t, "values loaded", func() bool { d, _, _ := h.dev.snapshot(); return d[8] != 0x02 && d[8] != 0 })
}

func lever(raw int32) micropanel.State { return micropanel.State{Lever: raw} }

// --- tests -------------------------------------------------------------------

func TestControllerRendersActiveCamera(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		if s := <-h.status; s != (app.SurfaceStatus{Active: true, Serial: "SN1"}) {
			t.Fatalf("status = %+v", s)
		}
		want := digitsOf(map[micropanel.Display]string{
			micropanel.DispND: "2", micropanel.DispGain: "06", micropanel.DispShutter: "50",
			micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "3",
		})
		// digits, matrix and LEDs are three reports: wait for the whole frame
		waitFor(t, "rendered frame", func() bool {
			d, m, l := h.dev.snapshot()
			return d == want && m == micropanel.MatrixNumber(2, micropanel.PixAmber) && cameraColour(l, 2) == panelmodel.ColourActive
		})
	})
}

func TestControllerBlanksPanelOnStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		waitFor(t, "first frame", func() bool { _, m, _ := h.dev.snapshot(); return m != [32]byte{} })
		h.stop()
		d, m, l := h.dev.snapshot()
		if d != [14]byte{} || m != [32]byte{} || l != (micropanel.LEDBits{}) || h.dev.clears == 0 {
			t.Error("panel not blanked on stop")
		}
		if !h.dev.closed.Load() {
			t.Error("device not closed")
		}
		if s := <-h.status; s != (app.SurfaceStatus{}) {
			t.Errorf("final status = %+v, want inactive", s)
		}
	})
}

func TestControllerCameraKeySelectsSlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		h.input(t, micropanel.State{}) // first report: seeds only
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x43}})
		waitFor(t, "SelectSlot(4)", func() bool {
			for _, c := range h.api.callsCopy() {
				if c.select_ && c.slot == 4 {
					return true
				}
			}
			return false
		})
		waitFor(t, "matrix 4", func() bool { _, m, _ := h.dev.snapshot(); return m == micropanel.MatrixNumber(4, micropanel.PixAmber) })
		// holding the key does not repeat
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x43}})
		settle(20 * time.Millisecond)
		n := 0
		for _, c := range h.api.callsCopy() {
			if c.select_ {
				n++
			}
		}
		if n != 1 {
			t.Errorf("SelectSlot called %d times, want 1", n)
		}
	})
}

func TestControllerKeysHeldAtConnectDoNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x40}})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x40}})
		settle(20 * time.Millisecond)
		if c := h.api.callsCopy(); len(c) != 0 {
			t.Errorf("calls = %+v, want none", c)
		}
	})
}

func (h *harness) waitCall(t *testing.T, id string, check func(core.Value) bool) {
	t.Helper()
	waitFor(t, "SetControl "+id, func() bool {
		for _, c := range h.api.callsCopy() {
			if c.id == id && check(c.v) {
				return true
			}
		}
		return false
	})
}

func TestControllerStepKeysAndActions(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		press := func(k micropanel.Key) {
			h.input(t, micropanel.State{Keys: []micropanel.Key{k}})
			h.input(t, micropanel.State{})
		}
		press(0x4E) // ND up: 1/8 -> 1/16
		h.waitCall(t, "nd", func(v core.Value) bool { return v.Choice == "1/16" })
		press(0x53) // GAIN down: 6 -> 5
		h.waitCall(t, "gain", func(v core.Value) bool { return v.Number == 5 })
		press(0x54) // SHUTTER up: 1/50 -> 1/60
		h.waitCall(t, "shutterSpeed", func(v core.Value) bool { return v.Choice == "1/60" })
		press(0x54) // again from the optimistic value: 1/60 -> 1/100
		h.waitCall(t, "shutterSpeed", func(v core.Value) bool { return v.Choice == "1/100" })
		press(0x5B) // AUTO IRIS
		h.waitCall(t, "autoIris", func(v core.Value) bool { return v.Kind == core.KindAction })
		press(0x58) // BARS: known off -> on
		h.waitCall(t, "colorBars", func(v core.Value) bool { return v.On })
	})
}

func TestControllerWhiteBalanceMode(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{}) // 5600 K step 50, tint 0
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		press := func(k micropanel.Key) {
			h.input(t, micropanel.State{Keys: []micropanel.Key{k}})
			h.input(t, micropanel.State{})
		}
		press(0x57) // W/B: mode on
		waitFor(t, "W/B lit, 5600 shown", func() bool {
			d, _, l := h.dev.snapshot()
			return panelmodel.LEDOn(&l, micropanel.LEDWB) && [4]byte(d[4:8]) == shutterOf("5600")
		})
		press(0x54) // SHUTTER up: +100 K (spec step 50)
		h.waitCall(t, "whiteBalance", func(v core.Value) bool { return v.Number == 5700 })
		press(0x53) // GAIN down: tint -1
		h.waitCall(t, "tint", func(v core.Value) bool { return v.Number == -1 })
		for _, c := range h.api.callsCopy() {
			if c.id == "shutterSpeed" || c.id == "gain" || c.id == "autoWhiteBalance" {
				t.Fatalf("white balance mode wrote %s", c.id)
			}
		}
		press(0x57) // W/B: mode off, keys back to shutter
		waitFor(t, "W/B off", func() bool { _, _, l := h.dev.snapshot(); return !panelmodel.LEDOn(&l, micropanel.LEDWB) })
		press(0x54)
		h.waitCall(t, "shutterSpeed", func(v core.Value) bool { return v.Choice == "1/60" })

		// A camera switch leaves the mode.
		press(0x57)
		waitFor(t, "W/B lit", func() bool { _, _, l := h.dev.snapshot(); return panelmodel.LEDOn(&l, micropanel.LEDWB) })
		h.api.SelectSlot(1)
		waitFor(t, "W/B off after a switch", func() bool {
			_, m, l := h.dev.snapshot()
			return m == micropanel.MatrixNumber(1, micropanel.PixAmber) && !panelmodel.LEDOn(&l, micropanel.LEDWB)
		})
	})
}

func TestControllerWhiteBalanceModeNeedsWBOrTint(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		var specs []core.ControlSpec
		for _, s := range sonySpecs() {
			if s.ID != core.WhiteBalance && s.ID != core.Tint {
				specs = append(specs, s)
			}
		}
		api.specs = specs
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x57}}) // W/B: nothing
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x54}}) // SHUTTER up stays shutter
		h.waitCall(t, "shutterSpeed", func(v core.Value) bool { return v.Choice == "1/60" })
		if _, _, l := h.dev.snapshot(); panelmodel.LEDOn(&l, micropanel.LEDWB) {
			t.Error("W/B lit for a camera without white balance")
		}
		for _, c := range api.callsCopy() {
			if c.id == "autoWhiteBalance" {
				t.Error("W/B must not trigger auto white balance any more")
			}
		}
	})
}

func TestControllerStoreAndScenes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		api.rig.Slots[1].Presets = []int{2}
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		press := func(k micropanel.Key) {
			h.input(t, micropanel.State{Keys: []micropanel.Key{k}})
			h.input(t, micropanel.State{})
		}
		waitFor(t, "SCENE 2 lit only", func() bool {
			_, _, l := h.dev.snapshot()
			return panelmodel.LEDOn(&l, micropanel.LEDScene1+1) && !panelmodel.LEDOn(&l, micropanel.LEDScene1) && !panelmodel.LEDOn(&l, micropanel.LEDStoreRed)
		})
		press(0x48) // SCENE 1: nothing stored, nothing done
		press(0x49) // SCENE 2: recall
		waitFor(t, "recall 2", func() bool { return len(api.presetCalls("recall")) == 1 })
		if c := api.presetCalls("recall")[0]; c.slot != 2 || c.n != 2 {
			t.Fatalf("recall = %+v, want slot 2 preset 2", c)
		}

		press(0x4D) // STORE: armed
		waitFor(t, "armed", api.StoreArmed)
		waitFor(t, "STORE red", func() bool { _, _, l := h.dev.snapshot(); return panelmodel.LEDOn(&l, micropanel.LEDStoreRed) })
		press(0x4A) // SCENE 3: store, the app disarms
		waitFor(t, "store 3", func() bool { return len(api.presetCalls("store")) == 1 })
		if c := api.presetCalls("store")[0]; c.slot != 2 || c.n != 3 {
			t.Fatalf("store = %+v, want slot 2 preset 3", c)
		}
		waitFor(t, "disarmed, SCENE 2 and 3 lit", func() bool {
			_, _, l := h.dev.snapshot()
			return !panelmodel.LEDOn(&l, micropanel.LEDStoreRed) && panelmodel.LEDOn(&l, micropanel.LEDScene1+1) && panelmodel.LEDOn(&l, micropanel.LEDScene1+2) &&
				!panelmodel.LEDOn(&l, micropanel.LEDScene1+3)
		})
		if api.StoreArmed() {
			t.Error("still armed after a store")
		}
		if n := len(api.presetCalls("recall")); n != 1 {
			t.Errorf("%d recalls, want 1 (SCENE 1 was empty, SCENE 3 stored)", n)
		}

		// STORE twice: armed then disarmed.
		press(0x4D)
		waitFor(t, "armed", api.StoreArmed)
		press(0x4D)
		waitFor(t, "disarmed", func() bool { return !api.StoreArmed() })
	})
}

func TestControllerFollowsStoreArmingFromTheApp(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		api.armed = true // armed before the panel starts
		h := start(t, api, panel.Options{})
		<-h.status
		waitFor(t, "STORE red at start", func() bool { _, _, l := h.dev.snapshot(); return panelmodel.LEDOn(&l, micropanel.LEDStoreRed) })
		api.SetStoreArmed(false) // the GUI's Store button
		waitFor(t, "STORE off", func() bool { _, _, l := h.dev.snapshot(); return !panelmodel.LEDOn(&l, micropanel.LEDStoreRed) })
		api.SetStoreArmed(true)
		waitFor(t, "STORE red", func() bool { _, _, l := h.dev.snapshot(); return panelmodel.LEDOn(&l, micropanel.LEDStoreRed) })
	})
}

func TestControllerSceneWithoutActiveSlotDoesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		api.rig.Active = 0
		api.armed = true
		h := start(t, api, panel.Options{})
		<-h.status
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x48}})
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x40}}) // CAM1 as a marker
		waitFor(t, "SelectSlot(1)", func() bool {
			for _, c := range api.callsCopy() {
				if c.select_ {
					return true
				}
			}
			return false
		})
		if c := api.presetCalls("store"); len(c) != 0 {
			t.Fatalf("stored without an active slot: %+v", c)
		}
	})
}

func TestControllerPanelActiveOff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI() // gainR = 1, iris F4
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		s := sonySpecs()[0]
		raw := func(f float64) int32 { return int32(panelmodel.IrisPos(s, whole(s), f)*8192 - 4096) }
		var st micropanel.State
		st.Analog[0], st.Lever = 500, raw(11)
		h.input(t, st) // seed
		send := func(keys ...micropanel.Key) {
			st.Keys = keys
			h.input(t, st)
			st.Keys = nil
			h.input(t, st)
		}
		send(0x57) // W/B mode on, to check it is left
		waitFor(t, "W/B lit", func() bool { _, _, l := h.dev.snapshot(); return panelmodel.LEDOn(&l, micropanel.LEDWB) })
		send(0x5D) // PANEL ACTIVE: off
		waitFor(t, "PANEL ACTIVE white, W/B off", func() bool {
			_, _, l := h.dev.snapshot()
			return panelmodel.LEDOn(&l, micropanel.LEDPanelWhite) && !panelmodel.LEDOn(&l, micropanel.LEDPanelRed) && !panelmodel.LEDOn(&l, micropanel.LEDWB)
		})
		before := len(api.callsCopy())
		for _, k := range []micropanel.Key{0x4E, 0x54, 0x52, 0x58, 0x5B, 0x57, 0x4D, 0x48, 0x40} {
			send(k)
		}
		st.Analog[0] = 900 // knob turned while off
		h.input(t, st)
		st.Lever = raw(2.8) // lever crosses the camera iris while off
		h.input(t, st)
		st.Ring = 4096
		h.input(t, st)
		settle(40 * time.Millisecond)
		if c := api.callsCopy()[before:]; len(c) != 0 {
			t.Fatalf("panel off wrote %+v", c)
		}

		send(0x5D) // back on: references re-taken
		waitFor(t, "PANEL ACTIVE red", func() bool {
			_, _, l := h.dev.snapshot()
			return panelmodel.LEDOn(&l, micropanel.LEDPanelRed) && !panelmodel.LEDOn(&l, micropanel.LEDPanelWhite)
		})
		settle(20 * time.Millisecond)
		if c := api.callsCopy()[before:]; len(c) != 0 {
			t.Fatalf("turning the panel on wrote %+v", c)
		}
		// The knob counts from where it is now: +64 counts = +0.0625 (a full
		// travel spans 1), not the +400 counts it moved while off.
		st.Analog[0] = 964
		h.input(t, st)
		h.waitCall(t, "gainR", func(v core.Value) bool { return v.Number > 1.06 && v.Number < 1.065 })
		// The lever must pick up the camera iris (F4) again.
		st.Lever = raw(2.4)
		h.input(t, st)
		settle(30 * time.Millisecond)
		for _, c := range api.callsCopy() {
			if c.id == string(core.Iris) {
				t.Fatalf("lever wrote %v without picking up again", c.v.Number)
			}
		}
		st.Lever = raw(5.6) // crosses F4
		h.input(t, st)
		h.waitCall(t, "iris", func(v core.Value) bool { return v.Number > 5.5 && v.Number < 5.7 })
	})
}

func TestControllerBarsUnknownDoesNothing(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		delete(api.values, core.ColorBars)
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x58}})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x5B}}) // a mapped key as a marker
		h.waitCall(t, "autoIris", func(core.Value) bool { return true })
		for _, c := range api.callsCopy() {
			if c.id == "colorBars" {
				t.Fatal("BARS toggled from an unknown state")
			}
		}
	})
}

func TestControllerIrisPickup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{}) // camera at F4
		<-h.status
		s := sonySpecs()[0]
		raw := func(f float64) int32 { return int32(panelmodel.IrisPos(s, whole(s), f)*8192 - 4096) }
		h.loaded(t)

		h.input(t, lever(raw(11))) // seed far from F4
		h.input(t, lever(raw(8)))
		h.input(t, lever(raw(5.6)))
		settle(30 * time.Millisecond)
		if c := h.api.callsCopy(); len(c) != 0 {
			t.Fatalf("lever wrote before pickup: %+v", c)
		}
		h.input(t, lever(raw(2.8))) // crosses F4: takes over
		h.waitCall(t, "iris", func(v core.Value) bool { return v.Number > 2.7 && v.Number < 2.9 })
		waitFor(t, "IRIS/MB white (ring not picked up)", func() bool {
			_, _, l := h.dev.snapshot()
			// master black is not picked up yet: still white
			return panelmodel.LEDOn(&l, micropanel.LEDIrisMBWhite)
		})

		// camera switch: give control back
		h.api.SelectSlot(1)
		waitFor(t, "slot 1", func() bool { _, m, _ := h.dev.snapshot(); return m == micropanel.MatrixNumber(1, micropanel.PixAmber) })
		settle(20 * time.Millisecond)
		before := len(h.api.callsCopy())
		h.input(t, lever(raw(2.4)))
		settle(30 * time.Millisecond)
		if got := len(h.api.callsCopy()); got != before {
			t.Fatal("lever wrote right after a camera switch")
		}
	})
}

func TestControllerIrisUnknownNeverWrites(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		api.values = nil
		h := start(t, api, panel.Options{})
		<-h.status
		h.input(t, lever(-4096))
		for r := int32(-4096); r <= 4096; r += 512 {
			h.input(t, lever(r))
		}
		settle(30 * time.Millisecond)
		if c := api.callsCopy(); len(c) != 0 {
			t.Fatalf("calls = %+v, want none", c)
		}
	})
}

func TestControllerExternalChangeReleasesPickup(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		s := sonySpecs()[0]
		raw := func(f float64) int32 { return int32(panelmodel.IrisPos(s, whole(s), f)*8192 - 4096) }
		h.loaded(t)
		h.input(t, lever(raw(5.6)))
		h.input(t, lever(raw(3.5))) // crosses F4: picked
		h.waitCall(t, "iris", func(core.Value) bool { return true })

		h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Iris, Value: core.Number(11)}})
		before := len(h.api.callsCopy())
		h.input(t, lever(raw(3)))
		settle(30 * time.Millisecond)
		if len(h.api.callsCopy()) != before {
			t.Fatal("lever still drives after the iris changed elsewhere")
		}
		want := digitsOf(map[micropanel.Display]string{micropanel.DispIris: "F11"})
		waitFor(t, "F11 shown", func() bool { d, _, _ := h.dev.snapshot(); return [3]byte(d[8:11]) == [3]byte(want[8:11]) })
	})
}

func TestControllerKnobsAreRelative(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{}) // gainR = 1
		<-h.status
		h.loaded(t)
		var st micropanel.State
		st.Analog[0] = 900 // the absolute position is irrelevant
		h.input(t, st)
		st.Analog[0] = 964 // +64 counts over the gain travel (span 1) = +0.0625
		h.input(t, st)
		h.waitCall(t, "gainR", func(v core.Value) bool { return v.Number > 1.06 && v.Number < 1.065 })
		st.Analog[1] = 512 // BLACK R +512 counts: half the lift travel, 0 -> +0.5
		h.input(t, st)
		h.waitCall(t, "liftR", func(v core.Value) bool { return v.Number == 0.5 })
	})
}

func TestControllerThrottlesContinuousWrites(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		var st micropanel.State
		h.input(t, st)
		for i := 1; i <= 200; i++ {
			st.Analog[0] = uint16(i)
			h.input(t, st)
		}
		final := panelmodel.KnobValue(sonySpecs()[9], 1, 200)
		h.waitCall(t, "gainR", func(v core.Value) bool { return v.Number == final })
		n := 0
		for _, c := range api.callsCopy() {
			if c.id == "gainR" {
				n++
			}
		}
		if n >= 200 {
			t.Errorf("%d writes for 200 knob reports: not throttled", n)
		}
	})
}

func TestControllerExpectedErrorsDoNotStop(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		api.setErr = &app.Error{Code: app.CodeNotImplemented, Message: "not implemented"}
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		for i := 0; i < 3; i++ {
			h.input(t, micropanel.State{Keys: []micropanel.Key{0x5B}})
			h.input(t, micropanel.State{})
		}
		waitFor(t, "3 auto iris attempts", func() bool {
			n := 0
			for _, c := range api.callsCopy() {
				if c.id == "autoIris" {
					n++
				}
			}
			return n == 3
		})
	})
}

func TestControllerReconnectsAfterUnplug(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		var opens atomic.Int32
		devs := make(chan *fakeDevice, 4)
		h := start(t, api, panel.Options{Open: func() (panel.Device, error) {
			opens.Add(1)
			d := newFakeDevice()
			devs <- d
			return d, nil
		}})
		d1 := <-devs
		<-h.status
		waitFor(t, "running", d1.running.Load)
		d1.fail <- errors.New("device removed")
		if s := <-h.status; s.Active {
			t.Fatalf("status after unplug = %+v", s)
		}
		d2 := <-devs
		if s := <-h.status; !s.Active {
			t.Fatalf("status after replug = %+v", s)
		}
		waitFor(t, "second session renders", func() bool { _, m, _ := d2.snapshot(); return m != [32]byte{} })
	})
}

func TestControllerReconnectSwitchesDevice(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		devs := make(chan *fakeDevice, 4)
		h := start(t, api, panel.Options{Open: func() (panel.Device, error) {
			d := newFakeDevice()
			devs <- d
			return d, nil
		}})
		d1 := <-devs
		<-h.status
		waitFor(t, "first session renders", func() bool { _, m, _ := d1.snapshot(); return m != [32]byte{} })
		h.ctl.Reconnect()
		d2 := <-devs
		waitFor(t, "first panel closed", d1.closed.Load)
		if dg, m, _ := d1.snapshot(); dg != ([14]byte{}) || m != ([32]byte{}) {
			t.Fatal("first panel not blanked before switching")
		}
		waitFor(t, "second session renders", func() bool { _, m, _ := d2.snapshot(); return m != [32]byte{} })
	})
}

func TestControllerNoDeviceAndOpenErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var fail atomic.Int32
		h := start(t, sonyAPI(), panel.Options{Open: func() (panel.Device, error) {
			if fail.Add(1) == 1 {
				return nil, errors.New("auth: rejected")
			}
			return nil, panel.ErrNoDevice
		}})
		if s := <-h.status; s.Detail != panel.DetailOpenFail || s.Active {
			t.Fatalf("status = %+v, want open failure detail", s)
		}
		if s := <-h.status; s != (app.SurfaceStatus{}) {
			t.Fatalf("status = %+v, want empty once unplugged", s)
		}
	})
}

func TestControllerLeavesPanelToOtherProgram(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var busy atomic.Bool
		busy.Store(true)
		var opened atomic.Bool
		h := start(t, sonyAPI(), panel.Options{
			Busy: busy.Load,
			Open: func() (panel.Device, error) { opened.Store(true); return newFakeDevice(), nil },
		})
		if s := <-h.status; s.Detail != panel.DetailBusy {
			t.Fatalf("status = %+v, want busy", s)
		}
		settle(20 * time.Millisecond)
		if opened.Load() {
			t.Fatal("opened the panel while another program holds it")
		}
		busy.Store(false)
		if s := <-h.status; !s.Active {
			t.Fatalf("status = %+v, want active once free", s)
		}
	})
}

func TestControllerReleasesPanelWhenOtherProgramStarts(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var busy atomic.Bool
		h := start(t, sonyAPI(), panel.Options{Busy: busy.Load, BusyEvery: 5 * time.Millisecond})
		<-h.status
		busy.Store(true)
		if s := <-h.status; s.Detail != panel.DetailBusy || s.Active {
			t.Fatalf("status = %+v, want busy", s)
		}
		waitFor(t, "device closed", h.dev.closed.Load)
	})
}

func TestControllerTallyFromEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 4, Event: core.Event{Control: core.Tally, Value: core.Bool(true)}})
		waitFor(t, "camera 4 red", func() bool { _, _, l := h.dev.snapshot(); return cameraColour(l, 4) == panelmodel.ColourTally })
		h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Status: &core.Status{State: core.StateConnected}}})
		waitFor(t, "NETWORK on", func() bool { _, _, l := h.dev.snapshot(); return panelmodel.LEDOn(&l, micropanel.LEDNetwork) })
	})
}

func TestControllerIgnoresCameraKeyOfEmptySlot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{}) // slots 1, 2 and 4 assigned
		<-h.status
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x42}}) // CAM3: empty
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x40}}) // CAM1: assigned
		waitFor(t, "SelectSlot(1)", func() bool {
			for _, c := range h.api.callsCopy() {
				if c.select_ && c.slot == 1 {
					return true
				}
			}
			return false
		})
		for _, c := range h.api.callsCopy() {
			if c.select_ && c.slot == 3 {
				t.Fatal("CAM3 selected an empty slot")
			}
		}
	})
}

func TestFailedCommandRaisesAlarmForAMoment(t *testing.T) {
	alarm := func(h *harness) bool {
		_, _, l := h.dev.snapshot()
		return panelmodel.LEDOn(&l, micropanel.LEDAlarm)
	}
	t.Run("blinks, then stops after AlarmHold", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h := start(t, sonyAPI(), panel.Options{})
			<-h.status
			h.loaded(t)
			h.ctl.Emit(app.EventCommandFailed, app.CommandFailed{Slot: 2, Control: core.Gain, Code: app.CodeNotConnected})
			on := false
			for el := time.Duration(0); el < panel.AlarmHold-100*time.Millisecond; el += 50 * time.Millisecond {
				settle(50 * time.Millisecond)
				on = on || alarm(h)
			}
			if !on {
				t.Error("ALARM must blink after a failed command")
			}
			settle(200 * time.Millisecond)
			for i := 0; i < 20; i++ {
				settle(50 * time.Millisecond)
				if alarm(h) {
					t.Fatal("ALARM must stop after AlarmHold")
				}
			}
		})
	})
	t.Run("asking for confirmation is not an error", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			h := start(t, sonyAPI(), panel.Options{})
			<-h.status
			h.loaded(t)
			h.ctl.Emit(app.EventCommandFailed, app.CommandFailed{Slot: 2, Control: core.Record, Code: app.CodeConfirmRequired})
			for i := 0; i < 20; i++ {
				settle(50 * time.Millisecond)
				if alarm(h) {
					t.Fatal("ALARM lit for a confirmation request")
				}
			}
		})
	})
}

// The lever stops at the lens' widest aperture: pushed fully open on a
// f/2.8 lens it asks for f/2.8, never a bigger aperture the lens lacks.
func TestControllerIrisLeverStaysInsideTheLens(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI() // camera at F4
		api.lens = core.Lens{IrisMin: 2.8, IrisMax: 22}
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		s := (panelmodel.Camera{Specs: map[core.ControlID]core.ControlSpec{core.Iris: sonySpecs()[0]}, Lens: api.lens}).LensIris(sonySpecs()[0])
		raw := func(f float64) int32 { return int32(panelmodel.IrisPos(s, whole(s), f)*8192 - 4096) }
		h.input(t, lever(raw(5.6)))
		h.input(t, lever(raw(3.5))) // crosses F4: picked
		for r := raw(3.5); r <= 4096; r += 64 {
			h.input(t, lever(r)) // pushed slowly to the end
		}
		h.input(t, lever(4096))
		h.waitCall(t, "iris", func(v core.Value) bool { return v.Number == 2.8 })
		for _, c := range h.api.callsCopy() {
			if c.id == string(core.Iris) && c.v.Number < 2.8 {
				t.Errorf("lever asked for f/%v, beyond the lens' f/2.8", c.v.Number)
			}
		}
	})
}

// The ring never asks for more than ±0.99 (MB display ±99) on a -2..2
// master black, and cannot pick up a value set beyond that elsewhere.
func TestControllerRingLimitedTo99(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		specs := sonySpecs()
		for i := range specs {
			if specs[i].ID == core.MasterBlack {
				specs[i] = rng(core.MasterBlack, -2, 2, 0.01, "")
			}
		}
		api.specs = specs
		api.values[core.MasterBlack] = core.Number(0)
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		ring := func(raw int32) micropanel.State { return micropanel.State{Ring: raw} }
		h.input(t, ring(-100))
		h.input(t, ring(100)) // crosses 0: picked
		for r := int32(100); r <= 4096; r += 128 {
			h.input(t, ring(r))
		}
		h.input(t, ring(4096))
		h.waitCall(t, "masterBlack", func(v core.Value) bool { return v.Number == 0.99 })
		for _, c := range api.callsCopy() {
			if c.id == string(core.MasterBlack) && (c.v.Number > 0.99 || c.v.Number < -0.99) {
				t.Errorf("ring asked for %v, beyond ±0.99", c.v.Number)
			}
		}

		// Set to 1.5 from the GUI: shown as 99, and beyond the ring's reach, so
		// the ring takes over at once (never stuck) and stays within ±0.99.
		h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.MasterBlack, Value: core.Number(1.5)}})
		want := digitsOf(map[micropanel.Display]string{micropanel.DispMasterBlack: "99"})
		waitFor(t, "99 shown", func() bool { d, _, _ := h.dev.snapshot(); return [3]byte(d[11:14]) == [3]byte(want[11:14]) })
		before := len(api.callsCopy())
		for r := int32(4096); r >= -4096; r -= 256 {
			h.input(t, ring(r))
		}
		h.waitCall(t, "masterBlack", func(v core.Value) bool { return v.Number == -0.99 })
		for _, c := range api.callsCopy()[before:] {
			if c.id == string(core.MasterBlack) && (c.v.Number > 0.99 || c.v.Number < -0.99) {
				t.Errorf("ring asked for %v, beyond ±0.99", c.v.Number)
			}
		}
	})
}

// Held for a second, W/B runs the auto white balance once: its LED blinks
// while held, then stays on; the release does not toggle the mode.
func TestControllerWBLongPressRunsAWB(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x57}}) // hold W/B
		var on, off bool
		deadline := time.Now().Add(panel.WBLongPress - 200*time.Millisecond)
		for time.Now().Before(deadline) {
			_, _, l := h.dev.snapshot()
			on, off = on || panelmodel.LEDOn(&l, micropanel.LEDWB), off || !panelmodel.LEDOn(&l, micropanel.LEDWB)
			settle(20 * time.Millisecond)
		}
		if !on || !off {
			t.Errorf("W/B must blink while held (seen on %v, off %v)", on, off)
		}
		h.waitCall(t, "autoWhiteBalance", func(v core.Value) bool { return v.Kind == core.KindAction })
		settle(300 * time.Millisecond) // a few blink phases
		if _, _, l := h.dev.snapshot(); !panelmodel.LEDOn(&l, micropanel.LEDWB) {
			t.Error("AWB triggered: W/B must be steady on while still held")
		}
		h.input(t, micropanel.State{}) // release
		waitFor(t, "W/B off, mode not toggled", func() bool { _, _, l := h.dev.snapshot(); return !panelmodel.LEDOn(&l, micropanel.LEDWB) })
		n := 0
		for _, c := range h.api.callsCopy() {
			if c.id == "autoWhiteBalance" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("auto white balance ran %d times, want once", n)
		}
	})
}

// A camera without auto white balance: the long press does nothing (no
// blink, no call, no mode).
func TestControllerWBLongPressWithoutAWB(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		api := sonyAPI()
		var specs []core.ControlSpec
		for _, s := range sonySpecs() {
			if s.ID != core.AutoWhiteBalance {
				specs = append(specs, s)
			}
		}
		api.specs = specs
		h := start(t, api, panel.Options{})
		<-h.status
		h.loaded(t)
		h.input(t, micropanel.State{})
		h.input(t, micropanel.State{Keys: []micropanel.Key{0x57}})
		deadline := time.Now().Add(panel.WBLongPress + 300*time.Millisecond)
		for time.Now().Before(deadline) {
			if _, _, l := h.dev.snapshot(); panelmodel.LEDOn(&l, micropanel.LEDWB) {
				t.Fatal("no AWB: the W/B LED stays off while held")
			}
			settle(20 * time.Millisecond)
		}
		h.input(t, micropanel.State{})
		settle(100 * time.Millisecond)
		if _, _, l := h.dev.snapshot(); panelmodel.LEDOn(&l, micropanel.LEDWB) {
			t.Error("a long press must not toggle the mode")
		}
		for _, c := range h.api.callsCopy() {
			if c.id == "autoWhiteBalance" {
				t.Error("no AWB call expected")
			}
		}
	})
}

// A color knob whose counter sits at the panel's 0 clamp could not be turned
// negative. The controller recenters the counters (feature 0x0E, the other
// knobs kept), so turning down works again.
func TestColorKnobsAreRecenteredSoNegativeTurnsWork(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{}) // liftR 0
		<-h.status
		h.loaded(t)
		st := micropanel.State{AnalogRead: true}
		st.Analog[panelmodel.KnobSens], st.Analog[panelmodel.KnobCoarse] = 700, 300 // absolute: must be kept
		h.input(t, st)                                                              // connect: every color knob at 0
		waitFor(t, "recenter written", func() bool { return len(h.dev.featureWrites()) > 0 })
		w := h.dev.featureWrites()[0]
		pos, ok := micropanel.ParseAnalog(w)
		if !ok || len(w) != 19 {
			t.Fatalf("feature %x is not a 19-byte 0x0E report", w)
		}
		for i := range panelmodel.KnobControls {
			if pos[i] != panelmodel.KnobCentre {
				t.Errorf("knob %d set to %d, want %d", i, pos[i], panelmodel.KnobCentre)
			}
		}
		if pos[panelmodel.KnobSens] != 700 || pos[panelmodel.KnobCoarse] != 300 {
			t.Errorf("SENS/COARSE must be kept: %d/%d", pos[panelmodel.KnobSens], pos[panelmodel.KnobCoarse])
		}
		settle(50 * time.Millisecond)              // the read-back is taken as reference
		st.Analog[1] = panelmodel.KnobCentre - 100 // BLACK R turned down
		h.input(t, st)
		h.waitCall(t, "liftR", func(v core.Value) bool { return v.Number < 0 })
	})
}

// A panel that ignores the write keeps its counters: the read-back makes
// them the references, so nothing jumps.
func TestKnobRecenterIgnoredDoesNotJump(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		h := start(t, sonyAPI(), panel.Options{})
		h.dev.mu.Lock()
		h.dev.ignoreKnobs = true
		h.dev.mu.Unlock()
		<-h.status
		h.loaded(t)
		st := micropanel.State{AnalogRead: true}
		st.Analog[1] = 40
		h.dev.mu.Lock()
		h.dev.knobs = st.Analog
		h.dev.mu.Unlock()
		h.input(t, st)
		waitFor(t, "recenter tried", func() bool { return len(h.dev.featureWrites()) > 0 })
		settle(50 * time.Millisecond)
		st.Analog[1] = 50 // +10 counts from where the counter really is
		h.input(t, st)
		h.waitCall(t, "liftR", func(v core.Value) bool { return v.Number > 0 })
		for _, c := range h.api.callsCopy() {
			if c.id == "liftR" && c.v.Number > 0.1 {
				t.Errorf("liftR jumped to %v", c.v.Number)
			}
		}
	})
}
