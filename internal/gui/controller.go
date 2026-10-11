// Package gui is the native desktop interface (Fyne). The Controller holds the
// UI-side state (rig, controls, values, notices), the pages and widgets
// subscribe to its changes. Everything that touches widgets runs on the Fyne
// thread; camera calls run on worker goroutines.
package gui

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/fr"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// API is the part of app.Service the GUI uses (*app.Service implements it).
type API interface {
	Rig() app.Rig
	Models() []core.ModelInfo
	AssignSlot(slot int, in app.SlotInput) (app.Slot, error)
	ClearSlot(slot int) error
	SelectSlot(slot int) error
	Controls(slot int) ([]app.ControlView, error)
	SetControl(slot int, id string, v core.Value) error
	ConnectSlot(slot int) error
	PanelStatus() app.PanelStatus
}

var _ API = (*app.Service)(nil)

// ChangeKind says what changed in the Controller.
type ChangeKind int

const (
	ChRig    ChangeKind = iota // assignments, active slot or connection state
	ChPanel                    // USB panel detection
	ChSlot                     // a slot's camera or control list changed
	ChValue                    // one control value changed (Slot, ID)
	ChNotice                   // status-bar notice
)

// Change is passed to subscribers.
type Change struct {
	Kind ChangeKind
	Slot int
	ID   core.ControlID
}

// slotData is what the GUI knows about one slot.
type slotData struct {
	camera *app.CameraView
	specs  []core.ControlSpec
	byID   map[core.ControlID]int
	values map[core.ControlID]core.Value
	loaded bool
	gen    int // bumped on every (re)load to drop stale answers
}

func (s *slotData) spec(id core.ControlID) (core.ControlSpec, bool) {
	if s == nil {
		return core.ControlSpec{}, false
	}
	i, ok := s.byID[id]
	if !ok {
		return core.ControlSpec{}, false
	}
	return s.specs[i], true
}

// Options tune a Controller; the zero value is the production setup.
type Options struct {
	// UI runs f on the Fyne thread (default fyne.Do).
	UI func(f func())
	// Go runs f on a worker goroutine (default go f()).
	Go func(f func())
	// WriteInterval is the minimum delay between two writes of one control.
	WriteInterval time.Duration
	// NoticeTTL is how long a notice stays (default 8 s, 0 = default).
	NoticeTTL time.Duration
}

// Controller is the UI-side model.
type Controller struct {
	api   API
	ui    func(func())
	async func(func())
	thr   *widgets.Throttler
	ttl   time.Duration

	rig    app.Rig
	panel  app.PanelStatus
	models []core.ModelInfo
	slots  [app.SlotCount]*slotData
	notice fr.Notice
	// latest write requested per slot/control, to tell stale answers apart.
	latest map[string]core.Value

	lmu       sync.Mutex
	listeners map[int]func(Change)
	nextL     int
}

// NewController builds the model. Call Load once before showing the UI.
func NewController(api API, o Options) *Controller {
	if o.UI == nil {
		o.UI = fyne.Do
	}
	if o.Go == nil {
		o.Go = func(f func()) { go f() }
	}
	if o.WriteInterval == 0 {
		o.WriteInterval = 33 * time.Millisecond // about 30 writes per second per control
	}
	if o.NoticeTTL == 0 {
		o.NoticeTTL = 8 * time.Second
	}
	return &Controller{
		api: api, ui: o.UI, async: o.Go, ttl: o.NoticeTTL,
		thr:       widgets.NewThrottler(o.WriteInterval),
		latest:    map[string]core.Value{},
		listeners: map[int]func(Change){},
		rig:       app.Rig{Slots: make([]app.Slot, app.SlotCount)},
	}
}

// Load reads the initial state from the service.
func (c *Controller) Load() {
	c.models = c.api.Models()
	c.panel = c.api.PanelStatus()
	c.applyRig(c.api.Rig())
}

// --- subscriptions -------------------------------------------------------------

// Subscribe registers fn for changes; call the returned func to stop.
func (c *Controller) Subscribe(fn func(Change)) (cancel func()) {
	c.lmu.Lock()
	defer c.lmu.Unlock()
	id := c.nextL
	c.nextL++
	c.listeners[id] = fn
	return func() {
		c.lmu.Lock()
		delete(c.listeners, id)
		c.lmu.Unlock()
	}
}

func (c *Controller) notify(ch Change) {
	c.lmu.Lock()
	fns := make([]func(Change), 0, len(c.listeners))
	for _, f := range c.listeners {
		fns = append(fns, f)
	}
	c.lmu.Unlock()
	for _, f := range fns {
		f(ch)
	}
}

// --- read access -----------------------------------------------------------------

func (c *Controller) Rig() app.Rig             { return c.rig }
func (c *Controller) Panel() app.PanelStatus   { return c.panel }
func (c *Controller) Models() []core.ModelInfo { return c.models }
func (c *Controller) Notice() fr.Notice        { return c.notice }
func (c *Controller) Active() int              { return c.rig.Active }
func (c *Controller) Camera(slot int) *app.CameraView {
	if slot < 1 || slot > app.SlotCount {
		return nil
	}
	return c.rig.Slots[slot-1].Camera
}

// Specs returns the declared controls of a slot's camera (nil when empty).
func (c *Controller) Specs(slot int) []core.ControlSpec {
	if sd := c.data(slot); sd != nil {
		return sd.specs
	}
	return nil
}

// Spec returns one declared control.
func (c *Controller) Spec(slot int, id core.ControlID) (core.ControlSpec, bool) {
	return c.data(slot).spec(id)
}

// Has reports whether the camera declares id.
func (c *Controller) Has(slot int, id core.ControlID) bool {
	_, ok := c.Spec(slot, id)
	return ok
}

// Value returns the last known value, nil while unknown.
func (c *Controller) Value(slot int, id core.ControlID) *core.Value {
	sd := c.data(slot)
	if sd == nil {
		return nil
	}
	if v, ok := sd.values[id]; ok {
		return &v
	}
	return nil
}

// Loaded reports whether the controls of the slot have been read.
func (c *Controller) Loaded(slot int) bool {
	sd := c.data(slot)
	return sd != nil && sd.loaded
}

func (c *Controller) data(slot int) *slotData {
	if slot < 1 || slot > app.SlotCount {
		return nil
	}
	return c.slots[slot-1]
}

// --- events from the service -----------------------------------------------------

// Emit implements app.Emitter. It may be called from any goroutine.
func (c *Controller) Emit(name string, payload any) {
	c.ui(func() { c.handle(name, payload) })
}

func (c *Controller) handle(name string, payload any) {
	switch p := payload.(type) {
	case app.PanelStatus:
		c.applyPanel(p)
	case *app.PanelStatus:
		c.applyPanel(*p)
	case app.Rig:
		c.applyRig(p)
	case *app.Rig:
		c.applyRig(*p)
	case app.CameraEvent:
		c.applyCameraEvent(p)
	case *app.CameraEvent:
		c.applyCameraEvent(*p)
	}
}

func (c *Controller) applyPanel(p app.PanelStatus) {
	if c.panel == p {
		return
	}
	c.panel = p
	c.notify(Change{Kind: ChPanel})
}

func cameraKey(v *app.CameraView) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%s|%s|%s|%d|%s|%s|%t", v.Model, v.Name, v.Host, v.Port, v.Transport, v.Username, v.HasPassword)
}

func (c *Controller) applyRig(r app.Rig) {
	old := c.rig
	if len(r.Slots) != app.SlotCount {
		return
	}
	c.rig = r
	for i := 0; i < app.SlotCount; i++ {
		slot := i + 1
		var prev *app.CameraView
		if len(old.Slots) == app.SlotCount {
			prev = old.Slots[i].Camera
		}
		cur := r.Slots[i].Camera
		switch {
		case cur == nil:
			if c.slots[i] != nil {
				c.slots[i] = nil
				c.notify(Change{Kind: ChSlot, Slot: slot})
			}
		case cameraKey(prev) != cameraKey(cur) || c.slots[i] == nil:
			c.slots[i] = &slotData{camera: cur, gen: gen(c.slots[i]) + 1}
			c.loadControls(slot)
		case prev != nil && prev.Status != cur.Status:
			c.slots[i].camera = cur
			if cur.Status.State == core.StateConnected {
				c.loadControls(slot)
			} else {
				// connection lost: the values are no longer current
				c.slots[i].values = nil
				c.slots[i].gen++
				c.notify(Change{Kind: ChSlot, Slot: slot})
			}
		default:
			c.slots[i].camera = cur
		}
	}
	c.notify(Change{Kind: ChRig})
}

func gen(s *slotData) int {
	if s == nil {
		return 0
	}
	return s.gen
}

func (c *Controller) applyCameraEvent(e app.CameraEvent) {
	sd := c.data(e.Slot)
	if sd == nil {
		return
	}
	ev := e.Event
	if ev.Status != nil {
		c.RefreshRig()
		return
	}
	if ev.Control == "" {
		return
	}
	if _, ok := sd.spec(ev.Control); !ok {
		return
	}
	if sd.values == nil {
		sd.values = map[core.ControlID]core.Value{}
	}
	sd.values[ev.Control] = ev.Value
	c.notify(Change{Kind: ChValue, Slot: e.Slot, ID: ev.Control})
}

// RefreshRig re-reads the rig from the service (used by the poller and after
// calls whose effect the service does not announce).
func (c *Controller) RefreshRig() {
	rig := c.api.Rig()
	c.applyRig(rig)
}

// Poll refreshes the rig on every tick until ctx ends. The service does not
// push camera status yet, so this keeps the connection state current.
func (c *Controller) Poll(ctx context.Context, every time.Duration) {
	go func() {
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				rig := c.api.Rig()
				c.ui(func() { c.applyRig(rig) })
			}
		}
	}()
}

// loadControls reads the slot's controls on a worker and applies them.
func (c *Controller) loadControls(slot int) {
	sd := c.data(slot)
	if sd == nil {
		return
	}
	g := sd.gen
	c.async(func() {
		views, err := c.api.Controls(slot)
		c.ui(func() {
			cur := c.data(slot)
			if cur == nil || cur.gen != g {
				return
			}
			if err != nil {
				c.setNotice(fr.NoticeFor(err))
				return
			}
			cur.specs = make([]core.ControlSpec, len(views))
			cur.byID = make(map[core.ControlID]int, len(views))
			cur.values = map[core.ControlID]core.Value{}
			for i, v := range views {
				cur.specs[i] = v.Spec
				cur.byID[v.Spec.ID] = i
				if v.Value != nil {
					cur.values[v.Spec.ID] = *v.Value
				}
			}
			cur.loaded = true
			c.notify(Change{Kind: ChSlot, Slot: slot})
		})
	})
}

// --- commands ---------------------------------------------------------------------

// Select makes slot the active camera (0 = none).
func (c *Controller) Select(slot int) {
	if slot == c.rig.Active {
		return
	}
	// optimistic: tabs and headers update at once
	c.rig.Active = slot
	c.notify(Change{Kind: ChRig})
	c.async(func() {
		if err := c.api.SelectSlot(slot); err != nil {
			c.ui(func() {
				c.setNotice(fr.NoticeFor(err))
				c.RefreshRig()
			})
		}
	})
}

// Write sends one control value. Continuous drags are throttled; the final
// value is always sent. On failure the control returns to its last known value
// and a notice is shown.
func (c *Controller) Write(slot int, id core.ControlID, v core.Value) {
	key := fmt.Sprintf("%d/%s", slot, id)
	c.latest[key] = v
	c.thr.Do(key, func() {
		err := c.api.SetControl(slot, string(id), v)
		c.ui(func() { c.writeDone(slot, id, key, v, err) })
	})
}

func (c *Controller) writeDone(slot int, id core.ControlID, key string, v core.Value, err error) {
	if err != nil {
		delete(c.latest, key)
		c.setNotice(fr.NoticeFor(err))
		c.notify(Change{Kind: ChValue, Slot: slot, ID: id}) // back to the last known value
		return
	}
	sd := c.data(slot)
	if sd == nil {
		return
	}
	if v.Kind == core.KindAction {
		return
	}
	if sd.values == nil {
		sd.values = map[core.ControlID]core.Value{}
	}
	sd.values[id] = v
	if last, ok := c.latest[key]; ok && last == v {
		delete(c.latest, key)
		c.notify(Change{Kind: ChValue, Slot: slot, ID: id})
	}
}

// Connect asks the slot's camera to connect.
func (c *Controller) Connect(slot int) {
	c.async(func() {
		err := c.api.ConnectSlot(slot)
		c.ui(func() {
			if err != nil {
				c.setNotice(fr.NoticeFor(err))
			}
			c.RefreshRig()
		})
	})
}

// Assign creates or edits a slot. The error is returned as is so the form can
// show it inline.
func (c *Controller) Assign(slot int, in app.SlotInput) error {
	if _, err := c.api.AssignSlot(slot, in); err != nil {
		return err
	}
	c.RefreshRig()
	return nil
}

// Clear removes the camera of a slot.
func (c *Controller) Clear(slot int) error {
	if err := c.api.ClearSlot(slot); err != nil {
		return err
	}
	c.RefreshRig()
	return nil
}

// SetNotice shows a message in the status bar.
func (c *Controller) SetNotice(n fr.Notice) { c.setNotice(n) }

func (c *Controller) setNotice(n fr.Notice) {
	c.notice = n
	c.notify(Change{Kind: ChNotice})
	if n.Text == "" {
		return
	}
	time.AfterFunc(c.ttl, func() {
		c.ui(func() {
			if c.notice == n {
				c.notice = fr.Notice{}
				c.notify(Change{Kind: ChNotice})
			}
		})
	})
}

// WaitWrites blocks until queued writes are done (tests).
func (c *Controller) WaitWrites() { c.thr.Wait() }

// IsAPIError reports whether err is an *app.Error with the given code.
func IsAPIError(err error, code string) bool {
	var ae *app.Error
	return errors.As(err, &ae) && ae.Code == code
}
