package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

// Page lays out the camera page. The Studio Camera 6K Pro uses the
// standard page: its offsets, contrast pivot and luma mix appear with the
// remaining color controls.
func (c *camera) Page() rcpmod.Page { return rcpmod.StandardPage() }
