package sony

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

func lookup(t *testing.T, id core.ModelID) (core.ModelInfo, map[core.ControlID]core.ControlSpec) {
	t.Helper()
	m := module(t, id)
	cam, err := m.New(core.CameraConfig{Model: id, Host: "192.0.2.1", Username: "u"})
	if err != nil {
		t.Fatal(err)
	}
	out := map[core.ControlID]core.ControlSpec{}
	for _, s := range cam.Controls() {
		out[s.ID] = s
	}
	return m.Info(), out
}

func TestModels(t *testing.T) {
	if n := len(Modules()); n != 5 {
		t.Fatalf("%d modules, want 5", n)
	}
	for _, m := range models() {
		if m.Brand != "Sony" || m.Support != core.SupportPlanned {
			t.Errorf("%s: %+v", m.ID, m)
		}
		if m.ID != "sony-fx3" && m.Auth != core.AuthRequired {
			t.Errorf("%s: auth %s, want required", m.ID, m.Auth)
		}
	}
}

func TestTransports(t *testing.T) {
	m, _ := lookup(t, "sony-fx6")
	if len(m.Transports) != 2 || m.Transports[0] != core.TransportWiFi || m.Transports[1] != core.TransportUSBEthernet {
		t.Errorf("fx6 transports %v", m.Transports)
	}
	m, _ = lookup(t, "sony-fx9")
	if len(m.Transports) != 1 || m.Transports[0] != core.TransportEthernet {
		t.Errorf("fx9 transports %v", m.Transports)
	}
	m, _ = lookup(t, "sony-fx3")
	if len(m.Transports) != 1 || m.Transports[0] != core.TransportWiFi {
		t.Errorf("fx3 transports %v", m.Transports)
	}
}

func TestPTZ(t *testing.T) {
	_, fr7 := lookup(t, "sony-fr7")
	for _, id := range []core.ControlID{core.Pan, core.Tilt, core.PresetRecall} {
		if _, ok := fr7[id]; !ok {
			t.Errorf("fr7 lacks %s", id)
		}
	}
	_, fx6 := lookup(t, "sony-fx6")
	if _, ok := fx6[core.Pan]; ok {
		t.Error("fx6 must not have pan")
	}
}

func TestND(t *testing.T) {
	for id, want := range map[core.ModelID]bool{"sony-fx3": false, "sony-fx6": true, "sony-fx9": true, "sony-fr7": true, "sony-generic": false} {
		if _, got := func() (core.ControlSpec, bool) { _, c := lookup(t, id); s, ok := c[core.ND]; return s, ok }(); got != want {
			t.Errorf("%s: ND present = %v, want %v", id, got, want)
		}
	}
}
