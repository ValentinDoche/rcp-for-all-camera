package core

import (
	"fmt"
	"slices"
	"sync"
)

// Registry is the directory of camera modules. The app registers the
// modules it loaded at startup, then asks it to build cameras by model id.
type Registry struct {
	mu      sync.RWMutex
	modules map[ModelID]registered
	order   []ModelID
}

type registered struct {
	info   ModelInfo
	module Module
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{modules: map[ModelID]registered{}}
}

// Register adds modules, in order. It registers nothing if any of them is
// malformed or reuses an id already taken.
func (r *Registry) Register(mods ...Module) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	infos := make([]ModelInfo, len(mods))
	taken := map[ModelID]bool{}
	for i, m := range mods {
		info := cloneInfo(m.Info())
		if err := info.Check(); err != nil {
			return err
		}
		if _, dup := r.modules[info.ID]; dup || taken[info.ID] {
			return fmt.Errorf("%s: %w", info.ID, ErrDuplicateModel)
		}
		taken[info.ID] = true
		infos[i] = info
	}
	for i, info := range infos {
		r.modules[info.ID] = registered{info: info, module: mods[i]}
		r.order = append(r.order, info.ID)
	}
	return nil
}

// Models returns every registered model in registration order.
func (r *Registry) Models() []ModelInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ModelInfo, 0, len(r.order))
	for _, id := range r.order {
		out = append(out, cloneInfo(r.modules[id].info))
	}
	return out
}

// Model returns one model's info.
func (r *Registry) Model(id ModelID) (ModelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.modules[id]
	return cloneInfo(e.info), ok
}

// New validates cfg and builds a camera with the model's module. An empty
// Port is filled with the model default.
func (r *Registry) New(cfg CameraConfig) (Camera, error) {
	r.mu.RLock()
	e, ok := r.modules[cfg.Model]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("model id: %w", ErrUnknownModel)
	}
	if err := ValidateConfig(e.info, cfg); err != nil {
		return nil, err
	}
	if cfg.Port == 0 {
		cfg.Port = e.info.DefaultPort
	}
	return e.module.New(cfg)
}

func cloneInfo(m ModelInfo) ModelInfo {
	m.Transports = slices.Clone(m.Transports)
	return m
}
