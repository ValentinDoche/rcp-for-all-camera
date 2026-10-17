package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

// page lays out the camera page: the standard page, where every control
// declared in main.go has its place (the standard cards, the color wheels,
// then the remaining controls by section).
func page(rcpmod.Config) rcpmod.Page { return rcpmod.StandardPage() }
