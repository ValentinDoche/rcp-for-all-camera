package panelmodel_test

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Regression: moved fast, the lever sent 4.0 then 3.2 and the camera
// reported 4.0 afterwards; the controller took that late echo for a change
// made elsewhere and the lever stopped driving.
func TestNormWindow(t *testing.T) {
	if w := panelmodel.NormWindow(panelmodel.IrisKnobs{}); w != (panelmodel.Window{0, 1}) {
		t.Fatalf("unread knobs: %v", w)
	}
	if w := panelmodel.NormWindow(panelmodel.IrisKnobs{Read: true, Coarse: 0, Sens: micropanel.AnalogMax}); w != (panelmodel.Window{0, 1}) {
		t.Fatalf("SENS fully clockwise: %v", w)
	}
	w := panelmodel.NormWindow(panelmodel.IrisKnobs{Read: true, Coarse: 0, Sens: 0})
	if w.Lo != 0 || w.Hi < panelmodel.NormMinWidth-1e-9 || w.Hi > panelmodel.NormMinWidth+1e-9 {
		t.Fatalf("narrowest window at the open end: %v", w)
	}
}
