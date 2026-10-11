package micropanel

import "strings"

// The panel has 14 seven-segment digits driven by output report 0x10, one raw
// segment byte per digit (byte 0 = first digit).

// Segment bits: MSB = segment a ... bit 1 = segment g, bit 0 = decimal point.
// ATEM Software Control writes EE 38 1E 3A to spell "Auto" on the shutter.
const (
	segDP = 1 << iota
	segG
	segF
	segE
	segD
	segC
	segB
	segA
)

var segFont = map[rune]byte{
	'0': segA | segB | segC | segD | segE | segF,
	'1': segB | segC,
	'2': segA | segB | segD | segE | segG,
	'3': segA | segB | segC | segD | segG,
	'4': segB | segC | segF | segG,
	'5': segA | segC | segD | segF | segG,
	'6': segA | segC | segD | segE | segF | segG,
	'7': segA | segB | segC,
	'8': segA | segB | segC | segD | segE | segF | segG,
	'9': segA | segB | segC | segD | segF | segG,
	'A': segA | segB | segC | segE | segF | segG,
	'b': segC | segD | segE | segF | segG,
	'C': segA | segD | segE | segF,
	'c': segD | segE | segG,
	'd': segB | segC | segD | segE | segG,
	'E': segA | segD | segE | segF | segG,
	'F': segA | segE | segF | segG,
	'H': segB | segC | segE | segF | segG,
	'L': segD | segE | segF,
	'S': segA | segC | segD | segF | segG,
	'u': segC | segD | segE,
	'n': segC | segE | segG,
	'o': segC | segD | segE | segG,
	'P': segA | segB | segE | segF | segG,
	'r': segE | segG,
	't': segD | segE | segF | segG,
	'U': segB | segC | segD | segE | segF,
	'-': segG,
	'_': segD,
	' ': 0,
}

// Display identifies one of the 7-segment groups on the panel.
type Display int

const (
	DispND Display = iota
	DispCC
	DispGain
	DispShutter
	DispIris
	DispMasterBlack
)

// digitSlots maps each display to its digit positions in report 0x10.
var digitSlots = map[Display][]int{
	DispND:          {0},
	DispCC:          {1},
	DispGain:        {2, 3},
	DispShutter:     {4, 5, 6, 7},
	DispIris:        {8, 9, 10},
	DispMasterBlack: {11, 12, 13},
}

// DisplayWidth returns the number of digits of a display.
func DisplayWidth(d Display) int { return len(digitSlots[d]) }

// EncodeText renders s right-aligned into a display's digits. A '.' lights the
// decimal point of the preceding character. Unknown characters render blank.
func EncodeText(buf *[14]byte, d Display, s string) {
	slots := digitSlots[d]
	var cells []byte
	for _, r := range s {
		if r == '.' && len(cells) > 0 {
			cells[len(cells)-1] |= segDP
			continue
		}
		g, ok := segFont[r]
		if !ok {
			g = segFont[rune(strings.ToUpper(string(r))[0])]
		}
		cells = append(cells, g)
	}
	if len(cells) > len(slots) {
		cells = cells[len(cells)-len(slots):]
	}
	pad := len(slots) - len(cells)
	for i, slot := range slots {
		if i < pad {
			buf[slot] = 0
		} else {
			buf[slot] = cells[i-pad]
		}
	}
}
