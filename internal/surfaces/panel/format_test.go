package panel

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

func TestDisplayTexts(t *testing.T) {
	iso := enum(core.ISO, "100", "125", "160", "800", "1600", "6400", "12800", "25600")
	nd := enum(core.ND, "clear", "1/4", "1/8")
	speed := enum(core.ShutterSpeed, "1/50", "1/2000")
	for _, tc := range []struct {
		name string
		f    func(core.ControlSpec, core.Value) string
		s    core.ControlSpec
		v    core.Value
		want string
	}{
		{"nd clear", ndText, nd, core.Choice("clear"), "0"},
		{"nd 1/8", ndText, nd, core.Choice("1/8"), "2"},
		{"nd unknown option", ndText, nd, core.Choice("x"), "-"},
		{"kelvin 5600", kelvinText, rng(core.WhiteBalance, 2000, 15000, 100, "K"), core.Number(5600), "5600"},
		{"kelvin 10000", kelvinText, rng(core.WhiteBalance, 2000, 15000, 100, "K"), core.Number(10000), "10.0"},
		{"kelvin 12500", kelvinText, rng(core.WhiteBalance, 2000, 15000, 100, "K"), core.Number(12500), "12.5"},
		{"tint 0", tintText, rng(core.Tint, -50, 50, 1, ""), core.Number(0), "0"},
		{"tint 5", tintText, rng(core.Tint, -50, 50, 1, ""), core.Number(5), "5"},
		{"tint -5", tintText, rng(core.Tint, -50, 50, 1, ""), core.Number(-5), "-5"},
		{"tint 12", tintText, rng(core.Tint, -50, 50, 1, ""), core.Number(12), "12"},
		{"tint -12", tintText, rng(core.Tint, -50, 50, 1, ""), core.Number(-12), "1.2."},
		{"tint -50", tintText, rng(core.Tint, -50, 50, 1, ""), core.Number(-50), "5.0."},
		{"tint 120", tintText, rng(core.Tint, -200, 200, 1, ""), core.Number(120), "Hi"},
		{"tint -120", tintText, rng(core.Tint, -200, 200, 1, ""), core.Number(-120), "Lo"},
		{"gain 0", gainText, rng(core.Gain, -12, 36, 2, "dB"), core.Number(0), "00"},
		{"gain 6", gainText, rng(core.Gain, -12, 36, 2, "dB"), core.Number(6), "06"},
		{"gain -3", gainText, rng(core.Gain, -12, 36, 2, "dB"), core.Number(-3), "-3"},
		{"gain -12", gainText, rng(core.Gain, -12, 36, 2, "dB"), core.Number(-12), "Lo"},
		{"gain 36", gainText, rng(core.Gain, -12, 36, 2, "dB"), core.Number(36), "36"},
		{"iso 100", gainText, iso, core.Choice("100"), "1.0"},
		{"iso 160", gainText, iso, core.Choice("160"), "1.6"},
		{"iso 800", gainText, iso, core.Choice("800"), "8.0"},
		{"iso 1600", gainText, iso, core.Choice("1600"), "16"},
		{"iso 6400", gainText, iso, core.Choice("6400"), "64"},
		{"iso 12800", gainText, iso, core.Choice("12800"), "13."},
		{"speed 1/50", shutterText, speed, core.Choice("1/50"), "50"},
		{"speed 1/2000", shutterText, speed, core.Choice("1/2000"), "2000"},
		{"angle 180", shutterText, rng(core.ShutterAngle, 11.2, 360, 0.1, "°"), core.Number(180), "180"},
		{"angle 172.8", shutterText, rng(core.ShutterAngle, 11.2, 360, 0.1, "°"), core.Number(172.8), "172.8"},
		{"iris f2.8", irisText, rng(core.Iris, 1.4, 22, 0.1, "f"), core.Number(2.8), "F2.8"},
		{"iris f11", irisText, rng(core.Iris, 1.4, 22, 0.1, "f"), core.Number(11), "F11"},
		{"iris f9.96 rounds up", irisText, rng(core.Iris, 1.4, 22, 0.01, "f"), core.Number(9.96), "F10"},
		{"iris normalized", irisText, rng(core.Iris, 0, 1, 0.01, ""), core.Number(0.45), "45"},
		{"mb 3", mbText, rng(core.MasterBlack, -15, 15, 1, ""), core.Number(3), "3"},
		{"mb -15", mbText, rng(core.MasterBlack, -15, 15, 1, ""), core.Number(-15), "-15"},
		{"mb zero has no sign", mbText, rng(core.MasterBlack, -15, 15, 1, ""), core.Number(0), "0"},
		{"mb -0.3 rounds to 0", mbText, rng(core.MasterBlack, -15, 15, 1, ""), core.Number(-0.3), "0"},
		{"mb fractional x100", mbText, rng(core.MasterBlack, -1, 1, 0.01, ""), core.Number(-0.25), "-25"},
		{"mb fractional +12", mbText, rng(core.MasterBlack, -2, 2, 0.01, ""), core.Number(0.12), "12"},
		{"mb +99", mbText, rng(core.MasterBlack, -2, 2, 0.01, ""), core.Number(0.99), "99"},
		{"mb -99", mbText, rng(core.MasterBlack, -2, 2, 0.01, ""), core.Number(-0.99), "-99"},
		{"mb above 99", mbText, rng(core.MasterBlack, -2, 2, 0.01, ""), core.Number(1.5), "Hi"},
		{"mb below -99", mbText, rng(core.MasterBlack, -2, 2, 0.01, ""), core.Number(-2), "-Lo"},
		{"mb integer above 99", mbText, rng(core.MasterBlack, -200, 200, 1, ""), core.Number(150), "Hi"},
	} {
		if got := tc.f(tc.s, tc.v); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestSectionTextBlankDashesValue(t *testing.T) {
	c := cam(1, sonySpecs(), map[core.ControlID]core.Value{core.Iris: core.Number(2.8)})
	if got := sectionText(c, micropanel.DispIris, []core.ControlID{core.Iris}, irisText); got != "F2.8" {
		t.Errorf("known iris = %q", got)
	}
	if got := sectionText(c, micropanel.DispShutter, shutterIDs, shutterText); got != "----" {
		t.Errorf("unknown shutter = %q, want dashes", got)
	}
	none := cam(1, nil, nil)
	if got := sectionText(none, micropanel.DispGain, gainIDs, gainText); got != "" {
		t.Errorf("absent gain = %q, want blank", got)
	}
	// angle known, speed unknown: the known one wins
	c2 := cam(1, sonySpecs(), map[core.ControlID]core.Value{core.ShutterAngle: core.Number(180)})
	if got := sectionText(c2, micropanel.DispShutter, shutterIDs, shutterText); got != "180" {
		t.Errorf("angle fallback = %q", got)
	}
	// wrong kind from a driver: dashes, no panic
	bad := cam(1, []core.ControlSpec{enum(core.Iris, "a")}, map[core.ControlID]core.Value{core.Iris: core.Choice("a")})
	if got := sectionText(bad, micropanel.DispIris, []core.ControlID{core.Iris}, irisText); got != "---" {
		t.Errorf("bad kind = %q", got)
	}
}
