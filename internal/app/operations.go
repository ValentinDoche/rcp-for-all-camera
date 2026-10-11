package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Log levels for Settings.LogLevel.
const (
	LogNormal = "normal"
	LogDebug  = "debug"
)

// ExportFormat tags exported rig documents.
const ExportFormat = "rcp-for-all-camera/rig"

// exportDoc is the shareable rig: cameras and selection only. Passwords and
// machine settings (bind address, log level, lock) never leave the PC.
type exportDoc struct {
	Format  string               `json:"format"`
	Version int                  `json:"version"`
	Slots   []*core.CameraConfig `json:"slots"`
	Active  int                  `json:"active"`
}

// --- auto-connect ------------------------------------------------------------------

// connectAll starts connecting every assigned camera (called at startup).
func (s *Service) connectAll() {
	s.mu.Lock()
	var cams []slotCam
	for i, st := range s.slots {
		if st != nil {
			cams = append(cams, slotCam{i + 1, st.cam})
		}
	}
	s.mu.Unlock()
	for _, c := range cams {
		s.connectAsync(c.slot, c.cam)
	}
}

type slotCam struct {
	slot int
	cam  core.Camera
}

// connectAsync connects a camera in the background. Drivers keep
// reconnecting by themselves after the first attempt (core.Camera contract).
func (s *Service) connectAsync(slot int, cam core.Camera) {
	s.goFn(func() {
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		defer cancel()
		if err := cam.Connect(ctx); err != nil {
			level := slog.LevelWarn
			if errors.Is(err, core.ErrNotImplemented) {
				level = slog.LevelDebug // expected while drivers are placeholders
			}
			s.log.Log(ctx, level, "camera connect", "slot", slot, "err", err)
		}
	})
}

// --- operator lock -----------------------------------------------------------------

// SetOperatorLock turns the confirmation of guarded controls on or off.
func (s *Service) SetOperatorLock(on bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.lock
	s.lock = on
	if err := s.saveLocked(); err != nil {
		s.lock = prev
		return toAPIError(err)
	}
	return nil
}

// ConfirmSetControl writes a control after the user confirmed it; it is
// SetControl without the operator lock.
func (s *Service) ConfirmSetControl(slot int, id string, v core.Value) error {
	return s.setControl(slot, id, v, true)
}

// --- logs ----------------------------------------------------------------------------

// SetLogLevel switches the log file between LogNormal and LogDebug.
func (s *Service) SetLogLevel(level string) error {
	if level != LogNormal && level != LogDebug {
		return &Error{Code: CodeInvalidConfig, Message: "log level must be normal or debug"}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.logLevel
	s.logLevel = level
	if err := s.saveLocked(); err != nil {
		s.logLevel = prev
		return toAPIError(err)
	}
	s.applyLogLevel()
	return nil
}

func (s *Service) applyLogLevel() {
	if s.levelVar == nil {
		return
	}
	if s.logLevel == LogDebug {
		s.levelVar.Set(slog.LevelDebug)
	} else {
		s.levelVar.Set(slog.LevelInfo)
	}
}

// --- export / import -------------------------------------------------------------------

// ExportConfig returns the rig as a JSON document to save or share.
func (s *Service) ExportConfig() ([]byte, error) {
	s.mu.Lock()
	doc := exportDoc{Format: ExportFormat, Version: config.Version, Slots: make([]*core.CameraConfig, SlotCount), Active: s.active}
	for i, st := range s.slots {
		if st != nil {
			c := st.cfg
			c.Password, c.BindIP = "", ""
			doc.Slots[i] = &c
		}
	}
	s.mu.Unlock()
	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, toAPIError(err)
	}
	return b, nil
}

// ImportConfig replaces the rig with an exported document. Every camera is
// validated first; nothing changes unless the whole document is valid.
// Passwords are not part of the document and must be entered again.
func (s *Service) ImportConfig(data []byte) error {
	invalid := func(msg string) error { return &Error{Code: CodeInvalidConfig, Message: msg} }
	if len(data) > config.MaxFileSize {
		return invalid("file too large")
	}
	var doc exportDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return invalid("not a configuration file")
	}
	if doc.Format != ExportFormat {
		return invalid("not a configuration file")
	}
	if doc.Version != config.Version {
		return invalid(fmt.Sprintf("unsupported configuration version %d", doc.Version))
	}
	if len(doc.Slots) > SlotCount {
		return invalid(fmt.Sprintf("more than %d cameras", SlotCount))
	}
	if doc.Active < 0 || doc.Active > SlotCount {
		doc.Active = 0
	}

	s.mu.Lock()
	var fresh [SlotCount]*slotState
	for i, c := range doc.Slots {
		if c == nil {
			continue
		}
		cfg := *c
		cfg.Password = ""
		st, err := s.build(cfg)
		if err != nil {
			s.mu.Unlock()
			for _, f := range fresh {
				if f != nil {
					f.close()
				}
			}
			return invalid(fmt.Sprintf("camera %d: %v", i+1, toAPIError(err).(*Error).Message))
		}
		fresh[i] = st
	}
	old, prevActive := s.slots, s.active
	s.slots, s.active = fresh, doc.Active
	if err := s.saveLocked(); err != nil {
		s.slots, s.active = old, prevActive
		s.mu.Unlock()
		for _, f := range fresh {
			if f != nil {
				f.close()
			}
		}
		return toAPIError(err)
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()

	for i, st := range old {
		if st != nil {
			st.close()
		}
		if err := s.secrets.Delete(secretKey(i + 1)); err != nil {
			s.log.Warn("password not deleted", "slot", i+1, "err", err)
		}
	}
	s.log.Info("configuration imported")
	emit.Emit(EventRigChanged, rig)
	s.connectAll()
	return nil
}

func findSpec(specs []core.ControlSpec, id core.ControlID) (core.ControlSpec, bool) {
	for _, sp := range specs {
		if sp.ID == id {
			return sp, true
		}
	}
	return core.ControlSpec{}, false
}
