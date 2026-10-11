package blackmagic

import (
	"context"
	"math"
	"os"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Hardware tests against a real Studio Camera 6K Pro. They are skipped unless
// BMD_CAMERA holds the camera's address:
//
//	BMD_CAMERA=192.168.10.11 go test ./internal/cameras/blackmagic -run Live -v
//
// TestLiveRead only reads. TestLiveWrite also needs BMD_CAMERA_WRITE=1: it
// changes image settings (never recording) and puts each one back.

func liveCamera(t *testing.T) *Camera {
	t.Helper()
	host := os.Getenv("BMD_CAMERA")
	if host == "" {
		t.Skip("BMD_CAMERA not set")
	}
	cam := newCamera(studio6kProSpecs(), core.CameraConfig{Model: "blackmagic.studio6kpro", Host: host}, 80)
	t.Cleanup(func() { _ = cam.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cam.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return cam
}

func TestLiveRead(t *testing.T) {
	cam := liveCamera(t)
	for _, s := range cam.Controls() {
		if s.Kind == core.KindAction {
			continue
		}
		v, err := cam.Get(context.Background(), s.ID)
		if err != nil {
			t.Errorf("%s: %v", s.ID, err)
			continue
		}
		t.Logf("%-28s %+v", s.ID, v)
	}
}

// TestLiveWrite moves each control a little, waits for the camera to report
// the new value on the event socket, then restores the original value.
func TestLiveWrite(t *testing.T) {
	if os.Getenv("BMD_CAMERA_WRITE") != "1" {
		t.Skip("BMD_CAMERA_WRITE=1 not set")
	}
	cam := liveCamera(t)
	ctx := context.Background()
	for _, tc := range []struct {
		id  core.ControlID
		v   core.Value
		tol float64 // accepted gap: the iris snaps to the lens' stops
	}{
		{core.Iris, core.Number(5.6), 1},
		{core.Focus, core.Number(0.2), 0},
		{core.Gain, core.Number(6), 0},
		{core.ISO, core.Choice("800"), 0},
		{core.ShutterSpeed, core.Choice("1/50"), 0},
		{core.ShutterAngle, core.Number(90), 0},
		{core.ND, core.Choice("2"), 0},
		{core.WhiteBalance, core.Number(5600), 0},
		{core.Tint, core.Number(0), 0},
		{core.LiftR, core.Number(0.1), 0},
		{core.MasterBlack, core.Number(-0.05), 0},
		{core.GammaB, core.Number(0.1), 0},
		{core.GainG, core.Number(1.1), 0},
		{OffsetY, core.Number(0.1), 0},
		{core.Contrast, core.Number(1.2), 0},
		{core.Saturation, core.Number(1.2), 0},
		{core.Hue, core.Number(0.1), 0},
	} {
		t.Run(string(tc.id), func(t *testing.T) {
			orig, err := cam.Get(ctx, tc.id)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			if err := cam.Set(ctx, tc.id, tc.v); err != nil {
				t.Fatalf("Set(%+v): %v", tc.v, err)
			}
			if got := waitValue(cam, tc.id, tc.v, tc.tol); !near(got, tc.v, tc.tol) {
				t.Errorf("camera reports %+v after Set(%+v)", got, tc.v)
			}
			if err := cam.Set(ctx, tc.id, orig); err != nil {
				t.Errorf("restore %+v: %v", orig, err)
			}
			if back := waitValue(cam, tc.id, orig, tc.tol); !near(back, orig, tc.tol) {
				t.Errorf("restored to %+v, want %+v", back, orig)
			}
		})
	}
}

func near(a, b core.Value, tol float64) bool {
	if a.Kind == core.KindRange && b.Kind == core.KindRange {
		return math.Abs(a.Number-b.Number) <= tol
	}
	return a == b
}

// waitValue polls the cached value for up to 2 s and returns the last one.
func waitValue(cam *Camera, id core.ControlID, want core.Value, tol float64) core.Value {
	var v core.Value
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(20 * time.Millisecond) {
		v, _ = cam.Get(context.Background(), id)
		if near(v, want, tol) {
			break
		}
	}
	return v
}
