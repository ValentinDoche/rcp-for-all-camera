package main

import (
	appsvc "github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/autostart"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// panelProbe reports whether the Micro Camera Panel is plugged in.
func panelProbe() (bool, error) { return micropanel.Present() }

// autoStarter is the per-user Run key entry for this executable.
func autoStarter() appsvc.AutoStarter {
	e, err := autostart.ForThisExe()
	if err != nil {
		return nil
	}
	return e
}
