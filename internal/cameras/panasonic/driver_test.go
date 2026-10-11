package panasonic

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestContract(t *testing.T) { cameratest.Driver(t, Driver{}) }

func controls(t *testing.T, id core.ModelID) map[core.ControlID]core.ControlSpec {
	t.Helper()
	d := Driver{}
	for _, m := range d.Models() {
		if m.ID != id {
			continue
		}
		cam, err := d.New(m, core.CameraConfig{Model: id, Host: "192.0.2.1"})
		if err != nil {
			t.Fatal(err)
		}
		out := map[core.ControlID]core.ControlSpec{}
		for _, s := range cam.Controls() {
			out[s.ID] = s
		}
		return out
	}
	t.Fatalf("model %s not declared", id)
	return nil
}

func TestModels(t *testing.T) {
	d := Driver{}
	if d.Name() != "panasonic" || len(d.Models()) != 4 {
		t.Fatalf("name %q, %d models", d.Name(), len(d.Models()))
	}
	for _, m := range d.Models() {
		if m.Brand != "Panasonic" || m.Auth != core.AuthOptional || m.DefaultPort != 80 || m.Support != core.SupportPlanned {
			t.Errorf("%s: %+v", m.ID, m)
		}
	}
}

func TestPTZ(t *testing.T) {
	for _, id := range []core.ModelID{"panasonic.awue100", "panasonic.awue150", "panasonic.awue160"} {
		c := controls(t, id)
		for _, cid := range []core.ControlID{core.Pan, core.Tilt, core.PresetRecall, core.Zoom} {
			if _, ok := c[cid]; !ok {
				t.Errorf("%s lacks %s", id, cid)
			}
		}
		if s := c[core.Pan]; s.Min != -1 || s.Max != 1 {
			t.Errorf("%s pan range [%v,%v]", id, s.Min, s.Max)
		}
	}
	if _, ok := controls(t, "panasonic.generic")[core.Pan]; ok {
		t.Error("generic model must not expose pan")
	}
}
