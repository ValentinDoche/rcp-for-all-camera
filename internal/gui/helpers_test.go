package gui

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"math"
	"runtime"
	"strconv"
	"strings"
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

// syncOptions makes the controller deterministic: worker calls run inline.
// UI callbacks run inline on the test goroutine, like on Fyne's UI thread;
// callbacks posted from another goroutine (the write throttler) are queued
// and run by waitWrites, so UI state is never touched concurrently.
func syncOptions() Options {
	q := &uiQueue{gid: goroutineID()}
	testUI = q
	return Options{UI: q.post, Go: func(f func()) { f() }}
}

// testUI is the queue of the controller built last (GUI tests are not parallel).
var testUI *uiQueue

type uiQueue struct {
	gid uint64
	mu  sync.Mutex
	q   []func()
}

func (u *uiQueue) post(f func()) {
	if goroutineID() == u.gid {
		f()
		return
	}
	u.mu.Lock()
	u.q = append(u.q, f)
	u.mu.Unlock()
}

// drain runs the queued callbacks; it reports whether any ran.
func (u *uiQueue) drain() bool {
	u.mu.Lock()
	q := u.q
	u.q = nil
	u.mu.Unlock()
	for _, f := range q {
		f()
	}
	return len(q) > 0
}

// waitWrites waits for pending writes and applies their UI callbacks, until
// nothing is left (a callback may start another write).
func waitWrites(c *Controller) {
	for {
		c.WaitWrites()
		if testUI == nil || !testUI.drain() {
			return
		}
	}
}

// goroutineID parses the current goroutine id from the stack header.
func goroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	f := strings.Fields(string(buf[:n]))
	if len(f) < 2 {
		return 0
	}
	id, _ := strconv.ParseUint(f[1], 10, 64)
	return id
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
	refuse bool // Connect fails: the camera stays offline or in error
	sets   []setCall
}

type setCall struct {
	ID core.ControlID
	V  core.Value
}

func (c *liveCam) Status() core.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return core.Status{State: c.state}
}
func (c *liveCam) Connect(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.refuse {
		return errors.New("unreachable")
	}
	c.state = core.StateConnected
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
		lc.state, lc.refuse = core.StateDisconnected, true
	case len(cfg.Host) > 3 && cfg.Host[len(cfg.Host)-3:] == ".45":
		lc.state, lc.refuse = core.StateError, true
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

// fakeAuto is the start-with-Windows switch of the tests.
type fakeAuto struct {
	mu  sync.Mutex
	on  bool
	err error
}

func (f *fakeAuto) Enabled() (bool, error) { f.mu.Lock(); defer f.mu.Unlock(); return f.on, nil }
func (f *fakeAuto) SetEnabled(on bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.on = on
	return nil
}

// ifaceList is the set of local addresses the tests expose.
type ifaceList struct {
	mu  sync.Mutex
	nic []app.NetInterface
}

func (l *ifaceList) list() []app.NetInterface {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]app.NetInterface(nil), l.nic...)
}
func (l *ifaceList) set(n ...app.NetInterface) { l.mu.Lock(); l.nic = n; l.mu.Unlock() }

// testAuto and testIfaces are fresh for every newLiveService call.
var (
	testAuto   *fakeAuto
	testIfaces *ifaceList
)

// newLiveService builds a Service whose three real drivers are wrapped.
func newLiveService(t testing.TB) (*app.Service, *memStore, *[]*liveCam) {
	t.Helper()
	cams := &[]*liveCam{}
	testAuto = &fakeAuto{}
	testIfaces = &ifaceList{nic: []app.NetInterface{{Name: "Ethernet", IP: "192.168.1.110"}, {Name: "Wi-Fi", IP: "10.0.0.5"}}}
	reg := core.NewRegistry()
	for _, d := range []core.Driver{blackmagic.Driver{}, sony.Driver{}, panasonic.Driver{}} {
		if err := reg.Register(liveDriver{Driver: d, cams: cams}); err != nil {
			t.Fatal(err)
		}
	}
	st := &memStore{}
	svc, err := app.NewService(app.Options{
		Registry: reg, Store: st, Secrets: config.NewMemorySecrets(),
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		Interfaces: testIfaces.list, AutoStart: testAuto, LogDir: t.TempDir(),
		Go: func(f func()) { f() },
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
		{Model: "sony.fx6", Name: "Régie", Host: "192.168.1.42", Username: "admin", Password: &pw},
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
	t.Cleanup(func() { waitWrites(a.ctl) })
	return a, svc, cams
}

var _ fyne.Window
