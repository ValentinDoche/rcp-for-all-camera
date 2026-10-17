package plugins_test

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/plugins"
)

// The sandbox tests: a hostile module (testdata/modules/hostile-module)
// tries one attack per control, and each test checks the host contains it.
// docs/vulnerabilities.html lists them.

// hostileServer is the "camera" the hostile module may reach: /redirect
// points elsewhere, /big answers more than the host accepts.
func hostileServer(t *testing.T) (string, int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/redirect":
			http.Redirect(w, r, "http://192.0.2.99/stolen", http.StatusFound)
		case "/big":
			_, _ = w.Write([]byte(strings.Repeat("x", 2<<20)))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	return host, p
}

// hostileCamera loads the hostile module in a host with a short call
// timeout and connects one camera.
func hostileCamera(t *testing.T) core.Camera {
	t.Helper()
	h, err := plugins.Open(context.Background(), plugins.Options{CallTimeout: 2 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Close(context.Background()) })
	m, err := h.LoadFile(context.Background(), filepath.Join(modulesDir, "hostile-module.wasm"))
	if err != nil {
		t.Fatal(err)
	}
	host, port := hostileServer(t)
	cam, err := m.New(core.CameraConfig{Model: "hostile-module", Host: host, Port: port})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cam.Close() })
	if err := cam.Connect(context.Background()); err != nil {
		t.Fatal(err)
	}
	return cam
}

func probe(t *testing.T, cam core.Camera, id core.ControlID) string {
	t.Helper()
	v, err := cam.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get(%s) = %v", id, err)
	}
	return v.Text
}

func TestSandboxHasNoFileSystem(t *testing.T) {
	if got := probe(t, hostileCamera(t), "hostile.readFile"); got != "no file" {
		t.Errorf("the module could %s", got)
	}
}

func TestSandboxHasNoEnvironment(t *testing.T) {
	if got := probe(t, hostileCamera(t), "hostile.env"); got != "0" {
		t.Errorf("the module sees %s environment variables, want 0", got)
	}
}

func TestSandboxNetworkOnlyReachesTheCamera(t *testing.T) {
	cam := hostileCamera(t)
	for _, id := range []core.ControlID{"hostile.httpOtherHost", "hostile.httpPath"} {
		t.Run(string(id), func(t *testing.T) {
			if got := probe(t, cam, id); got != "refused" {
				t.Errorf("request %s", got)
			}
		})
	}
}

func TestSandboxDoesNotFollowRedirects(t *testing.T) {
	if got := probe(t, hostileCamera(t), "hostile.redirect"); got != "302 allowed" {
		t.Errorf("redirect result %q, want the 302 itself, not followed", got)
	}
}

func TestSandboxRefusesOversizedAnswers(t *testing.T) {
	if got := probe(t, hostileCamera(t), "hostile.bigAnswer"); got != "refused" {
		t.Errorf("a 2 MiB answer was %s", got)
	}
}

func TestSandboxDropsMalformedEvents(t *testing.T) {
	cam := hostileCamera(t)
	var mu sync.Mutex
	var got []core.Event
	cam.Subscribe(func(e core.Event) { mu.Lock(); got = append(got, e); mu.Unlock() })
	for _, id := range []core.ControlID{"hostile.badEvent", "hostile.nanEvent", "hostile.undeclaredEvent"} {
		if err := cam.Set(context.Background(), id, core.Trigger()); err != nil {
			t.Errorf("Set(%s) = %v", id, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 0 {
		t.Errorf("malformed events reached the application: %+v", got)
	}
}

// A module that hangs, exhausts its memory or panics stops its own camera
// only: the call returns an error and the camera goes to the error state.
func TestSandboxStopsAMisbehavingModule(t *testing.T) {
	for _, id := range []core.ControlID{"hostile.loop", "hostile.memory", "hostile.panic"} {
		t.Run(string(id), func(t *testing.T) {
			cam := hostileCamera(t)
			start := time.Now()
			if err := cam.Set(context.Background(), id, core.Trigger()); err == nil {
				t.Fatal("Set returned no error")
			}
			if d := time.Since(start); d > 10*time.Second {
				t.Errorf("the module ran for %v", d)
			}
			if st := cam.Status().State; st != core.StateError {
				t.Errorf("status %s, want error", st)
			}
			if _, err := cam.Get(context.Background(), "hostile.env"); !errors.Is(err, core.ErrNotConnected) {
				t.Errorf("a stopped camera answered: %v", err)
			}
		})
	}
}

func TestOtherCamerasSurviveAMisbehavingModule(t *testing.T) {
	bad := hostileCamera(t)
	good := connectedDemo(t)
	_ = bad.Set(context.Background(), "hostile.panic", core.Trigger())
	if err := good.Set(context.Background(), core.Gain, core.Number(6)); err != nil {
		t.Errorf("the demo camera failed after another module crashed: %v", err)
	}
}
