package micropanel

import "testing"

// Glyph bytes as used by ATEM Software Control (table at 0x14119df18).
func TestSegFontMatchesBlackmagic(t *testing.T) {
	want := map[rune]byte{'0': 0xFC, '1': 0x60, '2': 0xDA, '3': 0xF2, '4': 0x66, '5': 0xB6, '6': 0xBE,
		'7': 0xE0, '8': 0xFE, '9': 0xF6, 'F': 0x8E, 'C': 0x9C, 'L': 0x1C, 'S': 0xB6, 'A': 0xEE, 'u': 0x38, 't': 0x1E, 'o': 0x3A, '-': 0x02}
	for r, b := range want {
		if segFont[r] != b {
			t.Errorf("segFont[%q] = %#02x, want %#02x", r, segFont[r], b)
		}
	}
}

func TestEncodeText(t *testing.T) {
	var buf [14]byte
	EncodeText(&buf, DispShutter, "Auto")
	if got := buf[4:8]; string(got) != "\xEE\x38\x1E\x3A" {
		t.Errorf("Auto = % x", got)
	}
	EncodeText(&buf, DispIris, "F2.8")
	if got := buf[8:11]; got[0] != 0x8E || got[1] != 0xDA|segDP || got[2] != 0xFE {
		t.Errorf("F2.8 = % x", got)
	}
	EncodeText(&buf, DispGain, "6")
	if buf[2] != 0 || buf[3] != 0xBE {
		t.Errorf("right-align gain = % x", buf[2:4])
	}
}
