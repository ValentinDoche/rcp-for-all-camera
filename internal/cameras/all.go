// Package cameras registers every camera driver of the application.
package cameras

import (
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/blackmagic"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/panasonic"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/sony"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Register adds blackmagic, sony and panasonic, in that order, to r.
func Register(r *core.Registry) error {
	for _, d := range []core.Driver{blackmagic.Driver{}, sony.Driver{}, panasonic.Driver{}} {
		if err := r.Register(d); err != nil {
			return fmt.Errorf("register %s: %w", d.Name(), err)
		}
	}
	return nil
}
