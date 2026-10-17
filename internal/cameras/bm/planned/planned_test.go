package planned_test

import (
	"errors"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm/planned"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestContract(t *testing.T) {
	for _, m := range planned.Modules() {
		t.Run(string(m.Info().ID), func(t *testing.T) { cameratest.Module(t, m) })
	}
}

func controls(t *testing.T, id core.ModelID) map[core.ControlID]core.ControlSpec {
	t.Helper()
	for _, m := range planned.Modules() {
		if m.Info().ID != id {
			continue
		}
		cam, err := m.New(core.CameraConfig{Model: id, Host: "192.0.2.1"})
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

func TestDeclarations(t *testing.T) {
	for _, mod := range planned.Modules() {
		m := mod.Info()
		if m.Brand != "Blackmagic Design" || m.Auth != core.AuthNone || m.DefaultPort != 80 ||
			m.Support != core.SupportPlanned || !m.InterchangeableLens || len(m.Aliases) != 1 {
			t.Errorf("%s: unexpected declaration %+v", m.ID, m)
		}
	}
}

func TestTransports(t *testing.T) {
	for _, mod := range planned.Modules() {
		m := mod.Info()
		want := core.TransportUSBEthernet
		if m.ID == "bm-studio-4k-plus-g2" || m.ID == "bm-studio-4k-pro-g2" {
			want = core.TransportEthernet
		}
		if len(m.Transports) != 1 || m.Transports[0] != want {
			t.Errorf("%s transports %v, want [%s]", m.ID, m.Transports, want)
		}
	}
}

func TestND(t *testing.T) {
	for id, want := range map[core.ModelID]bool{
		"bm-pocket-4k": false, "bm-pocket-6k-g2": false, "bm-pocket-6k-pro": true,
		"bm-studio-4k-plus-g2": false, "bm-studio-4k-pro-g2": true,
	} {
		if _, got := controls(t, id)[core.ND]; got != want {
			t.Errorf("%s: ND present = %t, want %t", id, got, want)
		}
	}
}

func TestStudioControls(t *testing.T) {
	c := controls(t, "bm-studio-4k-pro-g2")
	for _, id := range []core.ControlID{core.Iris, core.Focus, core.WhiteBalance, core.LiftR, core.GainY, core.Record, core.ColorBars} {
		if _, ok := c[id]; !ok {
			t.Errorf("lacks %s", id)
		}
	}
	if !c[core.Tally].ReadOnly {
		t.Error("tally must be read-only")
	}
	if _, ok := c[core.Pan]; ok {
		t.Error("blackmagic cameras have no PTZ")
	}
}

func TestPlannedModelsHaveNoProtocolYet(t *testing.T) {
	for _, m := range planned.Modules() {
		cam, _ := m.New(core.CameraConfig{Model: m.Info().ID, Host: "192.0.2.1"})
		if err := cam.Connect(t.Context()); !errors.Is(err, core.ErrNotImplemented) {
			t.Errorf("%s: Connect = %v, want ErrNotImplemented", m.Info().ID, err)
		}
	}
}
