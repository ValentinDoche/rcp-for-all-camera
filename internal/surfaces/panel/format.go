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

// isFStop reports whether an iris control is in f-stops; otherwise it is
// normalized 0..1.
func isFStop(s core.ControlSpec) bool { return s.Unit == "f" || s.Max > 1 }

// irisText: three digits. f-stops as "F2.8", "F11"; normalized iris as a
// percentage of the range.
func irisText(s core.ControlSpec, v core.Value) string {
	if isFStop(s) {
		switch {
		case v.Number < 9.95:
			return "F" + strconv.FormatFloat(v.Number, 'f', 1, 64)
		case v.Number < 99.5:
			return "F" + strconv.Itoa(int(math.Round(v.Number)))
		}
		return "F--"
	}
	return strconv.Itoa(int(math.Round(rangePos(s, v.Number) * 100)))
}

// mbText: three digits. Integer ranges show the value, fractional ranges
// (step < 1) show it x100. Clamped to -99..999.
func mbText(s core.ControlSpec, v core.Value) string {
	n := v.Number
	if s.Step > 0 && s.Step < 1 {
		n *= 100
	}
	return strconv.Itoa(min(max(int(math.Round(n)), -99), 999))
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
}
