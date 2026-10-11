package panel

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

func digitsOf(texts map[micropanel.Display]string) [14]byte {
	var d [14]byte
	for disp, s := range texts {
		micropanel.EncodeText(&d, disp, s)
	}
	return d
}

func cameraColour(l micropanel.LEDBits, slot int) micropanel.Colour {
	base := 16 + 3*(slot-1)
	var c micropanel.Colour
	for ch := 0; ch < 3; ch++ {
		if ledOn(&l, base+ch) {
			c |= 1 << ch
		}
	}
	return c
}

func baseView() view {
	return view{rig: rigWith(2, 1, 2, 4), cam: cam(2, sonySpecs(), sonyValues()), held: map[string]bool{}}
}

func TestRenderActiveCamera(t *testing.T) {
	v := baseView()
	v.tally[4] = true
	f := render(v)

	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "2", micropanel.DispGain: "06", micropanel.DispShutter: "50",
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "3",
	})
	if f.digits != want {
		t.Errorf("digits = % x, want % x", f.digits, want)
	}
	if f.matrix != micropanel.MatrixNumber(2, micropanel.PixAmber) {
		t.Error("matrix must show 2 in orange")
	}
	for slot, c := range map[int]micropanel.Colour{1: colourAssigned, 2: colourActive, 3: micropanel.Off, 4: colourTally, 8: micropanel.Off} {
		if got := cameraColour(f.leds, slot); got != c {
			t.Errorf("camera %d colour = %v, want %v", slot, got, c)
		}
	}
	for _, n := range []int{micropanel.LEDPanelRed, micropanel.LEDNDUp, micropanel.LEDNDDown, micropanel.LEDGainUp,
		micropanel.LEDGainDown, micropanel.LEDShutterUp, micropanel.LEDShutterDown} {
		if !ledOn(&f.leds, n) {
			t.Errorf("LED %d off, want on", n)
		}
	}
	for _, n := range []int{micropanel.LEDNetwork, micropanel.LEDAlarm, micropanel.LEDBars, micropanel.LEDWB} {
		if ledOn(&f.leds, n) {
			t.Errorf("LED %d on, want off", n)
		}
	}
	// F4 sits on segment 5: the bar is lit from 5 down to CLS.
	for i := 0; i < 16; i++ {
		if ledOn(&f.leds, i) != (i >= 5) {
			t.Errorf("iris bar segment %d = %v", i, ledOn(&f.leds, i))
		}
	}
}

func TestRenderUnknownValuesShowDashes(t *testing.T) {
	v := baseView()
	v.cam = cam(2, sonySpecs(), nil)
	f := render(v)
	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "-", micropanel.DispGain: "--", micropanel.DispShutter: "----",
		micropanel.DispIris: "---", micropanel.DispMasterBlack: "---",
	})
	if f.digits != want {
		t.Errorf("digits = % x, want dashes % x", f.digits, want)
	}
	for i := 0; i < 16; i++ {
		if ledOn(&f.leds, i) {
			t.Fatal("iris bar lit for an unknown iris")
		}
	}
	if ledOn(&f.leds, micropanel.LEDNDUp) {
		t.Error("arrows must be off for unknown values")
	}
}

func TestRenderAbsentControlsAreBlank(t *testing.T) {
	v := baseView()
	v.cam = cam(2, []core.ControlSpec{rng(core.Iris, 1.4, 22, 0.1, "f")}, nil)
	f := render(v)
	want := digitsOf(map[micropanel.Display]string{micropanel.DispIris: "---"})
	if f.digits != want {
		t.Errorf("digits = % x, want only iris dashes", f.digits)
	}
}

func TestRenderNoActiveSlot(t *testing.T) {
	f := render(view{rig: rigWith(0, 1, 3), held: map[string]bool{}})
	if f.digits != [14]byte{} {
		t.Error("digits must be blank with no active slot")
	}
	var dash micropanel.Matrix
	for c := 3; c <= 6; c++ {
		dash.Set(3, c, micropanel.PixAmber)
	}
	if f.matrix != [32]byte(dash) {
		t.Error("matrix must show a dash")
	}
	if cameraColour(f.leds, 1) != colourAssigned || cameraColour(f.leds, 2) != micropanel.Off {
		t.Error("camera colours")
	}
	if !ledOn(&f.leds, micropanel.LEDPanelRed) {
		t.Error("PANEL ACTIVE must be on while driven")
	}
}

func TestRenderActiveEmptySlot(t *testing.T) {
	f := render(view{rig: rigWith(5, 1), cam: camState{slot: 5}, held: map[string]bool{}})
	if f.digits != [14]byte{} || f.matrix != micropanel.MatrixNumber(5, micropanel.PixAmber) {
		t.Error("an empty active slot shows its number and blank displays")
	}
}

func TestRenderTallyActiveIsRed(t *testing.T) {
	v := baseView()
	v.tally[2] = true
	f := render(v)
	if cameraColour(f.leds, 2) != colourTally || f.matrix != micropanel.MatrixNumber(2, micropanel.PixRed) {
		t.Error("active camera on air must be red on the button and the matrix")
	}
}

func TestRenderStatusLEDsAndBars(t *testing.T) {
	v := baseView()
	v.rig.Slots[1].Camera.Status.State = core.StateConnected
	v.cam.values[core.ColorBars] = core.Bool(true)
	v.held["WB"] = true
	f := render(v)
	for _, n := range []int{micropanel.LEDNetwork, micropanel.LEDBars, micropanel.LEDBarsInd, micropanel.LEDWB} {
		if !ledOn(&f.leds, n) {
			t.Errorf("LED %d off", n)
		}
	}
	v.rig.Slots[1].Camera.Status.State = core.StateError
	if f := render(v); !ledOn(&f.leds, micropanel.LEDAlarm) || ledOn(&f.leds, micropanel.LEDNetwork) {
		t.Error("camera error must light ALARM only")
	}
}

func TestRenderPickupHints(t *testing.T) {
	v := baseView()
	s := sonySpecs()[0]
	v.iris = pickHint{declared: true, known: true, picked: false, pos: irisPos(s, 11), havePos: true}
	v.mb = pickHint{declared: true, known: true, picked: true}
	v.blink = true
	f := render(v)
	if !ledOn(&f.leds, micropanel.LEDIrisMBWhite) || ledOn(&f.leds, micropanel.LEDIrisMBRed) {
		t.Error("waiting for pickup must light IRIS/MB ACTIVE white")
	}
	seg := irisSegment(s, 11) // inside the lit bar: blinks off
	if ledOn(&f.leds, seg) {
		t.Error("lever position must blink on the iris bar")
	}
	v.blink = false
	if f := render(v); !ledOn(&f.leds, seg) {
		t.Error("blink off phase must restore the bar")
	}
	v.iris.picked = true
	if f := render(v); !ledOn(&f.leds, micropanel.LEDIrisMBRed) || ledOn(&f.leds, micropanel.LEDIrisMBWhite) {
		t.Error("both picked up must light IRIS/MB ACTIVE red")
	}
}

func TestRenderNeverSetsLEDsAbove74(t *testing.T) {
	all := baseView()
	for i := range all.tally {
		all.tally[i] = true
	}
	all.rig = rigWith(8, 1, 2, 3, 4, 5, 6, 7, 8)
	all.cam = cam(8, sonySpecs(), sonyValues())
	all.cam.values[core.ColorBars] = core.Bool(true)
	all.held = map[string]bool{"WB": true, "AUTO_IRIS": true}
	all.iris = pickHint{declared: true, known: true, havePos: true, pos: 1}
	all.blink = true
	for _, v := range []view{baseView(), {rig: rigWith(0), held: map[string]bool{}}, all} {
		f := render(v)
		for n := maxLED + 1; n < len(f.leds)*8; n++ {
			if ledOn(&f.leds, n) {
				t.Fatalf("LED bit %d set", n)
			}
		}
	}
}

func TestIrisSegment(t *testing.T) {
	s := rng(core.Iris, 1.4, 22, 0.1, "f")
	for f, want := range map[float64]int{2: 1, 2.8: 3, 4: 5, 16: 13, 1.4: 0, 22: 15} {
		if got := irisSegment(s, f); got != want {
			t.Errorf("segment(F%v) = %d, want %d", f, got, want)
		}
	}
}
