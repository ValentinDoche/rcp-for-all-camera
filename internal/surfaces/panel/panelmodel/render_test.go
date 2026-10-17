package panelmodel_test

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel/panelmodel"

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
		if panelmodel.LEDOn(&l, base+ch) {
			c |= 1 << ch
		}
	}
	return c
}

func baseView() panelmodel.State {
	return panelmodel.State{Rig: rigWith(2, 1, 2, 4), Cam: cam(2, sonySpecs(), sonyValues()), Held: map[string]bool{}}
}

func TestRenderActiveCamera(t *testing.T) {
	v := baseView()
	v.Tally[4] = true
	f := panelmodel.Render(v)

	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "2", micropanel.DispGain: "06", micropanel.DispShutter: "50",
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "3",
	})
	if f.Digits != want {
		t.Errorf("digits = % x, want % x", f.Digits, want)
	}
	if f.Matrix != micropanel.MatrixNumber(2, micropanel.PixAmber) {
		t.Error("matrix must show 2 in orange")
	}
	for slot, c := range map[int]micropanel.Colour{1: panelmodel.ColourAssigned, 2: panelmodel.ColourActive, 3: micropanel.Off, 4: panelmodel.ColourTally, 8: micropanel.Off} {
		if got := cameraColour(f.LEDs, slot); got != c {
			t.Errorf("camera %d colour = %v, want %v", slot, got, c)
		}
	}
	for _, n := range []int{micropanel.LEDPanelRed, micropanel.LEDNDUp, micropanel.LEDNDDown, micropanel.LEDGainUp,
		micropanel.LEDGainDown, micropanel.LEDShutterUp, micropanel.LEDShutterDown} {
		if !panelmodel.LEDOn(&f.LEDs, n) {
			t.Errorf("LED %d off, want on", n)
		}
	}
	for _, n := range []int{micropanel.LEDNetwork, micropanel.LEDAlarm, micropanel.LEDBars, micropanel.LEDWB} {
		if panelmodel.LEDOn(&f.LEDs, n) {
			t.Errorf("LED %d on, want off", n)
		}
	}
	// F4 sits on segment 5: the bar is lit from 5 down to CLS.
	for i := 0; i < 16; i++ {
		if panelmodel.LEDOn(&f.LEDs, i) != (i >= 5) {
			t.Errorf("iris bar segment %d = %v", i, panelmodel.LEDOn(&f.LEDs, i))
		}
	}
}

func TestRenderUnknownValuesShowDashes(t *testing.T) {
	v := baseView()
	v.Cam = cam(2, sonySpecs(), nil)
	f := panelmodel.Render(v)
	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "-", micropanel.DispGain: "--", micropanel.DispShutter: "----",
		micropanel.DispIris: "---", micropanel.DispMasterBlack: "---",
	})
	if f.Digits != want {
		t.Errorf("digits = % x, want dashes % x", f.Digits, want)
	}
	for i := 0; i < 16; i++ {
		if panelmodel.LEDOn(&f.LEDs, i) {
			t.Fatal("iris bar lit for an unknown iris")
		}
	}
	if panelmodel.LEDOn(&f.LEDs, micropanel.LEDNDUp) {
		t.Error("arrows must be off for unknown values")
	}
}

func TestRenderAbsentControlsAreBlank(t *testing.T) {
	v := baseView()
	v.Cam = cam(2, []core.ControlSpec{rng(core.Iris, 1.4, 22, 0.1, "f")}, nil)
	f := panelmodel.Render(v)
	want := digitsOf(map[micropanel.Display]string{micropanel.DispIris: "---"})
	if f.Digits != want {
		t.Errorf("digits = % x, want only iris dashes", f.Digits)
	}
}

func TestRenderNoActiveSlot(t *testing.T) {
	f := panelmodel.Render(panelmodel.State{Rig: rigWith(0, 1, 3), Held: map[string]bool{}})
	if f.Digits != [14]byte{} {
		t.Error("digits must be blank with no active slot")
	}
	var dash micropanel.Matrix
	for c := 3; c <= 6; c++ {
		dash.Set(3, c, micropanel.PixAmber)
	}
	if f.Matrix != [32]byte(dash) {
		t.Error("matrix must show a dash")
	}
	if cameraColour(f.LEDs, 1) != panelmodel.ColourAssigned || cameraColour(f.LEDs, 2) != micropanel.Off {
		t.Error("camera colours")
	}
	if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDPanelRed) {
		t.Error("PANEL ACTIVE must be on while driven")
	}
}

func TestRenderActiveEmptySlot(t *testing.T) {
	f := panelmodel.Render(panelmodel.State{Rig: rigWith(5, 1), Cam: panelmodel.Camera{Slot: 5}, Held: map[string]bool{}})
	if f.Digits != [14]byte{} || f.Matrix != micropanel.MatrixNumber(5, micropanel.PixAmber) {
		t.Error("an empty active slot shows its number and blank displays")
	}
}

func TestRenderTallyActiveIsRed(t *testing.T) {
	v := baseView()
	v.Tally[2] = true
	f := panelmodel.Render(v)
	if cameraColour(f.LEDs, 2) != panelmodel.ColourTally || f.Matrix != micropanel.MatrixNumber(2, micropanel.PixRed) {
		t.Error("active camera on air must be red on the button and the matrix")
	}
}

func TestRenderStatusLEDsAndBars(t *testing.T) {
	v := baseView()
	v.Rig.Slots[1].Camera.Status.State = core.StateConnected
	v.Cam.Values[core.ColorBars] = core.Bool(true)
	v.WB = true
	f := panelmodel.Render(v)
	for _, n := range []int{micropanel.LEDNetwork, micropanel.LEDBars, micropanel.LEDBarsInd, micropanel.LEDWB} {
		if !panelmodel.LEDOn(&f.LEDs, n) {
			t.Errorf("LED %d off", n)
		}
	}
	// W/B held towards an auto white balance: the LED blinks, then stays
	// on once it is triggered, whatever the mode.
	v.WB, v.WBHolding = false, true
	for _, ph := range []bool{true, false} {
		v.Blink = ph
		if f := panelmodel.Render(v); panelmodel.LEDOn(&f.LEDs, micropanel.LEDWB) != ph {
			t.Errorf("W/B held: LED must follow the blink phase (%v)", ph)
		}
	}
	v.WBHolding, v.WBFired = false, true
	for _, ph := range []bool{true, false} {
		v.Blink = ph
		if f := panelmodel.Render(v); !panelmodel.LEDOn(&f.LEDs, micropanel.LEDWB) {
			t.Error("AWB triggered: W/B steady on")
		}
	}
	v.WBFired = false

	v.Rig.Slots[1].Camera.Status.State = core.StateError
	v.Blink = true
	if f := panelmodel.Render(v); !panelmodel.LEDOn(&f.LEDs, micropanel.LEDAlarm) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDNetwork) {
		t.Error("camera error must light ALARM only")
	}
	v.Blink = false
	if f := panelmodel.Render(v); panelmodel.LEDOn(&f.LEDs, micropanel.LEDAlarm) {
		t.Error("ALARM must blink: off in the other phase")
	}
}

func TestRenderAlarmBlinksAfterAFailedCommand(t *testing.T) {
	v := baseView()
	v.Rig.Slots[1].Camera.Status.State = core.StateConnected
	v.Blink = true
	if f := panelmodel.Render(v); panelmodel.LEDOn(&f.LEDs, micropanel.LEDAlarm) {
		t.Error("no error, no ALARM")
	}
	v.Alarm = true
	if f := panelmodel.Render(v); !panelmodel.LEDOn(&f.LEDs, micropanel.LEDAlarm) {
		t.Error("a failed command must light ALARM")
	}
	v.Blink = false
	if f := panelmodel.Render(v); panelmodel.LEDOn(&f.LEDs, micropanel.LEDAlarm) {
		t.Error("ALARM must blink")
	}
	v.Rig.Active = 0
	v.Blink = true
	if f := panelmodel.Render(v); !panelmodel.LEDOn(&f.LEDs, micropanel.LEDAlarm) {
		t.Error("a failed command lights ALARM even without an active camera")
	}
}

func TestRenderPickupHints(t *testing.T) {
	v := baseView()
	s := sonySpecs()[0]
	v.Iris = panelmodel.PickHint{Declared: true, Known: true, Picked: false, Pos: panelmodel.IrisPos(s, whole(s), 11), HavePos: true}
	v.MB = panelmodel.PickHint{Declared: true, Known: true, Picked: true}
	v.Blink = true
	f := panelmodel.Render(v)
	if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisMBWhite) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisMBRed) {
		t.Error("waiting for pickup must light IRIS/MB ACTIVE white")
	}
	seg := panelmodel.IrisSegment(s, 11) // inside the lit bar: blinks off
	if panelmodel.LEDOn(&f.LEDs, seg) {
		t.Error("lever position must blink on the iris bar")
	}
	v.Blink = false
	if f := panelmodel.Render(v); !panelmodel.LEDOn(&f.LEDs, seg) {
		t.Error("blink off phase must restore the bar")
	}
	v.Iris.Picked = true
	if f := panelmodel.Render(v); !panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisMBRed) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisMBWhite) {
		t.Error("both picked up must light IRIS/MB ACTIVE red")
	}
}

func TestRenderNeverSetsLEDsAbove74(t *testing.T) {
	all := baseView()
	for i := range all.Tally {
		all.Tally[i] = true
	}
	all.Rig = rigWith(8, 1, 2, 3, 4, 5, 6, 7, 8)
	all.Cam = cam(8, sonySpecs(), sonyValues())
	all.Cam.Values[core.ColorBars] = core.Bool(true)
	all.Held = map[string]bool{"AUTO_IRIS": true}
	all.WB, all.Armed = true, true
	off := baseView()
	off.Off = true
	all.Iris = panelmodel.PickHint{Declared: true, Known: true, HavePos: true, Pos: 1}
	all.Blink = true
	for _, v := range []panelmodel.State{baseView(), {Rig: rigWith(0), Held: map[string]bool{}}, all, off} {
		f := panelmodel.Render(v)
		for n := panelmodel.MaxLED + 1; n < len(f.LEDs)*8; n++ {
			if panelmodel.LEDOn(&f.LEDs, n) {
				t.Fatalf("LED bit %d set", n)
			}
		}
	}
}

func TestRenderWhiteBalanceMode(t *testing.T) {
	v := baseView()
	v.WB = true
	v.Cam.Values[core.Tint] = core.Number(-7)
	f := panelmodel.Render(v)
	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "2", micropanel.DispGain: "-7", micropanel.DispShutter: "5600",
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "3",
	})
	if f.Digits != want {
		t.Errorf("digits = % x, want tint and Kelvin % x", f.Digits, want)
	}
	if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDWB) {
		t.Error("W/B must be lit in the white balance mode")
	}
	// tint at its minimum: GAIN down off, up on; Kelvin can move both ways
	if panelmodel.LEDOn(&f.LEDs, micropanel.LEDGainDown) || !panelmodel.LEDOn(&f.LEDs, micropanel.LEDGainUp) {
		t.Error("GAIN arrows must follow the tint")
	}
	if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDShutterUp) || !panelmodel.LEDOn(&f.LEDs, micropanel.LEDShutterDown) {
		t.Error("SHUTTER arrows must follow the Kelvin")
	}
	v.Cam.Values[core.WhiteBalance] = core.Number(15000)
	if f := panelmodel.Render(v); panelmodel.LEDOn(&f.LEDs, micropanel.LEDShutterUp) || [4]byte(f.Digits[4:8]) != shutterOf("15.0") {
		t.Error("15000 K: SHUTTER up off, shown 15.0")
	}
	// COARSE preview keeps priority on the SHUTTER display
	v.Coarse, v.Blink = true, true
	v.Knobs = panelmodel.IrisKnobs{Coarse: coarseFor(sonySpecs()[0], 5.6), Sens: 0, Read: true}
	f = panelmodel.Render(v)
	if got := [4]byte(f.Digits[4:8]); got == [4]byte(want[4:8]) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDShutterDown) {
		t.Error("COARSE preview must win over the Kelvin")
	}
	if [2]byte(f.Digits[2:4]) != [2]byte(want[2:4]) {
		t.Error("the tint stays on GAIN during the COARSE preview")
	}
}

func TestRenderStoreAndScenes(t *testing.T) {
	v := baseView()
	v.Rig.Slots[1].Presets = []int{1, 4}
	v.Rig.Slots[0].Presets = []int{2} // another slot: not shown
	v.Blink = true
	f := panelmodel.Render(v)
	for n := 1; n <= 5; n++ {
		if got := panelmodel.LEDOn(&f.LEDs, micropanel.LEDScene1+n-1); got != (n == 1 || n == 4) {
			t.Errorf("SCENE %d = %v", n, got)
		}
	}
	if panelmodel.LEDOn(&f.LEDs, micropanel.LEDStoreRed) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDStoreWhite) {
		t.Error("STORE must be off at rest")
	}
	v.Armed = true
	f = panelmodel.Render(v)
	if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDStoreRed) {
		t.Error("STORE must be red while armed")
	}
	for n := 1; n <= 5; n++ {
		if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDScene1+n-1) {
			t.Errorf("armed, blink on: SCENE %d off", n)
		}
	}
	v.Blink = false
	f = panelmodel.Render(v)
	for n := 1; n <= 5; n++ {
		if panelmodel.LEDOn(&f.LEDs, micropanel.LEDScene1+n-1) {
			t.Errorf("armed, blink off: SCENE %d on", n)
		}
	}
}

func TestRenderPanelOff(t *testing.T) {
	v := baseView()
	s := sonySpecs()[0]
	v.Iris = panelmodel.PickHint{Declared: true, Known: true, Picked: true, Pos: panelmodel.IrisPos(s, whole(s), 4), HavePos: true}
	v.MB = panelmodel.PickHint{Declared: true, Known: true, Picked: true}
	f := panelmodel.Render(v)
	if !panelmodel.LEDOn(&f.LEDs, micropanel.LEDPanelRed) || panelmodel.LEDOn(&f.LEDs, micropanel.LEDPanelWhite) {
		t.Error("PANEL ACTIVE must be red while on")
	}
	v.Off = true
	f = panelmodel.Render(v)
	if panelmodel.LEDOn(&f.LEDs, micropanel.LEDPanelRed) || !panelmodel.LEDOn(&f.LEDs, micropanel.LEDPanelWhite) {
		t.Error("PANEL ACTIVE must be white only while off")
	}
	if panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisMBRed) || !panelmodel.LEDOn(&f.LEDs, micropanel.LEDIrisMBWhite) {
		t.Error("IRIS/MB ACTIVE must not be red while the panel is off")
	}
	want := digitsOf(map[micropanel.Display]string{
		micropanel.DispND: "2", micropanel.DispGain: "06", micropanel.DispShutter: "50",
		micropanel.DispIris: "F4.0", micropanel.DispMasterBlack: "3",
	})
	if f.Digits != want {
		t.Error("the panel still shows the camera while off")
	}
}

func TestIrisSegment(t *testing.T) {
	s := rng(core.Iris, 1.4, 22, 0.1, "f")
	for f, want := range map[float64]int{2: 1, 2.8: 3, 4: 5, 16: 13, 1.4: 0, 22: 15} {
		if got := panelmodel.IrisSegment(s, f); got != want {
			t.Errorf("segment(F%v) = %d, want %d", f, got, want)
		}
	}
}
