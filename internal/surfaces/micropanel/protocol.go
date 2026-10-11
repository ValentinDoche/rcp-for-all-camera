package micropanel

import (
	"encoding/binary"
	"fmt"
)

// USB identity of the ATEM Micro Camera Panel (HID interface MI_02, usage page 0xFF00).
const (
	VendorID  = 0x1EDB
	ProductID = 0xBF01
)

// Report IDs, see docs/atem-micro-camera-panel-protocol.md.
const (
	repLEDs     = 0x02 // out: LEDs (layout not yet decoded)
	repKeys     = 0x03 // in: up to 6 held key codes, u16 LE
	repVersion  = 0x04 // feature
	repAuth     = 0x05 // feature: challenge/response
	repAnalog   = 0x0E // in+feature: 9 x u16 LE absolute knob positions 0..1024
	repJoystick = 0x0F // in+feature: 3 x int32 LE (mode, iris lever, master black ring)
	repDigits   = 0x10 // out: 14 raw 7-segment digits
	repMatrix   = 0x11 // out: camera dot matrix, 128 x 2 bits
)

// Key is a physical button code as reported in input report 0x03.
type Key uint16

// Button codes observed on hardware. Codes 0x59-0x5E are inferred from press
// order and must be confirmed with the paneltest tool.
var keyNames = map[Key]string{
	0x40: "CAM1", 0x41: "CAM2", 0x42: "CAM3", 0x43: "CAM4",
	0x44: "CAM5", 0x45: "CAM6", 0x46: "CAM7", 0x47: "CAM8",
	0x48: "SCENE1", 0x49: "SCENE2", 0x4A: "SCENE3", 0x4B: "SCENE4", 0x4C: "SCENE5", 0x4D: "STORE",
	0x4E: "ND_UP", 0x4F: "ND_DOWN", 0x50: "CC_UP", 0x51: "CC_DOWN",
	0x52: "GAIN_UP", 0x53: "GAIN_DOWN", 0x54: "SHUTTER_UP", 0x55: "SHUTTER_DOWN",
	0x56: "ON", 0x57: "WB", 0x58: "BARS",
	0x59: "BLACKFLARE", 0x5A: "IRISMB_ACTIVE", 0x5B: "AUTO_IRIS", 0x5C: "CALL",
	0x5D: "PANEL_ACTIVE", 0x5E: "PREVIEW",
}

// KeyName returns the control id of a key code, or KEY_0xNN when unknown.
func KeyName(k Key) string {
	if n, ok := keyNames[k]; ok {
		return n
	}
	return fmt.Sprintf("KEY_0x%02X", uint16(k))
}

// AnalogNames is the order of the 9 values in input report 0x0E.
var AnalogNames = [9]string{"WHITE_R", "BLACK_R", "WHITE_G", "BLACK_G", "WHITE_B", "BLACK_B", "KNOB7", "SENS", "COARSE"}

// AnalogMax is the full-scale value of the knobs in report 0x0E.
const AnalogMax = 1024

// JoystickMax is the full-scale magnitude of the iris lever and ring in report 0x0F.
const JoystickMax = 4096

// State is a decoded snapshot of all panel inputs.
type State struct {
	Keys   []Key     // currently held keys
	Analog [9]uint16 // see AnalogNames
	// AnalogRead is true once Analog holds positions read from the panel
	// (feature or input report 0x0E); all zero before that.
	AnalogRead bool
	Lever      int32 // iris lever, -4096..4096
	Ring       int32 // master black ring, -4096..4096
}

// parseReport updates s from one input report. It reports whether the report was understood.
func (s *State) parseReport(p []byte) bool {
	if len(p) == 0 {
		return false
	}
	switch p[0] {
	case repKeys:
		if len(p) < 13 {
			return false
		}
		s.Keys = s.Keys[:0]
		for i := 0; i < 6; i++ {
			if k := Key(binary.LittleEndian.Uint16(p[1+2*i:])); k != 0 {
				s.Keys = append(s.Keys, k)
			}
		}
	case repAnalog:
		if len(p) < 19 {
			return false
		}
		for i := range s.Analog {
			s.Analog[i] = binary.LittleEndian.Uint16(p[1+2*i:])
		}
		s.AnalogRead = true
	case repJoystick:
		if len(p) < 13 {
			return false
		}
		s.Lever = int32(binary.LittleEndian.Uint32(p[5:]))
		s.Ring = int32(binary.LittleEndian.Uint32(p[9:]))
	default:
		return false
	}
	return true
}
