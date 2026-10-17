//go:build !windows

package gui

import "fyne.io/fyne/v2"

// noChrome keeps the OS title bar: the custom one only shows the icon, the
// name and the panel status.
type noChrome struct{}

func newWindowChrome() windowChrome { return noChrome{} }

func (noChrome) Install(fyne.Window, *captionZones) bool { return false }
func (noChrome) Minimize()                               {}
func (noChrome) ToggleMaximize()                         {}
func (noChrome) Maximized() bool                         { return false }
