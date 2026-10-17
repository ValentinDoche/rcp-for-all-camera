package app_test

import (
	"encoding/json"
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"strings"
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// --- fakes ---------------------------------------------------------------

// builder builds the cameras of the test modules.
type builder interface {
	New(core.CameraConfig) (core.Camera, error)
}

// acmeModule is a test module: a model declaration and a builder.
type acmeModule struct {
	info core.ModelInfo
	b    builder
}

func (m acmeModule) Info() core.ModelInfo                           { return m.info }
func (m acmeModule) New(cfg core.CameraConfig) (core.Camera, error) { return m.b.New(cfg) }

// acme returns the two test modules, acme-cam and acme-secure (password
// required), whose cameras b builds.
func acme(b builder) []core.Module {
	return []core.Module{
		acmeModule{core.ModelInfo{ID: "acme-cam", Brand: "Acme", Name: "Cam",
			Transports: []core.Transport{core.TransportEthernet, core.TransportWiFi}, DefaultPort: 80,
			Auth: core.AuthNone, Support: core.SupportPlanned, InterchangeableLens: true}, b},
		acmeModule{core.ModelInfo{ID: "acme-secure", Brand: "Acme", Name: "Secure",
			Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 443,
			Auth: core.AuthRequired, Support: core.SupportPlanned}, b},
	}
}

// testDriver builds placeholder cameras with an iris.
type testDriver struct{}

func (testDriver) New(core.CameraConfig) (core.Camera, error) {
	iris := core.Std(core.Iris)
	iris.Kind, iris.Min, iris.Max = core.KindRange, 1.8, 22
	return camerakit.NewPlaceholder([]core.ControlSpec{iris}), nil
}

type memStore struct {
	mu    sync.Mutex
	rig   app.SavedRig
	saves int
}

func (m *memStore) Load() (app.SavedRig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rig, nil
}
func (m *memStore) Save(r app.SavedRig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rig = r
	m.saves++
	return nil
}

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) Emit(name string, _ any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, name)
}
func (r *recorder) count(name string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, e := range r.events {
		if e == name {
			n++
		}
	}
	return n
}

type fixture struct {
	svc     *app.Service
	store   *memStore
	secrets *memSecrets
	events  *recorder
}

func newFixture(t *testing.T, initial app.SavedRig) fixture {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(acme(testDriver{})...); err != nil {
		t.Fatal(err)
	}
	f := fixture{store: &memStore{rig: initial}, secrets: newMemSecrets(), events: &recorder{}}
	svc, err := app.NewService(app.Options{Registry: reg, Store: f.store, Secrets: f.secrets})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetEmitter(f.events)
	f.svc = svc
	return f
}

func validInput() app.SlotInput {
	return app.SlotInput{Model: "acme-cam", Name: "Cam A", Host: "192.168.10.11"}
}

func strPtr(s string) *string { return &s }

func code(t *testing.T, err error) string {
	t.Helper()
	var ae *app.Error
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not *app.Error", err)
	}
	var decoded app.Error
	if json.Unmarshal([]byte(err.Error()), &decoded) != nil || decoded.Code != ae.Code {
		t.Fatalf("Error() = %q is not the JSON contract", err.Error())
	}
	return ae.Code
}

// --- tests ---------------------------------------------------------------

func TestFreshRigHasEightEmptySlots(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	rig := f.svc.Rig()
	if len(rig.Slots) != app.SlotCount || rig.Active != 0 {
		t.Fatalf("Rig() = %+v, want 8 slots, active 0", rig)
	}
	for i, s := range rig.Slots {
		if s.Slot != i+1 || s.Camera != nil {
			t.Fatalf("slot %d = %+v, want empty slot %d", i, s, i+1)
		}
	}
}

func TestModelsListsRegisteredModels(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	if got := f.svc.Models(); len(got) != 2 || got[0].ID != "acme-cam" {
		t.Fatalf("Models() = %v", got)
	}
}

func TestAssignSlotRejectsOutOfRangeSlot(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	for _, n := range []int{0, 9, -1} {
		err := f.svc.AssignSlot(n, validInput())
		if code(t, err) != app.CodeInvalidSlot {
			t.Errorf("AssignSlot(%d) code = %s, want INVALID_SLOT", n, code(t, err))
		}
	}
}

func TestAssignSlotRejectsUnknownModel(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	in := validInput()
	in.Model = "acme.nope"
	err := f.svc.AssignSlot(1, in)
	if code(t, err) != app.CodeUnknownModel {
		t.Fatalf("code = %s, want UNKNOWN_MODEL", code(t, err))
	}
}

func TestAssignSlotRejectsInvalidHost(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	in := validInput()
	in.Host = "http://evil/x"
	err := f.svc.AssignSlot(1, in)
	if code(t, err) != app.CodeInvalidConfig {
		t.Fatalf("code = %s, want INVALID_CONFIG", code(t, err))
	}
	if f.store.saves != 0 {
		t.Fatal("invalid config was saved")
	}
}

func TestAssignSlotFillsModelDefaults(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	if err := f.svc.AssignSlot(3, validInput()); err != nil {
		t.Fatal(err)
	}
	c := f.svc.Rig().Slots[2].Camera
	if c == nil || c.Port != 80 || c.Brand != "Acme" || c.ModelName != "Cam" {
		t.Fatalf("slot 3 = %+v", c)
	}
	if c.Status.State != core.StateDisconnected {
		t.Fatalf("status = %v, want disconnected", c.Status)
	}
}

func TestAssignSlotPersistsAndEmits(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	if err := f.svc.AssignSlot(2, validInput()); err != nil {
		t.Fatal(err)
	}
	if f.store.rig.Slots[1] == nil || f.store.rig.Slots[1].Host != "192.168.10.11" {
		t.Fatalf("stored rig = %+v", f.store.rig)
	}
	if f.events.count(app.EventRigChanged) != 1 {
		t.Fatalf("rig:changed emitted %d times, want 1", f.events.count(app.EventRigChanged))
	}
}

func TestPasswordGoesToSecretsNeverToViewOrStore(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	in := app.SlotInput{Model: "acme-secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("s3cret")}
	if err := f.svc.AssignSlot(1, in); err != nil {
		t.Fatal(err)
	}
	if !f.svc.Rig().Slots[0].Camera.HasPassword {
		t.Fatal("HasPassword = false, want true")
	}
	if v, ok, _ := f.secrets.Get(app.SecretKey(1)); !ok || v != "s3cret" {
		t.Fatalf("secret = %q, %v", v, ok)
	}
	b, _ := json.Marshal(f.svc.Rig())
	if strings.Contains(string(b), "s3cret") {
		t.Fatal("password leaked into Rig()")
	}
	if f.store.rig.Slots[0].Password != "" {
		t.Fatal("password kept in stored config struct")
	}
}

func TestNilPasswordKeepsStoredOneEmptyClearsIt(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	in := app.SlotInput{Model: "acme-secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("s3cret")}
	_ = f.svc.AssignSlot(1, in)

	in.Password = nil
	in.Name = "renamed"
	_ = f.svc.AssignSlot(1, in)
	if !f.svc.Rig().Slots[0].Camera.HasPassword {
		t.Fatal("nil password dropped the stored one")
	}
	in.Password = strPtr("")
	_ = f.svc.AssignSlot(1, in)
	if f.svc.Rig().Slots[0].Camera.HasPassword {
		t.Fatal(`"" password did not clear the stored one`)
	}
}

func TestRequiredAuthNeedsUsername(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	err := f.svc.AssignSlot(1, app.SlotInput{Model: "acme-secure", Host: "10.0.0.2"})
	if code(t, err) != app.CodeInvalidConfig {
		t.Fatalf("code = %s, want INVALID_CONFIG", code(t, err))
	}
}

func TestClearSlotIsIdempotentAndDropsSecretAndSelection(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	in := app.SlotInput{Model: "acme-secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("pw")}
	_ = f.svc.AssignSlot(4, in)
	_ = f.svc.SelectSlot(4)
	if err := f.svc.ClearSlot(4); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ClearSlot(4); err != nil {
		t.Fatalf("second ClearSlot = %v, want nil", err)
	}
	rig := f.svc.Rig()
	if rig.Slots[3].Camera != nil || rig.Active != 0 {
		t.Fatalf("after clear: %+v", rig)
	}
	if _, ok, _ := f.secrets.Get(app.SecretKey(4)); ok {
		t.Fatal("secret survived ClearSlot")
	}
}

func TestSelectSlot(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	_ = f.svc.AssignSlot(5, validInput())
	if err := f.svc.SelectSlot(5); err != nil {
		t.Fatal(err)
	}
	if f.svc.Rig().Active != 5 || f.store.rig.Active != 5 {
		t.Fatal("selection not applied or not persisted")
	}
	if err := f.svc.SelectSlot(0); err != nil || f.svc.Rig().Active != 0 {
		t.Fatalf("SelectSlot(0) = %v", err)
	}
	if code(t, f.svc.SelectSlot(9)) != app.CodeInvalidSlot {
		t.Fatal("SelectSlot(9) accepted")
	}
}

func TestControlsOfEmptySlot(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	_, err := f.svc.Controls(1)
	if code(t, err) != app.CodeEmptySlot {
		t.Fatalf("code = %s, want EMPTY_SLOT", code(t, err))
	}
}

func TestControlsReturnSpecsWithUnknownValues(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	_ = f.svc.AssignSlot(1, validInput())
	cv, err := f.svc.Controls(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cv) != 1 || cv[0].Spec.ID != core.Iris || cv[0].Value != nil {
		t.Fatalf("Controls = %+v", cv)
	}
}

func TestSetControlValidatesThenReportsNotImplemented(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	_ = f.svc.AssignSlot(1, validInput())
	if c := code(t, f.svc.SetControl(1, "iris", core.Number(99))); c != app.CodeInvalidValue {
		t.Errorf("out of range code = %s, want INVALID_VALUE", c)
	}
	if c := code(t, f.svc.SetControl(1, "zoom", core.Number(0.5))); c != app.CodeUnknownControl {
		t.Errorf("unsupported code = %s, want UNKNOWN_CONTROL", c)
	}
	if c := code(t, f.svc.SetControl(1, "iris", core.Number(4))); c != app.CodeNotImplemented {
		t.Errorf("valid code = %s, want NOT_IMPLEMENTED", c)
	}
}

func TestConnectSlotReportsNotImplemented(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	_ = f.svc.AssignSlot(1, validInput())
	if c := code(t, f.svc.ConnectSlot(1)); c != app.CodeNotImplemented {
		t.Fatalf("code = %s, want NOT_IMPLEMENTED", c)
	}
}

func TestStartupRestoresSavedRigAndDropsInvalidEntries(t *testing.T) {
	var saved app.SavedRig
	saved.Active = 2
	saved.Slots[1] = &core.CameraConfig{Model: "acme-cam", Name: "kept", Host: "10.0.0.1"}
	saved.Slots[2] = &core.CameraConfig{Model: "gone.model", Host: "10.0.0.2"}
	saved.Slots[3] = &core.CameraConfig{Model: "acme-cam", Host: "bad host"}
	f := newFixture(t, saved)
	rig := f.svc.Rig()
	if rig.Active != 2 || rig.Slots[1].Camera == nil || rig.Slots[1].Camera.Name != "kept" {
		t.Fatalf("restored rig = %+v", rig)
	}
	if rig.Slots[2].Camera != nil || rig.Slots[3].Camera != nil {
		t.Fatal("invalid saved entries were not dropped")
	}
}

func TestStartupRestoresPasswordFromSecrets(t *testing.T) {
	var saved app.SavedRig
	saved.Slots[0] = &core.CameraConfig{Model: "acme-secure", Host: "10.0.0.1", Username: "admin"}
	reg := core.NewRegistry()
	_ = reg.Register(acme(testDriver{})...)
	sec := newMemSecrets()
	_ = sec.Set(app.SecretKey(1), "pw")
	svc, err := app.NewService(app.Options{Registry: reg, Store: &memStore{rig: saved}, Secrets: sec})
	if err != nil {
		t.Fatal(err)
	}
	if !svc.Rig().Slots[0].Camera.HasPassword {
		t.Fatal("stored password not restored")
	}
}

func TestPanelStatusEmitsOnlyOnChange(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	f.svc.SetPanel(app.PanelStatus{Connected: true})
	f.svc.SetPanel(app.PanelStatus{Connected: true})
	f.svc.SetPanel(app.PanelStatus{Connected: false})
	if n := f.events.count(app.EventPanelStatus); n != 2 {
		t.Fatalf("panel:status emitted %d times, want 2", n)
	}
	if f.svc.PanelStatus().Connected {
		t.Fatal("PanelStatus() not updated")
	}
}

func TestInternalErrorsDoNotLeakDetails(t *testing.T) {
	err := app.ToAPIError(errors.New(`open C:\Users\secret\rig.json: access denied`))
	if code(t, err) != app.CodeInternal || strings.Contains(err.Error(), "secret") {
		t.Fatalf("internal error leaked: %s", err)
	}
}

func TestSelectSlotRefusesEmptySlots(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	_ = f.svc.AssignSlot(2, validInput())
	_ = f.svc.SelectSlot(2)
	if c := code(t, f.svc.SelectSlot(3)); c != app.CodeEmptySlot {
		t.Fatalf("SelectSlot(empty) code = %s, want EMPTY_SLOT", c)
	}
	if f.svc.Rig().Active != 2 {
		t.Fatal("a refused selection changed the active slot")
	}
}

func TestSavedSelectionOfAnEmptySlotIsDropped(t *testing.T) {
	saved := app.SavedRig{Active: 6}
	f := newFixture(t, saved)
	if f.svc.Rig().Active != 0 {
		t.Fatalf("active = %d, want 0 (slot 6 is empty)", f.svc.Rig().Active)
	}
}

func TestPowerZoomIsStoredOnlyForInterchangeableLenses(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	in := validInput()
	in.PowerZoom = true
	if err := f.svc.AssignSlot(1, in); err != nil {
		t.Fatal(err)
	}
	if c := f.svc.Rig().Slots[0].Camera; !c.PowerZoom || !c.InterchangeableLens || !f.store.rig.Slots[0].PowerZoom {
		t.Errorf("view %+v, stored %+v", c, f.store.rig.Slots[0])
	}
	sec := app.SlotInput{Model: "acme-secure", Host: "192.168.10.12", Username: "op", PowerZoom: true}
	if err := f.svc.AssignSlot(2, sec); err != nil {
		t.Fatal(err)
	}
	if c := f.svc.Rig().Slots[1].Camera; c.PowerZoom || c.InterchangeableLens || f.store.rig.Slots[1].PowerZoom {
		t.Errorf("built-in lens: view %+v, stored %+v", c, f.store.rig.Slots[1])
	}
}

func TestLensOfACameraWithoutLensInfo(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	if _, err := f.svc.Lens(1); code(t, err) != app.CodeEmptySlot {
		t.Errorf("empty slot: %v", err)
	}
	if err := f.svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	if l, err := f.svc.Lens(1); err != nil || l != (core.Lens{}) {
		t.Errorf("lens = %+v, %v", l, err)
	}
}

func TestFailedWriteEmitsCommandFailed(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	if err := f.svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	// the placeholder camera answers NOT_IMPLEMENTED
	if err := f.svc.SetControl(1, string(core.Iris), core.Number(4)); err == nil {
		t.Fatal("placeholder write must fail")
	}
	if f.events.count(app.EventCommandFailed) != 1 {
		t.Errorf("%s emitted %d times, want 1", app.EventCommandFailed, f.events.count(app.EventCommandFailed))
	}
	// a write refused before reaching a camera (empty slot) is not a command
	_ = f.svc.SetControl(2, string(core.Iris), core.Number(4))
	if f.events.count(app.EventCommandFailed) != 1 {
		t.Error("an empty slot must not raise the alarm")
	}
}

// memSecrets keeps passwords in memory for the tests.
type memSecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func newMemSecrets() *memSecrets { return &memSecrets{m: map[string]string{}} }

func (s *memSecrets) Get(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	return v, ok, nil
}

func (s *memSecrets) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

func (s *memSecrets) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}
