package panel

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Device is an opened, authenticated panel (*micropanel.Panel on Windows).
type Device interface {
	Serial() string
	// Run reads input until ctx ends or the device fails, then closes it.
	Run(ctx context.Context, fn func(micropanel.State), raw func([]byte)) error
	SetDigitsRaw(seg [14]byte) error
	SetMatrixRaw(px [32]byte) error
	WriteReport(b []byte) error
	ClearLEDs(keep int) error
	Close() error
}

// ErrNoDevice is returned by an opener when no panel is plugged in.
var ErrNoDevice = errors.New("panel: no device")

// Detail texts reported to the GUI (French, shown as is).
const (
	DetailBusy     = "utilisé par un autre logiciel (fermez ATEM Software Control)"
	DetailOpenFail = "panneau détecté mais inaccessible, nouvel essai…"
)

// Options configure a Controller. Zero values get defaults.
type Options struct {
	Open func() (Device, error) // default: the USB panel (Windows)
	// Busy reports whether another program drives the panel; the controller
	// then leaves it alone. Default: ATEM Software Control is running.
	Busy   func() bool
	Logger *slog.Logger

	Throttle   time.Duration // continuous writes period per control (default 33 ms, ~30/s)
	FrameEvery time.Duration // render period (default 33 ms)
	RetryMin   time.Duration // reconnect backoff (default 1 s)
	RetryMax   time.Duration // (default 10 s)
	BusyEvery  time.Duration // Busy check while driving (default 2 s)
}

// Controller drives the panel from app state. It implements app.Surface.
type Controller struct {
	opt Options
	log *slog.Logger

	wake chan struct{} // asks the writer goroutine to refresh the cache

	mu        sync.Mutex
	api       app.SurfaceAPI
	w         *writer
	rig       app.Rig
	cam       camState
	tally     [app.SlotCount + 1]bool
	held      map[string]bool
	prev      micropanel.State
	havePrev  bool
	iris, mb  pickup
	written   map[core.ControlID]float64 // last lever/ring value sent
	connected bool
}

var _ app.Surface = (*Controller)(nil)

// New returns a controller; call Run (through app.Host) to drive the panel.
func New(o Options) *Controller {
	if o.Open == nil {
		o.Open = OpenUSB
	}
	if o.Busy == nil {
		o.Busy = OtherOwnerRunning
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	def := func(d *time.Duration, v time.Duration) {
		if *d <= 0 {
			*d = v
		}
	}
	def(&o.Throttle, 33*time.Millisecond)
	def(&o.FrameEvery, 33*time.Millisecond)
	def(&o.RetryMin, time.Second)
	def(&o.RetryMax, 10*time.Second)
	def(&o.BusyEvery, 2*time.Second)
	return &Controller{opt: o, log: o.Logger, wake: make(chan struct{}, 1),
		held: map[string]bool{}, written: map[core.ControlID]float64{}}
}

// --- events from the app ---------------------------------------------------

// Emit receives app events (app.Emitter). It never blocks.
func (c *Controller) Emit(name string, payload any) {
	switch name {
	case app.EventRigChanged:
		switch r := payload.(type) {
		case app.Rig:
			c.setRig(r)
		case *app.Rig:
			c.setRig(*r)
		}
	case app.EventCamera:
		switch e := payload.(type) {
		case app.CameraEvent:
			c.cameraEvent(e)
		case *app.CameraEvent:
			c.cameraEvent(*e)
		}
	}
}

func (c *Controller) setRig(r app.Rig) {
	c.mu.Lock()
	if r.Active != c.rig.Active {
		c.releaseLocked()
		c.cam = camState{slot: r.Active}
	}
	c.rig = r
	for i := range c.tally {
		if i > 0 && slotCamera(r, i) == nil {
			c.tally[i] = false
		}
	}
	c.mu.Unlock()
	c.requestRefresh()
}

func (c *Controller) cameraEvent(e app.CameraEvent) {
	if e.Slot < 1 || e.Slot > app.SlotCount {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	ev := e.Event
	if ev.Status != nil {
		for i, s := range c.rig.Slots {
			if s.Slot == e.Slot && s.Camera != nil {
				// Copy: the rig's views are shared with other listeners.
				cv := *s.Camera
				cv.Status = *ev.Status
				c.rig.Slots = slices.Clone(c.rig.Slots)
				c.rig.Slots[i].Camera = &cv
			}
		}
		if e.Slot == c.rig.Active {
			c.requestRefresh()
		}
		return
	}
	if ev.Control == core.Tally {
		c.tally[e.Slot] = ev.Value.On
	}
	if e.Slot != c.cam.slot || !c.cam.has(ev.Control) {
		return
	}
	c.cam.values[ev.Control] = ev.Value
	// A lever/ring value changed by someone else: pick it up again.
	if w, ok := c.written[ev.Control]; !ok || w != ev.Value.Number {
		switch ev.Control {
		case core.Iris:
			c.iris.release()
		case core.MasterBlack:
			c.mb.release()
		}
	}
}

func (c *Controller) releaseLocked() {
	c.iris.release()
	c.mb.release()
	clear(c.written)
}

func (c *Controller) requestRefresh() {
	select {
	case c.wake <- struct{}{}:
	default:
	}
}

// refresh reloads the active camera's controls (writer goroutine).
func (c *Controller) refresh() {
	c.mu.Lock()
	api, slot := c.api, c.rig.Active
	c.mu.Unlock()
	if api == nil {
		return
	}
	var views []app.ControlView
	if slot != 0 {
		v, err := api.Controls(slot)
		if err != nil {
			c.log.Debug("panel: controls not read", "slot", slot, "err", err)
		}
		views = v
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rig.Active != slot {
		return // a newer refresh is queued
	}
	next := newCamState(slot, views)
	for id, p := range map[core.ControlID]*pickup{core.Iris: &c.iris, core.MasterBlack: &c.mb} {
		old, oldOK := c.cam.values[id]
		nv, ok := next.values[id]
		if !ok || !oldOK || old != nv {
			if w, sent := c.written[id]; !ok || !sent || w != nv.Number {
				p.release()
			}
		}
	}
	c.cam = next
}

// --- input from the panel --------------------------------------------------

// onInput handles one panel state (device reader goroutine).
func (c *Controller) onInput(s micropanel.State) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := map[string]bool{}
	for _, k := range s.Keys {
		now[micropanel.KeyName(k)] = true
	}
	if !c.havePrev {
		// First report after (re)connect: positions only, no action.
		c.prev, c.havePrev, c.held = s, true, now
		c.iris.seed(leverPos(s.Lever))
		c.mb.seed(leverPos(s.Ring))
		return
	}
	for _, k := range s.Keys {
		if name := micropanel.KeyName(k); !c.held[name] {
			c.pressLocked(name)
		}
	}
	c.held = now

	slot := c.cam.slot
	if slot != 0 && slot == c.rig.Active {
		for i, id := range knobControls {
			d := int(s.Analog[i]) - int(c.prev.Analog[i])
			if d == 0 {
				continue
			}
			if spec, cur, ok := c.cam.number(id); ok && !spec.ReadOnly {
				if nv := knobValue(spec, cur, d); nv != cur {
					c.cam.values[id] = core.Number(nv)
					c.w.set(slot, id, core.Number(nv))
				}
			}
		}
	}
	if s.Lever != c.prev.Lever {
		c.moveLocked(&c.iris, core.Iris, leverPos(s.Lever), irisAt, irisPos)
	}
	if s.Ring != c.prev.Ring {
		c.moveLocked(&c.mb, core.MasterBlack, leverPos(s.Ring), mbAt, mbPos)
	}
	c.prev = s
}

// moveLocked applies a lever/ring movement with soft takeover.
func (c *Controller) moveLocked(p *pickup, id core.ControlID, pos float64,
	at func(core.ControlSpec, float64) float64, posOf func(core.ControlSpec, float64) float64) {
	spec, cur, known := c.cam.number(id)
	slot := c.cam.slot
	if slot == 0 || slot != c.rig.Active || spec.ReadOnly {
		known = false
	}
	target := 0.0
	if known {
		target = posOf(spec, cur)
	}
	if !p.move(pos, target, known) {
		return
	}
	nv := at(spec, pos)
	if nv == cur {
		return
	}
	c.cam.values[id] = core.Number(nv)
	c.written[id] = nv
	c.w.set(slot, id, core.Number(nv))
}

// pressLocked maps a key press (docs/panel-mapping.md).
func (c *Controller) pressLocked(name string) {
	if strings.HasPrefix(name, "CAM") && len(name) == 4 && name[3] >= '1' && name[3] <= '8' {
		c.w.do(op{select_: true, slot: int(name[3] - '0')})
		return
	}
	slot := c.cam.slot
	if slot == 0 || slot != c.rig.Active {
		return
	}
	stepKey := func(ids []core.ControlID, dir int) {
		id, _, known := c.cam.pick(ids...)
		if !known {
			return
		}
		s, cur, _, _ := c.cam.get(id)
		if s.ReadOnly {
			return
		}
		if nv, ok := step(s, cur, dir); ok {
			c.cam.values[id] = nv
			c.w.do(op{slot: slot, id: id, v: nv})
		}
	}
	trigger := func(id core.ControlID) {
		if c.cam.has(id) {
			c.w.do(op{slot: slot, id: id, v: core.Trigger()})
		}
	}
	switch name {
	case "ND_UP":
		stepKey([]core.ControlID{core.ND}, +1)
	case "ND_DOWN":
		stepKey([]core.ControlID{core.ND}, -1)
	case "GAIN_UP":
		stepKey(gainIDs, +1)
	case "GAIN_DOWN":
		stepKey(gainIDs, -1)
	case "SHUTTER_UP":
		stepKey(shutterIDs, +1)
	case "SHUTTER_DOWN":
		stepKey(shutterIDs, -1)
	case "AUTO_IRIS":
		trigger(core.AutoIris)
	case "WB":
		trigger(core.AutoWhiteBalance)
	case "BARS":
		// Toggle only from a known state: never guess what goes on air.
		if s, v, _, known := c.cam.get(core.ColorBars); known && s.Kind == core.KindToggle && !s.ReadOnly {
			c.w.do(op{slot: slot, id: core.ColorBars, v: core.Bool(!v.On)})
		}
	default:
		c.log.Debug("panel key not mapped", "key", name)
	}
}

// onWriteError resyncs the cache after a failed write (writer goroutine).
func (c *Controller) onWriteError(o op, _ error) {
	if !o.select_ {
		c.requestRefresh()
	}
}

// --- rendering -------------------------------------------------------------

func (c *Controller) frame(now time.Time) frame {
	c.mu.Lock()
	defer c.mu.Unlock()
	hint := func(p pickup, id core.ControlID) pickHint {
		_, _, declared, known := c.cam.get(id)
		return pickHint{declared: declared, known: known, picked: p.picked, pos: p.pos, havePos: p.havePos}
	}
	return render(view{
		rig: c.rig, cam: c.cam, tally: c.tally, held: c.held,
		iris: hint(c.iris, core.Iris), mb: hint(c.mb, core.MasterBlack),
		blink: now.UnixMilli()/250%2 == 0,
	})
}

func slotCamera(r app.Rig, slot int) *app.CameraView {
	for _, s := range r.Slots {
		if s.Slot == slot {
			return s.Camera
		}
	}
	return nil
}
