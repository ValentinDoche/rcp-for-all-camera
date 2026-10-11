package demo

import (
	"context"
	"reflect"
	"slices"
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/blackmagic"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestContract(t *testing.T) { cameratest.Driver(t, Driver{}) }

func connected(t *testing.T) (*Camera, *[]core.Event) {
	t.Helper()
	c := newCamera()
	var mu sync.Mutex
	var events []core.Event
	c.Subscribe(func(e core.Event) { mu.Lock(); events = append(events, e); mu.Unlock() })
	if err := c.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	return c, &events
}

func get(t *testing.T, c *Camera, id core.ControlID) core.Value {
	t.Helper()
	v, err := c.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestDemoNeedsNoAddress(t *testing.T) {
	m := Driver{}.Models()[0]
	if !m.NoAddress {
		t.Fatal("the demo camera has no address")
	}
	if err := core.ValidateConfig(m, core.CameraConfig{Model: m.ID}); err != nil {
		t.Errorf("a config without host must be valid: %v", err)
	}
}

func TestDemoAnswersLikeACamera(t *testing.T) {
	c, events := connected(t)
	ctx := context.Background()
	if c.Status().State != core.StateConnected {
		t.Fatal("the demo connects at once")
	}
	_ = c.Set(ctx, core.Iris, core.Number(1.4))
	if v := get(t, c, core.Iris); v.Number != 2.8 {
		t.Errorf("iris stays inside the lens: %v", v.Number)
	}
	_ = c.Set(ctx, core.ShutterSpeed, core.Choice("1/50"))
	if v := get(t, c, core.ShutterAngle); v.Number != 360 {
		t.Errorf("1/50 at 50 fps is 360°, got %v", v.Number)
	}
	_ = c.Set(ctx, core.ShutterAngle, core.Number(90))
	if v := get(t, c, core.ShutterSpeed); v.Choice != "1/200" {
		t.Errorf("90° at 50 fps is 1/200, got %v", v.Choice)
	}
	_ = c.Set(ctx, core.MasterBlack, core.Number(0.1))
	if v := get(t, c, core.LiftY); v.Number != 0.1 {
		t.Errorf("master black is the lift luma: %v", v.Number)
	}
	_ = c.Set(ctx, core.AutoWhiteBalance, core.Trigger())
	if v := get(t, c, core.WhiteBalance); v.Number != 5600 {
		t.Errorf("AWB settles on 5600 K: %v", v.Number)
	}
	_ = c.Set(ctx, core.Zoom, core.Number(1))
	if l := c.Lens(); l.FocalLength != 70 {
		t.Errorf("full zoom is 70 mm: %v", l.FocalLength)
	}
	var lensEvent bool
	for _, e := range *events {
		lensEvent = lensEvent || (e.Lens != nil && e.Lens.FocalLength == 70)
	}
	if !lensEvent {
		t.Error("a zoom move announces the new focal length")
	}
}

func TestDemoLooksLikeAStudio6kPro(t *testing.T) {
	m := Driver{}.Models()[0]
	for _, pz := range []bool{false, true} {
		cam, err := Driver{}.New(m, core.CameraConfig{Model: m.ID, PowerZoom: pz})
		if err != nil {
			t.Fatal(err)
		}
		want := blackmagic.Studio6kProSpecs()
		if !pz {
			want = slices.DeleteFunc(want, func(s core.ControlSpec) bool { return s.ID == core.Zoom })
		}
		if got := cam.Controls(); !reflect.DeepEqual(got, want) {
			t.Errorf("power zoom %v: the demo must declare the Studio 6K Pro controls", pz)
		}
	}
}
