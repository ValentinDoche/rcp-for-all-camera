package main

import (
	"errors"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod/rcpmodtest"
)

func TestContract(t *testing.T) { rcpmodtest.Module(t, module{}) }

func connected(t *testing.T) (rcpmod.Camera, *rcpmodtest.Host) {
	t.Helper()
	h := rcpmodtest.NewHost()
	return h.Connected(t, module{}, rcpmod.Config{Model: "demo", PowerZoom: true}), h
}

func TestOnline(t *testing.T) {
	cam, _ := connected(t)
	rcpmodtest.Online(t, cam)
}

func set(t *testing.T, cam rcpmod.Camera, id string, v rcpmod.Value) {
	t.Helper()
	if err := cam.Set(id, v); err != nil {
		t.Fatalf("Set(%s) = %v", id, err)
	}
}

func get(t *testing.T, cam rcpmod.Camera, id string) rcpmod.Value {
	t.Helper()
	v, err := cam.Get(id)
	if err != nil {
		t.Fatalf("Get(%s) = %v", id, err)
	}
	return v
}

func TestAnswersLikeACamera(t *testing.T) {
	for _, tc := range []struct {
		name  string
		id    string
		v     rcpmod.Value
		check string
		want  rcpmod.Value
	}{
		{"the iris stays inside the lens", rcpmod.Iris, rcpmod.Number(1.4), rcpmod.Iris, rcpmod.Number(2.8)},
		{"the lever's closed end is f/22", rcpmod.IrisNorm, rcpmod.Number(1), rcpmod.Iris, rcpmod.Number(22)},
		{"a shutter speed sets the angle", rcpmod.ShutterSpeed, rcpmod.Choice("1/50"), rcpmod.ShutterAngle, rcpmod.Number(360)},
		{"a shutter angle sets the speed", rcpmod.ShutterAngle, rcpmod.Number(90), rcpmod.ShutterSpeed, rcpmod.Choice("1/200")},
		{"the master black is the lift luma", rcpmod.MasterBlack, rcpmod.Number(0.1), rcpmod.LiftY, rcpmod.Number(0.1)},
		{"auto white balance settles on 5600 K", rcpmod.AutoWhiteBalance, rcpmod.Trigger(), rcpmod.WhiteBalance, rcpmod.Number(5600)},
		{"the custom button flips the tally lamp", tallyTest, rcpmod.Trigger(), tallyLamp, rcpmod.Bool(true)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cam, host := connected(t)
			set(t, cam, tc.id, tc.v)
			if got := get(t, cam, tc.check); got != tc.want {
				t.Errorf("%s = %+v, want %+v", tc.check, got, tc.want)
			}
			if got, ok := host.Last(tc.check); ok && got != tc.want {
				t.Errorf("event for %s = %+v, want %+v", tc.check, got, tc.want)
			}
		})
	}
}

func TestZoomMovesTheFocalLength(t *testing.T) {
	cam, host := connected(t)
	set(t, cam, rcpmod.Zoom, rcpmod.Number(1))
	if l, _ := host.Lens(); l.FocalLength != 70 {
		t.Errorf("focal length at full zoom = %g mm, want 70 mm", l.FocalLength)
	}
}

func TestZoomOnlyWithAPowerZoom(t *testing.T) {
	for _, pz := range []bool{false, true} {
		cam, _ := module{}.New(rcpmod.Config{Model: "demo", PowerZoom: pz}, rcpmodtest.NewHost())
		has := false
		for _, s := range cam.Controls() {
			has = has || s.ID == rcpmod.Zoom
		}
		if has != pz {
			t.Errorf("power zoom %t: zoom declared %t", pz, has)
		}
	}
}

func TestRejectsWritesWhileDisconnected(t *testing.T) {
	cam, _ := module{}.New(rcpmod.Config{Model: "demo"}, rcpmodtest.NewHost())
	if err := cam.Set(rcpmod.Gain, rcpmod.Number(6)); !errors.Is(err, rcpmod.ErrNotConnected) {
		t.Errorf("Set before Connect = %v, want ErrNotConnected", err)
	}
}
