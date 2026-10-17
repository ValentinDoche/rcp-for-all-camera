package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Emitter pushes events to the GUI.
type Emitter interface {
	Emit(name string, payload any)
}

// Options are Service dependencies. Registry, Store and Secrets are required.
type Options struct {
	Registry *core.Registry
	Store    Store
	Secrets  Secrets
	Logger   *slog.Logger
	// Interfaces lists the local IPv4 addresses (sysinfo.LocalIPv4s).
	Interfaces func() []NetInterface
	// AutoStart controls start with Windows; nil = not available.
	AutoStart AutoStarter
	// LogLevel is the level of the app logger, driven by Settings.LogLevel.
	LogLevel *slog.LevelVar
	// LogDir is the folder holding the log file (shown in the GUI).
	LogDir string
	// Go runs background work (default: a new goroutine). Tests run it inline.
	Go func(func())
	// Panels switches the panel source (USB, simulator); nil = no choice.
	Panels PanelSources
	// CommandTimeout bounds every camera call made through the Service
	// (default 3 s).
	CommandTimeout time.Duration
}

// defaultCommandTimeout bounds every camera call made through the Service.
const defaultCommandTimeout = 3 * time.Second

// Service is the GUI-facing API. Lifecycle hooks are kept off it (see Host)
// so its method set is exactly the API. All methods are
// safe for concurrent use and return *Error on failure.
type Service struct {
	reg     *core.Registry
	store   Store
	secrets Secrets
	log     *slog.Logger

	interfaces  func() []NetInterface
	autoStart   AutoStarter
	levelVar    *slog.LevelVar
	logDir      string
	goFn        func(func())
	panels      PanelSources
	panelSource string        // saved panel choice ("" = automatic USB)
	timeout     time.Duration // Options.CommandTimeout

	mu         sync.Mutex
	slots      [SlotCount]*slotState
	presets    [SlotCount]slotPresets
	active     int
	bindIP     string
	lock       bool
	logLevel   string
	panel      PanelStatus
	storeArmed bool // shared Store arming, see SetStoreArmed
	emit       Emitter
}

type slotState struct {
	cfg   core.CameraConfig // Password filled from secrets, never persisted
	info  core.ModelInfo
	cam   core.Camera
	unsub func() // camera event subscription
}

// NewService loads the saved rig. Saved entries that no longer validate
// (unknown model, bad host...) are dropped with a warning, so a stale or
// hand-edited file never prevents startup.
func NewService(o Options) (*Service, error) {
	if o.Registry == nil || o.Store == nil || o.Secrets == nil {
		return nil, fmt.Errorf("app: Registry, Store and Secrets are required")
	}
	if o.Logger == nil {
		o.Logger = slog.Default()
	}
	s := &Service{reg: o.Registry, store: o.Store, secrets: o.Secrets, log: o.Logger, emit: nopEmitter{},
		interfaces: o.Interfaces, autoStart: o.AutoStart, levelVar: o.LogLevel, logDir: o.LogDir, goFn: o.Go,
		panels: o.Panels}
	s.timeout = o.CommandTimeout
	if s.timeout <= 0 {
		s.timeout = defaultCommandTimeout
	}
	if s.goFn == nil {
		s.goFn = func(f func()) { go f() }
	}
	rig, err := o.Store.Load()
	if err != nil {
		s.log.Warn("config not loaded, starting with an empty rig", "err", err)
		rig = SavedRig{}
	}
	s.bindIP = rig.BindIP
	s.restorePanelSource(rig.Panel)
	s.lock = rig.OperatorLock == nil || *rig.OperatorLock
	s.logLevel = LogNormal
	if rig.LogLevel == LogDebug {
		s.logLevel = LogDebug
	}
	s.applyLogLevel()
	for i, cfg := range rig.Slots {
		if cfg == nil {
			continue
		}
		slot := i + 1
		c := *cfg
		if pw, ok, err := s.secrets.Get(secretKey(slot)); err != nil {
			s.log.Warn("password not readable", "slot", slot, "err", err)
		} else if ok {
			c.Password = pw
		}
		st, err := s.build(c)
		if err != nil {
			s.log.Warn("dropping saved camera", "slot", slot, "model", c.Model, "err", err)
			continue
		}
		s.slots[i] = st
	}
	s.active = rig.Active
	if s.active > 0 && s.slots[s.active-1] == nil {
		s.active = 0 // only an assigned slot can be selected
	}
	s.loadPresets(rig.Presets)
	return s, nil
}

func (s *Service) setEmitter(e Emitter) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e == nil {
		e = nopEmitter{}
	}
	s.emit = e
}

// PanelStatus returns the last panel detection result.
func (s *Service) PanelStatus() PanelStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.panel
}

// Models returns every known camera model in display order.
func (s *Service) Models() []core.ModelInfo { return s.reg.Models() }

// Rig returns the 8 slots and the active one.
func (s *Service) Rig() Rig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rigLocked()
}

// AssignSlot creates or replaces the camera on slot. It is a command: the
// new state is read back with Rig.
func (s *Service) AssignSlot(slot int, in SlotInput) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	s.mu.Lock()
	cfg := core.CameraConfig{
		Model: in.Model, Name: in.Name, Host: in.Host, Port: in.Port,
		Username: in.Username, PowerZoom: in.PowerZoom,
	}
	switch {
	case in.Password != nil:
		cfg.Password = *in.Password
	case s.slots[slot-1] != nil:
		cfg.Password = s.slots[slot-1].cfg.Password
	}
	st, err := s.build(cfg)
	if err != nil {
		s.mu.Unlock()
		return toAPIError(err)
	}
	if err := s.storePassword(slot, cfg.Password); err != nil {
		s.mu.Unlock()
		st.close()
		return toAPIError(err)
	}
	old := s.slots[slot-1]
	oldPresets := s.presets[slot-1]
	s.slots[slot-1] = st
	// Presets belong to a model: a different camera model starts empty, an
	// edit of the same model (name, address...) keeps them.
	if old != nil && old.info.ID != st.info.ID {
		s.presets[slot-1] = slotPresets{}
	}
	if err := s.saveLocked(); err != nil {
		s.slots[slot-1], s.presets[slot-1] = old, oldPresets
		// Keep the secret store in line with the rig that stays on disk.
		oldPassword := ""
		if old != nil {
			oldPassword = old.cfg.Password
		}
		if err := s.storePassword(slot, oldPassword); err != nil {
			s.log.Warn("password not restored after failed save", "slot", slot, "err", err)
		}
		s.mu.Unlock()
		st.close()
		return toAPIError(err)
	}
	if old != nil {
		old.close()
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	s.connectAsync(slot, st.cam)
	s.log.Info("camera assigned", "slot", slot, "model", cfg.Model, "host", cfg.Host)
	emit.Emit(EventRigChanged, rig)
	return nil
}

// ClearSlot removes the camera from slot. Clearing an empty slot succeeds.
func (s *Service) ClearSlot(slot int) error {
	if err := checkSlot(slot); err != nil {
		return err
	}
	s.mu.Lock()
	old := s.slots[slot-1]
	if old == nil {
		s.mu.Unlock()
		return nil
	}
	oldPresets := s.presets[slot-1]
	s.slots[slot-1], s.presets[slot-1] = nil, slotPresets{}
	prevActive := s.active
	if s.active == slot {
		s.active = 0
	}
	if err := s.saveLocked(); err != nil {
		s.slots[slot-1], s.presets[slot-1], s.active = old, oldPresets, prevActive
		s.mu.Unlock()
		return toAPIError(err)
	}
	if err := s.secrets.Delete(secretKey(slot)); err != nil {
		s.log.Warn("password not deleted", "slot", slot, "err", err)
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	old.close()
	emit.Emit(EventRigChanged, rig)
	return nil
}

// SelectSlot makes slot the active camera; 0 selects none. An empty slot
// cannot be selected (EMPTY_SLOT).
func (s *Service) SelectSlot(slot int) error {
	if slot != 0 {
		if err := checkSlot(slot); err != nil {
			return err
		}
	}
	s.mu.Lock()
	if s.active == slot {
		s.mu.Unlock()
		return nil
	}
	if slot != 0 && s.slots[slot-1] == nil {
		s.mu.Unlock()
		return &Error{Code: CodeEmptySlot, Message: fmt.Sprintf("slot %d has no camera", slot)}
	}
	prev := s.active
	s.active = slot
	if err := s.saveLocked(); err != nil {
		s.active = prev
		s.mu.Unlock()
		return toAPIError(err)
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	emit.Emit(EventRigChanged, rig)
	return nil
}

// Controls returns every control of the slot's camera.
func (s *Service) Controls(slot int) ([]ControlView, error) {
	cam, err := s.camera(slot)
	if err != nil {
		return nil, err
	}
	connected := cam.Status().State == core.StateConnected
	specs := cam.Controls()
	out := make([]ControlView, 0, len(specs))
	// One deadline for the whole read: a slow camera must not block the GUI
	// for the command timeout per control.
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	for _, sp := range specs {
		cv := ControlView{Spec: sp}
		if connected && ctx.Err() == nil {
			if v, err := cam.Get(ctx, sp.ID); err == nil {
				cv.Value = &v
			}
		}
		out = append(out, cv)
	}
	return out, nil
}

// Lens returns what the slot's camera reports about its lens. It is zero for
// a camera that does not report it, or while it is offline.
func (s *Service) Lens(slot int) (core.Lens, error) {
	cam, err := s.camera(slot)
	if err != nil {
		return core.Lens{}, err
	}
	if lr, ok := cam.(core.LensReporter); ok {
		return lr.Lens(), nil
	}
	return core.Lens{}, nil
}

// Page is the layout of the slot's camera page, as its module describes
// it in ui.go (core.StandardPage for a camera that does not).
func (s *Service) Page(slot int) (core.Page, error) {
	cam, err := s.camera(slot)
	if err != nil {
		return core.Page{}, err
	}
	if pr, ok := cam.(core.PageReporter); ok {
		return pr.Page(), nil
	}
	return core.StandardPage(), nil
}

// SetControl writes one control of the slot's camera.
func (s *Service) SetControl(slot int, id string, v core.Value) error {
	return s.setControl(slot, id, v, false)
}

// setControl writes a control; confirmed skips the operator lock.
func (s *Service) setControl(slot int, id string, v core.Value, confirmed bool) error {
	if err := core.CheckControlID(core.ControlID(id)); err != nil {
		return toAPIError(err)
	}
	if err := v.Check(); err != nil {
		return toAPIError(err)
	}
	cam, err := s.camera(slot)
	if err != nil {
		return err
	}
	if !confirmed {
		s.mu.Lock()
		locked := s.lock
		s.mu.Unlock()
		if locked {
			if spec, ok := findSpec(cam.Controls(), core.ControlID(id)); ok && spec.Guarded {
				return &Error{Code: CodeConfirmRequired, Message: "this control needs confirmation while the operator lock is on"}
			}
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	s.log.Debug("set control", "slot", slot, "control", id, "value", v)
	if err := cam.Set(ctx, core.ControlID(id), v); err != nil {
		ae := toAPIError(err)
		s.commandFailed(slot, core.ControlID(id), ae)
		return ae
	}
	return nil
}

// commandFailed announces a failed write (EventCommandFailed).
func (s *Service) commandFailed(slot int, id core.ControlID, err error) {
	code := CodeInternal
	var ae *Error
	if errors.As(err, &ae) {
		code = ae.Code
	}
	s.mu.Lock()
	emit := s.emit
	s.mu.Unlock()
	emit.Emit(EventCommandFailed, CommandFailed{Slot: slot, Control: id, Code: code})
}

// ConnectSlot starts connecting the slot's camera.
func (s *Service) ConnectSlot(slot int) error {
	cam, err := s.camera(slot)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	return toAPIError(cam.Connect(ctx))
}

// --- internals -------------------------------------------------------------

func (s *Service) setPanel(p PanelStatus) {
	s.mu.Lock()
	if s.panel == p {
		s.mu.Unlock()
		return
	}
	s.panel = p
	emit := s.emit
	s.mu.Unlock()
	s.log.Info("panel status", "connected", p.Connected, "active", p.Active, "detail", p.Detail)
	emit.Emit(EventPanelStatus, p)
}

// closeAll closes every camera. Cameras are closed outside the lock: a
// driver may emit a last status event while closing.
func (s *Service) closeAll() {
	s.mu.Lock()
	slots := s.slots
	s.mu.Unlock()
	for _, st := range slots {
		if st != nil {
			st.close()
		}
	}
}

func (s *Service) camera(slot int) (core.Camera, error) {
	if err := checkSlot(slot); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.slots[slot-1]
	if st == nil {
		return nil, &Error{Code: CodeEmptySlot, Message: fmt.Sprintf("slot %d has no camera", slot)}
	}
	return st.cam, nil
}

// build validates cfg, fills the model defaults and creates the camera.
func (s *Service) build(cfg core.CameraConfig) (*slotState, error) {
	if len(cfg.Model) > core.MaxIDLen {
		return nil, fmt.Errorf("model id longer than %d characters: %w", core.MaxIDLen, core.ErrUnknownModel)
	}
	info, ok := s.reg.Model(cfg.Model)
	if !ok {
		return nil, fmt.Errorf("%q: %w", cfg.Model, core.ErrUnknownModel)
	}
	if err := core.ValidateConfig(info, cfg); err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = info.DefaultPort
	}
	if info.NoAddress { // nothing to reach: no address is kept
		cfg.Host, cfg.Port = "", 0
	}
	// A built-in lens has its zoom or not whatever the user says.
	cfg.PowerZoom = cfg.PowerZoom && info.InterchangeableLens
	cfg.BindIP = s.bindIP
	cam, err := s.reg.New(cfg)
	if err != nil {
		return nil, err
	}
	st := &slotState{cfg: cfg, info: info, cam: cam}
	st.unsub = cam.Subscribe(func(e core.Event) { s.forward(cam, e) })
	return st, nil
}

func (s *Service) storePassword(slot int, pw string) error {
	if pw == "" {
		return s.secrets.Delete(secretKey(slot))
	}
	return s.secrets.Set(secretKey(slot), pw)
}

func (s *Service) saveLocked() error {
	lock := s.lock
	rig := SavedRig{Active: s.active, BindIP: s.bindIP, OperatorLock: &lock,
		Panel: s.panelSource}
	if s.logLevel == LogDebug {
		rig.LogLevel = LogDebug
	}
	for i, st := range s.slots {
		if st != nil {
			c := st.cfg
			c.Password = ""
			rig.Slots[i] = &c
		}
	}
	rig.Presets = s.savedPresetsLocked()
	return s.store.Save(rig)
}

func (s *Service) rigLocked() Rig {
	r := Rig{Slots: make([]Slot, SlotCount), Active: s.active}
	for i, st := range s.slots {
		r.Slots[i] = Slot{Slot: i + 1, Presets: s.presets[i].numbers()}
		if st != nil {
			r.Slots[i].Camera = viewOf(st)
		}
	}
	return r
}

func viewOf(st *slotState) *CameraView {
	return &CameraView{
		Model: st.info.ID, Brand: st.info.Brand, ModelName: st.info.Name, Support: st.info.Support,
		Name: st.cfg.Name, Host: st.cfg.Host, Port: st.cfg.Port,
		Username: st.cfg.Username, HasPassword: st.cfg.Password != "",
		PowerZoom: st.cfg.PowerZoom, InterchangeableLens: st.info.InterchangeableLens,
		Status: st.cam.Status(),
	}
}

func checkSlot(slot int) error {
	if slot < 1 || slot > SlotCount {
		return &Error{Code: CodeInvalidSlot, Message: fmt.Sprintf("slot must be 1 to %d", SlotCount)}
	}
	return nil
}

// secretKey is the Secrets key holding the password of a slot's camera.
func secretKey(slot int) string { return "camera/" + strconv.Itoa(slot) }

type nopEmitter struct{}

func (nopEmitter) Emit(string, any) {}

// close ends the event subscription, then closes the camera.
func (st *slotState) close() {
	if st.unsub != nil {
		st.unsub()
	}
	st.cam.Close()
}

// forward emits a camera event tagged with the camera's current slot. Events
// from a camera that is no longer in a slot (replaced, cleared) are dropped.
func (s *Service) forward(cam core.Camera, e core.Event) {
	s.mu.Lock()
	slot := 0
	for i, st := range s.slots {
		if st != nil && st.cam == cam {
			slot = i + 1
			break
		}
	}
	emit := s.emit
	s.mu.Unlock()
	if slot == 0 {
		return
	}
	emit.Emit(EventCamera, CameraEvent{Slot: slot, Event: e})
}
