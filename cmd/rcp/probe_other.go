//go:build !windows

package main

import appsvc "github.com/valentindoche/rcp-for-all-camera/internal/app"

// panelProbe: the USB panel driver is Windows only for now.
func panelProbe() (bool, error) { return false, nil }

// autoStarter: start with the session is Windows only for now.
func autoStarter() appsvc.AutoStarter { return nil }
