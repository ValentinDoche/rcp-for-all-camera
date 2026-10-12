package panel

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Display texts. Each function gets a control the camera declares with a
// known value and returns the text for micropanel.EncodeText ('.' lights the
// decimal point of the previous digit). Callers handle blank (control not
// declared) and dashes (value unknown).

// optionIndex returns the position of choice in the spec's options, or -1.
func optionIndex(s core.ControlSpec, choice string) int {
	for i, o := range s.Options {
		if o.Value == choice {
			return i
		}
	}
	return -1
}

// ndText: one digit, the filter position (0 = clear, 1 = first filter...).
func ndText(s core.ControlSpec, v core.Value) string {
	i := optionIndex(s, v.Choice)
	if i < 0 {
		return "-"
	}
	return strconv.Itoa(min(i, 9))
}

// gainText: two digits. Gain in dB ("06", "-3", "Lo"/"Hi" outside -9..99);
// ISO as hundreds with the digits read x100 ("1.6" = 160, "16" = 1600) and a
// trailing point for thousands ("12." = 12800).
func gainText(s core.ControlSpec, v core.Value) string {
	if s.Kind == core.KindRange {
		n := int(math.Round(v.Number))
		switch {
		case n < -9:
			return "Lo"
		case n > 99:
			return "Hi"
		case n < 0:
			return strconv.Itoa(n)
		}
		return fmt.Sprintf("%02d", n)
	}
	iso, err := strconv.ParseFloat(strings.TrimSpace(v.Choice), 64)
	if err != nil || iso <= 0 {
		if i := optionIndex(s, v.Choice); i >= 0 {
			return strconv.Itoa(min(i, 99))
		}
		return "--"
	}
	switch {
	case iso < 1000:
		return strconv.FormatFloat(math.Floor(iso/10)/10, 'f', 1, 64)
	case iso < 9950:
		return strconv.Itoa(int(math.Round(iso / 100)))
	}
	return strconv.Itoa(min(int(math.Round(iso/1000)), 99)) + "."
}

// shutterText: four digits. Speed "1/50" shows "50"; angle shows degrees
// ("180", "172.8").
func shutterText(s core.ControlSpec, v core.Value) string {
	if s.Kind == core.KindRange {
		if v.Number == math.Trunc(v.Number) {
			return strconv.Itoa(int(v.Number))
		}
		return strconv.FormatFloat(v.Number, 'f', 1, 64)
	}
	t := strings.TrimPrefix(strings.TrimSpace(v.Choice), "1/")
	if _, err := strconv.ParseFloat(t, 64); err != nil {
		return "----"
	}
	return t
}

// kelvinText: four digits, the colour temperature in Kelvin ("5600");
// from 10000 K in thousands with one decimal ("10.0", "12.5").
func kelvinText(_ core.ControlSpec, v core.Value) string {
	n := math.Round(v.Number)
	if n >= 9999.5 {
		return strconv.FormatFloat(n/1000, 'f', 1, 64)
	}
	return strconv.Itoa(int(n))
}

// tintText: two digits. "0"; 1..99 as is ("5", "12"); -9..-1 with a minus ("-5");
// -99..-10 with both decimal points lit as the minus sign ("1.2."
// = -12); "Hi" / "Lo" beyond ±99.
func tintText(_ core.ControlSpec, v core.Value) string {
	n := int(math.Round(v.Number))
	switch {
	case n > 99:
		return "Hi"
	case n < -99:
		return "Lo"
	case n <= -10:
		d := strconv.Itoa(-n)
		return d[:1] + "." + d[1:] + "."
	}
	return strconv.Itoa(n)
}

// isFStop reports whether an iris control is in f-stops; otherwise it is
// normalized 0..1.
func isFStop(s core.ControlSpec) bool { return s.Unit == "f" || s.Max > 1 }

// irisText: three digits. f-stops as "F2.8", "F11"; normalized iris as a
// percentage of the range.
func irisText(s core.ControlSpec, v core.Value) string {
	if isFStop(s) {
		return fstopText(v.Number)
	}
	return strconv.Itoa(int(math.Round(rangePos(s, v.Number) * 100)))
}

// fstopText: an f-number as "F2.8", "F5.6", "F11", "F22" (one decimal
// below 10, whole numbers above).
func fstopText(n float64) string {
	switch {
	case n < 9.95:
		return "F" + strconv.FormatFloat(n, 'f', 1, 64)
	case n < 99.5:
		return "F" + strconv.Itoa(int(math.Round(n)))
	}
	return "F--"
}

// mbScale is the factor between a master black value and the number on the
// MB display: fractional ranges (step < 1) are shown x100.
func mbScale(s core.ControlSpec) float64 {
	if s.Step > 0 && s.Step < 1 {
		return 100
	}
	return 1
}

// mbShown is the largest magnitude the MB display shows as a number.
const mbShown = 99

// mbText: three digits, -99..99; negatives carry a minus ("12", "-5", "0";
// the seven-segment '+' reads badly). The master black never shows beyond
// ±99: a value set further elsewhere (GUI, camera) shows 99 / -99.
func mbText(s core.ControlSpec, v core.Value) string {
	n := int(math.Round(v.Number * mbScale(s)))
	return strconv.Itoa(min(max(n, -mbShown), mbShown))
}

// rangePos is the position of v in the spec range, 0..1.
func rangePos(s core.ControlSpec, v float64) float64 {
	if s.Max <= s.Min {
		return 0
	}
	return clamp01((v - s.Min) / (s.Max - s.Min))
}

func clamp01(v float64) float64 { return min(max(v, 0), 1) }

// sectionText renders one panel display from the camera state: blank when no
// control is declared, dashes when its value is unknown.
func sectionText(c camState, d micropanel.Display, ids []core.ControlID, f func(core.ControlSpec, core.Value) string) string {
	id, declared, known := c.pick(ids...)
	switch {
	case !declared:
		return ""
	case !known:
		return strings.Repeat("-", micropanel.DisplayWidth(d))
	}
	s, v, _, _ := c.get(id)
	if want, ok := expectedKind[id]; ok && s.Kind != want {
		// A driver declared the control with an unexpected kind.
		return strings.Repeat("-", micropanel.DisplayWidth(d))
	}
	return f(s, v)
}

// expectedKind is the kind each displayed standard control must have.
var expectedKind = map[core.ControlID]core.ControlKind{
	core.ND: core.KindEnum, core.ISO: core.KindEnum, core.ShutterSpeed: core.KindEnum,
	core.Gain: core.KindRange, core.ShutterAngle: core.KindRange,
	core.Iris: core.KindRange, core.MasterBlack: core.KindRange,
	core.WhiteBalance: core.KindRange, core.Tint: core.KindRange,
}
