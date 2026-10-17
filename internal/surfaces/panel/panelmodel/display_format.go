package panelmodel

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

// OptionIndex returns the position of choice in the spec's options, or -1.
func OptionIndex(s core.ControlSpec, choice string) int {
	for i, o := range s.Options {
		if o.Value == choice {
			return i
		}
	}
	return -1
}

// NDText: one digit, the filter position (0 = clear, 1 = first filter...).
func NDText(s core.ControlSpec, v core.Value) string {
	i := OptionIndex(s, v.Choice)
	if i < 0 {
		return "-"
	}
	return strconv.Itoa(min(i, 9))
}

// GainText: two digits. Gain in dB ("06", "-3", "Lo"/"Hi" outside -9..99);
// ISO as hundreds with the digits read x100 ("1.6" = 160, "16" = 1600) and a
// trailing point for thousands ("12." = 12800).
func GainText(s core.ControlSpec, v core.Value) string {
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
		if i := OptionIndex(s, v.Choice); i >= 0 {
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

// ShutterText: four digits. Speed "1/50" shows "50"; angle shows degrees
// ("180", "172.8").
func ShutterText(s core.ControlSpec, v core.Value) string {
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

// KelvinText: four digits, the colour temperature in Kelvin ("5600");
// from 10000 K in thousands with one decimal ("10.0", "12.5").
func KelvinText(_ core.ControlSpec, v core.Value) string {
	n := math.Round(v.Number)
	if n >= 9999.5 {
		return strconv.FormatFloat(n/1000, 'f', 1, 64)
	}
	return strconv.Itoa(int(n))
}

// TintText: two digits. "0"; 1..99 as is ("5", "12"); -9..-1 with a minus ("-5");
// -99..-10 with both decimal points lit as the minus sign ("1.2."
// = -12); "Hi" / "Lo" beyond ±99.
func TintText(_ core.ControlSpec, v core.Value) string {
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

// IsFStop reports whether an iris control is in f-stops; otherwise it is
// normalized 0..1.
func IsFStop(s core.ControlSpec) bool { return s.Unit == "f" || s.Max > 1 }

// IrisText: three digits. f-stops as "F2.8", "F11"; normalized iris as a
// percentage of the range.
func IrisText(s core.ControlSpec, v core.Value) string {
	if IsFStop(s) {
		return FStopText(v.Number)
	}
	return strconv.Itoa(int(math.Round(RangePos(s, v.Number) * 100)))
}

// FStopText: an f-number as "F2.8", "F5.6", "F11", "F22" (one decimal
// below 10, whole numbers above).
func FStopText(n float64) string {
	switch {
	case n < 9.95:
		return "F" + strconv.FormatFloat(n, 'f', 1, 64)
	case n < 99.5:
		return "F" + strconv.Itoa(int(math.Round(n)))
	}
	return "F--"
}

// MBScale is the factor between a master black value and the number on the
// MB display: fractional ranges (step < 1) are shown x100.
func MBScale(s core.ControlSpec) float64 {
	if s.Step > 0 && s.Step < 1 {
		return 100
	}
	return 1
}

// MBShown is the largest magnitude the MB display shows as a number.
const MBShown = 99

// MBText: three digits, -99..99; negatives carry a minus ("12", "-5", "0";
// the seven-segment '+' reads badly). The master black never shows beyond
// ±99: a value set further elsewhere (GUI, camera) shows 99 / -99.
func MBText(s core.ControlSpec, v core.Value) string {
	n := int(math.Round(v.Number * MBScale(s)))
	return strconv.Itoa(min(max(n, -MBShown), MBShown))
}

// RangePos is the position of v in the spec range, 0..1.
func RangePos(s core.ControlSpec, v float64) float64 {
	if s.Max <= s.Min {
		return 0
	}
	return Clamp01((v - s.Min) / (s.Max - s.Min))
}

func Clamp01(v float64) float64 { return min(max(v, 0), 1) }

// SectionText renders one panel display from the camera state: blank when no
// control is declared, dashes when its value is unknown.
func SectionText(c Camera, d micropanel.Display, ids []core.ControlID, f func(core.ControlSpec, core.Value) string) string {
	id, declared, known := c.Pick(ids...)
	switch {
	case !declared:
		return ""
	case !known:
		return strings.Repeat("-", micropanel.DisplayWidth(d))
	}
	s, v, _, _ := c.Get(id)
	if want, ok := ExpectedKind[id]; ok && s.Kind != want {
		// A driver declared the control with an unexpected kind.
		return strings.Repeat("-", micropanel.DisplayWidth(d))
	}
	return f(s, v)
}

// ExpectedKind is the kind each displayed standard control must have.
var ExpectedKind = map[core.ControlID]core.ControlKind{
	core.ND: core.KindEnum, core.ISO: core.KindEnum, core.ShutterSpeed: core.KindEnum,
	core.Gain: core.KindRange, core.ShutterAngle: core.KindRange,
	core.Iris: core.KindRange, core.MasterBlack: core.KindRange,
	core.WhiteBalance: core.KindRange, core.Tint: core.KindRange,
}
