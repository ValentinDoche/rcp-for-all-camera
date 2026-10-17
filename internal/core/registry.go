package core

import (
	"fmt"
	"slices"
	"sync"
)

// Registry is the directory of camera modules. The app builds one at
// startup and registers every module explicitly (cameras.Register, no
// init() magic), then asks it to build cameras by model id.
type Registry struct {
	mu      sync.RWMutex
	modules map[ModelID]registered
	aliases map[ModelID]ModelID // former id -> current id
	order   []ModelID
}

type registered struct {
	info   ModelInfo
	module Module
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{modules: map[ModelID]registered{}, aliases: map[ModelID]ModelID{}}
}

// Register adds modules, in order. It registers nothing if any of them is
// malformed or reuses an id or an alias already taken.
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
		for _, id := range append([]ModelID{info.ID}, info.Aliases...) {
			if r.knownLocked(id) || taken[id] {
				return fmt.Errorf("%s: %w", id, ErrDuplicateModel)
			}
			taken[id] = true
		}
		infos[i] = info
	}
	for i, info := range infos {
		r.modules[info.ID] = registered{info: info, module: mods[i]}
		for _, a := range info.Aliases {
			r.aliases[a] = info.ID
		}
		r.order = append(r.order, info.ID)
	}
	return nil
}

func (r *Registry) knownLocked(id ModelID) bool {
	_, isModule := r.modules[id]
	_, isAlias := r.aliases[id]
	return isModule || isAlias
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

// Model returns one model's info. id may be a former id (alias): the info
// returned then carries the current id.
func (r *Registry) Model(id ModelID) (ModelInfo, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.lookupLocked(id)
	return cloneInfo(e.info), ok
}

func (r *Registry) lookupLocked(id ModelID) (registered, bool) {
	if cur, ok := r.aliases[id]; ok {
		id = cur
	}
	e, ok := r.modules[id]
	return e, ok
}

// New validates cfg and builds a camera with the model's module. cfg.Model
// may be an alias; the module receives the current id. An empty Port is
// filled with the model default.
func (r *Registry) New(cfg CameraConfig) (Camera, error) {
	r.mu.RLock()
	e, ok := r.lookupLocked(cfg.Model)
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("model id: %w", ErrUnknownModel)
	}
	cfg.Model = e.info.ID
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
	m.Aliases = slices.Clone(m.Aliases)
	return m
}
