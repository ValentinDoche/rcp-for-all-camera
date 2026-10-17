package app

import "github.com/valentindoche/rcp-for-all-camera/internal/core"

// Internals the external tests check or drive.

var (
	SecretKey     = secretKey
	ToAPIError    = toAPIError
	PanelStatusOf = panelStatusOf
	FindSpec      = findSpec
)

const (
	MaxImportSize = maxImportSize
	ExportVersion = exportVersion
)

type (
	ExportDoc    = exportDoc
	ExportCamera = exportCamera
)

// SetEmitter attaches an emitter, as Host.Start does.
func (s *Service) SetEmitter(e Emitter) { s.setEmitter(e) }

// Camera returns the slot's camera.
func (s *Service) Camera(slot int) (core.Camera, error) { return s.camera(slot) }

// SetPanel reports a panel status, as the Host does.
func (s *Service) SetPanel(p PanelStatus) { s.setPanel(p) }

// ConnectAll connects every camera, as the Host does at start.
func (s *Service) ConnectAll() { s.connectAll() }

// EditPreset runs f on a stored preset under the service lock, to fake a
// tampered or stale preset.
func (s *Service) EditPreset(slot, n int, f func(p *SavedPreset)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s.presets[slot-1][n-1])
}

// PresetValues returns a stored preset's values, read under the lock.
func (s *Service) PresetValues(slot, n int) map[core.ControlID]core.Value {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.presets[slot-1][n-1].Values
}
