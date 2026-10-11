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

// shutterOf is the SHUTTER display (digits 4..7) showing text.
func shutterOf(text string) [4]byte {
	d := digitsOf(map[micropanel.Display]string{micropanel.DispShutter: text})
	return [4]byte(d[4:8])
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
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "+3",
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
	v.wb = true
	f := render(v)
	for _, n := range []int{micropanel.LEDNetwork, micropanel.LEDBars, micropanel.LEDBarsInd, micropanel.LEDWB} {
		if !ledOn(&f.leds, n) {
			t.Errorf("LED %d off", n)
		}
	}
	v.rig.Slots[1].Camera.Status.State = core.StateError
	v.blink = true
	if f := render(v); !ledOn(&f.leds, micropanel.LEDAlarm) || ledOn(&f.leds, micropanel.LEDNetwork) {
		t.Error("camera error must light ALARM only")
	}
	v.blink = false
	if f := render(v); ledOn(&f.leds, micropanel.LEDAlarm) {
		t.Error("ALARM must blink: off in the other phase")
	}
}

func TestRenderAlarmBlinksAfterAFailedCommand(t *testing.T) {
	v := baseView()
	v.rig.Slots[1].Camera.Status.State = core.StateConnected
	v.blink = true
	if f := render(v); ledOn(&f.leds, micropanel.LEDAlarm) {
		t.Error("no error, no ALARM")
	}
	v.alarm = true
	if f := render(v); !ledOn(&f.leds, micropanel.LEDAlarm) {
		t.Error("a failed command must light ALARM")
	}
	v.blink = false
	if f := render(v); ledOn(&f.leds, micropanel.LEDAlarm) {
		t.Error("ALARM must blink")
	}
	v.rig.Active = 0
	v.blink = true
	if f := render(v); !ledOn(&f.leds, micropanel.LEDAlarm) {
		t.Error("a failed command lights ALARM even without an active camera")
	}
}

func TestRenderPickupHints(t *testing.T) {
	v := baseView()
	s := sonySpecs()[0]
	v.iris = pickHint{declared: true, known: true, picked: false, pos: irisPos(s, whole(s), 11), havePos: true}
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
	all.held = map[string]bool{"AUTO_IRIS": true}
	all.wb, all.armed = true, true
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

func TestRenderWhiteBalanceMode(t *testing.T) {
	v := baseView()
	v.wb = true
	v.cam.values[core.Tint] = core.Number(-7)
	f := render(v)
	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "2", micropanel.DispGain: "-7", micropanel.DispShutter: "5600",
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "+3",
	})
	if f.digits != want {
		t.Errorf("digits = % x, want tint and Kelvin % x", f.digits, want)
	}
	if !ledOn(&f.leds, micropanel.LEDWB) {
		t.Error("W/B must be lit in the white balance mode")
	}
	// tint at its minimum: GAIN down off, up on; Kelvin can move both ways
	if ledOn(&f.leds, micropanel.LEDGainDown) || !ledOn(&f.leds, micropanel.LEDGainUp) {
		t.Error("GAIN arrows must follow the tint")
	}
	if !ledOn(&f.leds, micropanel.LEDShutterUp) || !ledOn(&f.leds, micropanel.LEDShutterDown) {
		t.Error("SHUTTER arrows must follow the Kelvin")
	}
	v.cam.values[core.WhiteBalance] = core.Number(15000)
	if f := render(v); ledOn(&f.leds, micropanel.LEDShutterUp) || [4]byte(f.digits[4:8]) != shutterOf("15.0") {
		t.Error("15000 K: SHUTTER up off, shown 15.0")
	}
	// COARSE preview keeps priority on the SHUTTER display
	v.coarse, v.blink = true, true
	v.knobs = irisKnobs{coarse: coarseFor(sonySpecs()[0], 5.6), sens: 0, read: true}
	f = render(v)
	if got := [4]byte(f.digits[4:8]); got == [4]byte(want[4:8]) || ledOn(&f.leds, micropanel.LEDShutterDown) {
		t.Error("COARSE preview must win over the Kelvin")
	}
	if [2]byte(f.digits[2:4]) != [2]byte(want[2:4]) {
		t.Error("the tint stays on GAIN during the COARSE preview")
	}
}

func TestRenderStoreAndScenes(t *testing.T) {
	v := baseView()
	v.rig.Slots[1].Presets = []int{1, 4}
	v.rig.Slots[0].Presets = []int{2} // another slot: not shown
	v.blink = true
	f := render(v)
	for n := 1; n <= 5; n++ {
		if got := ledOn(&f.leds, micropanel.LEDScene1+n-1); got != (n == 1 || n == 4) {
			t.Errorf("SCENE %d = %v", n, got)
		}
	}
	if ledOn(&f.leds, micropanel.LEDStoreRed) || ledOn(&f.leds, micropanel.LEDStoreWhite) {
		t.Error("STORE must be off at rest")
	}
	v.armed = true
	f = render(v)
	if !ledOn(&f.leds, micropanel.LEDStoreRed) {
		t.Error("STORE must be red while armed")
	}
	for n := 1; n <= 5; n++ {
		if !ledOn(&f.leds, micropanel.LEDScene1+n-1) {
			t.Errorf("armed, blink on: SCENE %d off", n)
		}
	}
	v.blink = false
	f = render(v)
	for n := 1; n <= 5; n++ {
		if ledOn(&f.leds, micropanel.LEDScene1+n-1) {
			t.Errorf("armed, blink off: SCENE %d on", n)
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
