package plugins_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/cameratest"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/modbuild"
	"github.com/valentindoche/rcp-for-all-camera/internal/plugins"
)

// modulesDir holds the real modules, built once for the package.
var modulesDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "rcp-modules-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := modbuild.Build(context.Background(), "../../modules", dir, "demo", "bm-studio-6k-pro"); err != nil {
		fmt.Fprintln(os.Stderr, "build modules:", err)
		os.Exit(1)
	}
	if err := modbuild.Build(context.Background(), "testdata/modules", dir, "hostile-module"); err != nil {
		fmt.Fprintln(os.Stderr, "build the hostile test module:", err)
		os.Exit(1)
	}
	modulesDir = dir
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func openHost(t *testing.T) *plugins.Host {
	t.Helper()
	h, err := plugins.Open(context.Background(), plugins.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	return h
}

func loadModule(t *testing.T, id string) core.Module {
	t.Helper()
	m, err := openHost(t).LoadFile(context.Background(), filepath.Join(modulesDir, id+".wasm"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func demoModule(t *testing.T) core.Module { return loadModule(t, "demo") }

func TestLoadScansTheFolder(t *testing.T) {
	mods, problems := openHost(t).Load(context.Background(), modulesDir)
	if len(problems) != 0 || len(mods) != 3 || mods[0].Info().ID != "bm-studio-6k-pro" || mods[1].Info().ID != "demo" {
		t.Fatalf("Load = %d modules, problems %v; want bm-studio-6k-pro, demo and the hostile one", len(mods), problems)
	}
}

func TestLoadRefusesAFileNamedAfterAnotherModule(t *testing.T) {
	dir := t.TempDir()
	b, _ := os.ReadFile(filepath.Join(modulesDir, "demo.wasm"))
	_ = os.WriteFile(filepath.Join(dir, "sony-fx6.wasm"), b, 0o600)
	_ = os.WriteFile(filepath.Join(dir, "junk.wasm"), []byte("not wasm"), 0o600)
	mods, problems := openHost(t).Load(context.Background(), dir)
	if len(mods) != 0 || len(problems) != 2 {
		t.Errorf("Load = %d modules, %d problems; want 0 and 2", len(mods), len(problems))
	}
}

func TestLoadWithoutFolder(t *testing.T) {
	mods, problems := openHost(t).Load(context.Background(), filepath.Join(t.TempDir(), "none"))
	if len(mods) != 0 || len(problems) != 1 {
		t.Errorf("Load = %d modules, %v", len(mods), problems)
	}
}

func TestDemoPassesTheAppContract(t *testing.T) { cameratest.Module(t, demoModule(t)) }

func connectedDemo(t *testing.T) core.Camera {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(demoModule(t)); err != nil {
		t.Fatal(err)
	}
	cam, err := reg.New(core.CameraConfig{Model: "demo", PowerZoom: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cam.Close() })
	if err := cam.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	return cam
}

func TestDemoOnline(t *testing.T) { cameratest.Online(t, connectedDemo(t)) }

func TestDemoEventsAndLens(t *testing.T) {
	cam := connectedDemo(t)
	var mu sync.Mutex
	var got []core.Event
	cam.Subscribe(func(e core.Event) { mu.Lock(); got = append(got, e); mu.Unlock() })
	if err := cam.Set(context.Background(), core.Iris, core.Number(1.4)); err != nil {
		t.Fatal(err)
	}
	t.Run("the camera's answer arrives as an event", func(t *testing.T) {
		mu.Lock()
		defer mu.Unlock()
		for _, e := range got {
			if e.Control == core.Iris && e.Value == core.Number(2.8) {
				return
			}
		}
		t.Errorf("events %+v, want iris 2.8", got)
	})
	t.Run("the lens is known", func(t *testing.T) {
		if l := cam.(core.LensReporter).Lens(); l.IrisMin != 2.8 || l.FocalMax != 70 {
			t.Errorf("lens %+v", l)
		}
	})
}

func TestCustomBlockReachesTheApp(t *testing.T) {
	cam := connectedDemo(t)
	var block []core.ControlSpec
	for _, s := range cam.Controls() {
		if s.Block == "Demo" {
			block = append(block, s)
		}
	}
	if len(block) != 4 || block[0].Widget != core.WidgetSwitch || block[2].Kind != core.KindText {
		t.Fatalf("custom block %+v", block)
	}
	v, err := cam.Get(context.Background(), "demo.firmware")
	if err != nil || v.Text != "demo 0.3.0" {
		t.Errorf("Get(firmware) = %+v, %v", v, err)
	}
}

func TestSetIsCheckedBeforeTheModule(t *testing.T) {
	cam := connectedDemo(t)
	if err := cam.Set(context.Background(), core.Gain, core.Number(900)); !errors.Is(err, core.ErrInvalidValue) {
		t.Errorf("Set(gain 900) = %v, want ErrInvalidValue", err)
	}
}

func TestClosedCameraRefusesCalls(t *testing.T) {
	cam := connectedDemo(t)
	_ = cam.Close()
	if err := cam.Set(context.Background(), core.Gain, core.Number(6)); !errors.Is(err, core.ErrNotConnected) {
		t.Errorf("Set after Close = %v, want ErrNotConnected", err)
	}
	if st := cam.Status().State; st != core.StateDisconnected {
		t.Errorf("status after Close = %s", st)
	}
}

// The Blackmagic module over a real local network, against a fake camera.

func connectedBlackmagic(t *testing.T, f *plugins.FakeBlackmagic) core.Camera {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(loadModule(t, "bm-studio-6k-pro")); err != nil {
		t.Fatal(err)
	}
	host, port := f.HostPort()
	cam, err := reg.New(core.CameraConfig{Model: "bm-studio-6k-pro", Host: host, Port: port, PowerZoom: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cam.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := cam.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return cam
}

func TestBlackmagicPassesTheAppContract(t *testing.T) {
	cameratest.Module(t, loadModule(t, "bm-studio-6k-pro"))
}

func TestBlackmagicOnline(t *testing.T) {
	cameratest.Online(t, connectedBlackmagic(t, plugins.NewFakeBlackmagic(t)))
}

func TestBlackmagicWriteGoesOverTheNetwork(t *testing.T) {
	f := plugins.NewFakeBlackmagic(t)
	cam := connectedBlackmagic(t, f)
	if err := cam.Set(context.Background(), core.Gain, core.Number(6)); err != nil {
		t.Fatal(err)
	}
	if puts := f.Puts(); len(puts) != 1 || puts[0] != "/video/gain" {
		t.Errorf("PUTs %v, want /video/gain", puts)
	}
	waitFor(t, "gain echo", func() bool {
		v, err := cam.Get(context.Background(), core.Gain)
		return err == nil && v == core.Number(6)
	})
}

func TestBlackmagicReconnects(t *testing.T) {
	f := plugins.NewFakeBlackmagic(t)
	cam := connectedBlackmagic(t, f)
	f.DropSockets()
	waitFor(t, "offline", func() bool { return cam.Status().State == core.StateDisconnected })
	waitFor(t, "back online", func() bool { return cam.Status().State == core.StateConnected })
}

func TestUnreachableCameraIsOffline(t *testing.T) {
	reg := core.NewRegistry()
	_ = reg.Register(loadModule(t, "bm-studio-6k-pro"))
	cam, _ := reg.New(core.CameraConfig{Model: "bm-studio-6k-pro", Host: "127.0.0.1", Port: closedPort(t)})
	defer cam.Close()
	if err := cam.Connect(context.Background()); !errors.Is(err, core.ErrNotConnected) {
		t.Errorf("Connect = %v, want ErrNotConnected", err)
	}
	if st := cam.Status(); st.State != core.StateDisconnected || st.Detail == "" {
		t.Errorf("status %+v, want offline with a reason", st)
	}
}

// waitFor polls cond for up to 10 s: the module and the fake run on their
// own goroutines, over real sockets.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func closedPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close()
	return port
}
