package blackmagic

import (
	"errors"
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
	if d.Name() != "blackmagic" {
		t.Fatalf("name %q", d.Name())
	}
	if n := len(d.Models()); n != 6 {
		t.Fatalf("%d models, want 6", n)
	}
	for _, m := range d.Models() {
		want := core.SupportPlanned
		if m.ID == "blackmagic.studio6kpro" {
			want = core.SupportExperimental
		}
		if m.Brand != "Blackmagic Design" || m.Auth != core.AuthNone || m.DefaultPort != 80 || m.Support != want {
			t.Errorf("%s: unexpected declaration %+v", m.ID, m)
		}
	}
}

func TestOnlyStudio6kProHasProtocol(t *testing.T) {
	d := Driver{}
	for _, m := range d.Models() {
		cam, err := d.New(m, core.CameraConfig{Model: m.ID, Host: "192.0.2.1"})
		if err != nil {
			t.Fatal(err)
		}
		_, real := cam.(*Camera)
		if real != (m.ID == "blackmagic.studio6kpro") {
			t.Errorf("%s: real protocol = %v", m.ID, real)
		}
	}
}

func TestStudio6kProControls(t *testing.T) {
	c := controls(t, "blackmagic.studio6kpro")
	for _, id := range []core.ControlID{core.Iris, core.Gain, core.ISO, core.ND, core.MasterBlack, OffsetR, ContrastPivot, LumaContribution, core.Record} {
		if _, ok := c[id]; !ok {
			t.Errorf("studio6kpro lacks %s", id)
		}
	}
	// The camera's REST API has no endpoint for these.
	for _, id := range []core.ControlID{core.Tally, core.ColorBars, core.Detail} {
		if _, ok := c[id]; ok {
			t.Errorf("studio6kpro declares %s, which its API does not expose", id)
		}
	}
	// Every writable control must map to a REST command.
	for id, s := range c {
		if s.ReadOnly {
			continue
		}
		if _, err := commandFor(id, core.Value{Kind: s.Kind}); err != nil && !errors.Is(err, core.ErrInvalidValue) {
			t.Errorf("%s: no command: %v", id, err)
		}
	}
}

func TestTransports(t *testing.T) {
	for _, m := range (Driver{}).Models() {
		want := core.TransportUSBEthernet
		if m.ID == "blackmagic.studio4kplusg2" || m.ID == "blackmagic.studio4kprog2" || m.ID == "blackmagic.studio6kpro" {
			want = core.TransportEthernet
		}
		if len(m.Transports) != 1 || m.Transports[0] != want {
			t.Errorf("%s transports %v, want [%s]", m.ID, m.Transports, want)
		}
	}
}

func TestND(t *testing.T) {
	has := map[core.ModelID]bool{
		"blackmagic.pocket4k": false, "blackmagic.pocket6kg2": false, "blackmagic.pocket6kpro": true,
		"blackmagic.studio4kplusg2": false, "blackmagic.studio4kprog2": true, "blackmagic.studio6kpro": true,
	}
	for id, want := range has {
		if _, got := controls(t, id)[core.ND]; got != want {
			t.Errorf("%s: ND present = %v, want %v", id, got, want)
		}
	}
}

func TestCommonControls(t *testing.T) {
	c := controls(t, "blackmagic.studio4kprog2")
	for _, id := range []core.ControlID{core.Iris, core.Focus, core.WhiteBalance, core.LiftR, core.GainY, core.Record, core.ColorBars} {
		if _, ok := c[id]; !ok {
			t.Errorf("studio4kprog2 lacks %s", id)
		}
	}
	if !c[core.Tally].ReadOnly {
		t.Error("tally must be read-only")
	}
	if _, ok := c[core.Pan]; ok {
		t.Error("blackmagic cameras have no PTZ")
	}
}
