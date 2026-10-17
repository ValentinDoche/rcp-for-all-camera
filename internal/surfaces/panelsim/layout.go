package panelsim

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Geometry of the panel face, in millimetre-like units taken from the
// product drawing (145 x 257 mm body). The view multiplies them by scale.
const (
	scale   = 1.9
	originX = 152.0 // left edge of the body in drawing units
	bodyW   = 258.0
	bodyH   = 462.0
)

// Key codes of report 0x03 (docs/atem-micro-camera-panel-protocol.md §4.1).
const (
	keyCam1        micropanel.Key = 0x40
	keyScene1      micropanel.Key = 0x48
	keyStore       micropanel.Key = 0x4D
	keyNDUp        micropanel.Key = 0x4E
	keyNDDown      micropanel.Key = 0x4F
	keyCCUp        micropanel.Key = 0x50
	keyCCDown      micropanel.Key = 0x51
	keyGainUp      micropanel.Key = 0x52
	keyGainDown    micropanel.Key = 0x53
	keyShutterUp   micropanel.Key = 0x54
	keyShutterDown micropanel.Key = 0x55
	keyOn          micropanel.Key = 0x56
	keyWB          micropanel.Key = 0x57
	keyBars        micropanel.Key = 0x58
	keyBlackFlare  micropanel.Key = 0x59
	keyIrisMB      micropanel.Key = 0x5A
	keyAutoIris    micropanel.Key = 0x5B
	keyCall        micropanel.Key = 0x5C
	keyPanelActive micropanel.Key = 0x5D
	keyPreview     micropanel.Key = 0x5E
)

// noLED marks a button without a backlight.
const noLED = -1

// button is a key on the panel face: centre, size, label and the LEDs
// that light it (white backlight, red; noLED when absent).
type button struct {
	key        micropanel.Key
	x, y, w, h float64
	label      string
	white, red int
	arrow      int // 1 = ▲, -1 = ▼, 0 = text label
}

var buttons = func() []button {
	b := []button{
		{key: keyStore, x: 366, y: 45, w: 22, h: 11, label: "STORE", white: micropanel.LEDStoreWhite, red: micropanel.LEDStoreRed},

		{key: keyNDUp, x: 238, y: 70, w: 16, h: 11, white: micropanel.LEDNDUp, red: noLED, arrow: 1},
		{key: keyNDDown, x: 238, y: 84, w: 16, h: 11, white: micropanel.LEDNDDown, red: noLED, arrow: -1},
		{key: keyCCUp, x: 302, y: 70, w: 16, h: 11, white: micropanel.LEDCCUp, red: noLED, arrow: 1},
		{key: keyCCDown, x: 302, y: 84, w: 16, h: 11, white: micropanel.LEDCCDown, red: noLED, arrow: -1},
		{key: keyGainUp, x: 368, y: 70, w: 16, h: 11, white: micropanel.LEDGainUp, red: noLED, arrow: 1},
		{key: keyGainDown, x: 368, y: 84, w: 16, h: 11, white: micropanel.LEDGainDown, red: noLED, arrow: -1},
		{key: keyShutterUp, x: 368, y: 104, w: 16, h: 11, white: micropanel.LEDShutterUp, red: noLED, arrow: 1},
		{key: keyShutterDown, x: 368, y: 118, w: 16, h: 11, white: micropanel.LEDShutterDown, red: noLED, arrow: -1},

		{key: keyOn, x: 238, y: 124, w: 20, h: 11, label: "ON", white: micropanel.LEDOn, red: noLED},
		{key: keyWB, x: 262, y: 124, w: 20, h: 11, label: "W/B", white: micropanel.LEDWB, red: noLED},
		{key: keyBars, x: 286, y: 124, w: 20, h: 11, label: "BARS", white: micropanel.LEDBars, red: noLED},

		{key: keyBlackFlare, x: 262, y: 217, w: 12, h: 10, white: noLED, red: noLED},

		{key: keyIrisMB, x: 368, y: 322, w: 20, h: 15, label: "IRIS/MB\nACTIVE", white: micropanel.LEDIrisMBWhite, red: micropanel.LEDIrisMBRed},
		{key: keyAutoIris, x: 368, y: 346, w: 20, h: 15, label: "AUTO\nIRIS", white: micropanel.LEDAutoIris, red: noLED},
		{key: keyCall, x: 368, y: 370, w: 20, h: 13, label: "CALL", white: micropanel.LEDCallWhite, red: micropanel.LEDCallRed},
		{key: keyPanelActive, x: 238, y: 370, w: 20, h: 15, label: "PANEL\nACTIVE", white: micropanel.LEDPanelWhite, red: micropanel.LEDPanelRed},
		{key: keyPreview, x: 290, y: 370, w: 20, h: 13, label: "PREVIEW", white: micropanel.LEDPreview, red: noLED},
	}
	for i := range 5 {
		b = append(b, button{key: keyScene1 + micropanel.Key(i), x: 236 + 25*float64(i), y: 45, w: 18, h: 11,
			label: string(rune('1' + i)), white: micropanel.LEDScene1 + i, red: noLED})
	}
	return b
}()

// camButton is camera button n (1..8), coloured by its RGB LEDs.
type camButton struct {
	n    int
	x, y float64 // top-left
}

const camW, camH = 30.0, 24.0

var camButtons = func() []camButton {
	tops := [8]float64{116, 146, 176, 206, 256, 286, 316, 346}
	out := make([]camButton, 8)
	for i, y := range tops {
		out[i] = camButton{n: i + 1, x: 180, y: y}
	}
	return out
}()

// indicator is a small status LED with its silkscreen label.
type indicator struct {
	led   int
	x, y  float64
	label string
	alarm bool // lit red instead of white
}

var indicators = []indicator{
	{led: micropanel.LEDSceneFile, x: 231, y: 31, label: "SCENE FILE"},
	{led: micropanel.LEDRel, x: 231, y: 101, label: "REL"},
	{led: micropanel.LEDAbs, x: 253, y: 101, label: "ABS"},
	{led: micropanel.LEDBarsInd, x: 277, y: 101, label: "BARS"},
	{led: micropanel.LEDDExt, x: 362, y: 190, label: "D EXT"},
	{led: micropanel.LEDExt, x: 362, y: 202, label: "EXT"},
	{led: micropanel.LEDNetwork, x: 254, y: 362, label: "NETWORK"},
	{led: micropanel.LEDAlarm, x: 254, y: 370, label: "ALARM", alarm: true},
	{led: micropanel.LEDCable, x: 254, y: 378, label: "CABLE"},
}

// digitGroup is a seven-segment display: its digit indexes in report 0x10.
type digitGroup struct {
	first, n int
	cx, cy   float64 // centre
	label    string
}

var digitGroups = []digitGroup{
	{first: 0, n: 1, cx: 258, cy: 77, label: "ND"},
	{first: 1, n: 1, cx: 280, cy: 77, label: "CC"},
	{first: 2, n: 2, cx: 337, cy: 77, label: "MASTER GAIN"},
	{first: 4, n: 4, cx: 330, cy: 111, label: "SHUTTER"},
	{first: 8, n: 3, cx: 251, cy: 297, label: "IRIS"},
	{first: 11, n: 3, cx: 251, cy: 327, label: "MASTER BLACK"},
}

const digitW, digitH, digitGap = 7.0, 13.0, 3.0

// knob is a rotary encoder: index in micropanel.AnalogNames.
type knob struct {
	index int
	x, y  float64 // centre
	r     float64
	ring  string // colour of the ring: "r", "g", "b" or ""
	label string
}

var knobs = []knob{
	{index: 0, x: 241, y: 160, r: 11, ring: "r"},
	{index: 2, x: 283, y: 160, r: 11, ring: "g"},
	{index: 4, x: 325, y: 160, r: 11, ring: "b"},
	{index: 1, x: 241, y: 198, r: 11, ring: "r"},
	{index: 3, x: 283, y: 198, r: 11, ring: "g"},
	{index: 5, x: 325, y: 198, r: 11, ring: "b"},
	{index: 6, x: 368, y: 160, r: 9},
	{index: 7, x: 368, y: 251, r: 7, label: "SENS"},
	{index: 8, x: 368, y: 279, r: 7, label: "COARSE"},
}

// Camera matrix, iris scale and lever.
const (
	matrixX, matrixY, matrixPitch = 230.0, 246.0, 4.5

	irisBarX, irisBarY, irisBarPitch = 291.0, 254.0, 5.8

	leverX, leverTop, leverBottom = 326.0, 262.0, 356.0 // head centre range (forward = top)
	leverR                        = 15.0
)

// irisLabels are the f-stops printed next to the iris scale, by segment.
var irisLabels = map[int]string{1: "2.0", 3: "2.8", 5: "4.0", 7: "5.6", 9: "8.0", 11: "11", 13: "16", 15: "CLS"}
