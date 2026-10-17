// Package cameras lists every camera module of the application.
package cameras

import (
	"fmt"
	"log/slog"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm/planned"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm/studio6kpro"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/demo"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/panasonic"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/sony"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// All returns every camera module, in display order. This list is the only
// place a module is plugged in (no init() magic). log receives what modules
// log; nil discards it.
func All(log *slog.Logger) []core.Module {
	mods := planned.Modules()
	mods = append(mods, studio6kpro.Module{Logger: log})
	mods = append(mods, sony.Modules()...)
	mods = append(mods, panasonic.Modules()...)
	return append(mods, demo.Module{})
}

// Register adds every module to r.
func Register(r *core.Registry, log *slog.Logger) error {
	if err := r.Register(All(log)...); err != nil {
		return fmt.Errorf("register camera modules: %w", err)
	}
	return nil
}
