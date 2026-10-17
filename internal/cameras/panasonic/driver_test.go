package panasonic

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestContract(t *testing.T) {
	for _, m := range Modules() {
		t.Run(string(m.Info().ID), func(t *testing.T) { cameratest.Module(t, m) })
	}
}

// module returns the module with that id.
func module(t *testing.T, id core.ModelID) core.Module {
	t.Helper()
	for _, m := range Modules() {
		if m.Info().ID == id {
			return m
		}
	}
	t.Fatalf("model %s not declared", id)
	return nil
}

func controls(t *testing.T, id core.ModelID) map[core.ControlID]core.ControlSpec {
	t.Helper()
	cam, err := module(t, id).New(core.CameraConfig{Model: id, Host: "192.0.2.1"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[core.ControlID]core.ControlSpec{}
	for _, s := range cam.Controls() {
		out[s.ID] = s
	}
	return out
}

func TestModels(t *testing.T) {
	if n := len(Modules()); n != 4 {
		t.Fatalf("%d modules, want 4", n)
	}
	for _, m := range models() {
		if m.Brand != "Panasonic" || m.Auth != core.AuthOptional || m.DefaultPort != 80 || m.Support != core.SupportPlanned {
			t.Errorf("%s: %+v", m.ID, m)
		}
	}
}

func TestPTZ(t *testing.T) {
	for _, id := range []core.ModelID{"panasonic-aw-ue100", "panasonic-aw-ue150", "panasonic-aw-ue160"} {
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
	if _, ok := controls(t, "panasonic-generic")[core.Pan]; ok {
		t.Error("generic model must not expose pan")
	}
}
