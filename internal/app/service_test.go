package app

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// --- fakes ---------------------------------------------------------------

type testDriver struct{}

func (testDriver) Name() string { return "acme" }
func (testDriver) Models() []core.ModelInfo {
	return []core.ModelInfo{
		{ID: "acme.cam", Brand: "Acme", Name: "Cam", Driver: "acme",
			Transports: []core.Transport{core.TransportEthernet, core.TransportWiFi}, DefaultPort: 80,
			Auth: core.AuthNone, Support: core.SupportPlanned, InterchangeableLens: true},
		{ID: "acme.secure", Brand: "Acme", Name: "Secure", Driver: "acme",
			Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 443,
			Auth: core.AuthRequired, Support: core.SupportPlanned},
	}
}
func (testDriver) New(core.ModelInfo, core.CameraConfig) (core.Camera, error) {
	iris := core.Std(core.Iris)
	iris.Kind, iris.Min, iris.Max = core.KindRange, 1.8, 22
	return camerakit.NewPlaceholder([]core.ControlSpec{iris}), nil
}

type memStore struct {
	mu    sync.Mutex
	rig   config.Rig
	saves int
}

func (m *memStore) Load() (config.Rig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.rig, nil
}
func (m *memStore) Save(r config.Rig) error {
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
	svc     *Service
	store   *memStore
	secrets *config.MemorySecrets
	events  *recorder
}

func newFixture(t *testing.T, initial config.Rig) fixture {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(testDriver{}); err != nil {
		t.Fatal(err)
	}
	f := fixture{store: &memStore{rig: initial}, secrets: config.NewMemorySecrets(), events: &recorder{}}
	svc, err := NewService(Options{Registry: reg, Store: f.store, Secrets: f.secrets})
	if err != nil {
		t.Fatal(err)
	}
	svc.setEmitter(f.events)
	f.svc = svc
	return f
}

func validInput() SlotInput {
	return SlotInput{Model: "acme.cam", Name: "Cam A", Host: "192.168.10.11"}
}

func strPtr(s string) *string { return &s }

func code(t *testing.T, err error) string {
	t.Helper()
	var ae *Error
	if !errors.As(err, &ae) {
		t.Fatalf("error %v is not *app.Error", err)
	}
	var decoded Error
	if json.Unmarshal([]byte(err.Error()), &decoded) != nil || decoded.Code != ae.Code {
		t.Fatalf("Error() = %q is not the JSON contract", err.Error())
	}
	return ae.Code
}

// --- tests ---------------------------------------------------------------

func TestFreshRigHasEightEmptySlots(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	rig := f.svc.Rig()
	if len(rig.Slots) != SlotCount || rig.Active != 0 {
		t.Fatalf("Rig() = %+v, want 8 slots, active 0", rig)
	}
	for i, s := range rig.Slots {
		if s.Slot != i+1 || s.Camera != nil {
			t.Fatalf("slot %d = %+v, want empty slot %d", i, s, i+1)
		}
	}
}

func TestModelsListsRegisteredModels(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	if got := f.svc.Models(); len(got) != 2 || got[0].ID != "acme.cam" {
		t.Fatalf("Models() = %v", got)
	}
}

func TestAssignSlotRejectsOutOfRangeSlot(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	for _, n := range []int{0, 9, -1} {
		_, err := f.svc.AssignSlot(n, validInput())
		if code(t, err) != CodeInvalidSlot {
			t.Errorf("AssignSlot(%d) code = %s, want INVALID_SLOT", n, code(t, err))
		}
	}
}

func TestAssignSlotRejectsUnknownModel(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	in := validInput()
	in.Model = "acme.nope"
	_, err := f.svc.AssignSlot(1, in)
	if code(t, err) != CodeUnknownModel {
		t.Fatalf("code = %s, want UNKNOWN_MODEL", code(t, err))
	}
}

func TestAssignSlotRejectsInvalidHost(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	in := validInput()
	in.Host = "http://evil/x"
	_, err := f.svc.AssignSlot(1, in)
	if code(t, err) != CodeInvalidConfig {
		t.Fatalf("code = %s, want INVALID_CONFIG", code(t, err))
	}
	if f.store.saves != 0 {
		t.Fatal("invalid config was saved")
	}
}

func TestAssignSlotFillsModelDefaults(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	s, err := f.svc.AssignSlot(3, validInput())
	if err != nil {
		t.Fatal(err)
	}
	c := s.Camera
	if s.Slot != 3 || c == nil || c.Port != 80 || c.Brand != "Acme" || c.ModelName != "Cam" {
		t.Fatalf("AssignSlot = %+v / %+v", s, c)
	}
	if c.Status.State != core.StateDisconnected {
		t.Fatalf("status = %v, want disconnected", c.Status)
	}
}

func TestAssignSlotPersistsAndEmits(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	if _, err := f.svc.AssignSlot(2, validInput()); err != nil {
		t.Fatal(err)
	}
	if f.store.rig.Slots[1] == nil || f.store.rig.Slots[1].Host != "192.168.10.11" {
		t.Fatalf("stored rig = %+v", f.store.rig)
	}
	if f.events.count(EventRigChanged) != 1 {
		t.Fatalf("rig:changed emitted %d times, want 1", f.events.count(EventRigChanged))
	}
}

func TestPasswordGoesToSecretsNeverToViewOrStore(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	in := SlotInput{Model: "acme.secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("s3cret")}
	s, err := f.svc.AssignSlot(1, in)
	if err != nil {
		t.Fatal(err)
	}
	if !s.Camera.HasPassword {
		t.Fatal("HasPassword = false, want true")
	}
	if v, ok, _ := f.secrets.Get(secretKey(1)); !ok || v != "s3cret" {
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
	f := newFixture(t, config.Rig{Version: config.Version})
	in := SlotInput{Model: "acme.secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("s3cret")}
	_, _ = f.svc.AssignSlot(1, in)

	in.Password = nil
	in.Name = "renamed"
	s, _ := f.svc.AssignSlot(1, in)
	if !s.Camera.HasPassword {
		t.Fatal("nil password dropped the stored one")
	}
	in.Password = strPtr("")
	s, _ = f.svc.AssignSlot(1, in)
	if s.Camera.HasPassword {
		t.Fatal(`"" password did not clear the stored one`)
	}
}

func TestRequiredAuthNeedsUsername(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, err := f.svc.AssignSlot(1, SlotInput{Model: "acme.secure", Host: "10.0.0.2"})
	if code(t, err) != CodeInvalidConfig {
		t.Fatalf("code = %s, want INVALID_CONFIG", code(t, err))
	}
}

func TestClearSlotIsIdempotentAndDropsSecretAndSelection(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	in := SlotInput{Model: "acme.secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("pw")}
	_, _ = f.svc.AssignSlot(4, in)
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
	if _, ok, _ := f.secrets.Get(secretKey(4)); ok {
		t.Fatal("secret survived ClearSlot")
	}
}

func TestSelectSlot(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(5, validInput())
	if err := f.svc.SelectSlot(5); err != nil {
		t.Fatal(err)
	}
	if f.svc.Rig().Active != 5 || f.store.rig.Active != 5 {
		t.Fatal("selection not applied or not persisted")
	}
	if err := f.svc.SelectSlot(0); err != nil || f.svc.Rig().Active != 0 {
		t.Fatalf("SelectSlot(0) = %v", err)
	}
	if code(t, f.svc.SelectSlot(9)) != CodeInvalidSlot {
		t.Fatal("SelectSlot(9) accepted")
	}
}

func TestControlsOfEmptySlot(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, err := f.svc.Controls(1)
	if code(t, err) != CodeEmptySlot {
		t.Fatalf("code = %s, want EMPTY_SLOT", code(t, err))
	}
}

func TestControlsReturnSpecsWithUnknownValues(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(1, validInput())
	cv, err := f.svc.Controls(1)
	if err != nil {
		t.Fatal(err)
	}
	if len(cv) != 1 || cv[0].Spec.ID != core.Iris || cv[0].Value != nil {
		t.Fatalf("Controls = %+v", cv)
	}
}

func TestSetControlValidatesThenReportsNotImplemented(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(1, validInput())
	if c := code(t, f.svc.SetControl(1, "iris", core.Number(99))); c != CodeInvalidValue {
		t.Errorf("out of range code = %s, want INVALID_VALUE", c)
	}
	if c := code(t, f.svc.SetControl(1, "zoom", core.Number(0.5))); c != CodeUnknownControl {
		t.Errorf("unsupported code = %s, want UNKNOWN_CONTROL", c)
	}
	if c := code(t, f.svc.SetControl(1, "iris", core.Number(4))); c != CodeNotImplemented {
		t.Errorf("valid code = %s, want NOT_IMPLEMENTED", c)
	}
}

func TestConnectSlotReportsNotImplemented(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(1, validInput())
	if c := code(t, f.svc.ConnectSlot(1)); c != CodeNotImplemented {
		t.Fatalf("code = %s, want NOT_IMPLEMENTED", c)
	}
}

func TestStartupRestoresSavedRigAndDropsInvalidEntries(t *testing.T) {
	var saved config.Rig
	saved.Version = config.Version
	saved.Active = 2
	saved.Slots[1] = &core.CameraConfig{Model: "acme.cam", Name: "kept", Host: "10.0.0.1"}
	saved.Slots[2] = &core.CameraConfig{Model: "gone.model", Host: "10.0.0.2"}
	saved.Slots[3] = &core.CameraConfig{Model: "acme.cam", Host: "bad host"}
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
	var saved config.Rig
	saved.Version = config.Version
	saved.Slots[0] = &core.CameraConfig{Model: "acme.secure", Host: "10.0.0.1", Username: "admin"}
	reg := core.NewRegistry()
	_ = reg.Register(testDriver{})
	sec := config.NewMemorySecrets()
	_ = sec.Set(secretKey(1), "pw")
	svc, err := NewService(Options{Registry: reg, Store: &memStore{rig: saved}, Secrets: sec})
	if err != nil {
		t.Fatal(err)
	}
	if !svc.Rig().Slots[0].Camera.HasPassword {
		t.Fatal("stored password not restored")
	}
}

func TestPanelStatusEmitsOnlyOnChange(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	f.svc.setPanel(PanelStatus{Connected: true})
	f.svc.setPanel(PanelStatus{Connected: true})
	f.svc.setPanel(PanelStatus{Connected: false})
	if n := f.events.count(EventPanelStatus); n != 2 {
		t.Fatalf("panel:status emitted %d times, want 2", n)
	}
	if f.svc.PanelStatus().Connected {
		t.Fatal("PanelStatus() not updated")
	}
}

func TestInternalErrorsDoNotLeakDetails(t *testing.T) {
	err := toAPIError(errors.New(`open C:\Users\secret\rig.json: access denied`))
	if code(t, err) != CodeInternal || strings.Contains(err.Error(), "secret") {
		t.Fatalf("internal error leaked: %s", err)
	}
}

func TestSelectSlotRefusesEmptySlots(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(2, validInput())
	_ = f.svc.SelectSlot(2)
	if c := code(t, f.svc.SelectSlot(3)); c != CodeEmptySlot {
		t.Fatalf("SelectSlot(empty) code = %s, want EMPTY_SLOT", c)
	}
	if f.svc.Rig().Active != 2 {
		t.Fatal("a refused selection changed the active slot")
	}
}

func TestSavedSelectionOfAnEmptySlotIsDropped(t *testing.T) {
	saved := config.Rig{Version: config.Version, Active: 6}
	f := newFixture(t, saved)
	if f.svc.Rig().Active != 0 {
		t.Fatalf("active = %d, want 0 (slot 6 is empty)", f.svc.Rig().Active)
	}
}

func TestPowerZoomIsStoredOnlyForInterchangeableLenses(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	in := validInput()
	in.PowerZoom = true
	sl, err := f.svc.AssignSlot(1, in)
	if err != nil {
		t.Fatal(err)
	}
	if !sl.Camera.PowerZoom || !sl.Camera.InterchangeableLens || !f.store.rig.Slots[0].PowerZoom {
		t.Errorf("view %+v, stored %+v", sl.Camera, f.store.rig.Slots[0])
	}
	sec := SlotInput{Model: "acme.secure", Host: "192.168.10.12", Username: "op", PowerZoom: true}
	sl, err = f.svc.AssignSlot(2, sec)
	if err != nil {
		t.Fatal(err)
	}
	if sl.Camera.PowerZoom || sl.Camera.InterchangeableLens || f.store.rig.Slots[1].PowerZoom {
		t.Errorf("built-in lens: view %+v, stored %+v", sl.Camera, f.store.rig.Slots[1])
	}
}

func TestLensOfACameraWithoutLensInfo(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	if _, err := f.svc.Lens(1); code(t, err) != CodeEmptySlot {
		t.Errorf("empty slot: %v", err)
	}
	if _, err := f.svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	if l, err := f.svc.Lens(1); err != nil || l != (core.Lens{}) {
		t.Errorf("lens = %+v, %v", l, err)
	}
}
