package studio6kpro_test

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm/studio6kpro"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestContract(t *testing.T) { cameratest.Module(t, studio6kpro.Module{}) }

func controls(t *testing.T, powerZoom bool) map[core.ControlID]core.ControlSpec {
	t.Helper()
	cam, err := studio6kpro.Module{}.New(core.CameraConfig{Model: studio6kpro.ID, Host: "192.0.2.1", PowerZoom: powerZoom})
	if err != nil {
		t.Fatal(err)
	}
	out := map[core.ControlID]core.ControlSpec{}
	for _, s := range cam.Controls() {
		out[s.ID] = s
	}
	return out
}

func TestDeclaration(t *testing.T) {
	m := studio6kpro.Module{}.Info()
	if m.Brand != "Blackmagic Design" || m.Auth != core.AuthNone || m.DefaultPort != 80 ||
		m.Support != core.SupportExperimental || !m.InterchangeableLens {
		t.Errorf("unexpected declaration %+v", m)
	}
	if len(m.Transports) != 1 || m.Transports[0] != core.TransportEthernet {
		t.Errorf("transports %v, want [ethernet]", m.Transports)
	}
}

func TestUsesTheBlackmagicBase(t *testing.T) {
	cam, _ := studio6kpro.Module{}.New(core.CameraConfig{Model: studio6kpro.ID, Host: "192.0.2.1"})
	if _, ok := cam.(*bm.Camera); !ok {
		t.Errorf("camera is %T, want *bm.Camera", cam)
	}
}

func TestControls(t *testing.T) {
	c := controls(t, true)
	for _, id := range []core.ControlID{core.Iris, core.Gain, core.ISO, core.ND, core.MasterBlack,
		bm.OffsetR, bm.ContrastPivot, bm.LumaContribution, core.Record} {
		if _, ok := c[id]; !ok {
			t.Errorf("lacks %s", id)
		}
	}
}

func TestNoControlItsAPILacks(t *testing.T) {
	c := controls(t, true)
	for _, id := range []core.ControlID{core.Tally, core.ColorBars, core.Detail} {
		if _, ok := c[id]; ok {
			t.Errorf("declares %s, which its API does not expose", id)
		}
	}
}

func TestZoomNeedsAPowerZoom(t *testing.T) {
	for _, pz := range []bool{false, true} {
		if _, has := controls(t, pz)[core.Zoom]; has != pz {
			t.Errorf("power zoom %t: zoom declared %t", pz, has)
		}
	}
}

func TestIrisInFStops(t *testing.T) {
	s := controls(t, false)[core.Iris]
	if s.Unit != "f" || s.Min != 1 || s.Max != 22 {
		t.Errorf("iris spec %+v", s)
	}
}
