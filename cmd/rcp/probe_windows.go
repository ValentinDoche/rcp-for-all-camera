package main

import "github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"

// panelProbe reports whether the Micro Camera Panel is plugged in.
func panelProbe() (bool, error) { return micropanel.Present() }
