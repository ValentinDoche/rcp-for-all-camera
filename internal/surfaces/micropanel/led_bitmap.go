package micropanel

// Button and indicator LEDs, output report 0x02: a 75-bit LED bitmap (LED n in
// byte n>>3, bit n&7) padded to 30 bytes, followed by a layer number 0..5.
// ATEM Software Control uses layer 0 for almost everything; layers 1..5 are
// not understood yet.

// LEDBits is the 30-byte bitmap of one layer.
type LEDBits [30]byte

// Set turns LED n on or off.
func (l *LEDBits) Set(n int, on bool) {
	if n < 0 || n >= len(l)*8 {
		return
	}
	if on {
		l[n>>3] |= 1 << (n & 7)
	} else {
		l[n>>3] &^= 1 << (n & 7)
	}
}

// Colour is an RGB colour of a camera button (one bit per channel).
type Colour byte

const (
	Off     Colour = 0
	Red     Colour = 1
	Green   Colour = 2
	Amber   Colour = Red | Green // confirmed on hardware
	Blue    Colour = 4
	Cyan    Colour = Green | Blue // what ATEM Software Control uses for "selected"
	Magenta Colour = Red | Blue
	White   Colour = Red | Green | Blue
)

// SetCamera sets the colour of camera button cam (1..8). Each button has three
// LED channels R, G, B at 16+3c, 17+3c, 18+3c (c = cam-1).
func (l *LEDBits) SetCamera(cam int, c Colour) {
	if cam < 1 || cam > 8 {
		return
	}
	base := 16 + 3*(cam-1)
	for ch := 0; ch < 3; ch++ {
		l.Set(base+ch, c&(1<<ch) != 0)
	}
}

// Report returns the output report for a layer.
func (l *LEDBits) Report(layer byte) []byte {
	b := make([]byte, 0, 32)
	b = append(b, repLEDs)
	b = append(b, l[:]...)
	return append(b, layer)
}

// LED indices on layer 0, mapped on hardware on 2026-10-11 (see
// docs/atem-micro-camera-panel-protocol.md). Buttons with two colours have a
// white backlight LED and a red one.
const (
	LEDIrisBar0 = 0 // iris scale: 16 segments, 0 = top (open, above 2.0) .. 15 = CLS

	LEDSceneFile    = 40 // SCENE FILE indicator
	LEDScene1       = 41 // SCENE 1..5 backlights are 41..45
	LEDStoreWhite   = 46
	LEDStoreRed     = 47
	LEDNDUp         = 48
	LEDNDDown       = 49
	LEDCCUp         = 50
	LEDCCDown       = 51
	LEDGainUp       = 52
	LEDGainDown     = 53
	LEDShutterUp    = 54
	LEDShutterDown  = 55
	LEDRel          = 56
	LEDOn           = 57
	LEDAbs          = 58
	LEDWB           = 59
	LEDBarsInd      = 60
	LEDBars         = 61
	LEDDExt         = 62
	LEDExt          = 63
	LEDIrisMBWhite  = 64
	LEDIrisMBRed    = 65
	LEDAutoIris     = 66
	LEDCallWhite    = 67
	LEDCallRed      = 68
	LEDNetwork      = 69
	LEDAlarm        = 70
	LEDCable        = 71
	LEDPanelWhite   = 72
	LEDPanelRed     = 73
	LEDPreview      = 74
	LEDIrisBarCount = 16
)

// SetIrisBar lights the iris scale from segment 'from' (0 = top) down to CLS;
// from >= 16 turns the bar off.
func (l *LEDBits) SetIrisBar(from int) {
	for i := 0; i < LEDIrisBarCount; i++ {
		l.Set(LEDIrisBar0+i, i >= from)
	}
}
