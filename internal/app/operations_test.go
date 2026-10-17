package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// --- a driver whose cameras count Connect calls and have guarded controls --

type opsCamera struct {
	*camerakit.Placeholder
	mu       sync.Mutex
	connects int
	sets     []core.ControlID
}

func (c *opsCamera) Connect(context.Context) error {
	c.mu.Lock()
	c.connects++
	c.mu.Unlock()
	return core.ErrNotImplemented
}

func (c *opsCamera) Set(ctx context.Context, id core.ControlID, v core.Value) error {
	if err := c.Placeholder.Set(ctx, id, v); err != nil && !strings.Contains(err.Error(), "not implemented") {
		return err
	}
	c.mu.Lock()
	c.sets = append(c.sets, id)
	c.mu.Unlock()
	return nil
}

func (c *opsCamera) connectCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connects
}

type opsDriver struct {
	testDriver
	mu   sync.Mutex
	cams []*opsCamera
}

func (d *opsDriver) New(core.CameraConfig) (core.Camera, error) {
	iris := core.Std(core.Iris)
	iris.Kind, iris.Min, iris.Max = core.KindRange, 1.8, 22
	rec := core.Std(core.Record)
	rec.Kind = core.KindToggle
	c := &opsCamera{Placeholder: camerakit.NewPlaceholder([]core.ControlSpec{iris, rec})}
	d.mu.Lock()
	d.cams = append(d.cams, c)
	d.mu.Unlock()
	return c, nil
}

func (d *opsDriver) last() *opsCamera {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.cams[len(d.cams)-1]
}

func newOpsService(t *testing.T, saved SavedRig) (*Service, *memStore, *opsDriver, *slog.LevelVar) {
	t.Helper()
	reg := core.NewRegistry()
	drv := &opsDriver{}
	if err := reg.Register(acme(drv)...); err != nil {
		t.Fatal(err)
	}
	st := &memStore{rig: saved}
	lv := new(slog.LevelVar)
	svc, err := NewService(Options{
		Registry: reg, Store: st, Secrets: newMemSecrets(),
		LogLevel: lv, LogDir: `C:\logs\rcp`,
		Go: func(f func()) { f() }, // run background work inline in tests
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, drv, lv
}

// --- auto-connect -------------------------------------------------------------

func TestAssignedCameraConnectsAutomatically(t *testing.T) {
	svc, _, drv, _ := newOpsService(t, SavedRig{})
	if err := svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	if n := drv.last().connectCount(); n != 1 {
		t.Fatalf("Connect called %d times after assign, want 1", n)
	}
}

func TestConnectAllConnectsEverySavedCamera(t *testing.T) {
	var saved SavedRig
	saved.Slots[0] = &core.CameraConfig{Model: "acme-cam", Host: "10.0.0.1"}
	saved.Slots[4] = &core.CameraConfig{Model: "acme-cam", Host: "10.0.0.5"}
	svc, _, drv, _ := newOpsService(t, saved)
	svc.connectAll()
	for i, c := range drv.cams {
		if c.connectCount() != 1 {
			t.Errorf("camera %d: Connect called %d times, want 1", i, c.connectCount())
		}
	}
}

// --- operator lock --------------------------------------------------------------

func TestRecordAndColorBarsAreGuardedByDefault(t *testing.T) {
	if !core.Std(core.Record).Guarded || !core.Std(core.ColorBars).Guarded {
		t.Fatal("record and colorBars must be guarded in the standard catalog")
	}
	if core.Std(core.Iris).Guarded {
		t.Fatal("iris must not be guarded")
	}
}

func TestOperatorLockIsOnByDefault(t *testing.T) {
	svc, _, _, _ := newOpsService(t, SavedRig{})
	if !svc.Settings().OperatorLock {
		t.Fatal("operator lock must default to on")
	}
}

func TestLockedGuardedControlNeedsConfirmation(t *testing.T) {
	svc, _, drv, _ := newOpsService(t, SavedRig{})
	_ = svc.AssignSlot(1, validInput())
	if c := code(t, svc.SetControl(1, "record", core.Bool(true))); c != CodeConfirmRequired {
		t.Fatalf("code = %s, want CONFIRM_REQUIRED", c)
	}
	if len(drv.last().sets) != 0 {
		t.Fatal("guarded command reached the camera without confirmation")
	}
	if err := svc.ConfirmSetControl(1, "record", core.Bool(true)); err != nil {
		t.Fatalf("ConfirmSetControl = %v", err)
	}
	if got := drv.last().sets; len(got) != 1 || got[0] != core.Record {
		t.Fatalf("sets = %v, want [record]", got)
	}
}

func TestUnguardedControlIgnoresLock(t *testing.T) {
	svc, _, _, _ := newOpsService(t, SavedRig{})
	_ = svc.AssignSlot(1, validInput())
	if err := svc.SetControl(1, "iris", core.Number(4)); err != nil {
		t.Fatalf("SetControl(iris) = %v", err)
	}
}

func TestLockOffLetsGuardedControlsThroughAndPersists(t *testing.T) {
	svc, st, _, _ := newOpsService(t, SavedRig{})
	_ = svc.AssignSlot(1, validInput())
	if err := svc.SetOperatorLock(false); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetControl(1, "record", core.Bool(true)); err != nil {
		t.Fatalf("SetControl(record) with lock off = %v", err)
	}
	if st.rig.OperatorLock == nil || *st.rig.OperatorLock {
		t.Fatal("lock off not persisted")
	}
}

// --- logs ---------------------------------------------------------------------------

func TestLogLevelSettingDrivesTheLevelVar(t *testing.T) {
	svc, st, _, lv := newOpsService(t, SavedRig{})
	if svc.Settings().LogLevel != LogNormal || lv.Level() != slog.LevelInfo {
		t.Fatalf("default level = %s / %v", svc.Settings().LogLevel, lv.Level())
	}
	if err := svc.SetLogLevel(LogDebug); err != nil {
		t.Fatal(err)
	}
	if lv.Level() != slog.LevelDebug || st.rig.LogLevel != "debug" {
		t.Fatalf("debug not applied: %v / %q", lv.Level(), st.rig.LogLevel)
	}
	if code(t, svc.SetLogLevel("verbose")) != CodeInvalidConfig {
		t.Fatal("unknown level accepted")
	}
	if svc.Settings().LogDir != `C:\logs\rcp` {
		t.Fatalf("LogDir = %q", svc.Settings().LogDir)
	}
}

func TestSavedLogLevelIsAppliedAtStartup(t *testing.T) {
	_, _, _, lv := newOpsService(t, SavedRig{LogLevel: "debug"})
	if lv.Level() != slog.LevelDebug {
		t.Fatalf("level = %v, want debug", lv.Level())
	}
}

// --- export / import ------------------------------------------------------------------

func TestExportHasCamerasButNoPasswordsOrMachineSettings(t *testing.T) {
	svc, _, _, _ := newOpsService(t, SavedRig{})
	in := validInput()
	in.Password = strPtr("s3cret")
	_ = svc.AssignSlot(2, in)
	data, err := svc.ExportConfig()
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "s3cret") || strings.Contains(s, "bindIp") || strings.Contains(s, "logLevel") {
		t.Fatalf("export leaks secrets or machine settings: %s", s)
	}
	var doc map[string]any
	if json.Unmarshal(data, &doc) != nil || doc["format"] != ExportFormat {
		t.Fatalf("export is not a tagged JSON document: %s", s)
	}
}

func TestImportReplacesTheRigAndConnects(t *testing.T) {
	src, _, _, _ := newOpsService(t, SavedRig{})
	_ = src.AssignSlot(3, validInput())
	data, _ := src.ExportConfig()

	dst, st, drv, _ := newOpsService(t, SavedRig{})
	_ = dst.AssignSlot(1, validInput())
	if err := dst.ImportConfig(data); err != nil {
		t.Fatal(err)
	}
	rig := dst.Rig()
	if rig.Slots[0].Camera != nil || rig.Slots[2].Camera == nil {
		t.Fatalf("imported rig = %+v", rig)
	}
	if st.rig.Slots[2] == nil {
		t.Fatal("imported rig not saved")
	}
	if drv.last().connectCount() != 1 {
		t.Fatal("imported camera not connected")
	}
}

func TestImportRejectsInvalidDocumentsAtomically(t *testing.T) {
	svc, _, _, _ := newOpsService(t, SavedRig{})
	_ = svc.AssignSlot(1, validInput())
	bad := []string{
		`not json`,
		`{"format":"something-else","version":1,"slots":[]}`,
		`{"format":"` + ExportFormat + `","version":99,"slots":[]}`,
		`{"format":"` + ExportFormat + `","version":1,"slots":[{"model":"acme-cam","host":"http://x/"}]}`,
		`{"format":"` + ExportFormat + `","version":1,"slots":[{"model":"nope.x","host":"10.0.0.1"}]}`,
		`{"format":"` + ExportFormat + `","version":1,"slots":[null,null,null,null,null,null,null,null,{"model":"acme-cam","host":"10.0.0.1"}]}`,
		strings.Repeat(" ", maxImportSize+1),
	}
	for _, b := range bad {
		if c := code(t, svc.ImportConfig([]byte(b))); c != CodeInvalidConfig {
			t.Errorf("import %.40q: code %s, want INVALID_CONFIG", b, c)
		}
	}
	if svc.Rig().Slots[0].Camera == nil {
		t.Fatal("a rejected import changed the rig")
	}
}

func TestHostStartConnectsSavedCameras(t *testing.T) {
	var saved SavedRig
	saved.Slots[2] = &core.CameraConfig{Model: "acme-cam", Host: "10.0.0.3"}
	svc, _, drv, _ := newOpsService(t, saved)
	h := NewHost(svc, func() (bool, error) { return false, nil }, time.Hour)
	h.Start(context.Background(), nil)
	defer h.Stop()
	if drv.last().connectCount() != 1 {
		t.Fatal("saved camera not connected at start")
	}
}
