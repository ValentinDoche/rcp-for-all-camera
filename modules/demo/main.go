// Command demo is the "demo" camera module: a simulated studio camera with
// no network address. It answers like a real camera (values, consequences,
// events, lens), so the GUI and the panel can be tried without one. It is
// the reference module that docs/modules-camera.html walks through.
//
// Build: go run ./cmd/buildmodules demo
package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// main stays empty: the SDK plugs the module into the application.
func main() {}

func init() { rcpmod.Register(module{}) }

// fps is the simulated frame rate, for the shutter speed and angle.
const fps = 50

// lens is the simulated lens: a 24-70 mm f/2.8 zoom focusing from 0.38 m.
var lens = rcpmod.Lens{IrisMin: 2.8, IrisMax: 22, FocalMin: 24, FocalMax: 70, FocusNear: 0.38, FocusFar: rcpmod.FocusInfinity}

type module struct{}

func (module) Info() rcpmod.Info {
	return rcpmod.Info{
		ID: "demo", Brand: "Démo", Name: "Caméra de démonstration",
		Transports: []string{rcpmod.TransportEthernet}, Auth: rcpmod.AuthNone,
		Support: rcpmod.SupportSupported, NoAddress: true, InterchangeableLens: true,
		Notes: "Caméra simulée sans adresse, avec les réglages d'une caméra de studio : tout répond, pour essayer l'interface et le panneau sans caméra.",
	}
}

// New builds a demo camera. Like a studio camera with an interchangeable
// lens, it has a zoom only when the lens has a power zoom.
func (module) New(cfg rcpmod.Config, host rcpmod.Host) (rcpmod.Camera, error) {
	s := rcpmod.NewSpecs(specs())
	if !cfg.PowerZoom {
		s = s.Without(rcpmod.Zoom)
	}
	return &camera{host: host, specs: s, vals: initial()}, nil
}

// camera is the simulated camera. The application calls it one call at a
// time: no lock is needed.
type camera struct {
	host  rcpmod.Host
	specs rcpmod.Specs
	on    bool
	vals  map[string]rcpmod.Value
}

func (c *camera) Controls() []rcpmod.ControlSpec { return c.specs.All() }

// Connect succeeds at once: there is nothing to reach.
func (c *camera) Connect() error {
	c.on = true
	c.host.Emit(rcpmod.StatusEvent(rcpmod.StateConnected, ""))
	c.host.Emit(rcpmod.LensEvent(c.lens()))
	return nil
}

func (c *camera) Close() { c.on = false }

func (c *camera) Get(id string) (rcpmod.Value, error) {
	s, err := c.specs.Find(id)
	if err != nil {
		return rcpmod.Value{}, err
	}
	if s.Kind == rcpmod.KindAction {
		return rcpmod.Trigger(), nil
	}
	if !c.on {
		return rcpmod.Value{}, rcpmod.ErrNotConnected
	}
	return c.vals[id], nil
}

// Set validates the value, applies it with its consequences, and emits
// every change.
func (c *camera) Set(id string, v rcpmod.Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	if !c.on {
		return rcpmod.ErrNotConnected
	}
	for _, e := range c.apply(id, v) {
		c.host.Emit(e)
	}
	return nil
}

// apply changes the state like a camera would: the iris stays inside the
// lens, speed and angle follow each other, the master black is the lift
// luma, the auto functions settle on a value. It returns the events.
func (c *camera) apply(id string, v rcpmod.Value) []rcpmod.Event {
	oldLens := c.lens()
	var events []rcpmod.Event
	put := func(id string, v rcpmod.Value) {
		if c.vals[id] != v {
			c.vals[id] = v
			events = append(events, rcpmod.ControlEvent(id, v))
		}
	}
	switch id {
	case rcpmod.Iris:
		f := math.Max(lens.IrisMin, math.Min(lens.IrisMax, v.Number))
		put(id, rcpmod.Number(f))
		put(rcpmod.IrisNorm, rcpmod.Number(irisNormOf(f)))
	case rcpmod.IrisNorm:
		// Like a Blackmagic camera: the lens' range, open (0) to closed (1).
		n := math.Max(0, math.Min(1, v.Number))
		put(id, rcpmod.Number(n))
		put(rcpmod.Iris, rcpmod.Number(irisStopOf(n)))
	case rcpmod.AutoIris:
		put(rcpmod.Iris, rcpmod.Number(5.6))
		put(rcpmod.IrisNorm, rcpmod.Number(irisNormOf(5.6)))
	case rcpmod.AutoFocus:
		put(rcpmod.Focus, rcpmod.Number(0.5))
	case rcpmod.AutoWhiteBalance:
		put(rcpmod.WhiteBalance, rcpmod.Number(5600))
		put(rcpmod.Tint, rcpmod.Number(0))
	case rcpmod.ShutterSpeed:
		put(id, v)
		if n, err := strconv.Atoi(strings.TrimPrefix(v.Choice, "1/")); err == nil && n > 0 {
			put(rcpmod.ShutterAngle, rcpmod.Number(math.Min(360, math.Round(360*fps/float64(n)*10)/10)))
		}
	case rcpmod.ShutterAngle:
		put(id, v)
		put(rcpmod.ShutterSpeed, rcpmod.Choice(c.nearestSpeed(360*fps/v.Number)))
	case rcpmod.MasterBlack, rcpmod.LiftY:
		put(rcpmod.MasterBlack, v)
		put(rcpmod.LiftY, v)
	case tallyTest:
		put(tallyLamp, rcpmod.Bool(!c.vals[tallyLamp].On))
	default:
		put(id, v)
	}
	if l := c.lens(); l != oldLens {
		events = append(events, rcpmod.LensEvent(l))
	}
	return events
}

// lens is the simulated lens, its focal length following the zoom.
func (c *camera) lens() rcpmod.Lens {
	l := lens
	l.FocalLength = math.Round(lens.FocalMin + c.vals[rcpmod.Zoom].Number*(lens.FocalMax-lens.FocalMin))
	return l
}

// nearestSpeed is the declared shutter speed closest to 1/speed.
func (c *camera) nearestSpeed(speed float64) string {
	sp, _ := c.specs.Find(rcpmod.ShutterSpeed)
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
// the lens' range (logarithmic, 0 = open), rounded like a camera reports.
func irisNormOf(f float64) float64 {
	return math.Round(math.Log(f/lens.IrisMin)/math.Log(lens.IrisMax/lens.IrisMin)*1000) / 1000
}

func irisStopOf(n float64) float64 {
	return math.Round(lens.IrisMin*math.Pow(lens.IrisMax/lens.IrisMin, n)*10) / 10
}
