package gui

import (
	"context"
	"io"
	"log/slog"
	"math"
	"sync"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/blackmagic"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/panasonic"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/sony"
	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// syncOptions makes the controller deterministic: UI and worker calls run
// inline. Writes still go through the real throttler goroutines.
func syncOptions() Options {
	return Options{
		UI: func(f func()) { f() },
		Go: func(f func()) { f() },
	}
}

type memStore struct {
	mu  sync.Mutex
	rig config.Rig
}

func (m *memStore) Load() (config.Rig, error) { m.mu.Lock(); defer m.mu.Unlock(); return m.rig, nil }
func (m *memStore) Save(r config.Rig) error   { m.mu.Lock(); defer m.mu.Unlock(); m.rig = r; return nil }

// liveCam wraps a real driver's camera (same declared controls) but behaves
// as a reachable camera holding values in memory, so the GUI can be exercised
// and photographed as if connected.
type liveCam struct {
	core.Camera
	mu     sync.Mutex
	state  core.ConnState
	values map[core.ControlID]core.Value
	setErr error
	sets   []setCall
}

type setCall struct {
	ID core.ControlID
	V  core.Value
}

func (c *liveCam) Status() core.Status { return core.Status{State: c.state} }
func (c *liveCam) Connect(context.Context) error {
	c.mu.Lock()
	c.state = core.StateConnected
	c.mu.Unlock()
	return nil
}
func (c *liveCam) Get(_ context.Context, id core.ControlID) (core.Value, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.values[id]
	if !ok {
		return core.Value{}, core.ErrNotConnected
	}
	return v, nil
}
func (c *liveCam) Set(_ context.Context, id core.ControlID, v core.Value) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sets = append(c.sets, setCall{id, v})
	if c.setErr != nil {
		return c.setErr
	}
	c.values[id] = v
	return nil
}
func (c *liveCam) setCalls() []setCall {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]setCall(nil), c.sets...)
}

// sample returns a plausible value for a spec, for photographs.
func sample(sp core.ControlSpec) core.Value {
	switch sp.ID {
	case core.Iris:
		return core.Number(math.Min(sp.Max, math.Max(sp.Min, sp.Min+0.62*(sp.Max-sp.Min))))
	case core.MasterBlack:
		return core.Number((sp.Min + sp.Max) / 2)
	case core.Gain:
		return core.Number(12)
	case core.ShutterAngle:
		return core.Number(180)
	case core.WhiteBalance:
		return core.Number(5600)
	case core.Zoom:
		return core.Number(0.35)
	case core.Focus:
		return core.Number(0.72)
	case core.LiftR, core.LiftG, core.LiftB:
		return core.Number(map[core.ControlID]float64{core.LiftR: 0.12, core.LiftG: -0.04, core.LiftB: -0.08}[sp.ID])
	case core.GammaR, core.GammaG, core.GammaB:
		return core.Number(map[core.ControlID]float64{core.GammaR: -0.2, core.GammaG: 0.1, core.GammaB: 0.3}[sp.ID])
	case core.GainR, core.GainG, core.GainB:
		return core.Number(map[core.ControlID]float64{core.GainR: 1.15, core.GainG: 1.0, core.GainB: 0.92}[sp.ID])
	case core.GainY:
		return core.Number(1)
	case core.Contrast, core.Saturation:
		return core.Number(1)
	case core.Tally:
		return core.Bool(false)
	}
	switch sp.Kind {
	case core.KindRange:
		return core.Number(widgetsNeutral(sp))
	case core.KindEnum:
		return core.Choice(sp.Options[len(sp.Options)/2].Value)
	case core.KindToggle:
		return core.Bool(false)
	}
	return core.Value{}
}

func widgetsNeutral(sp core.ControlSpec) float64 {
	v := 0.0
	if v < sp.Min {
		v = sp.Min
	}
	if v > sp.Max {
		v = sp.Max
	}
	return v
}

// liveDriver wraps a real driver so that its cameras become liveCams. The
// host's last octet picks the state: ...44 offline, ...45 error, else connected.
type liveDriver struct {
	core.Driver
	cams *[]*liveCam
}

func (d liveDriver) New(m core.ModelInfo, cfg core.CameraConfig) (core.Camera, error) {
	inner, err := d.Driver.New(m, cfg)
	if err != nil {
		return nil, err
	}
	lc := &liveCam{Camera: inner, values: map[core.ControlID]core.Value{}, state: core.StateConnected}
	switch {
	case len(cfg.Host) > 3 && cfg.Host[len(cfg.Host)-3:] == ".44":
		lc.state = core.StateDisconnected
	case len(cfg.Host) > 3 && cfg.Host[len(cfg.Host)-3:] == ".45":
		lc.state = core.StateError
	}
	for _, sp := range inner.Controls() {
		if sp.Kind != core.KindAction {
			lc.values[sp.ID] = sample(sp)
		}
	}
	if d.cams != nil {
		*d.cams = append(*d.cams, lc)
	}
	return lc, nil
}

// newLiveService builds a Service whose three real drivers are wrapped.
func newLiveService(t testing.TB) (*app.Service, *memStore, *[]*liveCam) {
	t.Helper()
	cams := &[]*liveCam{}
	reg := core.NewRegistry()
	for _, d := range []core.Driver{blackmagic.Driver{}, sony.Driver{}, panasonic.Driver{}} {
		if err := reg.Register(liveDriver{Driver: d, cams: cams}); err != nil {
			t.Fatal(err)
		}
	}
	st := &memStore{}
	svc, err := app.NewService(app.Options{
		Registry: reg, Store: st, Secrets: config.NewMemorySecrets(),
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, cams
}

// seedRig assigns the cameras of the screenshots.
func seedRig(t testing.TB, svc *app.Service) {
	t.Helper()
	pw := "secret"
	for i, in := range []app.SlotInput{
		{Model: "blackmagic.studio6kpro", Name: "Plateau", Host: "192.168.1.41"},
		{Model: "sony.fx6", Name: "Régie", Host: "192.168.1.42", Transport: core.TransportWiFi, Username: "admin", Password: &pw},
		{Model: "panasonic.awue150", Name: "Cyclo", Host: "192.168.1.44"},
	} {
		slot := i + 1
		if _, err := svc.AssignSlot(slot, in); err != nil {
			t.Fatal(err)
		}
	}
	if err := svc.SelectSlot(2); err != nil {
		t.Fatal(err)
	}
}

// newTestApp builds the GUI on a test driver app over the live service.
func newTestApp(t *testing.T, seed bool) (*App, *app.Service, *[]*liveCam) {
	t.Helper()
	fa := test.NewTempApp(t)
	svc, _, cams := newLiveService(t)
	if seed {
		seedRig(t, svc)
	}
	a := New(fa, svc, syncOptions())
	t.Cleanup(func() { a.ctl.WaitWrites() })
	return a, svc, cams
}

var _ fyne.Window
