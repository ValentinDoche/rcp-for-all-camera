package panelsim

import (
	"fyne.io/fyne/v2"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Layout points for the external view tests.

// KeyCam1 is the key code of the first camera button.
const KeyCam1 = keyCam1

// CamButtonCentre is the view position of camera button n (0 = CAM1).
func CamButtonCentre(n int) fyne.Position {
	b := camButtons[n]
	return pos(b.x+camW/2, b.y+camH/2)
}

// KnobCentre is the view position of knob i (micropanel.AnalogNames order).
func KnobCentre(i int) fyne.Position { return pos(knobs[i].x, knobs[i].y) }

// LeverTopCentre is the view position of the lever head pushed forward.
func LeverTopCentre() fyne.Position { return pos(leverX, leverTop) }

// ViewSize is the size of the whole panel drawing.
func ViewSize() fyne.Size { return size(bodyW, bodyH) }

var _ micropanel.Key = KeyCam1
