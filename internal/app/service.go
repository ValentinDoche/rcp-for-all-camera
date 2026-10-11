package app

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Emitter pushes events to the GUI.
type Emitter interface {
	Emit(name string, payload any)
}

// Store loads and saves the rig (config.FileStore in production).
type Store interface {
	Load() (config.Rig, error)
	Save(config.Rig) error
}

// Options are Service dependencies. Registry, Store and Secrets are required.
type Options struct {
	Registry *core.Registry
	Store    Store
	Secrets  config.Secrets
	Logger   *slog.Logger
	// Interfaces lists the local IPv4 addresses (sysinfo.LocalIPv4s).
	Interfaces func() []NetInterface
	// AutoStart controls start with Windows; nil = not available.
	AutoStart AutoStarter
}

// commandTimeout bounds every camera call made from the GUI (a variable only
// so tests can shorten it).
var commandTimeout = 3 * time.Second

// Service is the GUI-facing API. Lifecycle hooks are kept off it (see Host)
// so its method set is exactly the API. All methods are
// safe for concurrent use and return *Error on failure.
type Service struct {
	reg     *core.Registry
	store   Store
	secrets config.Secrets
	log     *slog.Logger

	interfaces func() []NetInterface
	autoStart  AutoStarter

	mu     sync.Mutex
	slots  [SlotCount]*slotState
	active int
	bindIP string
	panel  PanelStatus
	emit   Emitter
}

type slotState struct {
	cfg  core.CameraConfig // Password filled from secrets, never persisted
	info core.ModelInfo
	cam  core.Camera
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
		interfaces: o.Interfaces, autoStart: o.AutoStart}
	rig, err := o.Store.Load()
	if err != nil {
		s.log.Warn("config not loaded, starting with an empty rig", "err", err)
		rig = config.Rig{Version: config.Version}
	}
	s.bindIP = rig.BindIP
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

// AssignSlot creates or replaces the camera on slot.
func (s *Service) AssignSlot(slot int, in SlotInput) (Slot, error) {
	if err := checkSlot(slot); err != nil {
		return Slot{}, err
	}
	s.mu.Lock()
	cfg := core.CameraConfig{
		Model: in.Model, Name: in.Name, Host: in.Host, Port: in.Port,
		Username: in.Username,
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
		return Slot{}, toAPIError(err)
	}
	if err := s.storePassword(slot, cfg.Password); err != nil {
		s.mu.Unlock()
		st.cam.Close()
		return Slot{}, toAPIError(err)
	}
	old := s.slots[slot-1]
	s.slots[slot-1] = st
	if err := s.saveLocked(); err != nil {
		s.slots[slot-1] = old
		// Keep the secret store in line with the rig that stays on disk.
		oldPassword := ""
		if old != nil {
			oldPassword = old.cfg.Password
		}
		if err := s.storePassword(slot, oldPassword); err != nil {
			s.log.Warn("password not restored after failed save", "slot", slot, "err", err)
		}
		s.mu.Unlock()
		st.cam.Close()
		return Slot{}, toAPIError(err)
	}
	if old != nil {
		old.cam.Close()
	}
	out := Slot{Slot: slot, Camera: viewOf(st)}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	s.log.Info("camera assigned", "slot", slot, "model", cfg.Model, "host", cfg.Host)
	emit.Emit(EventRigChanged, rig)
	return out, nil
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
	s.slots[slot-1] = nil
	prevActive := s.active
	if s.active == slot {
		s.active = 0
	}
	if err := s.saveLocked(); err != nil {
		s.slots[slot-1], s.active = old, prevActive
		s.mu.Unlock()
		return toAPIError(err)
	}
	if err := s.secrets.Delete(secretKey(slot)); err != nil {
		s.log.Warn("password not deleted", "slot", slot, "err", err)
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	old.cam.Close()
	emit.Emit(EventRigChanged, rig)
	return nil
}

// SelectSlot makes slot the active camera; 0 selects none.
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
	// for commandTimeout per control.
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
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

// SetControl writes one control of the slot's camera.
func (s *Service) SetControl(slot int, id string, v core.Value) error {
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
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()
	s.log.Debug("set control", "slot", slot, "control", id, "value", v)
	return toAPIError(cam.Set(ctx, core.ControlID(id), v))
}

// ConnectSlot starts connecting the slot's camera.
func (s *Service) ConnectSlot(slot int) error {
	cam, err := s.camera(slot)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
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
	s.log.Info("panel status", "connected", p.Connected)
	emit.Emit(EventPanelStatus, p)
}

func (s *Service) closeAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.slots {
		if st != nil {
			st.cam.Close()
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
	cfg.BindIP = s.bindIP
	cam, err := s.reg.New(cfg)
	if err != nil {
		return nil, err
	}
	return &slotState{cfg: cfg, info: info, cam: cam}, nil
}

func (s *Service) storePassword(slot int, pw string) error {
	if pw == "" {
		return s.secrets.Delete(secretKey(slot))
	}
	return s.secrets.Set(secretKey(slot), pw)
}

func (s *Service) saveLocked() error {
	rig := config.Rig{Version: config.Version, Active: s.active, BindIP: s.bindIP}
	for i, st := range s.slots {
		if st != nil {
			c := st.cfg
			c.Password = ""
			rig.Slots[i] = &c
		}
	}
	return s.store.Save(rig)
}

func (s *Service) rigLocked() Rig {
	r := Rig{Slots: make([]Slot, SlotCount), Active: s.active}
	for i, st := range s.slots {
		r.Slots[i] = Slot{Slot: i + 1}
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
