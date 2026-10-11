package core

import (
	"fmt"
	"sync"
)

// Registry maps model ids to the driver that handles them. The app builds
// one at startup and registers every driver explicitly (no init() magic).
type Registry struct {
	mu     sync.RWMutex
	models map[ModelID]registered
	order  []ModelID
}

type registered struct {
	info   ModelInfo
	driver Driver
}

func NewRegistry() *Registry {
	return &Registry{models: make(map[ModelID]registered)}
}

// Register adds all models of d. It fails, registering nothing, if any model
// is malformed, belongs to another driver or is already registered.
func (r *Registry) Register(d Driver) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	models := d.Models()
	seen := make(map[ModelID]bool, len(models))
	for _, m := range models {
		if m.Driver != d.Name() {
			return fmt.Errorf("driver %q declares model %s of driver %q", d.Name(), m.ID, m.Driver)
		}
		if err := m.Check(); err != nil {
			return err
		}
		if _, dup := r.models[m.ID]; dup || seen[m.ID] {
			return fmt.Errorf("%s: %w", m.ID, ErrDuplicateModel)
		}
		seen[m.ID] = true
	}
	for _, m := range models {
		r.models[m.ID] = registered{info: m, driver: d}
		r.order = append(r.order, m.ID)
	}
	return nil
}

// Models returns every registered model in registration order.
func (r *Registry) Models() []ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ModelInfo, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, r.models[id].info)
	}
	return out
}

// Model returns one model's info.
func (r *Registry) Model(id ModelID) (ModelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.models[id]
	return e.info, ok
}

// New validates cfg and builds a camera with the model's driver. Empty Port
// and Transport are filled with the model defaults.
func (r *Registry) New(cfg CameraConfig) (Camera, error) {
	r.mu.RLock()
	e, ok := r.models[cfg.Model]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("%q: %w", cfg.Model, ErrUnknownModel)
	}
	if err := ValidateConfig(e.info, cfg); err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = e.info.DefaultPort
	}
	if cfg.Transport == "" {
		cfg.Transport = e.info.Transports[0]
	}
	return e.driver.New(e.info, cfg)
}
