package main

import (
	"log/slog"

	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

func panelProbe() (bool, error) { return micropanel.Present() }

// newSecrets returns the password store. In-memory until the OS-protected
// store lands: passwords must be re-entered after a restart.
func newSecrets(dir string, log *slog.Logger) config.Secrets {
	_ = dir
	log.Warn("camera passwords are kept in memory only")
	return config.NewMemorySecrets()
}
