//go:build !windows

package main

// panelProbe: the USB panel driver is Windows only for now.
func panelProbe() (bool, error) { return false, nil }
