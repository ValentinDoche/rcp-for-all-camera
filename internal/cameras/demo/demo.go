// Package demo is a simulated camera with no network address. It declares
// every standard control and answers like a real one (values, events, lens,
// auto functions), so the whole GUI and the panel can be tried without a
// camera.
package demo

import (
	"context"
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// ModelID is the demo camera's model.
const ModelID core.ModelID = "demo.camera"

// fps is the simulated frame rate, for the shutter speed and angle.
const fps = 50

// The simulated lens: a 24-70 mm f/2.8 zoom focusing from 0.38 m.
var lens = core.Lens{IrisMin: 2.8, IrisMax: 22, FocalMin: 24, FocalMax: 70, FocusNear: 0.38, FocusFar: core.FocusInfinity}

// Driver implements core.Driver for the demo camera.
type Driver struct{}

var _ core.Driver = Driver{}

func (Driver) Name() string { return "demo" }

func (Driver) Models() []core.ModelInfo {
	return []core.ModelInfo{{
		ID: ModelID, Brand: "Démo", Name: "Caméra de démonstration", Driver: "demo",
		Transports: []core.Transport{core.TransportEthernet}, Auth: core.AuthNone,
		Support: core.SupportSupported, NoAddress: true,
		Notes: "Caméra simulée, sans adresse : tous les réglages répondent, pour essayer l'interface et le panneau sans caméra.",
	}}
}

// New builds a demo camera; it does no I/O.
func (Driver) New(m core.ModelInfo, _ core.CameraConfig) (core.Camera, error) {
	if m.ID != ModelID {
		return nil, fmt.Errorf("%q: %w", m.ID, core.ErrUnknownModel)
	}
	return newCamera(), nil
}

func specs() []core.ControlSpec {
	s := []core.ControlSpec{
		speckit.Range(core.Iris, 1, 22, 0.1, "f"),
		speckit.Action(core.AutoIris),
		speckit.Range(core.Focus, 0, 1, 0.001, ""),
		speckit.Action(core.AutoFocus),
		speckit.Range(core.Zoom, 0, 1, 0.001, ""),
		speckit.Range(core.Gain, -12, 36, 2, "dB"),
		speckit.Enum(core.ShutterSpeed, speckit.Opts("1/25", "1/50", "1/60", "1/100", "1/120", "1/250", "1/500", "1/1000")),
		speckit.Range(core.ShutterAngle, 11.2, 360, 0.1, "°"),
		speckit.Enum(core.ND, speckit.Labeled("0", "Clear", "2", "ND 0.6 (2 stops)", "4", "ND 1.2 (4 stops)", "6", "ND 1.8 (6 stops)")),
		speckit.Range(core.WhiteBalance, 2500, 10000, 50, "K"),
		speckit.Range(core.Tint, -50, 50, 1, ""),
		speckit.Action(core.AutoWhiteBalance),
	}
	for _, id := range []core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY, core.MasterBlack} {
		s = append(s, speckit.Range(id, -2, 2, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GammaR, core.GammaG, core.GammaB, core.GammaY} {
		s = append(s, speckit.Range(id, -4, 4, 0.01, ""))
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		s = append(s, speckit.Range(id, 0, 16, 0.01, ""))
	}
	return append(s,
		speckit.Range(core.Contrast, 0, 2, 0.01, ""),
		speckit.Range(core.Saturation, 0, 2, 0.01, ""),
		speckit.Range(core.Hue, -1, 1, 0.01, ""),
		speckit.Enum(core.Detail, speckit.Labeled("off", "Off", "low", "Low", "medium", "Medium", "high", "High")),
		speckit.Toggle(core.ColorBars),
		speckit.Toggle(core.Record),
		speckit.ReadOnly(speckit.Toggle(core.Tally)),
	)
}

// initial is the state the camera starts in.
func initial() map[core.ControlID]core.Value {
	v := map[core.ControlID]core.Value{
		core.Iris: core.Number(4), core.Focus: core.Number(0.6), core.Zoom: core.Number(0.25),
		core.Gain: core.Number(0), core.ShutterSpeed: core.Choice("1/100"), core.ShutterAngle: core.Number(180),
		core.ND: core.Choice("2"), core.WhiteBalance: core.Number(5600), core.Tint: core.Number(0),
		core.Contrast: core.Number(1), core.Saturation: core.Number(1), core.Hue: core.Number(0),
		core.Detail: core.Choice("medium"), core.ColorBars: core.Bool(false), core.Record: core.Bool(false),
		core.Tally: core.Bool(false), core.MasterBlack: core.Number(0),
	}
	for _, id := range []core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY, core.GammaR, core.GammaG, core.GammaB, core.GammaY} {
		v[id] = core.Number(0)
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		v[id] = core.Number(1)
	}
	return v
}

// Camera is the simulated camera. It is safe for concurrent use.
type Camera struct {
	specs []core.ControlSpec

	mu      sync.Mutex
	state   core.ConnState
	vals    map[core.ControlID]core.Value
	subs    map[int]func(core.Event)
	nextSub int
}

var (
	_ core.Camera       = (*Camera)(nil)
	_ core.LensReporter = (*Camera)(nil)
)

func newCamera() *Camera {
	return &Camera{specs: specs(), state: core.StateDisconnected, vals: initial(), subs: map[int]func(core.Event){}}
}

func (c *Camera) Controls() []core.ControlSpec {
	out := make([]core.ControlSpec, len(c.specs))
	for i, s := range c.specs {
		s.Options = slices.Clone(s.Options)
		out[i] = s
	}
	return out
}

func (c *Camera) Status() core.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return core.Status{State: c.state}
}

// Connect always succeeds: there is nothing to reach.
func (c *Camera) Connect(context.Context) error {
	c.setState(core.StateConnected)
	return nil
}

func (c *Camera) Close() error {
	c.setState(core.StateDisconnected)
	c.mu.Lock()
	clear(c.subs)
	c.mu.Unlock()
	return nil
}

func (c *Camera) setState(s core.ConnState) {
	c.mu.Lock()
	if c.state == s {
		c.mu.Unlock()
		return
	}
	c.state = s
	subs := slices.Collect(maps.Values(c.subs))
	c.mu.Unlock()
	st := core.Status{State: s}
	for _, fn := range subs {
		fn(core.Event{Status: &st})
	}
}

func (c *Camera) Subscribe(fn func(core.Event)) (cancel func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextSub
	c.nextSub++
	c.subs[id] = fn
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.subs, id)
	}
}

// Lens is the simulated 24-70 mm, its focal length following the zoom.
func (c *Camera) Lens() core.Lens {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lensLocked()
}

func (c *Camera) lensLocked() core.Lens {
	l := lens
	l.FocalLength = math.Round(lens.FocalMin + c.vals[core.Zoom].Number*(lens.FocalMax-lens.FocalMin))
	return l
}

func (c *Camera) Get(_ context.Context, id core.ControlID) (core.Value, error) {
	s, err := camerakit.FindSpec(c.specs, id)
	if err != nil {
		return core.Value{}, err
	}
	if s.Kind == core.KindAction {
		return core.Trigger(), nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state != core.StateConnected {
		return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotConnected)
	}
	return c.vals[id], nil
}

// Set applies a value and its consequences, like a camera would: the iris
// stays inside the lens, speed and angle follow each other, the master
// black is the lift luma, and the auto functions settle on a value.
func (c *Camera) Set(_ context.Context, id core.ControlID, v core.Value) error {
	s, err := camerakit.FindSpec(c.specs, id)
	if err != nil {
		return err
	}
	if err := s.Validate(v); err != nil {
		return err
	}
	c.mu.Lock()
	if c.state != core.StateConnected {
		c.mu.Unlock()
		return fmt.Errorf("set %s: %w", id, core.ErrNotConnected)
	}
	oldLens := c.lensLocked()
	changed := map[core.ControlID]core.Value{}
	put := func(id core.ControlID, v core.Value) {
		if c.vals[id] != v {
			c.vals[id] = v
			changed[id] = v
		}
	}
	switch id {
	case core.Iris:
		put(id, core.Number(math.Max(lens.IrisMin, math.Min(lens.IrisMax, v.Number))))
	case core.AutoIris:
		put(core.Iris, core.Number(5.6))
	case core.AutoFocus:
		put(core.Focus, core.Number(0.5))
	case core.AutoWhiteBalance:
		put(core.WhiteBalance, core.Number(5600))
		put(core.Tint, core.Number(0))
	case core.ShutterSpeed:
		put(id, v)
		if n, err := strconv.Atoi(strings.TrimPrefix(v.Choice, "1/")); err == nil && n > 0 {
			put(core.ShutterAngle, core.Number(math.Min(360, math.Round(360*fps/float64(n)*10)/10)))
		}
	case core.ShutterAngle:
		put(id, v)
		put(core.ShutterSpeed, core.Choice(nearestSpeed(360*fps/v.Number, c.specs)))
	case core.MasterBlack, core.LiftY:
		put(core.MasterBlack, v)
		put(core.LiftY, v)
	default:
		put(id, v)
	}
	var events []core.Event
	for id, v := range changed {
		events = append(events, core.Event{Control: id, Value: v})
	}
	if l := c.lensLocked(); l != oldLens {
		events = append(events, core.Event{Lens: &l})
	}
	subs := slices.Collect(maps.Values(c.subs))
	c.mu.Unlock()
	for _, e := range events {
		for _, fn := range subs {
			fn(e)
		}
	}
	return nil
}

// nearestSpeed is the declared shutter speed closest to 1/speed.
func nearestSpeed(speed float64, specs []core.ControlSpec) string {
	sp, _ := camerakit.FindSpec(specs, core.ShutterSpeed)
	best, gap := sp.Options[0].Value, math.Inf(1)
	for _, o := range sp.Options {
		n, err := strconv.Atoi(strings.TrimPrefix(o.Value, "1/"))
		if err != nil {
			continue
		}
		if d := math.Abs(math.Log(float64(n) / speed)); d < gap {
			best, gap = o.Value, d
		}
	}
	return best
}
