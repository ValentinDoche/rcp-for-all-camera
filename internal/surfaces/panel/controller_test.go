package panel

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
}

type fakeAPI struct {
	mu     sync.Mutex
	rig    app.Rig
	specs  []core.ControlSpec
	values map[core.ControlID]core.Value
	calls  []call
	setErr error
	lens   core.Lens
	ctl    *Controller // receives rig:changed like app.Host would
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

func (a *fakeAPI) StorePreset(slot, n int) error  { return nil }
func (a *fakeAPI) RecallPreset(slot, n int) error { return nil }
func (a *fakeAPI) SetStoreArmed(on bool)          {}
func (a *fakeAPI) StoreArmed() bool               { return false }

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
	ctl    *Controller
	status chan app.SurfaceStatus
	cancel context.CancelFunc
	done   chan struct{}
}

func start(t *testing.T, api *fakeAPI, opts Options) *harness {
	t.Helper()
	h := &harness{api: api, dev: newFakeDevice(), status: make(chan app.SurfaceStatus, 16), done: make(chan struct{})}
	if opts.Open == nil {
		opts.Open = func() (Device, error) { return h.dev, nil }
	}
	if opts.Busy == nil {
		opts.Busy = func() bool { return false }
	}
	opts.Logger = quietLog()
	opts.Throttle, opts.FrameEvery, opts.RetryMin = 5*time.Millisecond, 5*time.Millisecond, 5*time.Millisecond
	h.ctl = New(opts)
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
	h := start(t, sonyAPI(), Options{})
	if s := <-h.status; s != (app.SurfaceStatus{Active: true, Serial: "SN1"}) {
		t.Fatalf("status = %+v", s)
	}
	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "2", micropanel.DispGain: "06", micropanel.DispShutter: "50",
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "+3",
	})
	// digits, matrix and LEDs are three reports: wait for the whole frame
	waitFor(t, "rendered frame", func() bool {
		d, m, l := h.dev.snapshot()
		return d == want && m == micropanel.MatrixNumber(2, micropanel.PixAmber) && cameraColour(l, 2) == colourActive
	})
}

func TestControllerBlanksPanelOnStop(t *testing.T) {
	h := start(t, sonyAPI(), Options{})
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
}

func TestControllerCameraKeySelectsSlot(t *testing.T) {
	h := start(t, sonyAPI(), Options{})
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
	time.Sleep(20 * time.Millisecond)
	n := 0
	for _, c := range h.api.callsCopy() {
		if c.select_ {
			n++
		}
	}
	if n != 1 {
		t.Errorf("SelectSlot called %d times, want 1", n)
	}
}

func TestControllerKeysHeldAtConnectDoNothing(t *testing.T) {
	h := start(t, sonyAPI(), Options{})
	<-h.status
	h.input(t, micropanel.State{Keys: []micropanel.Key{0x40}})
	h.input(t, micropanel.State{Keys: []micropanel.Key{0x40}})
	time.Sleep(20 * time.Millisecond)
	if c := h.api.callsCopy(); len(c) != 0 {
		t.Errorf("calls = %+v, want none", c)
	}
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
	h := start(t, sonyAPI(), Options{})
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
	press(0x57) // W/B
	h.waitCall(t, "autoWhiteBalance", func(v core.Value) bool { return v.Kind == core.KindAction })
	press(0x58) // BARS: known off -> on
	h.waitCall(t, "colorBars", func(v core.Value) bool { return v.On })
}

func TestControllerBarsUnknownDoesNothing(t *testing.T) {
	api := sonyAPI()
	delete(api.values, core.ColorBars)
	h := start(t, api, Options{})
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
}

func TestControllerIrisPickup(t *testing.T) {
	h := start(t, sonyAPI(), Options{}) // camera at F4
	<-h.status
	s := sonySpecs()[0]
	raw := func(f float64) int32 { return int32(irisPos(s, f)*8192 - 4096) }
	h.loaded(t)

	h.input(t, lever(raw(11))) // seed far from F4
	h.input(t, lever(raw(8)))
	h.input(t, lever(raw(5.6)))
	time.Sleep(30 * time.Millisecond)
	if c := h.api.callsCopy(); len(c) != 0 {
		t.Fatalf("lever wrote before pickup: %+v", c)
	}
	h.input(t, lever(raw(2.8))) // crosses F4: takes over
	h.waitCall(t, "iris", func(v core.Value) bool { return v.Number > 2.7 && v.Number < 2.9 })
	waitFor(t, "IRIS/MB white (ring not picked up)", func() bool {
		_, _, l := h.dev.snapshot()
		// master black is not picked up yet: still white
		return ledOn(&l, micropanel.LEDIrisMBWhite)
	})

	// camera switch: give control back
	h.api.SelectSlot(1)
	waitFor(t, "slot 1", func() bool { _, m, _ := h.dev.snapshot(); return m == micropanel.MatrixNumber(1, micropanel.PixAmber) })
	time.Sleep(20 * time.Millisecond)
	before := len(h.api.callsCopy())
	h.input(t, lever(raw(2.4)))
	time.Sleep(30 * time.Millisecond)
	if got := len(h.api.callsCopy()); got != before {
		t.Fatal("lever wrote right after a camera switch")
	}
}

func TestControllerIrisUnknownNeverWrites(t *testing.T) {
	api := sonyAPI()
	api.values = nil
	h := start(t, api, Options{})
	<-h.status
	h.input(t, lever(-4096))
	for r := int32(-4096); r <= 4096; r += 512 {
		h.input(t, lever(r))
	}
	time.Sleep(30 * time.Millisecond)
	if c := api.callsCopy(); len(c) != 0 {
		t.Fatalf("calls = %+v, want none", c)
	}
}

func TestControllerExternalChangeReleasesPickup(t *testing.T) {
	h := start(t, sonyAPI(), Options{})
	<-h.status
	s := sonySpecs()[0]
	raw := func(f float64) int32 { return int32(irisPos(s, f)*8192 - 4096) }
	h.loaded(t)
	h.input(t, lever(raw(5.6)))
	h.input(t, lever(raw(3.5))) // crosses F4: picked
	h.waitCall(t, "iris", func(core.Value) bool { return true })

	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Iris, Value: core.Number(11)}})
	before := len(h.api.callsCopy())
	h.input(t, lever(raw(3)))
	time.Sleep(30 * time.Millisecond)
	if len(h.api.callsCopy()) != before {
		t.Fatal("lever still drives after the iris changed elsewhere")
	}
	want := digitsOf(map[micropanel.Display]string{micropanel.DispIris: "F11"})
	waitFor(t, "F11 shown", func() bool { d, _, _ := h.dev.snapshot(); return [3]byte(d[8:11]) == [3]byte(want[8:11]) })
}

func TestControllerKnobsAreRelative(t *testing.T) {
	h := start(t, sonyAPI(), Options{}) // gainR = 1
	<-h.status
	h.loaded(t)
	var st micropanel.State
	st.Analog[0] = 900 // the absolute position is irrelevant
	h.input(t, st)
	st.Analog[0] = 964 // +64 counts over span 16 = +1
	h.input(t, st)
	h.waitCall(t, "gainR", func(v core.Value) bool { return v.Number > 1.99 && v.Number < 2.01 })
	st.Analog[1] = 512 // BLACK R from 0: lift unknown? lift known at 0 -> +2
	h.input(t, st)
	h.waitCall(t, "liftR", func(v core.Value) bool { return v.Number == 2 })
}

func TestControllerThrottlesContinuousWrites(t *testing.T) {
	api := sonyAPI()
	h := start(t, api, Options{})
	<-h.status
	h.loaded(t)
	var st micropanel.State
	h.input(t, st)
	for i := 1; i <= 200; i++ {
		st.Analog[0] = uint16(i)
		h.input(t, st)
	}
	final := knobValue(sonySpecs()[9], 1, 200)
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
}

func TestControllerExpectedErrorsDoNotStop(t *testing.T) {
	api := sonyAPI()
	api.setErr = &app.Error{Code: app.CodeNotImplemented, Message: "not implemented"}
	h := start(t, api, Options{})
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
}

func TestControllerReconnectsAfterUnplug(t *testing.T) {
	api := sonyAPI()
	var opens atomic.Int32
	devs := make(chan *fakeDevice, 4)
	h := start(t, api, Options{Open: func() (Device, error) {
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
}

func TestControllerNoDeviceAndOpenErrors(t *testing.T) {
	var fail atomic.Int32
	h := start(t, sonyAPI(), Options{Open: func() (Device, error) {
		if fail.Add(1) == 1 {
			return nil, errors.New("auth: rejected")
		}
		return nil, ErrNoDevice
	}})
	if s := <-h.status; s.Detail != DetailOpenFail || s.Active {
		t.Fatalf("status = %+v, want open failure detail", s)
	}
	if s := <-h.status; s != (app.SurfaceStatus{}) {
		t.Fatalf("status = %+v, want empty once unplugged", s)
	}
}

func TestControllerLeavesPanelToOtherProgram(t *testing.T) {
	var busy atomic.Bool
	busy.Store(true)
	var opened atomic.Bool
	h := start(t, sonyAPI(), Options{
		Busy: busy.Load,
		Open: func() (Device, error) { opened.Store(true); return newFakeDevice(), nil },
	})
	if s := <-h.status; s.Detail != DetailBusy {
		t.Fatalf("status = %+v, want busy", s)
	}
	time.Sleep(20 * time.Millisecond)
	if opened.Load() {
		t.Fatal("opened the panel while another program holds it")
	}
	busy.Store(false)
	if s := <-h.status; !s.Active {
		t.Fatalf("status = %+v, want active once free", s)
	}
}

func TestControllerReleasesPanelWhenOtherProgramStarts(t *testing.T) {
	var busy atomic.Bool
	h := start(t, sonyAPI(), Options{Busy: busy.Load, BusyEvery: 5 * time.Millisecond})
	<-h.status
	busy.Store(true)
	if s := <-h.status; s.Detail != DetailBusy || s.Active {
		t.Fatalf("status = %+v, want busy", s)
	}
	waitFor(t, "device closed", h.dev.closed.Load)
}

func TestControllerTallyFromEvents(t *testing.T) {
	h := start(t, sonyAPI(), Options{})
	<-h.status
	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 4, Event: core.Event{Control: core.Tally, Value: core.Bool(true)}})
	waitFor(t, "camera 4 red", func() bool { _, _, l := h.dev.snapshot(); return cameraColour(l, 4) == colourTally })
	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Status: &core.Status{State: core.StateConnected}}})
	waitFor(t, "NETWORK on", func() bool { _, _, l := h.dev.snapshot(); return ledOn(&l, micropanel.LEDNetwork) })
}

func TestControllerIgnoresCameraKeyOfEmptySlot(t *testing.T) {
	h := start(t, sonyAPI(), Options{}) // slots 1, 2 and 4 assigned
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
}

func TestFailedCommandRaisesAlarmForAMoment(t *testing.T) {
	c := New(Options{})
	t0 := time.Unix(1000, 0)
	c.now = func() time.Time { return t0 }
	c.Emit(app.EventCommandFailed, app.CommandFailed{Slot: 1, Control: core.Gain, Code: app.CodeNotConnected})
	alarm := func(c *Controller, at time.Time) bool {
		f := c.frame(at)
		return ledOn(&f.leds, micropanel.LEDAlarm)
	}
	if !alarm(c, t0.Add(time.Second)) && !alarm(c, t0.Add(time.Second+250*time.Millisecond)) {
		t.Error("ALARM must blink after a failed command")
	}
	for ms := int64(0); ms < 1000; ms += 50 {
		if alarm(c, t0.Add(alarmHold+time.Duration(ms)*time.Millisecond)) {
			t.Fatal("ALARM must stop after alarmHold")
		}
	}
	c2 := New(Options{})
	c2.now = func() time.Time { return t0 }
	c2.Emit(app.EventCommandFailed, app.CommandFailed{Slot: 1, Control: core.Record, Code: app.CodeConfirmRequired})
	for ms := int64(0); ms < 1000; ms += 50 {
		if alarm(c2, t0.Add(time.Duration(ms)*time.Millisecond)) {
			t.Fatal("asking for confirmation is not an error")
		}
	}
}

// The lever stops at the lens' widest aperture: pushed fully open on a
// f/2.8 lens it asks for f/2.8, never a bigger aperture the lens lacks.
func TestControllerIrisLeverStaysInsideTheLens(t *testing.T) {
	api := sonyAPI() // camera at F4
	api.lens = core.Lens{IrisMin: 2.8, IrisMax: 22}
	h := start(t, api, Options{})
	<-h.status
	h.loaded(t)
	s := (camState{specs: map[core.ControlID]core.ControlSpec{core.Iris: sonySpecs()[0]}, lens: api.lens}).lensIris(sonySpecs()[0])
	raw := func(f float64) int32 { return int32(irisPos(s, f)*8192 - 4096) }
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
}

// The ring never asks for more than ±0.99 (MB display ±99) on a -2..2
// master black, and cannot pick up a value set beyond that elsewhere.
func TestControllerRingLimitedTo99(t *testing.T) {
	api := sonyAPI()
	specs := sonySpecs()
	for i := range specs {
		if specs[i].ID == core.MasterBlack {
			specs[i] = rng(core.MasterBlack, -2, 2, 0.01, "")
		}
	}
	api.specs = specs
	api.values[core.MasterBlack] = core.Number(0)
	h := start(t, api, Options{})
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

	// Set to 1.5 from the GUI: out of the ring's reach.
	h.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.MasterBlack, Value: core.Number(1.5)}})
	before := len(api.callsCopy())
	for r := int32(4096); r >= -4096; r -= 256 {
		h.input(t, ring(r))
	}
	time.Sleep(30 * time.Millisecond)
	if got := len(api.callsCopy()); got != before {
		t.Fatalf("ring picked up a value beyond its travel: %+v", api.callsCopy()[before:])
	}
	want := digitsOf(map[micropanel.Display]string{micropanel.DispMasterBlack: "+Hi"})
	waitFor(t, "+Hi shown", func() bool { d, _, _ := h.dev.snapshot(); return [3]byte(d[11:14]) == [3]byte(want[11:14]) })
}
