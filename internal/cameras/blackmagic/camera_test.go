package blackmagic

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// fakeCamera serves the REST API and the event socket from an in-memory state.
// A PUT merges into the property and pushes the change on every socket.
type fakeCamera struct {
	t   *testing.T
	srv *httptest.Server

	mu    sync.Mutex
	state map[string]map[string]any
	puts  []put
	fail  map[string]int // path -> HTTP status to answer PUTs with
	socks []*wsConn
}

type put struct {
	path string
	body map[string]any
}

func newFakeCamera(t *testing.T) *fakeCamera {
	f := &fakeCamera{t: t, fail: map[string]int{}, state: map[string]map[string]any{
		pathFormat:   {"frameRate": "50", "offSpeedEnabled": false, "offSpeedFrameRate": 50.0},
		pathIris:     {"apertureNumber": 320.0, "apertureStop": 4.0, "normalised": 0.25},
		pathFocus:    {"normalised": 0.0},
		pathZoom:     {"focalLength": 24.0, "normalised": 0.0},
		pathGain:     {"gain": 0.0},
		pathISO:      {"iso": 400.0},
		pathShutter:  {"continuousShutterAutoExposure": false, "shutterSpeed": 100.0},
		pathND:       {"stop": 0.0},
		pathWB:       {"whiteBalance": 4400.0},
		pathTint:     {"whiteBalanceTint": 10.0},
		pathRecord:   {"recording": false},
		pathLift:     {"red": 0.0, "green": 0.0, "blue": 0.0, "luma": 0.0},
		pathGamma:    {"red": 0.0, "green": 0.0, "blue": 0.0, "luma": 0.0},
		pathGainCC:   {"red": 1.0, "green": 1.0, "blue": 1.0, "luma": 1.0},
		pathOffset:   {"red": 0.0, "green": 0.0, "blue": 0.0, "luma": 0.0},
		pathContrast: {"pivot": 0.5, "adjust": 1.0},
		pathColor:    {"hue": 0.0, "saturation": 1.0},
		pathLuma:     {"lumaContribution": 1.0},
		pathIrisDesc: {"controllable": true, "apertureStop": map[string]any{"min": 2.8, "max": 22.0}},
		pathZoomDesc: {"controllable": true, "focalLength": map[string]any{"adjustable": true, "min": 24.0, "max": 70.0}},
		pathFocDesc:  {"controllable": true, "focusDistance": map[string]any{"adjustable": true, "min": 380.0, "max": 1e7}},
	}}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeCamera) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == eventPath {
		f.serveSocket(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, apiBase)
	switch r.Method {
	case http.MethodGet:
		f.mu.Lock()
		o, ok := f.state[path]
		b, _ := json.Marshal(o)
		f.mu.Unlock()
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(b)
	case http.MethodPut:
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.puts = append(f.puts, put{path, body})
		if code := f.fail[path]; code != 0 {
			f.mu.Unlock()
			w.WriteHeader(code)
			return
		}
		o, ok := f.state[path]
		if ok {
			o = maps.Clone(o)
			maps.Copy(o, body)
			f.state[path] = o
		}
		f.mu.Unlock()
		if ok {
			f.push(path, o)
		}
	}
}

func (f *fakeCamera) serveSocket(w http.ResponseWriter, r *http.Request) {
	conn, rw, err := w.(http.Hijacker).Hijack()
	if err != nil {
		f.t.Error(err)
		return
	}
	_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n" +
		"Sec-WebSocket-Accept: " + wsAccept(r.Header.Get("Sec-WebSocket-Key")) + "\r\n\r\n")
	_ = rw.Flush()
	ws := &wsConn{c: conn, br: rw.Reader}
	f.mu.Lock()
	f.socks = append(f.socks, ws)
	f.mu.Unlock()
	_ = ws.writeText([]byte(`{"data":{"action":"websocketOpened"},"type":"event"}`))
	for {
		msg, err := ws.readText()
		if err != nil {
			return
		}
		var req struct {
			ID   int `json:"id"`
			Data struct {
				Properties []string `json:"properties"`
			} `json:"data"`
		}
		_ = json.Unmarshal(msg, &req)
		vals := map[string]any{}
		f.mu.Lock()
		for _, p := range req.Data.Properties {
			if o, ok := f.state[p]; ok {
				vals[p] = o
			}
		}
		f.mu.Unlock()
		b, _ := json.Marshal(map[string]any{"type": "response", "id": req.ID, "data": map[string]any{
			"action": "subscribe", "properties": req.Data.Properties, "success": true, "values": vals,
		}})
		_ = ws.writeText(b)
	}
}

func (f *fakeCamera) push(path string, o map[string]any) {
	b, _ := json.Marshal(map[string]any{"type": "event", "data": map[string]any{
		"action": "propertyValueChanged", "property": path, "value": o,
	}})
	f.mu.Lock()
	socks := snapshot(f.socks)
	f.mu.Unlock()
	for _, s := range socks {
		_ = s.writeText(b)
	}
}

// dropSockets cuts every open socket, as a cable pull would.
func (f *fakeCamera) dropSockets() {
	f.mu.Lock()
	socks := snapshot(f.socks)
	f.socks = nil
	f.mu.Unlock()
	for _, s := range socks {
		_ = s.Close()
	}
}

func (f *fakeCamera) lastPut() put {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.puts) == 0 {
		return put{}
	}
	return f.puts[len(f.puts)-1]
}

func (f *fakeCamera) putCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.puts)
}

func snapshot[T any](s []T) []T { return append([]T(nil), s...) }

// connected builds a 6K Pro camera on the fake and connects it.
func connected(t *testing.T, f *fakeCamera) (*Camera, *recorder) {
	t.Helper()
	host, port, _ := net.SplitHostPort(strings.TrimPrefix(f.srv.URL, "http://"))
	p, _ := strconv.Atoi(port)
	cam := newCamera(studio6kProSpecs(), core.CameraConfig{Model: "blackmagic.studio6kpro", Host: host}, p)
	t.Cleanup(func() { _ = cam.Close() })
	rec := &recorder{}
	cam.Subscribe(rec.add)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cam.Connect(ctx); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if st := cam.Status().State; st != core.StateConnected {
		t.Fatalf("status %s after Connect", st)
	}
	return cam, rec
}

type recorder struct {
	mu     sync.Mutex
	events []core.Event
}

func (r *recorder) add(e core.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

// waitFor polls until cond holds on the recorded events.
func (r *recorder) waitFor(t *testing.T, what string, cond func([]core.Event) bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r.mu.Lock()
		ok := cond(r.events)
		r.mu.Unlock()
		if ok {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func hasValue(id core.ControlID, v core.Value) func([]core.Event) bool {
	return func(es []core.Event) bool {
		for _, e := range es {
			if e.Control == id && e.Value == v {
				return true
			}
		}
		return false
	}
}

func get(t *testing.T, cam *Camera, id core.ControlID) core.Value {
	t.Helper()
	v, err := cam.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	return v
}

func TestCameraInitialState(t *testing.T) {
	cam, _ := connected(t, newFakeCamera(t))
	for id, want := range map[core.ControlID]core.Value{
		core.Iris:         core.Number(4),
		core.ISO:          core.Choice("400"),
		core.ShutterSpeed: core.Choice("1/100"),
		core.ShutterAngle: core.Number(180), // 360 * 50 fps / 100
		core.ND:           core.Choice("0"),
		core.WhiteBalance: core.Number(4400),
		core.Tint:         core.Number(10),
		core.GainY:        core.Number(1),
		core.MasterBlack:  core.Number(0),
		ContrastPivot:     core.Number(0.5),
		core.Record:       core.Bool(false),
	} {
		if got := get(t, cam, id); got != want {
			t.Errorf("%s = %+v, want %+v", id, got, want)
		}
	}
}

func TestCameraSetSendsPutAndEchoes(t *testing.T) {
	f := newFakeCamera(t)
	cam, rec := connected(t, f)
	ctx := context.Background()
	if err := cam.Set(ctx, core.Iris, core.Number(5.6)); err != nil {
		t.Fatal(err)
	}
	if p := f.lastPut(); p.path != pathIris || p.body["apertureStop"] != 5.6 {
		t.Errorf("PUT %+v", p)
	}
	rec.waitFor(t, "iris event", hasValue(core.Iris, core.Number(5.6)))
	if got := get(t, cam, core.Iris); got != core.Number(5.6) {
		t.Errorf("cached iris %+v", got)
	}
}

func TestCameraCommandBodies(t *testing.T) {
	f := newFakeCamera(t)
	cam, _ := connected(t, f)
	ctx := context.Background()
	for _, tc := range []struct {
		id   core.ControlID
		v    core.Value
		path string
		key  string
		want any
	}{
		{core.ShutterAngle, core.Number(172.8), pathShutter, "shutterAngle", 17280.0},
		{core.ShutterSpeed, core.Choice("1/50"), pathShutter, "shutterSpeed", 50.0},
		{core.ISO, core.Choice("800"), pathISO, "iso", 800.0},
		{core.ND, core.Choice("4"), pathND, "stop", 4.0},
		{core.Gain, core.Number(6), pathGain, "gain", 6.0},
		{core.WhiteBalance, core.Number(5600), pathWB, "whiteBalance", 5600.0},
		{core.Tint, core.Number(-5), pathTint, "whiteBalanceTint", -5.0},
		{core.Focus, core.Number(0.3), pathFocus, "normalised", 0.3},
		{core.AutoIris, core.Trigger(), pathAutoExp, "mode", "OneShot"},
		{core.Record, core.Bool(true), pathRecord, "recording", true},
	} {
		if err := cam.Set(ctx, tc.id, tc.v); err != nil {
			t.Errorf("Set(%s): %v", tc.id, err)
			continue
		}
		if p := f.lastPut(); p.path != tc.path || p.body[tc.key] != tc.want {
			t.Errorf("Set(%s) sent %+v, want %s %s=%v", tc.id, p, tc.path, tc.key, tc.want)
		}
	}
}

func TestCameraColorWritesMerge(t *testing.T) {
	f := newFakeCamera(t)
	cam, _ := connected(t, f)
	ctx := context.Background()
	if err := cam.Set(ctx, core.LiftR, core.Number(0.1)); err != nil {
		t.Fatal(err)
	}
	if err := cam.Set(ctx, core.LiftG, core.Number(0.2)); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"red": 0.1, "green": 0.2, "blue": 0.0, "luma": 0.0}
	if p := f.lastPut(); p.path != pathLift || !maps.Equal(p.body, want) {
		t.Errorf("PUT %+v, want lift %v", p, want)
	}
	if err := cam.Set(ctx, core.MasterBlack, core.Number(-0.25)); err != nil {
		t.Fatal(err)
	}
	if p := f.lastPut(); p.body["luma"] != -0.25 || p.body["red"] != 0.1 {
		t.Errorf("master black PUT %+v", p)
	}
}

func TestCameraShutterAngleFromCamera(t *testing.T) {
	f := newFakeCamera(t)
	f.state[pathShutter] = map[string]any{"shutterAngle": 18000.0}
	cam, _ := connected(t, f)
	if got := get(t, cam, core.ShutterAngle); got != core.Number(180) {
		t.Errorf("angle %+v", got)
	}
	if got := get(t, cam, core.ShutterSpeed); got != core.Choice("1/100") {
		t.Errorf("speed %+v", got)
	}
}

func TestCameraRejectsBeforeIO(t *testing.T) {
	f := newFakeCamera(t)
	cam := newCamera(studio6kProSpecs(), core.CameraConfig{Host: "127.0.0.1"}, 1)
	ctx := context.Background()
	if err := cam.Set(ctx, core.Iris, core.Number(5.6)); !errors.Is(err, core.ErrNotConnected) {
		t.Errorf("Set before Connect = %v, want ErrNotConnected", err)
	}
	cam2, _ := connected(t, f)
	before := f.putCount()
	if err := cam2.Set(ctx, core.Iris, core.Number(40)); !errors.Is(err, core.ErrInvalidValue) {
		t.Errorf("out of range = %v", err)
	}
	if err := cam2.Set(ctx, core.ND, core.Choice("3")); !errors.Is(err, core.ErrInvalidValue) {
		t.Errorf("bad ND = %v", err)
	}
	if f.putCount() != before {
		t.Error("an invalid value reached the camera")
	}
}

func TestCameraWireErrorIsReported(t *testing.T) {
	f := newFakeCamera(t)
	cam, _ := connected(t, f)
	f.mu.Lock()
	f.fail[pathGain] = http.StatusBadRequest
	f.mu.Unlock()
	if err := cam.Set(context.Background(), core.Gain, core.Number(6)); err == nil {
		t.Error("a refused PUT must return an error")
	}
}

func TestCameraReconnects(t *testing.T) {
	f := newFakeCamera(t)
	cam, rec := connected(t, f)
	f.dropSockets()
	rec.waitFor(t, "offline status", func(es []core.Event) bool {
		for _, e := range es {
			if e.Status != nil && e.Status.State == core.StateDisconnected {
				return true
			}
		}
		return false
	})
	deadline := time.Now().Add(5 * time.Second)
	for cam.Status().State != core.StateConnected {
		if time.Now().After(deadline) {
			t.Fatalf("no reconnect, status %+v", cam.Status())
		}
		time.Sleep(20 * time.Millisecond)
	}
	if got := get(t, cam, core.Iris); got != core.Number(4) {
		t.Errorf("iris after reconnect %+v", got)
	}
}

func TestCameraClose(t *testing.T) {
	cam, _ := connected(t, newFakeCamera(t))
	if err := cam.Close(); err != nil {
		t.Fatal(err)
	}
	if st := cam.Status().State; st != core.StateDisconnected {
		t.Errorf("status after Close %s", st)
	}
	if err := cam.Connect(context.Background()); err == nil {
		t.Error("Connect after Close must fail")
	}
	_ = cam.Close() // idempotent
}

func TestCameraFirstAttemptFails(t *testing.T) {
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	port := l.Addr().(*net.TCPAddr).Port
	_ = l.Close() // nothing listens there now
	cam := newCamera(studio6kProSpecs(), core.CameraConfig{Host: "127.0.0.1"}, port)
	defer cam.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cam.Connect(ctx); err == nil {
		t.Fatal("Connect to a closed port must fail")
	}
	if st := cam.Status(); st.State != core.StateDisconnected || st.Detail == "" {
		t.Errorf("status %+v, want offline with a reason", st)
	}
	// retries keep it offline instead of flickering through "connecting"
	time.Sleep(1500 * time.Millisecond)
	if st := cam.Status().State; st != core.StateDisconnected {
		t.Errorf("status while retrying %s, want offline", st)
	}
}

func TestCameraThatRefusesUsIsInError(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler()) // answers, but no event socket
	defer srv.Close()
	port := srv.Listener.Addr().(*net.TCPAddr).Port
	cam := newCamera(studio6kProSpecs(), core.CameraConfig{Host: "127.0.0.1"}, port)
	defer cam.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := cam.Connect(ctx); err == nil {
		t.Fatal("Connect must fail")
	}
	if st := cam.Status().State; st != core.StateError {
		t.Errorf("status %s, want error", st)
	}
}

func TestUnreachable(t *testing.T) {
	if !unreachable(&net.OpError{Op: "dial", Err: errors.New("refused")}) || !unreachable(io.EOF) || !unreachable(errWSClosed) {
		t.Error("network failures are unreachable")
	}
	if unreachable(errors.New("camera refused the event subscription")) {
		t.Error("a refusal is an error, not offline")
	}
}

func TestCameraLens(t *testing.T) {
	f := newFakeCamera(t)
	cam, rec := connected(t, f)
	want := core.Lens{
		IrisMin: 2.8, IrisMax: 22, FocalLength: 24, FocalMin: 24, FocalMax: 70,
		FocusNear: 0.38, FocusFar: core.FocusInfinity,
	}
	if got := cam.Lens(); got != want {
		t.Errorf("lens = %+v, want %+v", got, want)
	}
	f.push(pathZoom, map[string]any{"focalLength": 35.0, "normalised": 0.2})
	rec.waitFor(t, "lens event", func(es []core.Event) bool {
		for _, e := range es {
			if e.Lens != nil && e.Lens.FocalLength == 35 {
				return true
			}
		}
		return false
	})
}

func TestFocusMetres(t *testing.T) {
	for mm, want := range map[float64]float64{0: 0, -1: 0, 450: 0.45, 2500: 2.5, 1e6: core.FocusInfinity} {
		if got := focusMetres(mm); got != want {
			t.Errorf("focusMetres(%v) = %v, want %v", mm, got, want)
		}
	}
}
