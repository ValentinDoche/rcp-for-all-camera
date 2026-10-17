package texts_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func rng(id core.ControlID, min, max, step float64, unit string) core.ControlSpec {
	return core.ControlSpec{ID: id, Kind: core.KindRange, Min: min, Max: max, Step: step, Unit: unit}
}

func TestExplicitValues(t *testing.T) {
	x := texts.Default()
	for _, c := range []struct {
		s    core.ControlSpec
		v    float64
		want string
	}{
		{rng(core.Iris, 1, 22, 0.1, "f"), 4, "f/4.0"},
		{rng(core.Iris, 1, 22, 0.1, "f"), 11, "f/11"},
		{rng(core.Contrast, 0, 2, 0.01, ""), 1, "100 %"},
		{rng(core.Saturation, 0, 2, 0.01, ""), 0.85, "85 %"},
		{rng(core.Hue, -1, 1, 0.01, ""), 0.5, "+90°"},
		{rng(core.Hue, -1, 1, 0.01, ""), 0, "0°"},
		{rng(core.Focus, 0, 1, 0.001, ""), 0.62, "62 %"},
		{rng(core.Gain, -12, 36, 2, "dB"), 12, "+12 dB"},
		{rng(core.WhiteBalance, 2500, 10000, 50, "K"), 5600, "5600 K"},
		{rng(core.Tint, -50, 50, 1, ""), 3, "+3"},
	} {
		if got := x.Explicit(c.s, c.v); got != c.want {
			t.Errorf("%s %v = %q, want %q", c.s.ID, c.v, got, c.want)
		}
	}
}

func TestParseExplicitIsTheInverse(t *testing.T) {
	for _, c := range []struct {
		s    core.ControlSpec
		in   string
		want float64
	}{
		{rng(core.Contrast, 0, 2, 0.01, ""), "120 %", 1.2},
		{rng(core.Hue, -1, 1, 0.01, ""), "-90", -0.5},
		{rng(core.Iris, 1, 22, 0.1, "f"), "f/5,6", 5.6},
		{rng(core.Gain, -12, 36, 2, "dB"), "18 dB", 18},
	} {
		if got, ok := texts.ParseExplicit(c.s, c.in); !ok || got != c.want {
			t.Errorf("%s %q = %v %v, want %v", c.s.ID, c.in, got, ok, c.want)
		}
	}
}

func TestDistanceAndFocal(t *testing.T) {
	x := texts.Default()
	if got := x.Distance(0.38); got != "0,38 m" {
		t.Errorf("distance %q", got)
	}
	if got := x.Distance(core.FocusInfinity); got != "∞" {
		t.Errorf("infinity %q", got)
	}
	if got := x.Focal(35); got != "35 mm" {
		t.Errorf("focal %q", got)
	}
}
