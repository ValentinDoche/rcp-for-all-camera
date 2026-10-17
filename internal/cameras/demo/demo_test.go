package demo_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/demo"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestContract(t *testing.T) { cameratest.Module(t, demo.Module{}) }

func TestOnline(t *testing.T) {
	cam, _ := connected(t)
	cameratest.Online(t, cam)
}

// eventLog records the events a camera publishes.
type eventLog struct {
	mu     sync.Mutex
	events []core.Event
}

func (l *eventLog) add(e core.Event) { l.mu.Lock(); l.events = append(l.events, e); l.mu.Unlock() }

func (l *eventLog) has(match func(core.Event) bool) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.events {
		if match(e) {
			return true
		}
	}
	return false
}

// connected builds a demo camera with a power zoom and connects it.
func connected(t *testing.T) (core.Camera, *eventLog) {
	t.Helper()
	cam, err := demo.Module{}.New(core.CameraConfig{Model: demo.ID, PowerZoom: true})
	if err != nil {
		t.Fatal(err)
	}
	log := &eventLog{}
	cam.Subscribe(log.add)
	if err := cam.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cam.Close() })
	return cam, log
}

func set(t *testing.T, cam core.Camera, id core.ControlID, v core.Value) {
	t.Helper()
	if err := cam.Set(context.Background(), id, v); err != nil {
		t.Fatalf("Set(%s) = %v", id, err)
	}
}

func get(t *testing.T, cam core.Camera, id core.ControlID) core.Value {
	t.Helper()
	v, err := cam.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get(%s) = %v", id, err)
	}
	return v
}

func TestDemoNeedsNoAddress(t *testing.T) {
	m := demo.Module{}.Info()
	if err := core.ValidateConfig(m, core.CameraConfig{Model: m.ID}); err != nil {
		t.Errorf("a config without host must be valid: %v", err)
	}
}

func TestDemoKeepsItsFormerID(t *testing.T) {
	reg := core.NewRegistry()
	if err := reg.Register(demo.Module{}); err != nil {
		t.Fatal(err)
	}
	if info, ok := reg.Model("demo.camera"); !ok || info.ID != demo.ID {
		t.Errorf("Model(demo.camera) = %s, %t; want the demo module", info.ID, ok)
	}
}

func TestDemoConnectsAtOnce(t *testing.T) {
	cam, log := connected(t)
	if cam.Status().State != core.StateConnected {
		t.Fatalf("Status() = %s, want connected", cam.Status().State)
	}
	if !log.has(func(e core.Event) bool { return e.Status != nil && e.Status.State == core.StateConnected }) {
		t.Error("no connected event")
	}
}

func TestDemoAnswersLikeACamera(t *testing.T) {
	t.Run("the iris stays inside the lens", func(t *testing.T) {
		cam, _ := connected(t)
		set(t, cam, core.Iris, core.Number(1.4))
		if v := get(t, cam, core.Iris); v.Number != 2.8 {
			t.Errorf("iris = f/%g, want f/2.8", v.Number)
		}
	})
	t.Run("the lever position follows the f-stop", func(t *testing.T) {
		cam, _ := connected(t)
		set(t, cam, core.IrisNorm, core.Number(1))
		if v := get(t, cam, core.Iris); v.Number != 22 {
			t.Errorf("iris at the closed end = f/%g, want f/22", v.Number)
		}
	})
	t.Run("a shutter speed sets the angle", func(t *testing.T) {
		cam, _ := connected(t)
		set(t, cam, core.ShutterSpeed, core.Choice("1/50"))
		if v := get(t, cam, core.ShutterAngle); v.Number != 360 {
			t.Errorf("1/50 at 50 fps = %g°, want 360°", v.Number)
		}
	})
	t.Run("a shutter angle sets the speed", func(t *testing.T) {
		cam, _ := connected(t)
		set(t, cam, core.ShutterAngle, core.Number(90))
		if v := get(t, cam, core.ShutterSpeed); v.Choice != "1/200" {
			t.Errorf("90° at 50 fps = %s, want 1/200", v.Choice)
		}
	})
	t.Run("the master black is the lift luma", func(t *testing.T) {
		cam, _ := connected(t)
		set(t, cam, core.MasterBlack, core.Number(0.1))
		if v := get(t, cam, core.LiftY); v.Number != 0.1 {
			t.Errorf("lift Y = %g, want 0.1", v.Number)
		}
	})
	t.Run("auto white balance settles on 5600 K", func(t *testing.T) {
		cam, _ := connected(t)
		set(t, cam, core.AutoWhiteBalance, core.Trigger())
		if v := get(t, cam, core.WhiteBalance); v.Number != 5600 {
			t.Errorf("white balance = %g K, want 5600 K", v.Number)
		}
	})
}

func TestDemoZoomMovesTheFocalLength(t *testing.T) {
	cam, log := connected(t)
	set(t, cam, core.Zoom, core.Number(1))
	if l := cam.(core.LensReporter).Lens(); l.FocalLength != 70 {
		t.Errorf("focal length at full zoom = %g mm, want 70 mm", l.FocalLength)
	}
	if !log.has(func(e core.Event) bool { return e.Lens != nil && e.Lens.FocalLength == 70 }) {
		t.Error("a zoom move must announce the new focal length")
	}
}

func TestDemoHasAZoomOnlyWithAPowerZoom(t *testing.T) {
	for _, pz := range []bool{false, true} {
		cam, err := demo.Module{}.New(core.CameraConfig{Model: demo.ID, PowerZoom: pz})
		if err != nil {
			t.Fatal(err)
		}
		has := false
		for _, s := range cam.Controls() {
			has = has || s.ID == core.Zoom
		}
		if has != pz {
			t.Errorf("power zoom %t: zoom declared = %t", pz, has)
		}
	}
}

func TestDemoRejectsWritesWhileDisconnected(t *testing.T) {
	cam, _ := demo.Module{}.New(core.CameraConfig{Model: demo.ID})
	if err := cam.Set(context.Background(), core.Gain, core.Number(6)); !errors.Is(err, core.ErrNotConnected) {
		t.Errorf("Set before Connect = %v, want ErrNotConnected", err)
	}
}
