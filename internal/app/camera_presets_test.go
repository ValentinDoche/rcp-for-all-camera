package app_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"reflect"
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// --- a connected camera that remembers its values ----------------------------

type presetCamera struct {
	mu      sync.Mutex
	specs   []core.ControlSpec
	state   core.ConnState
	values  map[core.ControlID]core.Value
	failGet map[core.ControlID]bool
	failSet map[core.ControlID]bool
	sets    []core.ControlID
}

func presetSpecs() []core.ControlSpec {
	iris := core.Std(core.Iris)
	iris.Kind, iris.Min, iris.Max = core.KindRange, 1.8, 22
	focus := core.Std(core.Focus)
	focus.Kind, focus.Min, focus.Max = core.KindRange, 0, 1
	nd := core.Std(core.ND)
	nd.Kind, nd.Options = core.KindEnum, []core.Option{{Value: "0", Label: "Clear"}, {Value: "2", Label: "2 stops"}}
	wb := core.Std(core.WhiteBalance)
	wb.Kind, wb.Min, wb.Max = core.KindRange, 2500, 10000
	tally := core.Std(core.Tally)
	tally.Kind, tally.ReadOnly = core.KindToggle, true
	rec := core.Std(core.Record) // guarded
	rec.Kind = core.KindToggle
	af := core.Std(core.AutoFocus)
	af.Kind = core.KindAction
	pan := core.Std(core.Pan)
	pan.Kind, pan.Min, pan.Max = core.KindRange, -1, 1
	ptzPreset := core.Std(core.PresetRecall)
	ptzPreset.Kind, ptzPreset.Options = core.KindEnum, []core.Option{{Value: "1", Label: "1"}}
	return []core.ControlSpec{iris, focus, nd, wb, tally, rec, af, pan, ptzPreset}
}

func newPresetCamera() *presetCamera {
	return &presetCamera{
		specs: presetSpecs(), state: core.StateConnected,
		values: map[core.ControlID]core.Value{
			core.Iris: core.Number(4), core.Focus: core.Number(0.5), core.ND: core.Choice("0"),
			core.WhiteBalance: core.Number(5600), core.Tally: core.Bool(true), core.Record: core.Bool(false),
			core.Pan: core.Number(0), core.PresetRecall: core.Choice("1"),
		},
		failGet: map[core.ControlID]bool{}, failSet: map[core.ControlID]bool{},
	}
}

func (c *presetCamera) Controls() []core.ControlSpec {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]core.ControlSpec(nil), c.specs...)
}
func (c *presetCamera) Connect(context.Context) error { return nil }
func (c *presetCamera) Close() error                  { return nil }
func (c *presetCamera) Status() core.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return core.Status{State: c.state}
}
func (c *presetCamera) Subscribe(func(core.Event)) func() { return func() {} }

func (c *presetCamera) Get(_ context.Context, id core.ControlID) (core.Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != core.StateConnected {
		return core.Value{}, core.ErrNotConnected
	}
	if c.failGet[id] {
		return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotConnected)
	}
	v, ok := c.values[id]
	if !ok {
		return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrUnknownControl)
	}
	return v, nil
}

func (c *presetCamera) Set(_ context.Context, id core.ControlID, v core.Value) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	sp, ok := app.FindSpec(c.specs, id)
	if !ok {
		return core.ErrUnknownControl
	}
	if err := sp.Validate(v); err != nil {
		return err
	}
	if c.state != core.StateConnected {
		return core.ErrNotConnected
	}
	c.sets = append(c.sets, id)
	if c.failSet[id] {
		return fmt.Errorf("set %s: %w", id, core.ErrInvalidValue)
	}
	c.values[id] = v
	return nil
}

func (c *presetCamera) set(id core.ControlID, v core.Value) {
	c.mu.Lock()
	c.values[id] = v
	c.mu.Unlock()
}

func (c *presetCamera) value(id core.ControlID) core.Value {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.values[id]
}

func (c *presetCamera) written() []core.ControlID {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]core.ControlID(nil), c.sets...)
}

func (c *presetCamera) setState(s core.ConnState) {
	c.mu.Lock()
	c.state = s
	c.mu.Unlock()
}

type presetDriver struct{ testDriver }

func (presetDriver) New(core.CameraConfig) (core.Camera, error) {
	return newPresetCamera(), nil
}

// flakyStore is a memStore whose Save can be made to fail.
type flakyStore struct {
	memStore
	fail bool
}

func (f *flakyStore) Save(r app.SavedRig) error {
	if f.fail {
		return errors.New("disk full")
	}
	return f.memStore.Save(r)
}

type presetFixture struct {
	svc    *app.Service
	reg    *core.Registry
	store  *flakyStore
	events *rigLog
}

// rigLog keeps the Rig payloads of rig:changed.
type rigLog struct {
	mu    sync.Mutex
	rigs  []app.Rig
	armed []bool // payloads of preset:storeArmed
}

func (l *rigLog) Emit(name string, p any) {
	if name == app.EventStoreArmed {
		l.mu.Lock()
		l.armed = append(l.armed, p.(app.StoreArmedState).Armed)
		l.mu.Unlock()
		return
	}
	if name != app.EventRigChanged {
		return
	}
	l.mu.Lock()
	l.rigs = append(l.rigs, p.(app.Rig))
	l.mu.Unlock()
}

func (l *rigLog) last(t *testing.T) app.Rig {
	t.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	if len(l.rigs) == 0 {
		t.Fatal("no rig:changed event")
	}
	return l.rigs[len(l.rigs)-1]
}

func (l *rigLog) count() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.rigs)
}

func newPresetFixture(t *testing.T, saved app.SavedRig) presetFixture {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(acme(presetDriver{})...); err != nil {
		t.Fatal(err)
	}
	f := presetFixture{reg: reg, store: &flakyStore{memStore: memStore{rig: saved}}, events: &rigLog{}}
	f.svc = f.newService(t)
	return f
}

// newService builds a Service on the fixture's store (a restart).
func (f presetFixture) newService(t *testing.T) *app.Service {
	t.Helper()
	svc, err := app.NewService(app.Options{Registry: f.reg, Store: f.store, Secrets: newMemSecrets(),
		Go: func(func()) {}})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetEmitter(f.events)
	return svc
}

func presetCam(t *testing.T, svc *app.Service, slot int) *presetCamera {
	t.Helper()
	cam, err := svc.Camera(slot)
	if err != nil {
		t.Fatal(err)
	}
	return cam.(*presetCamera)
}

func assigned(t *testing.T, f presetFixture, slot int) *presetCamera {
	t.Helper()
	if err := f.svc.AssignSlot(slot, validInput()); err != nil {
		t.Fatal(err)
	}
	return presetCam(t, f.svc, slot)
}

// --- tests ---------------------------------------------------------------------

func TestStoreThenRecallWritesStoredValuesBack(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	if err := f.svc.StorePreset(1, 2); err != nil {
		t.Fatal(err)
	}
	cam.set(core.Iris, core.Number(8))
	cam.set(core.ND, core.Choice("2"))
	cam.set(core.WhiteBalance, core.Number(3200))
	if err := f.svc.RecallPreset(1, 2); err != nil {
		t.Fatal(err)
	}
	if v := cam.value(core.Iris); v != core.Number(4) {
		t.Errorf("iris = %+v, want 4", v)
	}
	if v := cam.value(core.ND); v != core.Choice("0") {
		t.Errorf("nd = %+v, want 0", v)
	}
	if v := cam.value(core.WhiteBalance); v != core.Number(5600) {
		t.Errorf("whiteBalance = %+v, want 5600", v)
	}
	// Written in the camera's Controls() order.
	want := []core.ControlID{core.Iris, core.Focus, core.ND, core.WhiteBalance}
	if got := cam.written(); !reflect.DeepEqual(got, want) {
		t.Errorf("written = %v, want %v", got, want)
	}
}

func TestStorePresetKeepsOnlyStorableControls(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	if err := f.svc.StorePreset(1, 1); err != nil {
		t.Fatal(err)
	}
	if len(f.store.rig.Presets) != 1 {
		t.Fatalf("stored presets = %+v", f.store.rig.Presets)
	}
	p := f.store.rig.Presets[0]
	if p.Slot != 1 || p.Number != 1 || p.Model != "acme-cam" {
		t.Fatalf("preset = %+v", p)
	}
	for _, id := range []core.ControlID{core.Tally, core.Record, core.AutoFocus, core.Pan, core.PresetRecall} {
		if _, ok := p.Values[id]; ok {
			t.Errorf("%s was stored; read-only, guarded, action and PTZ controls must not be", id)
		}
	}
	if len(p.Values) != 4 {
		t.Errorf("values = %v, want iris, focus, nd, whiteBalance", p.Values)
	}
}

func TestStorePresetSkipsControlsThatFailToRead(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	cam.failGet[core.Focus] = true
	if err := f.svc.StorePreset(1, 1); err != nil {
		t.Fatal(err)
	}
	if _, ok := f.store.rig.Presets[0].Values[core.Focus]; ok {
		t.Fatal("a control that failed to read was stored")
	}
	for id := range cam.values {
		cam.failGet[id] = true
	}
	if c := code(t, f.svc.StorePreset(1, 2)); c != app.CodeNotConnected {
		t.Fatalf("store with nothing readable code = %s, want NOT_CONNECTED", c)
	}
}

func TestPresetsSurviveARestart(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 3)
	if err := f.svc.StorePreset(3, 5); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(f.store.rig)
	if err != nil {
		t.Fatal(err)
	}
	var reloaded app.SavedRig
	if err := json.Unmarshal(b, &reloaded); err != nil {
		t.Fatal(err)
	}
	f.store.rig = reloaded
	svc := f.newService(t)
	if got := svc.Rig().Slots[2].Presets; !reflect.DeepEqual(got, []int{5}) {
		t.Fatalf("presets after restart = %v, want [5]", got)
	}
	cam := presetCam(t, svc, 3)
	cam.set(core.Iris, core.Number(11))
	if err := svc.RecallPreset(3, 5); err != nil {
		t.Fatal(err)
	}
	if v := cam.value(core.Iris); v != core.Number(4) {
		t.Fatalf("iris after recall = %+v, want 4", v)
	}
}

func TestRigListsStoredPresetNumbers(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	for _, n := range []int{4, 1, 3} {
		if err := f.svc.StorePreset(1, n); err != nil {
			t.Fatal(err)
		}
	}
	if got := f.events.last(t).Slots[0].Presets; !reflect.DeepEqual(got, []int{1, 3, 4}) {
		t.Fatalf("rig:changed presets = %v, want [1 3 4]", got)
	}
	rig := f.svc.Rig()
	if got := rig.Slots[0].Presets; !reflect.DeepEqual(got, []int{1, 3, 4}) {
		t.Fatalf("Rig() presets = %v, want [1 3 4]", got)
	}
	if rig.Slots[1].Presets == nil {
		t.Fatal("empty slot presets must be [] in JSON, not null")
	}
	if err := f.svc.DeletePreset(1, 3); err != nil {
		t.Fatal(err)
	}
	if got := f.events.last(t).Slots[0].Presets; !reflect.DeepEqual(got, []int{1, 4}) {
		t.Fatalf("after delete presets = %v, want [1 4]", got)
	}
	n := f.events.count()
	if err := f.svc.DeletePreset(1, 3); err != nil {
		t.Fatalf("second delete = %v, want nil", err)
	}
	if f.events.count() != n {
		t.Fatal("deleting a missing preset emitted rig:changed")
	}
	if len(f.store.rig.Presets) != 2 {
		t.Fatalf("stored presets after delete = %+v", f.store.rig.Presets)
	}
}

func TestStoreReplacesAnExistingPreset(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	cam.set(core.Iris, core.Number(16))
	if err := f.svc.StorePreset(1, 1); err != nil {
		t.Fatal(err)
	}
	if len(f.store.rig.Presets) != 1 || f.store.rig.Presets[0].Values[core.Iris] != core.Number(16) {
		t.Fatalf("stored presets = %+v", f.store.rig.Presets)
	}
}

func TestModelChangeDropsPresetsSameModelEditKeepsThem(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	in := validInput()
	in.Name, in.Host, in.Port = "renamed", "10.0.0.9", 8080
	if err := f.svc.AssignSlot(1, in); err != nil {
		t.Fatal(err)
	}
	if got := f.svc.Rig().Slots[0].Presets; !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("same-model edit: presets = %v, want [1]", got)
	}
	if err := f.svc.AssignSlot(1, app.SlotInput{Model: "acme-secure", Host: "10.0.0.9", Username: "admin"}); err != nil {
		t.Fatal(err)
	}
	if got := f.svc.Rig().Slots[0].Presets; len(got) != 0 {
		t.Fatalf("model change: presets = %v, want none", got)
	}
	if len(f.store.rig.Presets) != 0 {
		t.Fatalf("model change: stored presets = %+v", f.store.rig.Presets)
	}
}

func TestClearSlotDropsPresets(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 2)
	_ = f.svc.StorePreset(2, 1)
	if err := f.svc.ClearSlot(2); err != nil {
		t.Fatal(err)
	}
	if len(f.store.rig.Presets) != 0 {
		t.Fatalf("stored presets = %+v", f.store.rig.Presets)
	}
	assigned(t, f, 2)
	if c := code(t, f.svc.RecallPreset(2, 1)); c != app.CodeEmptyPreset {
		t.Fatalf("recall after clear code = %s, want EMPTY_PRESET", c)
	}
}

func TestFailedSaveRollsPresetChangesBack(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	f.store.fail = true
	if c := code(t, f.svc.StorePreset(1, 2)); c != app.CodeInternal {
		t.Fatalf("store with failing save code = %s, want INTERNAL", c)
	}
	if c := code(t, f.svc.ClearSlot(1)); c != app.CodeInternal {
		t.Fatalf("clear with failing save code = %s, want INTERNAL", c)
	}
	if err := f.svc.AssignSlot(1, app.SlotInput{Model: "acme-secure", Host: "10.0.0.9", Username: "admin"}); err == nil {
		t.Fatal("assign with failing save succeeded")
	}
	if c := code(t, f.svc.DeletePreset(1, 1)); c != app.CodeInternal {
		t.Fatalf("delete with failing save code = %s, want INTERNAL", c)
	}
	if got := f.svc.Rig().Slots[0].Presets; !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("presets after failed saves = %v, want [1]", got)
	}
}

func TestPresetNumberMustBeInRange(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	for _, n := range []int{0, -1, app.PresetCount + 1} {
		if c := code(t, f.svc.StorePreset(1, n)); c != app.CodeInvalidPreset {
			t.Errorf("StorePreset(1, %d) code = %s, want INVALID_PRESET", n, c)
		}
		if c := code(t, f.svc.RecallPreset(1, n)); c != app.CodeInvalidPreset {
			t.Errorf("RecallPreset(1, %d) code = %s, want INVALID_PRESET", n, c)
		}
		if c := code(t, f.svc.DeletePreset(1, n)); c != app.CodeInvalidPreset {
			t.Errorf("DeletePreset(1, %d) code = %s, want INVALID_PRESET", n, c)
		}
	}
	if c := code(t, f.svc.StorePreset(9, 1)); c != app.CodeInvalidSlot {
		t.Errorf("StorePreset(9, 1) code = %s, want INVALID_SLOT", c)
	}
	if c := code(t, f.svc.StorePreset(2, 1)); c != app.CodeEmptySlot {
		t.Errorf("StorePreset(empty slot) code = %s, want EMPTY_SLOT", c)
	}
	if c := code(t, f.svc.RecallPreset(2, 1)); c != app.CodeEmptySlot {
		t.Errorf("RecallPreset(empty slot) code = %s, want EMPTY_SLOT", c)
	}
}

func TestRecallOfAMissingPreset(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	if c := code(t, f.svc.RecallPreset(1, 1)); c != app.CodeEmptyPreset {
		t.Fatalf("code = %s, want EMPTY_PRESET", c)
	}
}

func TestRecallRefusesAPresetOfAnotherModel(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	f.svc.EditPreset(1, 1, func(p *app.SavedPreset) { p.Model = "acme-secure" })
	if c := code(t, f.svc.RecallPreset(1, 1)); c != app.CodeInvalidPreset {
		t.Fatalf("code = %s, want INVALID_PRESET", c)
	}
	if len(cam.written()) != 0 {
		t.Fatal("a preset of another model was written")
	}
}

func TestRecallContinuesPastAFailingControl(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	cam.set(core.Iris, core.Number(8))
	cam.set(core.WhiteBalance, core.Number(3200))
	cam.failSet[core.Focus] = true
	if c := code(t, f.svc.RecallPreset(1, 1)); c != app.CodeInvalidValue {
		t.Fatalf("code = %s, want the failing control's INVALID_VALUE", c)
	}
	if cam.value(core.Iris) != core.Number(4) || cam.value(core.WhiteBalance) != core.Number(5600) {
		t.Fatal("controls around the failing one were not written")
	}
}

func TestRecallSkipsControlsNoLongerStorable(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	f.svc.EditPreset(1, 1, func(p *app.SavedPreset) {
		p.Values[core.Record] = core.Bool(true)        // guarded
		p.Values[core.Pan] = core.Number(1)            // PTZ
		p.Values["gone"] = core.Number(1)              // no longer exists
		p.Values[core.Tally] = core.Bool(false)        // read-only
		p.Values[core.AutoFocus] = core.Trigger()      // action
		p.Values[core.PresetRecall] = core.Choice("1") // PTZ
	})
	if err := f.svc.RecallPreset(1, 1); err != nil {
		t.Fatal(err)
	}
	want := []core.ControlID{core.Iris, core.Focus, core.ND, core.WhiteBalance}
	if got := cam.written(); !reflect.DeepEqual(got, want) {
		t.Fatalf("written = %v, want %v", got, want)
	}
}

func TestPresetsNeedAConnectedCamera(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	cam := assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	cam.setState(core.StateDisconnected)
	if c := code(t, f.svc.StorePreset(1, 2)); c != app.CodeNotConnected {
		t.Errorf("store code = %s, want NOT_CONNECTED", c)
	}
	if c := code(t, f.svc.RecallPreset(1, 1)); c != app.CodeNotConnected {
		t.Errorf("recall code = %s, want NOT_CONNECTED", c)
	}
	if len(cam.written()) != 0 {
		t.Error("recall wrote to a disconnected camera")
	}
}

func TestInvalidSavedPresetsAreDroppedAtLoad(t *testing.T) {
	saved := app.SavedRig{}
	saved.Slots[0] = &core.CameraConfig{Model: "acme-cam", Host: "10.0.0.1"}
	saved.Slots[1] = &core.CameraConfig{Model: "acme-cam", Host: "10.0.0.2"}
	saved.Slots[3] = &core.CameraConfig{Model: "gone.model", Host: "10.0.0.4"}
	vals := func(iris float64) map[core.ControlID]core.Value {
		return map[core.ControlID]core.Value{core.Iris: core.Number(iris)}
	}
	saved.Presets = []app.SavedPreset{
		{Slot: 1, Number: 1, Model: "acme-cam", Values: vals(4)},
		{Slot: 1, Number: 1, Model: "acme-cam", Values: vals(8)}, // duplicate: the last wins
		{Slot: 1, Number: 2, Model: "acme-cam", Values: map[core.ControlID]core.Value{
			core.Iris: core.Number(5.6), "bad id!": core.Number(1), core.ND: {Kind: "weird"},
		}},
		{Slot: 1, Number: 0, Model: "acme-cam", Values: vals(4)},                   // number out of range
		{Slot: 1, Number: app.PresetCount + 1, Model: "acme-cam", Values: vals(4)}, // number out of range
		{Slot: 0, Number: 1, Model: "acme-cam", Values: vals(4)},                   // slot out of range
		{Slot: 9, Number: 1, Model: "acme-cam", Values: vals(4)},                   // slot out of range
		{Slot: 2, Number: 1, Model: "acme-secure", Values: vals(4)},                // other model
		{Slot: 3, Number: 1, Model: "acme-cam", Values: vals(4)},                   // empty slot
		{Slot: 4, Number: 1, Model: "gone.model", Values: vals(4)},                 // slot dropped at load
	}
	f := newPresetFixture(t, saved)
	rig := f.svc.Rig()
	if got := rig.Slots[0].Presets; !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("slot 1 presets = %v, want [1 2]", got)
	}
	for i := 1; i < app.SlotCount; i++ {
		if len(rig.Slots[i].Presets) != 0 {
			t.Errorf("slot %d presets = %v, want none", i+1, rig.Slots[i].Presets)
		}
	}
	p1 := struct{ Values map[core.ControlID]core.Value }{f.svc.PresetValues(1, 1)}
	p2 := struct{ Values map[core.ControlID]core.Value }{f.svc.PresetValues(1, 2)}
	if p1.Values[core.Iris] != core.Number(8) {
		t.Errorf("duplicate kept %+v, want the last one (iris 8)", p1.Values)
	}
	if len(p2.Values) != 1 || p2.Values[core.Iris] != core.Number(5.6) {
		t.Errorf("invalid values kept: %+v", p2.Values)
	}
}

func TestSavedPresetValuesAreNotShared(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	_ = f.svc.StorePreset(1, 1)
	f.store.rig.Presets[0].Values[core.Iris] = core.Number(22)
	got := f.svc.PresetValues(1, 1)[core.Iris]
	if got != core.Number(4) {
		t.Fatal("the saved rig shares its values map with the service")
	}
}

func TestPresetsAreSafeForConcurrentUse(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			n := i%app.PresetCount + 1
			_ = f.svc.StorePreset(1, n)
			_ = f.svc.RecallPreset(1, n)
			_ = f.svc.Rig()
			_ = f.svc.DeletePreset(1, n)
		}(i)
	}
	wg.Wait()
}

func TestImportKeepsPresetsOnlyWhereTheModelStays(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	assigned(t, f, 2)
	assigned(t, f, 3)
	for slot := 1; slot <= 3; slot++ {
		if err := f.svc.StorePreset(slot, 1); err != nil {
			t.Fatal(err)
		}
	}
	doc := app.ExportDoc{Format: app.ExportFormat, Version: app.ExportVersion, Slots: []*app.ExportCamera{
		{Model: "acme-cam", Host: "10.0.0.1"},
		{Model: "acme-secure", Host: "10.0.0.2", Username: "admin"},
		nil,
	}}
	b, _ := json.Marshal(doc)
	if err := f.svc.ImportConfig(b); err != nil {
		t.Fatal(err)
	}
	rig := f.svc.Rig()
	if !reflect.DeepEqual(rig.Slots[0].Presets, []int{1}) || len(rig.Slots[1].Presets) != 0 || len(rig.Slots[2].Presets) != 0 {
		t.Fatalf("presets after import = %v / %v / %v, want [1] / [] / []",
			rig.Slots[0].Presets, rig.Slots[1].Presets, rig.Slots[2].Presets)
	}
	if len(f.store.rig.Presets) != 1 {
		t.Fatalf("stored presets after import = %+v", f.store.rig.Presets)
	}
}

func (l *rigLog) armedEvents() []bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]bool(nil), l.armed...)
}

func TestStoreArmingEmitsOnlyOnChange(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	if f.svc.StoreArmed() {
		t.Fatal("must start disarmed")
	}
	f.svc.SetStoreArmed(false)
	if got := f.events.armedEvents(); len(got) != 0 {
		t.Fatalf("disarming a disarmed Store emitted %v", got)
	}
	f.svc.SetStoreArmed(true)
	f.svc.SetStoreArmed(true)
	if got := f.events.armedEvents(); !f.svc.StoreArmed() || !reflect.DeepEqual(got, []bool{true}) {
		t.Fatalf("armed=%v events=%v", f.svc.StoreArmed(), got)
	}
	f.svc.SetStoreArmed(false)
	if got := f.events.armedEvents(); f.svc.StoreArmed() || !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatalf("armed=%v events=%v", f.svc.StoreArmed(), got)
	}
}

func TestStorePresetDisarmsStore(t *testing.T) {
	f := newPresetFixture(t, app.SavedRig{})
	assigned(t, f, 1)
	f.svc.SetStoreArmed(true)
	if err := f.svc.StorePreset(1, 2); err != nil {
		t.Fatal(err)
	}
	if got := f.events.armedEvents(); f.svc.StoreArmed() || !reflect.DeepEqual(got, []bool{true, false}) {
		t.Fatalf("armed=%v events=%v", f.svc.StoreArmed(), got)
	}
	// A failed store also ends the gesture.
	f.svc.SetStoreArmed(true)
	if c := code(t, f.svc.StorePreset(1, 99)); c != app.CodeInvalidPreset {
		t.Fatalf("code = %s", c)
	}
	if f.svc.StoreArmed() {
		t.Fatal("a failed store must disarm")
	}
}
