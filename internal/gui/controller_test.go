package gui

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
)

// fakeAPI is a scriptable service for controller tests.
type fakeAPI struct {
	armed      bool
	mu         sync.Mutex
	rig        app.Rig
	controls   map[int][]app.ControlView
	panel      app.PanelStatus
	setErr     error
	selErr     error
	sets       []setCall
	selects    []int
	loads      int
	confirms   []setCall
	confirmErr error
}

func newFakeAPI() *fakeAPI {
	f := &fakeAPI{controls: map[int][]app.ControlView{}}
	f.rig = app.Rig{Slots: make([]app.Slot, app.SlotCount)}
	for i := range f.rig.Slots {
		f.rig.Slots[i].Slot = i + 1
	}
	return f
}

func rangeSpec(id core.ControlID, min, max, step float64) core.ControlSpec {
	s := core.Std(id)
	s.Kind, s.Min, s.Max, s.Step = core.KindRange, min, max, step
	return s
}

func (f *fakeAPI) assign(slot int, model string, state core.ConnState, ctls ...app.ControlView) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.rig.Slots[slot-1].Camera = &app.CameraView{
		Model: core.ModelID(model), Brand: "Acme", ModelName: "Cam", Name: fmt.Sprintf("C%d", slot),
		Host: "10.0.0.1", Port: 80, Status: core.Status{State: state},
	}
	f.controls[slot] = ctls
}

// setStatus replaces the camera view, like the real service which builds a
// new view on every Rig call.
func (f *fakeAPI) setStatus(slot int, s core.ConnState) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *f.rig.Slots[slot-1].Camera
	cp.Status = core.Status{State: s}
	f.rig.Slots[slot-1].Camera = &cp
}

func (f *fakeAPI) Rig() app.Rig {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.rig
	r.Slots = append([]app.Slot(nil), f.rig.Slots...)
	return r
}
func (f *fakeAPI) Models() []core.ModelInfo { return nil }
func (f *fakeAPI) AssignSlot(int, app.SlotInput) (app.Slot, error) {
	return app.Slot{}, nil
}
func (f *fakeAPI) ClearSlot(int) error { return nil }
func (f *fakeAPI) SelectSlot(slot int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.selects = append(f.selects, slot)
	if f.selErr == nil {
		f.rig.Active = slot
	}
	return f.selErr
}
func (f *fakeAPI) Controls(slot int) ([]app.ControlView, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.loads++
	return append([]app.ControlView(nil), f.controls[slot]...), nil
}
func (f *fakeAPI) Lens(int) (core.Lens, error) { return core.Lens{}, nil }
func (f *fakeAPI) SetControl(slot int, id string, v core.Value) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sets = append(f.sets, setCall{core.ControlID(id), v})
	return f.setErr
}
func (f *fakeAPI) ConfirmSetControl(slot int, id string, v core.Value) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.confirms = append(f.confirms, setCall{core.ControlID(id), v})
	return f.confirmErr
}
func (f *fakeAPI) confirmCalls() []setCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]setCall(nil), f.confirms...)
}
func (f *fakeAPI) PanelStatus() app.PanelStatus { return f.panel }
func (f *fakeAPI) StorePreset(int, int) error   { return nil }
func (f *fakeAPI) RecallPreset(int, int) error  { return nil }
func (f *fakeAPI) DeletePreset(int, int) error  { return nil }
func (f *fakeAPI) SetStoreArmed(on bool)        { f.armed = on }
func (f *fakeAPI) StoreArmed() bool             { return f.armed }
func (f *fakeAPI) setCalls() []setCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]setCall(nil), f.sets...)
}

func numView(sp core.ControlSpec, v float64) app.ControlView {
	val := core.Number(v)
	return app.ControlView{Spec: sp, Value: &val}
}

func newCtl(f *fakeAPI) *Controller {
	o := syncOptions()
	o.WriteInterval = 5 * time.Millisecond
	c := NewController(f, o)
	c.Load()
	return c
}

type recorder struct {
	mu  sync.Mutex
	got []Change
}

func (r *recorder) fn(ch Change) { r.mu.Lock(); r.got = append(r.got, ch); r.mu.Unlock() }
func (r *recorder) count(k ChangeKind) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.got {
		if c.Kind == k {
			n++
		}
	}
	return n
}

func TestLoadReadsRigControlsAndValues(t *testing.T) {
	f := newFakeAPI()
	iris := rangeSpec(core.Iris, 0, 1, 0.01)
	f.assign(1, "acme.cam", core.StateConnected, numView(iris, 0.4), app.ControlView{Spec: rangeSpec(core.Zoom, 0, 1, 0.01)})
	c := newCtl(f)
	if c.Camera(1) == nil || c.Camera(2) != nil {
		t.Fatal("camera presence")
	}
	if !c.Loaded(1) || !c.Has(1, core.Iris) || c.Has(1, core.Focus) {
		t.Errorf("loaded=%v hasIris=%v hasFocus=%v", c.Loaded(1), c.Has(1, core.Iris), c.Has(1, core.Focus))
	}
	if v := c.Value(1, core.Iris); v == nil || v.Number != 0.4 {
		t.Errorf("iris = %v", v)
	}
	if v := c.Value(1, core.Zoom); v != nil {
		t.Errorf("unknown value must be nil, got %v", v)
	}
	if c.Value(2, core.Iris) != nil || c.Specs(2) != nil || c.Camera(0) != nil || c.Camera(9) != nil {
		t.Error("empty or invalid slots must give nothing")
	}
}

func TestWriteSuccessUpdatesCacheAndNotifies(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected, numView(rangeSpec(core.Iris, 0, 1, 0.01), 0.4))
	c := newCtl(f)
	var rec recorder
	c.Subscribe(rec.fn)
	c.Write(1, core.Iris, core.Number(0.7))
	waitWrites(c)
	if v := c.Value(1, core.Iris); v == nil || v.Number != 0.7 {
		t.Fatalf("cache = %v", v)
	}
	if rec.count(ChValue) != 1 {
		t.Errorf("ChValue notifications = %d", rec.count(ChValue))
	}
	if got := f.setCalls(); len(got) != 1 || got[0].ID != core.Iris || got[0].V.Number != 0.7 {
		t.Errorf("service calls = %+v", got)
	}
}

func TestWriteNotImplementedKeepsLastKnownAndShowsDiscreetNotice(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected, numView(rangeSpec(core.Iris, 0, 1, 0.01), 0.4))
	f.setErr = &app.Error{Code: app.CodeNotImplemented, Message: "x"}
	c := newCtl(f)
	var rec recorder
	c.Subscribe(rec.fn)
	c.Write(1, core.Iris, core.Number(0.9))
	waitWrites(c)
	if v := c.Value(1, core.Iris); v == nil || v.Number != 0.4 {
		t.Errorf("value must stay at the last known 0.4, got %v", v)
	}
	n := c.Notice()
	if n.Level != texts.LevelInfo || n.Text != "Contrôle pas encore disponible : pilote caméra en développement" {
		t.Errorf("notice = %+v", n)
	}
	if rec.count(ChValue) != 1 {
		t.Error("widgets must be told to return to the last known value")
	}
}

func TestWriteInvalidValueIsAnError(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected, numView(rangeSpec(core.Iris, 0, 1, 0.01), 0.4))
	f.setErr = &app.Error{Code: app.CodeInvalidValue, Message: "x"}
	c := newCtl(f)
	c.Write(1, core.Iris, core.Number(5))
	waitWrites(c)
	if n := c.Notice(); n.Level != texts.LevelError || n.Text != "Valeur refusée par la caméra" {
		t.Errorf("notice = %+v", n)
	}
	f.setErr = &app.Error{Code: app.CodeUnknownControl, Message: "x"}
	c.Write(1, core.Iris, core.Number(0.5))
	waitWrites(c)
	if n := c.Notice(); n.Level != texts.LevelError || n.Text != "Contrôle inconnu pour cette caméra" {
		t.Errorf("notice = %+v", n)
	}
}

func TestWriteIsThrottledAndFinalValueAlwaysSent(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected, numView(rangeSpec(core.Iris, 0, 1, 0.001), 0))
	o := syncOptions()
	o.WriteInterval = 20 * time.Millisecond
	c := NewController(f, o)
	c.Load()
	start := time.Now()
	for i := 1; i <= 100; i++ {
		c.Write(1, core.Iris, core.Number(float64(i)/100))
		time.Sleep(time.Millisecond)
	}
	waitWrites(c)
	calls := f.setCalls()
	if len(calls) == 0 || calls[len(calls)-1].V.Number != 1 {
		t.Fatalf("final value not sent: %+v", calls[len(calls)-1:])
	}
	if max := int(time.Since(start)/(20*time.Millisecond)) + 2; len(calls) > max {
		t.Errorf("%d writes in %v, want at most %d", len(calls), time.Since(start), max)
	}
	if v := c.Value(1, core.Iris); v == nil || v.Number != 1 {
		t.Errorf("cache = %v", v)
	}
}

func TestActionWriteDoesNotStoreAValue(t *testing.T) {
	f := newFakeAPI()
	ai := core.Std(core.AutoIris)
	ai.Kind = core.KindAction
	f.assign(1, "acme.cam", core.StateConnected, app.ControlView{Spec: ai})
	c := newCtl(f)
	c.Write(1, core.AutoIris, core.Trigger())
	waitWrites(c)
	if c.Value(1, core.AutoIris) != nil {
		t.Error("an action has no value")
	}
	if len(f.setCalls()) != 1 {
		t.Error("trigger not sent")
	}
}

func TestSelectIsOptimisticAndRevertsOnError(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected)
	f.assign(2, "acme.cam", core.StateConnected)
	c := newCtl(f)
	c.Select(2)
	if c.Active() != 2 || len(f.selects) != 1 || f.selects[0] != 2 {
		t.Errorf("active=%d selects=%v", c.Active(), f.selects)
	}
	f.selErr = &app.Error{Code: app.CodeInvalidSlot, Message: "x"}
	c.Select(1)
	if c.Active() != 2 {
		t.Errorf("must revert to the service state, got %d", c.Active())
	}
	if c.Notice().Text == "" {
		t.Error("error must be shown")
	}
}

func TestServiceEventsUpdateState(t *testing.T) {
	f := newFakeAPI()
	iris := rangeSpec(core.Iris, 0, 1, 0.01)
	f.assign(1, "acme.cam", core.StateConnected, numView(iris, 0.1))
	c := newCtl(f)
	var rec recorder
	c.Subscribe(rec.fn)

	c.Emit(app.EventPanelStatus, app.PanelStatus{Connected: true})
	if !c.Panel().Connected || rec.count(ChPanel) != 1 {
		t.Error("panel status")
	}
	c.Emit(app.EventPanelStatus, app.PanelStatus{Connected: true}) // same value: no change
	if rec.count(ChPanel) != 1 {
		t.Error("identical panel status must not notify")
	}

	c.Emit(app.EventCamera, app.CameraEvent{Slot: 1, Event: core.Event{Control: core.Iris, Value: core.Number(0.8)}})
	if v := c.Value(1, core.Iris); v == nil || v.Number != 0.8 || rec.count(ChValue) != 1 {
		t.Errorf("camera event: %v, %d notifications", v, rec.count(ChValue))
	}
	c.Emit(app.EventCamera, app.CameraEvent{Slot: 1, Event: core.Event{Control: "nope", Value: core.Number(1)}})
	c.Emit(app.EventCamera, app.CameraEvent{Slot: 7, Event: core.Event{Control: core.Iris, Value: core.Number(1)}})
	if rec.count(ChValue) != 1 {
		t.Error("events for unknown controls or empty slots are ignored")
	}

	r := f.Rig()
	r.Active = 3
	c.Emit(app.EventRigChanged, r)
	if c.Active() != 3 {
		t.Errorf("active = %d", c.Active())
	}
	c.Emit(app.EventRigChanged, &r) // pointer payloads too
}

func TestRigChangeReloadsOnNewCameraAndOnConnect(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateDisconnected, app.ControlView{Spec: rangeSpec(core.Iris, 0, 1, 0.01)})
	c := newCtl(f)
	if c.Value(1, core.Iris) != nil {
		t.Fatal("offline camera has no values")
	}
	loads := f.loads

	// the camera connects: values are read
	val := core.Number(0.3)
	f.controls[1] = []app.ControlView{{Spec: rangeSpec(core.Iris, 0, 1, 0.01), Value: &val}}
	f.setStatus(1, core.StateConnected)
	c.RefreshRig()
	if f.loads != loads+1 {
		t.Errorf("controls must be reloaded on connect (loads %d -> %d)", loads, f.loads)
	}
	if v := c.Value(1, core.Iris); v == nil || v.Number != 0.3 {
		t.Errorf("iris = %v", v)
	}

	// the connection drops: values are forgotten, specs stay (the service
	// returns no value for a camera that is not connected)
	f.controls[1] = []app.ControlView{{Spec: rangeSpec(core.Iris, 0, 1, 0.01)}}
	f.setStatus(1, core.StateError)
	c.RefreshRig()
	if c.Value(1, core.Iris) != nil || !c.Has(1, core.Iris) {
		t.Error("lost connection must clear values but keep the declared controls")
	}

	// a different camera replaces it
	f.assign(1, "acme.other", core.StateDisconnected, app.ControlView{Spec: rangeSpec(core.Focus, 0, 1, 0.01)})
	c.RefreshRig()
	if c.Has(1, core.Iris) || !c.Has(1, core.Focus) {
		t.Error("a new camera must bring its own controls")
	}

	// and is removed
	f.rig.Slots[0].Camera = nil
	c.RefreshRig()
	if c.Camera(1) != nil || c.Specs(1) != nil {
		t.Error("cleared slot")
	}
}

func TestStaleControlLoadIsDropped(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected, numView(rangeSpec(core.Iris, 0, 1, 0.01), 0.5))
	var pending []func()
	o := syncOptions()
	o.Go = func(fn func()) { pending = append(pending, fn) }
	c := NewController(f, o)
	c.Load() // load queued, not run
	f.rig.Slots[0].Camera = nil
	c.RefreshRig() // slot cleared before the answer comes
	for _, fn := range pending {
		fn()
	}
	if c.Camera(1) != nil || c.Specs(1) != nil {
		t.Error("late answer must not resurrect a cleared slot")
	}
}

func TestNoticeExpires(t *testing.T) {
	f := newFakeAPI()
	o := syncOptions()
	o.NoticeTTL = 20 * time.Millisecond
	var mu sync.Mutex
	o.UI = func(fn func()) { mu.Lock(); defer mu.Unlock(); fn() }
	c := NewController(f, o)
	c.Load()
	c.SetNotice(texts.Notice{Text: "x"})
	time.Sleep(80 * time.Millisecond)
	mu.Lock()
	got := c.Notice().Text
	mu.Unlock()
	if got != "" {
		t.Errorf("notice still %q", got)
	}
}

func TestPollRefreshesRig(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateDisconnected)
	var mu sync.Mutex
	o := Options{}
	o.UI = func(fn func()) { mu.Lock(); defer mu.Unlock(); fn() }
	c := NewController(f, o)
	c.Load()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.Poll(ctx, 5*time.Millisecond)
	f.setStatus(1, core.StateConnected)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		st := c.Camera(1).Status.State
		mu.Unlock()
		if st == core.StateConnected {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Error("poller did not pick up the new status")
}

func TestIsAPIError(t *testing.T) {
	err := fmt.Errorf("w: %w", &app.Error{Code: app.CodeInvalidConfig})
	if !IsAPIError(err, app.CodeInvalidConfig) || IsAPIError(err, app.CodeInternal) || IsAPIError(errors.New("x"), app.CodeInternal) {
		t.Error("IsAPIError")
	}
}

func TestConfirmRequiredAsksAndNeverAutoConfirms(t *testing.T) {
	f := newFakeAPI()
	bars := rangeSpec(core.ColorBars, 0, 1, 1)
	f.assign(1, "acme.cam", core.StateConnected, numView(bars, 0))
	f.setErr = &app.Error{Code: app.CodeConfirmRequired}
	c := newCtl(f)
	rec := &recorder{}
	c.Subscribe(rec.fn)

	// no handler: the write is dropped with a notice, not confirmed
	c.Write(1, core.ColorBars, core.Number(1))
	waitWrites(c)
	if len(f.confirmCalls()) != 0 || c.Notice().Text == "" {
		t.Fatalf("without a handler: confirms %v notice %q", f.confirmCalls(), c.Notice().Text)
	}

	var asked []ConfirmRequest
	c.OnConfirm(func(r ConfirmRequest) { asked = append(asked, r) })
	c.Write(1, core.ColorBars, core.Number(1))
	waitWrites(c)
	if len(asked) != 1 || asked[0].Slot != 1 || asked[0].Value.Number != 1 || asked[0].Spec.ID != core.ColorBars {
		t.Fatalf("asked = %+v", asked)
	}
	if len(f.confirmCalls()) != 0 {
		t.Fatal("nothing may be confirmed before the user answers")
	}
	if v := c.Value(1, core.ColorBars); v == nil || v.Number != 0 {
		t.Fatalf("value while asking = %v, want the last known 0", v)
	}

	// cancel: the widget is told to show the last known value again
	n := rec.count(ChValue)
	c.Decline(asked[0])
	if rec.count(ChValue) != n+1 || len(f.confirmCalls()) != 0 {
		t.Error("decline must refresh the control and send nothing")
	}

	// confirm: ConfirmSetControl is called and the value is kept
	c.Confirm(asked[0])
	waitWrites(c)
	if got := f.confirmCalls(); len(got) != 1 || got[0].ID != core.ColorBars || got[0].V.Number != 1 {
		t.Fatalf("confirm calls = %v", got)
	}
	if v := c.Value(1, core.ColorBars); v == nil || v.Number != 1 {
		t.Errorf("value after confirm = %v", v)
	}
}

// A camera that fails to connect while its controls are being read must
// still end up with its controls (offline, without values), not stuck on
// "reading controls".
func TestStatusChangeDuringTheFirstLoadDoesNotLoseTheControls(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnecting, app.ControlView{Spec: rangeSpec(core.Gain, 0, 36, 1)})
	var pending []func()
	o := syncOptions()
	o.Go = func(fn func()) { pending = append(pending, fn) }
	c := NewController(f, o)
	c.Load() // the first read is in flight
	f.setStatus(1, core.StateError)
	c.RefreshRig()
	for len(pending) > 0 {
		fn := pending[0]
		pending = pending[1:]
		fn()
		testUI.drain()
	}
	testUI.drain()
	if !c.Loaded(1) || !c.Has(1, core.Gain) {
		t.Fatalf("loaded=%v: the controls of an offline camera must still be shown", c.Loaded(1))
	}
	if c.Value(1, core.Gain) != nil {
		t.Error("an offline camera has no values")
	}
}
