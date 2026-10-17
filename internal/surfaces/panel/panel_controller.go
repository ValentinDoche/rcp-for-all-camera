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
	// SetFeature and GetFeature write and read a feature report (b[0] and
	// id = report ID; the size must be exact, trap 1).
	SetFeature(b []byte) error
	GetFeature(id byte, size int) ([]byte, error)
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

	wake   chan struct{} // asks the writer goroutine to refresh the cache
	reopen chan struct{} // ends the session and opens the panel again (Reconnect)

	mu        sync.Mutex
	api       app.SurfaceAPI
	w         *writer
	rig       app.Rig
	cam       camState
	tally     [app.SlotCount + 1]bool
	held      map[string]bool
	prev      micropanel.State
	havePrev  bool
	knobRef   [len(knobControls)]int // knob positions last applied (deadband reference)
	iris, mb  pickup
	written   map[core.ControlID]float64 // last lever/ring value sent
	recent    map[core.ControlID]recentWrites
	late      map[core.ControlID]float64 // echoed values to adopt once settled (lever_echo.go)
	connected bool

	// knobs are COARSE and SENS (the iris lever window); coarseAt and
	// sensAt are when they last moved (panel previews).
	knobs            irisKnobs
	coarseAt, sensAt time.Time

	// off: PANEL ACTIVE turned panel control off. Only PANEL ACTIVE acts;
	// the panel still shows the cameras. Not persisted (on at start).
	off bool

	// storeArmed is the app's shared Store arming (EventStoreArmed).
	storeArmed bool

	// wbMode: W/B toggled the white balance mode (SHUTTER keys = Kelvin,
	// GAIN keys = tint). Left on a camera switch.
	wbMode bool
	// wbDownAt: when W/B was pressed (zero when it is not held). A short
	// press toggles the mode on release; held wbLongPress it triggers the
	// auto white balance once (wbFired).
	wbDownAt time.Time
	wbFired  bool

	// recenter asks the render loop to bring the color knobs' counters
	// back to the middle (see recenterKnobs).
	recenter bool

	// alarmUntil is when the ALARM blink after a failed command stops.
	alarmUntil time.Time
	now        func() time.Time
}

// alarmHold is how long ALARM blinks after a failed command.
const alarmHold = 3 * time.Second

// raiseAlarm makes ALARM blink for alarmHold. Asking for confirmation
// (operator lock) is not an error.
func (c *Controller) raiseAlarm(code string) {
	if code == app.CodeConfirmRequired {
		return
	}
	c.mu.Lock()
	c.alarmUntil = c.now().Add(alarmHold)
	c.mu.Unlock()
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
	return &Controller{opt: o, log: o.Logger, wake: make(chan struct{}, 1), reopen: make(chan struct{}, 1),
		held: map[string]bool{}, written: map[core.ControlID]float64{},
		recent: map[core.ControlID]recentWrites{}, late: map[core.ControlID]float64{}, now: time.Now}
}

// Reconnect blanks and closes the panel being driven, then opens one again
// at once through Options.Open. Call it after changing what Open returns
// (another USB panel, the simulator). It never blocks.
func (c *Controller) Reconnect() {
	select {
	case c.reopen <- struct{}{}:
	default:
	}
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
	case app.EventStoreArmed:
		switch e := payload.(type) {
		case app.StoreArmedState:
			c.setStoreArmed(e.Armed)
		case *app.StoreArmedState:
			c.setStoreArmed(e.Armed)
		}
	case app.EventCommandFailed:
		switch e := payload.(type) {
		case app.CommandFailed:
			c.raiseAlarm(e.Code)
		case *app.CommandFailed:
			c.raiseAlarm(e.Code)
		}
	}
}

func (c *Controller) setStoreArmed(on bool) {
	c.mu.Lock()
	c.storeArmed = on
	c.mu.Unlock()
}

func (c *Controller) setRig(r app.Rig) {
	c.mu.Lock()
	if r.Active != c.rig.Active {
		c.releaseLocked()
		c.cam = camState{slot: r.Active}
		c.wbMode = false
		c.wbDownAt, c.wbFired = time.Time{}, false
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
	if ev.Lens != nil {
		if e.Slot == c.cam.slot {
			c.cam.lens = *ev.Lens
		}
		return
	}
	if ev.Control == core.Tally {
		c.tally[e.Slot] = ev.Value.On
	}
	if e.Slot != c.cam.slot || !c.cam.has(ev.Control) {
		return
	}
	if (ev.Control == irisLeverID(c.cam) || ev.Control == core.MasterBlack) && c.echoLocked(ev.Control, ev.Value.Number) {
		return
	}
	c.cam.values[ev.Control] = ev.Value
	// A lever/ring value changed by someone else: pick it up again.
	if w, ok := c.written[ev.Control]; !ok || w != ev.Value.Number {
		switch ev.Control {
		case irisLeverID(c.cam):
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
	clear(c.recent)
	clear(c.late)
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
	var lens core.Lens
	if slot != 0 {
		v, err := api.Controls(slot)
		if err != nil {
			c.log.Debug("panel: controls not read", "slot", slot, "err", err)
		}
		views = v
		lens, _ = api.Lens(slot)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rig.Active != slot {
		return // a newer refresh is queued
	}
	next := newCamState(slot, views)
	next.lens = lens
	for id, p := range map[core.ControlID]*pickup{irisLeverID(next): &c.iris, core.MasterBlack: &c.mb} {
		old, oldOK := c.cam.values[id]
		nv, ok := next.values[id]
		if ok && oldOK && old != nv && c.echoLocked(id, nv.Number) {
			next.values[id] = old // the camera has not caught up with the lever yet
			continue
		}
		if !ok || !oldOK || old != nv {
			if w, sent := c.written[id]; !ok || !sent || w != nv.Number {
				p.release()
			}
		}
	}
	c.cam = next
	if c.wbMode && !c.wbAvailableLocked() {
		c.wbMode = false
	}
}

// wbAvailableLocked reports whether the active camera has a white balance
// or a tint the white balance mode can step.
func (c *Controller) wbAvailableLocked() bool {
	for _, id := range []core.ControlID{core.WhiteBalance, core.Tint} {
		if s, _, declared, _ := c.cam.get(id); declared && s.Kind == core.KindRange {
			return true
		}
	}
	return false
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
		for i := range c.knobRef {
			c.knobRef[i] = int(s.Analog[i])
		}
		c.recenter = c.recenter || s.AnalogRead
		c.iris.seed(leverPos(s.Lever))
		c.mb.seed(leverPos(s.Ring))
		// COARSE/SENS as found at connect, without preview.
		c.knobs, c.coarseAt, c.sensAt = irisKnobs{}, time.Time{}, time.Time{}
		c.irisKnobsLocked(s)
		return
	}
	toggled := false
	for _, k := range s.Keys {
		name := micropanel.KeyName(k)
		switch {
		case c.held[name]:
		case name == "PANEL_ACTIVE":
			c.off, toggled = !c.off, true
		case !c.off:
			c.pressLocked(name)
		}
	}
	if c.held["WB"] && !now["WB"] && !c.wbDownAt.IsZero() {
		// W/B released: a short press toggles the mode, a long one did
		// (or, without auto white balance, did nothing) already.
		if !c.off && !c.wbFired && c.now().Sub(c.wbDownAt) < wbLongPress {
			c.wbMode = !c.wbMode && c.wbAvailableLocked()
		}
		c.wbDownAt, c.wbFired = time.Time{}, false
	}
	c.held = now
	if c.off {
		// Panel control off: knobs, lever, ring, COARSE and SENS do nothing.
		if toggled {
			c.wbMode = false
		}
		c.prev = s
		return
	}
	if toggled {
		// Back on: re-take every reference from where the controls are now,
		// so nothing jumps; the lever and ring must pick up again.
		c.retakeLocked(s)
		c.prev = s
		return
	}

	slot := c.cam.slot
	if slot != 0 && slot == c.rig.Active {
		for i, id := range knobControls {
			if s.AnalogRead && nearKnobEnd(int(s.Analog[i])) {
				c.recenter = true
			}
			var d int
			d, c.knobRef[i] = knobStep(c.knobRef[i], int(s.Analog[i]))
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
	c.irisKnobsLocked(s)
	if s.Lever != c.prev.Lever {
		if irisLeverID(c.cam) == core.IrisNorm {
			c.moveLocked(&c.iris, core.IrisNorm, leverPos(s.Lever), c.normWindowLocked, normAt, normPos)
		} else {
			c.moveLocked(&c.iris, core.Iris, leverPos(s.Lever), c.irisWindowLocked, irisAt, irisPos)
		}
	}
	if s.Ring != c.prev.Ring {
		c.moveLocked(&c.mb, core.MasterBlack, leverPos(s.Ring), mbWindow, mbAt, mbPos)
	}
	c.prev = s
}

// retakeLocked takes the current positions as references: knobs relative
// from here, lever and ring waiting for pickup, the iris lever window
// where COARSE and SENS are now (without preview).
func (c *Controller) retakeLocked(s micropanel.State) {
	for i := range c.knobRef {
		c.knobRef[i] = int(s.Analog[i])
	}
	c.iris.seed(leverPos(s.Lever))
	c.mb.seed(leverPos(s.Ring))
	if s.AnalogRead {
		c.knobs = irisKnobs{coarse: int(s.Analog[knobCoarse]), sens: int(s.Analog[knobSens]), read: true}
	}
	c.coarseAt, c.sensAt = time.Time{}, time.Time{}
}

// irisKnobsLocked follows COARSE and SENS. Moving them writes nothing to the
// camera: it moves the lever window, so the lever must pick the iris up
// again, and starts the panel preview of what was turned.
func (c *Controller) irisKnobsLocked(s micropanel.State) {
	if !s.AnalogRead {
		return
	}
	coarse, sens := int(s.Analog[knobCoarse]), int(s.Analog[knobSens])
	if !c.knobs.read {
		c.knobs = irisKnobs{coarse: coarse, sens: sens, read: true}
		c.iris.release()
		return
	}
	var dc, ds int
	if dc, c.knobs.coarse = knobStep(c.knobs.coarse, coarse); dc != 0 {
		c.coarseAt = c.now()
		c.iris.release()
	}
	if ds, c.knobs.sens = knobStep(c.knobs.sens, sens); ds != 0 {
		c.sensAt = c.now()
		c.iris.release()
	}
	if dc != 0 || ds != 0 {
		c.fitIrisLocked()
	}
}

// fitIrisLocked keeps the lever usable after COARSE or SENS moved its
// window: a camera iris left outside the window is set to the value under
// the lever, which then drives it.
func (c *Controller) fitIrisLocked() {
	id := irisLeverID(c.cam)
	spec, cur, known := c.cam.number(id)
	slot := c.cam.slot
	if !known || slot == 0 || slot != c.rig.Active || spec.ReadOnly || !c.iris.havePos {
		return
	}
	at, w := irisAt, c.irisWindowLocked(spec)
	if id == core.IrisNorm {
		at, w = normAt, c.normWindowLocked(spec)
	}
	if w.reaches(spec, cur) {
		return
	}
	c.iris.picked = true
	if nv := at(spec, w, c.iris.pos); nv != cur {
		c.cam.values[id] = core.Number(nv)
		c.noteWriteLocked(id, nv)
		c.w.set(slot, id, core.Number(nv))
	}
}

// takeRecenter returns the feature report that puts the color knobs back in
// the middle of their counter, keeping the other knobs where they are, when
// a recenter is due.
func (c *Controller) takeRecenter() ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.recenter || !c.havePrev || !c.prev.AnalogRead {
		return nil, false
	}
	c.recenter = false
	pos := c.prev.Analog
	for i := range knobControls {
		pos[i] = knobCentre
	}
	return micropanel.AnalogReport(pos), true
}

// knobsRead takes the knob positions read back after a recenter as the new
// references, so the next movement is measured from where the counters
// really are (whether or not the panel accepted the write).
func (c *Controller) knobsRead(pos [9]uint16) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range knobControls {
		c.knobRef[i] = int(pos[i])
	}
	c.prev.Analog = pos
}

// irisWindowLocked is the iris lever window for the active camera's iris.
func (c *Controller) irisWindowLocked(s core.ControlSpec) window { return irisWindow(s, c.knobs) }

// normWindowLocked is the lever window over the lens' range (IrisNorm).
func (c *Controller) normWindowLocked(core.ControlSpec) window { return normWindow(c.knobs) }

// moveLocked applies a lever/ring movement with soft takeover. A camera
// value outside the control's window can never be reached, so the control
// takes over at once instead of staying stuck.
func (c *Controller) moveLocked(p *pickup, id core.ControlID, pos float64, win func(core.ControlSpec) window,
	at func(core.ControlSpec, window, float64) float64, posOf func(core.ControlSpec, window, float64) float64) {
	spec, cur, known := c.cam.number(id)
	slot := c.cam.slot
	w := win(spec)
	if slot == 0 || slot != c.rig.Active || spec.ReadOnly {
		known = false
	}
	if known && !w.reaches(spec, cur) {
		p.picked = true
	}
	target := 0.0
	if known {
		target = posOf(spec, w, cur)
	}
	if !p.move(pos, target, known) {
		return
	}
	nv := at(spec, w, pos)
	if nv == cur {
		return
	}
	c.cam.values[id] = core.Number(nv)
	c.noteWriteLocked(id, nv)
	c.w.set(slot, id, core.Number(nv))
}

// pressLocked maps a key press (docs/panel-mapping.md).
func (c *Controller) pressLocked(name string) {
	if strings.HasPrefix(name, "CAM") && len(name) == 4 && name[3] >= '1' && name[3] <= '8' {
		// An empty slot's button is inert (its LED is off too).
		if slot := int(name[3] - '0'); c.assignedLocked(slot) {
			c.w.do(op{act: actSelect, slot: slot})
		}
		return
	}
	if name == "STORE" {
		// The app holds the arming; the LED follows EventStoreArmed.
		c.w.do(op{act: actStoreToggle})
		return
	}
	slot := c.cam.slot
	if slot == 0 || slot != c.rig.Active {
		return
	}
	if n, ok := sceneKey(name); ok {
		if c.assignedLocked(slot) {
			c.w.do(op{act: actScene, slot: slot, n: n})
		}
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
	case "GAIN_UP", "GAIN_DOWN":
		ids := gainIDs
		if c.wbMode {
			ids = []core.ControlID{core.Tint}
		}
		stepKey(ids, keyDir(name))
	case "SHUTTER_UP", "SHUTTER_DOWN":
		ids := shutterIDs
		if c.wbMode {
			ids = []core.ControlID{core.WhiteBalance}
		}
		stepKey(ids, keyDir(name))
	case "AUTO_IRIS":
		trigger(core.AutoIris)
	case "WB":
		// Short press: white balance mode (on release). Long press: auto
		// white balance (fired from the render tick, see wbTickLocked).
		c.wbDownAt, c.wbFired = c.now(), false
	case "BARS":
		// Toggle only from a known state: never guess what goes on air.
		if s, v, _, known := c.cam.get(core.ColorBars); known && s.Kind == core.KindToggle && !s.ReadOnly {
			c.w.do(op{slot: slot, id: core.ColorBars, v: core.Bool(!v.On)})
		}
	default:
		c.log.Debug("panel key not mapped", "key", name)
	}
}

// sceneKey returns n for the key "SCENEn" (1..app.PresetCount).
func sceneKey(name string) (int, bool) {
	if len(name) == 6 && strings.HasPrefix(name, "SCENE") {
		if n := int(name[5] - '0'); n >= 1 && n <= app.PresetCount {
			return n, true
		}
	}
	return 0, false
}

// keyDir is +1 for a *_UP key, -1 otherwise.
func keyDir(name string) int {
	if strings.HasSuffix(name, "_UP") {
		return +1
	}
	return -1
}

// onWriteError resyncs the cache after a failed write (writer goroutine).
func (c *Controller) onWriteError(o op, err error) {
	code := app.CodeInternal
	var ae *app.Error
	if errors.As(err, &ae) {
		code = ae.Code
	}
	c.raiseAlarm(code)
	if o.act != actSelect {
		c.requestRefresh()
	}
}

// --- rendering -------------------------------------------------------------

// wbLongPress is how long W/B must be held to run the auto white balance.
const wbLongPress = time.Second

// wbAWBLocked reports whether the active camera has the auto white balance.
func (c *Controller) wbAWBLocked() bool {
	s, _, declared, _ := c.cam.get(core.AutoWhiteBalance)
	return declared && s.Kind == core.KindAction && c.cam.slot != 0 && c.cam.slot == c.rig.Active
}

// wbTickLocked fires the auto white balance once W/B has been held long
// enough, on a camera that has it.
func (c *Controller) wbTickLocked(now time.Time) {
	if c.wbDownAt.IsZero() || c.wbFired || c.off || now.Sub(c.wbDownAt) < wbLongPress {
		return
	}
	c.wbFired = true
	if c.wbAWBLocked() {
		c.w.do(op{slot: c.cam.slot, id: core.AutoWhiteBalance, v: core.Trigger()})
	}
}

func (c *Controller) frame(now time.Time) frame {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wbTickLocked(now)
	c.settleLocked(now)
	irisWin, irisC := irisView(c.cam, c.knobs)
	awb := c.wbAWBLocked()
	hint := func(p pickup, id core.ControlID) pickHint {
		_, _, declared, known := c.cam.get(id)
		return pickHint{declared: declared, known: known, picked: p.picked, pos: p.pos, havePos: p.havePos}
	}
	return render(view{
		rig: c.rig, cam: c.cam, tally: c.tally, held: c.held,
		iris: hint(c.iris, irisLeverID(c.cam)), mb: hint(c.mb, core.MasterBlack),
		irisWin: irisWin, irisCentre: irisC,
		blink:  now.UnixMilli()/250%2 == 0,
		alarm:  now.Before(c.alarmUntil),
		knobs:  c.knobs,
		coarse: previewing(c.coarseAt, now),
		sens:   previewing(c.sensAt, now),
		wb:     c.wbMode,
		// held W/B on a camera with AWB: blink, then steady once fired
		wbHolding: !c.wbDownAt.IsZero() && !c.wbFired && awb,
		wbFired:   !c.wbDownAt.IsZero() && c.wbFired && awb,
		armed:     c.storeArmed,
		off:       c.off,
	})
}

// previewHold is how long the panel previews COARSE or SENS after the
// knob last moved.
const previewHold = 5 * time.Second

func previewing(at, now time.Time) bool { return !at.IsZero() && now.Before(at.Add(previewHold)) }

func slotCamera(r app.Rig, slot int) *app.CameraView {
	for _, s := range r.Slots {
		if s.Slot == slot {
			return s.Camera
		}
	}
	return nil
}

// assignedLocked reports whether slot has a camera in the last known rig.
func (c *Controller) assignedLocked(slot int) bool {
	for _, sl := range c.rig.Slots {
		if sl.Slot == slot {
			return sl.Camera != nil
		}
	}
	return false
}
