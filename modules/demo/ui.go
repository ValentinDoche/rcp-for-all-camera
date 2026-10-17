package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

// Page lays out the demo camera's page: the standard page, then a "Demo"
// block with the demo's own controls, each with its widget.
func (c *camera) Page() rcpmod.Page {
	return rcpmod.StandardPage().AddRow(
		rcpmod.Controls("Demo", 2,
			rcpmod.With(tallyLamp, rcpmod.WidgetSwitch),
			rcpmod.With(tallyTest, rcpmod.WidgetButton),
			rcpmod.With(firmware, rcpmod.WidgetReadout),
			rcpmod.With(cameraID, rcpmod.WidgetText),
		),
	)
}
