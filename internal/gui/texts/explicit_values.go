package texts

import (
	"math"
	"strconv"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Scale turns a control's raw number into the explicit value the operator
// reads and types: shown = raw * Factor, followed by Unit.
type Scale struct {
	Factor   float64
	Unit     string // "" for a plain number
	Decimals int
	Signed   bool // "+" in front of positive values
}

// ScaleOf is the explicit display of a range control. Contrast and
// saturation read as percentages (1.00 is 100 %), hue as degrees (±1 is
// ±180°), and a normalised focus or zoom (0..1) as a percentage of travel.
func ScaleOf(s core.ControlSpec) Scale {
	sc := Scale{Factor: 1, Unit: s.Unit, Decimals: Decimals(s.Step), Signed: s.Min < 0}
	switch {
	case s.ID == core.Contrast || s.ID == core.Saturation:
		if s.Max <= 4 {
			sc = Scale{Factor: 100, Unit: "%", Decimals: 0}
		}
	case s.ID == core.Hue:
		if s.Min >= -1 && s.Max <= 1 {
			sc = Scale{Factor: 180, Unit: "°", Decimals: 0, Signed: true}
		}
	case s.ID == core.Focus || s.ID == core.Zoom:
		if s.Min == 0 && s.Max == 1 {
			sc = Scale{Factor: 100, Unit: "%", Decimals: 0}
		}
	}
	return sc
}

// Explicit formats a raw value of s with its Scale ("100 %", "+12 dB",
// "f/4.0", "180°").
func (x Texts) Explicit(s core.ControlSpec, v float64) string {
	if s.Unit == "f" {
		return Iris(v)
	}
	sc := ScaleOf(s)
	n := v * sc.Factor
	txt := x.Number(n, sc.Decimals)
	if sc.Signed && math.Round(n*math.Pow10(sc.Decimals)) > 0 {
		txt = "+" + txt
	}
	switch sc.Unit {
	case "":
		return txt
	case "°":
		return txt + "°"
	}
	return txt + " " + sc.Unit
}

// ParseExplicit reads a value typed in the explicit unit of s and returns
// the raw value.
func ParseExplicit(s core.ControlSpec, in string) (float64, bool) {
	for len(in) > 0 && (in[0] == 'f' || in[0] == 'F' || in[0] == '/') {
		in = in[1:]
	}
	n, ok := ParseNumber(in)
	if !ok {
		return 0, false
	}
	return n / ScaleOf(s).Factor, true
}

// Iris formats an f-stop: "f/4.0" under 10, "f/11" from 10.
func Iris(f float64) string {
	if f <= 0 || math.IsNaN(f) {
		return Missing
	}
	if f < 9.95 {
		return "f/" + strconv.FormatFloat(f, 'f', 1, 64)
	}
	return "f/" + strconv.Itoa(int(math.Round(f)))
}

// Distance formats a focus distance in metres: "0,38 m", "12 m", "∞".
func (x Texts) Distance(m float64) string {
	switch {
	case core.IsInfinity(m):
		return "∞"
	case m <= 0:
		return Missing
	case m < 10:
		return x.Number(m, 2) + " m"
	}
	return x.Number(m, 0) + " m"
}

// Focal formats a focal length: "35 mm".
func (x Texts) Focal(mm float64) string {
	if mm <= 0 {
		return Missing
	}
	return x.Number(mm, 0) + " mm"
}
