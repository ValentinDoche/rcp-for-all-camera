package main

import (
	"encoding/json"
	"errors"
	"maps"
	"strings"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod/rcpmodtest"
)

// fakeCamera plays a Studio Camera 6K Pro on a test host: GETs read its
// state, PUTs merge into it and come back as socket events, and it answers
// the subscriptions like the real camera (one unknown property refuses the
// whole request).
type fakeCamera struct {
	host  *rcpmodtest.Host
	state map[string]map[string]any
	puts  []put
	fail  map[string]int // path -> HTTP status to answer PUTs with
}

type put struct {
	path string
	body map[string]any
}

func newFake() *fakeCamera {
	f := &fakeCamera{host: rcpmodtest.NewHost(), fail: map[string]int{}, state: map[string]map[string]any{
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
	f.host.HTTPHandler = f.serve
	return f
}

func (f *fakeCamera) serve(method, path string, body []byte) (int, []byte) {
	path = strings.TrimPrefix(path, apiBase)
	switch method {
	case "GET":
		o, ok := f.state[path]
		if !ok {
			return 404, nil
		}
		b, _ := json.Marshal(o)
		return 200, b
	case "PUT":
		var b map[string]any
		_ = json.Unmarshal(body, &b)
		f.puts = append(f.puts, put{path, b})
		if code := f.fail[path]; code != 0 {
			return code, nil
		}
		if o, ok := f.state[path]; ok {
			o = maps.Clone(o)
			maps.Copy(o, b)
			f.state[path] = o
			f.push(path, o)
		}
		return 200, nil
	}
	return 405, nil
}

// answer replies to the subscriptions the module sent on its socket.
func (f *fakeCamera) answer() {
	s := f.host.Socket(eventPath)
	for _, raw := range s.Sent {
		var req struct {
			ID   int `json:"id"`
			Data struct {
				Properties []string `json:"properties"`
			} `json:"data"`
		}
		_ = json.Unmarshal(raw, &req)
		vals, ok := map[string]any{}, true
		for _, p := range req.Data.Properties {
			if o, known := f.state[p]; known {
				vals[p] = o
			} else {
				ok = false
			}
		}
		if !ok {
			vals = map[string]any{}
		}
		b, _ := json.Marshal(map[string]any{"type": "response", "id": req.ID, "data": map[string]any{
			"action": "subscribe", "properties": req.Data.Properties, "success": ok, "values": vals,
		}})
		s.Push(b)
	}
	s.Sent = nil
}

func (f *fakeCamera) push(path string, o map[string]any) {
	if s := f.host.Socket(eventPath); s != nil {
		b, _ := json.Marshal(map[string]any{"type": "event", "data": map[string]any{
			"action": "propertyValueChanged", "property": path, "value": o,
		}})
		s.Push(b)
	}
}

func (f *fakeCamera) lastPut() put {
	if len(f.puts) == 0 {
		return put{}
	}
	return f.puts[len(f.puts)-1]
}

// connect builds a camera with a power zoom on the fake and connects it.
func (f *fakeCamera) connect(t *testing.T) rcpmod.Camera {
	t.Helper()
	cam, err := module{}.New(rcpmod.Config{Model: "bm-studio-6k-pro", Host: "192.0.2.1", Port: 80, PowerZoom: true}, f.host)
	if err != nil {
		t.Fatal(err)
	}
	_ = cam.Connect()
	f.answer()
	if st := f.host.Status(); st.State != rcpmod.StateConnected {
		t.Fatalf("status %+v, want connected", st)
	}
	return cam
}

func get(t *testing.T, cam rcpmod.Camera, id string) rcpmod.Value {
	t.Helper()
	v, err := cam.Get(id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	return v
}

func TestContract(t *testing.T) { rcpmodtest.Module(t, module{}) }

func TestOnline(t *testing.T) { rcpmodtest.Online(t, newFake().connect(t)) }

func TestInitialState(t *testing.T) {
	cam := newFake().connect(t)
	for id, want := range map[string]rcpmod.Value{
		rcpmod.Iris:         rcpmod.Number(4),
		rcpmod.ISO:          rcpmod.Choice("400"),
		rcpmod.ShutterSpeed: rcpmod.Choice("1/100"),
		rcpmod.ShutterAngle: rcpmod.Number(180), // 360 * 50 fps / 100
		rcpmod.ND:           rcpmod.Choice("0"),
		rcpmod.WhiteBalance: rcpmod.Number(4400),
		rcpmod.Tint:         rcpmod.Number(10),
		rcpmod.GainY:        rcpmod.Number(1),
		rcpmod.MasterBlack:  rcpmod.Number(0),
		rcpmod.IrisNorm:     rcpmod.Number(0.25),
		contrastPivot:       rcpmod.Number(0.5),
		rcpmod.Record:       rcpmod.Bool(false),
	} {
		if got := get(t, cam, id); got != want {
			t.Errorf("%s = %+v, want %+v", id, got, want)
		}
	}
}

func TestSetSendsAPutAndTheEchoUpdatesTheValue(t *testing.T) {
	f := newFake()
	cam := f.connect(t)
	if err := cam.Set(rcpmod.Iris, rcpmod.Number(5.6)); err != nil {
		t.Fatal(err)
	}
	if p := f.lastPut(); p.path != pathIris || p.body["apertureStop"] != 5.6 {
		t.Errorf("PUT %+v", p)
	}
	if got := get(t, cam, rcpmod.Iris); got != rcpmod.Number(5.6) {
		t.Errorf("iris after the echo = %+v", got)
	}
}

func TestCommandBodies(t *testing.T) {
	f := newFake()
	cam := f.connect(t)
	for _, tc := range []struct {
		id   string
		v    rcpmod.Value
		path string
		key  string
		want any
	}{
		{rcpmod.ShutterAngle, rcpmod.Number(172.8), pathShutter, "shutterAngle", 17280.0},
		{rcpmod.ShutterSpeed, rcpmod.Choice("1/50"), pathShutter, "shutterSpeed", 50.0},
		{rcpmod.ISO, rcpmod.Choice("800"), pathISO, "iso", 800.0},
		{rcpmod.ND, rcpmod.Choice("4"), pathND, "stop", 4.0},
		{rcpmod.Gain, rcpmod.Number(6), pathGain, "gain", 6.0},
		{rcpmod.WhiteBalance, rcpmod.Number(5600), pathWB, "whiteBalance", 5600.0},
		{rcpmod.Tint, rcpmod.Number(-5), pathTint, "whiteBalanceTint", -5.0},
		{rcpmod.Focus, rcpmod.Number(0.3), pathFocus, "normalised", 0.3},
		{rcpmod.IrisNorm, rcpmod.Number(0.4), pathIris, "normalised", 0.4},
		{rcpmod.AutoIris, rcpmod.Trigger(), pathAutoExp, "mode", "OneShot"},
		{rcpmod.Record, rcpmod.Bool(true), pathRecord, "recording", true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			if err := cam.Set(tc.id, tc.v); err != nil {
				t.Fatal(err)
			}
			if p := f.lastPut(); p.path != tc.path || p.body[tc.key] != tc.want {
				t.Errorf("sent %+v, want %s %s=%v", p, tc.path, tc.key, tc.want)
			}
		})
	}
}

func TestEveryWritableControlHasACommand(t *testing.T) {
	for _, s := range specs(true) {
		if s.ReadOnly {
			continue
		}
		if _, err := commandFor(s.ID, rcpmod.Value{Kind: s.Kind}); err != nil && !errors.Is(err, rcpmod.ErrInvalidValue) {
			t.Errorf("%s: no command: %v", s.ID, err)
		}
	}
}

func TestColorWritesMerge(t *testing.T) {
	f := newFake()
	cam := f.connect(t)
	_ = cam.Set(rcpmod.LiftR, rcpmod.Number(0.1))
	_ = cam.Set(rcpmod.LiftG, rcpmod.Number(0.2))
	t.Run("every key of the property is sent", func(t *testing.T) {
		want := map[string]any{"red": 0.1, "green": 0.2, "blue": 0.0, "luma": 0.0}
		if p := f.lastPut(); p.path != pathLift || !maps.Equal(p.body, want) {
			t.Errorf("PUT %+v, want lift %v", p, want)
		}
	})
	t.Run("the master black is the lift luma", func(t *testing.T) {
		_ = cam.Set(rcpmod.MasterBlack, rcpmod.Number(-0.25))
		if p := f.lastPut(); p.body["luma"] != -0.25 || p.body["red"] != 0.1 {
			t.Errorf("PUT %+v", p)
		}
	})
}

func TestShutterAngleFromTheCamera(t *testing.T) {
	f := newFake()
	f.state[pathShutter] = map[string]any{"shutterAngle": 18000.0}
	cam := f.connect(t)
	if got := get(t, cam, rcpmod.ShutterSpeed); got != rcpmod.Choice("1/100") {
		t.Errorf("speed %+v, want 1/100 at 50 fps and 180°", got)
	}
}

func TestInvalidValuesNeverReachTheCamera(t *testing.T) {
	f := newFake()
	cam := f.connect(t)
	_ = cam.Set(rcpmod.Iris, rcpmod.Number(40))
	_ = cam.Set(rcpmod.ND, rcpmod.Choice("3"))
	if len(f.puts) != 0 {
		t.Errorf("%d PUTs sent for invalid values", len(f.puts))
	}
}

func TestRefusedWriteIsAnError(t *testing.T) {
	f := newFake()
	cam := f.connect(t)
	f.fail[pathGain] = 400
	if err := cam.Set(rcpmod.Gain, rcpmod.Number(6)); err == nil {
		t.Error("a refused PUT must return an error")
	}
}

func TestReconnectsAfterADrop(t *testing.T) {
	f := newFake()
	cam := f.connect(t)
	f.host.Socket(eventPath).Drop(errors.New("cable pulled"))
	t.Run("offline at once, values forgotten", func(t *testing.T) {
		if st := f.host.Status(); st.State != rcpmod.StateDisconnected {
			t.Errorf("status %+v", st)
		}
		if _, err := cam.Get(rcpmod.Iris); !errors.Is(err, rcpmod.ErrNotConnected) {
			t.Errorf("Get while offline = %v", err)
		}
	})
	t.Run("back after the retry delay", func(t *testing.T) {
		f.host.Advance(time.Second)
		f.answer()
		if st := f.host.Status(); st.State != rcpmod.StateConnected {
			t.Errorf("status %+v, want connected", st)
		}
	})
}

func TestFirstAttemptFailsThenRetries(t *testing.T) {
	f := newFake()
	f.host.WSHandler = func(string) error { return errors.New("dial tcp: connection refused") }
	cam, _ := module{}.New(rcpmod.Config{Model: "bm-studio-6k-pro", Host: "192.0.2.1", Port: 80}, f.host)
	_ = cam.Connect()
	if st := f.host.Status(); st.State != rcpmod.StateDisconnected || st.Detail == "" {
		t.Errorf("status %+v, want offline with a reason", st)
	}
	if f.host.Pending() != 1 {
		t.Errorf("%d timers armed, want one retry", f.host.Pending())
	}
	before := len(f.host.Events())
	f.host.Advance(time.Second)
	for _, e := range f.host.Events()[before:] {
		if e.Status != nil && e.Status.State == rcpmod.StateConnecting {
			t.Error("a retry of an unreachable camera flickers through connecting")
		}
	}
	cam.Close()
	if f.host.Pending() != 0 {
		t.Error("Close must cancel the retry")
	}
}

func TestAServerThatIsNotTheCameraIsAnError(t *testing.T) {
	f := newFake()
	f.host.WSHandler = func(string) error { return errors.New("websocket upgrade refused: HTTP 404") }
	cam, _ := module{}.New(rcpmod.Config{Model: "bm-studio-6k-pro", Host: "192.0.2.1", Port: 80}, f.host)
	_ = cam.Connect()
	if st := f.host.Status(); st.State != rcpmod.StateError {
		t.Errorf("status %+v, want error", st)
	}
}

func TestLens(t *testing.T) {
	f := newFake()
	f.connect(t)
	want := rcpmod.Lens{IrisMin: 2.8, IrisMax: 22, FocalLength: 24, FocalMin: 24, FocalMax: 70, FocusNear: 0.38, FocusFar: rcpmod.FocusInfinity}
	if got, _ := f.host.Lens(); got != want {
		t.Errorf("lens %+v, want %+v", got, want)
	}
	f.push(pathZoom, map[string]any{"focalLength": 35.0, "normalised": 0.2})
	if got, _ := f.host.Lens(); got.FocalLength != 35 {
		t.Errorf("focal length after a zoom = %g", got.FocalLength)
	}
}

// Some firmwares do not know the lens descriptions: the camera connects,
// with no lens limits, and learns the iris ends from the iris reports.
func TestWithoutLensDescriptions(t *testing.T) {
	f := newFake()
	for _, p := range lensWatched {
		delete(f.state, p)
	}
	f.connect(t)
	f.push(pathIris, map[string]any{"apertureStop": 2.8, "normalised": 0.0})
	f.push(pathIris, map[string]any{"apertureStop": 16.0, "normalised": 1.0})
	if l, _ := f.host.Lens(); l.IrisMin != 2.8 || l.IrisMax != 16 {
		t.Errorf("learned lens %+v, want f/2.8 to f/16", l)
	}
	f.push(pathIris, map[string]any{"apertureStop": 2.0, "normalised": 0.1}) // a wider lens
	if l, _ := f.host.Lens(); l.IrisMin != 0 {
		t.Errorf("open end kept after a wider stop: %+v", l)
	}
}

func TestDescribedIrisLimitsAreKept(t *testing.T) {
	f := newFake()
	f.connect(t)
	f.push(pathIris, map[string]any{"apertureStop": 4.0, "normalised": 0.0})
	if l, _ := f.host.Lens(); l.IrisMin != 2.8 {
		t.Errorf("described open end replaced: %+v", l)
	}
}

func TestFocusMetres(t *testing.T) {
	for mm, want := range map[float64]float64{0: 0, -1: 0, 450: 0.45, 2500: 2.5, 1e6: rcpmod.FocusInfinity} {
		if got := focusMetres(mm); got != want {
			t.Errorf("focusMetres(%v) = %v, want %v", mm, got, want)
		}
	}
}
