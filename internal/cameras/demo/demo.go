// Package demo is the "demo" camera module: a simulated studio camera with
// no network address. It answers like a real camera (values, consequences,
// events, lens), so the whole GUI and the panel can be tried without one.
// It is also the reference module: docs/modules-camera.html walks through
// it.
package demo

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// ID is the demo module's id.
const ID core.ModelID = "demo"

// fps is the simulated frame rate, for the shutter speed and angle.
const fps = 50

// lens is the simulated lens: a 24-70 mm f/2.8 zoom focusing from 0.38 m.
var lens = core.Lens{IrisMin: 2.8, IrisMax: 22, FocalMin: 24, FocalMax: 70, FocusNear: 0.38, FocusFar: core.FocusInfinity}

// Module is the demo camera module. It needs no dependency.
type Module struct{}

var _ core.Module = Module{}

// Info declares the demo camera.
func (Module) Info() core.ModelInfo {
	return core.ModelInfo{
		ID: ID, Brand: "Démo", Name: "Caméra de démonstration",
		Transports: []core.Transport{core.TransportEthernet}, Auth: core.AuthNone,
		Support: core.SupportSupported, NoAddress: true, InterchangeableLens: true,
		Notes: "Caméra simulée sans adresse, avec les réglages d'une caméra de studio : tout répond, pour essayer l'interface et le panneau sans caméra.",
	}
}

// New builds a demo camera; it does no I/O. Like a studio camera with an
// interchangeable lens, it has a zoom only when the lens has a power zoom.
func (Module) New(cfg core.CameraConfig) (core.Camera, error) {
	s := camerakit.NewSpecs(specs())
	if !cfg.PowerZoom {
		s = s.Without(core.Zoom)
	}
	return newCamera(s), nil
}

// Camera is the simulated camera. It is safe for concurrent use.
type Camera struct {
	specs camerakit.Specs
	subs  camerakit.Subscribers

	mu    sync.Mutex // guards state and vals
	state core.ConnState
	vals  map[core.ControlID]core.Value
}

var (
	_ core.Camera       = (*Camera)(nil)
	_ core.LensReporter = (*Camera)(nil)
)

func newCamera(s camerakit.Specs) *Camera {
	return &Camera{specs: s, state: core.StateDisconnected, vals: initial()}
}

// initial is the state the camera starts in.
func initial() map[core.ControlID]core.Value {
	v := map[core.ControlID]core.Value{
		core.Iris: core.Number(4), core.IrisNorm: core.Number(irisNormOf(4)),
		core.Focus: core.Number(0.6), core.Zoom: core.Number(0.25),
		core.Gain: core.Number(0), core.ISO: core.Choice("400"),
		core.ShutterSpeed: core.Choice("1/100"), core.ShutterAngle: core.Number(180),
		core.ND: core.Choice("2"), core.WhiteBalance: core.Number(5600), core.Tint: core.Number(0),
		core.Contrast: core.Number(1), core.Saturation: core.Number(1), core.Hue: core.Number(0),
		ContrastPivot: core.Number(0.5), LumaContribution: core.Number(1),
		core.Record: core.Bool(false), core.MasterBlack: core.Number(0),
	}
	for _, id := range []core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY,
		core.GammaR, core.GammaG, core.GammaB, core.GammaY, OffsetR, OffsetG, OffsetB, OffsetY} {
		v[id] = core.Number(0)
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		v[id] = core.Number(1)
	}
	return v
}

func (c *Camera) Controls() []core.ControlSpec { return c.specs.All() }

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
	c.subs.Clear()
	return nil
}

func (c *Camera) setState(s core.ConnState) {
	c.mu.Lock()
	changed := c.state != s
	c.state = s
	c.mu.Unlock()
	if changed {
		c.subs.Publish(core.Event{Status: &core.Status{State: s}})
	}
}

func (c *Camera) Subscribe(fn func(core.Event)) (cancel func()) { return c.subs.Add(fn) }

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
	s, err := c.specs.Find(id)
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

// Set validates the value, applies it with its consequences, then notifies
// the subscribers outside the lock.
func (c *Camera) Set(_ context.Context, id core.ControlID, v core.Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	c.mu.Lock()
	if c.state != core.StateConnected {
		c.mu.Unlock()
		return fmt.Errorf("set %s: %w", id, core.ErrNotConnected)
	}
	events := c.apply(id, v)
	c.mu.Unlock()
	c.subs.Publish(events...)
	return nil
}

// apply changes the state like a camera would: the iris stays inside the
// lens, speed and angle follow each other, the master black is the lift
// luma, and the auto functions settle on a value. It returns the events
// to publish. c.mu must be held.
func (c *Camera) apply(id core.ControlID, v core.Value) []core.Event {
	oldLens := c.lensLocked()
	var events []core.Event
	put := func(id core.ControlID, v core.Value) {
		if c.vals[id] != v {
			c.vals[id] = v
			events = append(events, core.Event{Control: id, Value: v})
		}
	}
	switch id {
	case core.Iris:
		f := math.Max(lens.IrisMin, math.Min(lens.IrisMax, v.Number))
		put(id, core.Number(f))
		put(core.IrisNorm, core.Number(irisNormOf(f)))
	case core.IrisNorm:
		// Like a Blackmagic camera: the lens' range, open (0) to closed (1).
		n := math.Max(0, math.Min(1, v.Number))
		put(id, core.Number(n))
		put(core.Iris, core.Number(irisStopOf(n)))
	case core.AutoIris:
		put(core.Iris, core.Number(5.6))
		put(core.IrisNorm, core.Number(irisNormOf(5.6)))
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
		put(core.ShutterSpeed, core.Choice(c.nearestSpeed(360*fps/v.Number)))
	case core.MasterBlack, core.LiftY:
		put(core.MasterBlack, v)
		put(core.LiftY, v)
	default:
		put(id, v)
	}
	if l := c.lensLocked(); l != oldLens {
		events = append(events, core.Event{Lens: &l})
	}
	return events
}

// nearestSpeed is the declared shutter speed closest to 1/speed.
func (c *Camera) nearestSpeed(speed float64) string {
	sp, _ := c.specs.Find(core.ShutterSpeed)
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

// irisNormOf and irisStopOf convert between an f-stop and the position over
// the simulated lens' range (logarithmic, 0 = open), rounded like a camera
// reports them.
func irisNormOf(f float64) float64 {
	return math.Round(math.Log(f/lens.IrisMin)/math.Log(lens.IrisMax/lens.IrisMin)*1000) / 1000
}

func irisStopOf(n float64) float64 {
	return math.Round(lens.IrisMin*math.Pow(lens.IrisMax/lens.IrisMin, n)*10) / 10
}
