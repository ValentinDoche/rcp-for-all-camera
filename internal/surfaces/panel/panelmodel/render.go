package panelmodel

import (
	"math"
	"slices"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Camera button colours (docs/panel-mapping.md). The LEDs are on/off per
// channel, so "dim" is not possible: an assigned slot is blue.
const (
	ColourAssigned = micropanel.Blue
	ColourActive   = micropanel.Amber // orange, the GUI selection colour
	ColourTally    = micropanel.Red   // on air, wins over active
)

// MaxLED is the highest LED index the panel may receive (trap 3).
const MaxLED = micropanel.LEDPreview

// PickHint is the soft-takeover state of the iris lever or the ring.
type PickHint struct {
	Declared bool    // the camera has the control
	Known    bool    // its value is known
	Picked   bool    // the physical control drives it
	Pos      float64 // physical position 0..1 (havePos)
	HavePos  bool
}

func (h PickHint) Waiting() bool { return h.Declared && h.Known && !h.Picked }

// State is everything the panel shows.
type State struct {
	Rig   app.Rig
	Cam   Camera // the active camera (cam.slot == rig.Active)
	Tally [app.SlotCount + 1]bool
	Iris  PickHint
	MB    PickHint
	Held  map[string]bool
	Blink bool // blink phase, toggles a few times a second
	Alarm bool // a command failed a moment ago
	Knobs IrisKnobs
	// IrisWin and irisCentre are the iris lever window and the COARSE
	// centre in f-stops (irisView); zero = from the knobs (irisWindow).
	IrisWin    Window
	IrisCentre float64
	// Coarse and Sens: the knob moved a moment ago, preview its effect.
	Coarse, Sens bool
	// WB: white balance mode, GAIN shows the tint and SHUTTER the Kelvin.
	WB bool
	// WBHolding: W/B is held towards an auto white balance (LED blinks);
	// WBFired: it was triggered and W/B is still held (LED steady).
	WBHolding, WBFired bool
	// Armed: Store is Armed (the next SCENE key stores).
	Armed bool
	// Off: PANEL ACTIVE turned panel control Off.
	Off bool
}

// Frame is one complete panel image.
type Frame struct {
	Digits [14]byte
	Matrix [32]byte
	LEDs   micropanel.LEDBits
}

func Render(v State) Frame {
	var f Frame
	for _, s := range v.Rig.Slots {
		if s.Slot < 1 || s.Slot > app.SlotCount || s.Camera == nil {
			continue
		}
		c := ColourAssigned
		switch {
		case v.Tally[s.Slot]:
			c = ColourTally
		case s.Slot == v.Rig.Active:
			c = ColourActive
		}
		f.LEDs.SetCamera(s.Slot, c)
	}
	// PANEL ACTIVE: red while the panel controls the cameras, white when
	// turned off.
	f.LEDs.Set(micropanel.LEDPanelRed, !v.Off)
	f.LEDs.Set(micropanel.LEDPanelWhite, v.Off)
	// STORE: red while armed, off at rest.
	f.LEDs.Set(micropanel.LEDStoreRed, v.Armed)

	// ALARM blinks while the active camera is in error, and for a moment
	// after any command failed.
	active := ActiveCamera(v.Rig)
	camErr := active != nil && v.Cam.Slot == v.Rig.Active && active.Status.State == core.StateError
	f.LEDs.Set(micropanel.LEDAlarm, (camErr || v.Alarm) && v.Blink)

	f.Matrix = MatrixFor(v.Rig.Active, v.Tally[min(max(v.Rig.Active, 0), app.SlotCount)])
	if active == nil || v.Cam.Slot != v.Rig.Active {
		return f
	}
	if active.Status.State == core.StateConnected {
		f.LEDs.Set(micropanel.LEDNetwork, true)
	}
	// SCENE 1-5: lit for the presets stored on the active slot; all
	// blinking while Store is armed, to invite a choice.
	presets := ActivePresets(v.Rig)
	for n := 1; n <= app.PresetCount; n++ {
		lit := slices.Contains(presets, n)
		if v.Armed {
			lit = v.Blink
		}
		f.LEDs.Set(micropanel.LEDScene1+n-1, lit)
	}

	c := v.Cam
	micropanel.EncodeText(&f.Digits, micropanel.DispND, SectionText(c, micropanel.DispND, []core.ControlID{core.ND}, NDText))
	gainSection, gainFmt := GainIDs, GainText
	shutterSection, shutterFmt := ShutterIDs, ShutterText
	if v.WB {
		gainSection, gainFmt = []core.ControlID{core.Tint}, TintText
		shutterSection, shutterFmt = []core.ControlID{core.WhiteBalance}, KelvinText
	}
	micropanel.EncodeText(&f.Digits, micropanel.DispGain, SectionText(c, micropanel.DispGain, gainSection, gainFmt))
	irisSpec, _, irisDeclared, _ := c.Get(core.Iris)
	coarseView := v.Coarse && irisDeclared && irisSpec.Kind == core.KindRange && IsFStop(irisSpec) && irisSpec.Min > 0
	if coarseView {
		// COARSE preview: the window centre blinks on the SHUTTER display.
		text := ""
		if v.Blink {
			text = FStopText(v.IrisCentreOf(irisSpec))
		}
		micropanel.EncodeText(&f.Digits, micropanel.DispShutter, text)
	} else {
		micropanel.EncodeText(&f.Digits, micropanel.DispShutter, SectionText(c, micropanel.DispShutter, shutterSection, shutterFmt))
	}
	micropanel.EncodeText(&f.Digits, micropanel.DispIris, SectionText(c, micropanel.DispIris, []core.ControlID{core.Iris}, IrisText))
	micropanel.EncodeText(&f.Digits, micropanel.DispMasterBlack, SectionText(c, micropanel.DispMasterBlack, []core.ControlID{core.MasterBlack}, MBText))

	arrows := func(ids []core.ControlID, up, down int) {
		id, _, known := c.Pick(ids...)
		if !known {
			return
		}
		s, val, _, _ := c.Get(id)
		f.LEDs.Set(up, CanStep(s, val, +1))
		f.LEDs.Set(down, CanStep(s, val, -1))
	}
	arrows([]core.ControlID{core.ND}, micropanel.LEDNDUp, micropanel.LEDNDDown)
	arrows(gainSection, micropanel.LEDGainUp, micropanel.LEDGainDown)
	if !coarseView {
		arrows(shutterSection, micropanel.LEDShutterUp, micropanel.LEDShutterDown)
	}

	if _, bars, _, known := c.Get(core.ColorBars); known && bars.On {
		f.LEDs.Set(micropanel.LEDBars, true)
		f.LEDs.Set(micropanel.LEDBarsInd, true)
	}
	switch {
	case v.WBFired:
		f.LEDs.Set(micropanel.LEDWB, true)
	case v.WBHolding:
		f.LEDs.Set(micropanel.LEDWB, v.Blink)
	default:
		f.LEDs.Set(micropanel.LEDWB, v.WB)
	}
	f.LEDs.Set(micropanel.LEDAutoIris, v.Held["AUTO_IRIS"] && c.Has(core.AutoIris))

	// IRIS/MB ACTIVE: red = the lever/ring drive the camera, white = waiting
	// for pickup, value unknown or panel control off, off = neither control
	// exists.
	if v.Iris.Declared || v.MB.Declared {
		drives := !v.Off && (v.Iris.Picked || !v.Iris.Declared) && (v.MB.Picked || !v.MB.Declared)
		f.LEDs.Set(micropanel.LEDIrisMBRed, drives)
		f.LEDs.Set(micropanel.LEDIrisMBWhite, !drives)
	}

	switch s, iris, ok := c.Number(core.Iris); {
	case irisDeclared && v.Sens && irisSpec.Kind == core.KindRange && IsFStop(irisSpec):
		// SENS preview: the window itself, every reachable segment lit.
		reach := IrisReach(irisSpec, v.IrisWindowOf(irisSpec))
		for i := range micropanel.LEDIrisBarCount {
			f.LEDs.Set(micropanel.LEDIrisBar0+i, reach[i])
		}
	case ok:
		// The camera iris as a bar down to CLS, masked to the window.
		w := v.IrisWindowOf(s)
		reach := IrisReach(s, w)
		from := IrisSegment(s, iris)
		for i := range micropanel.LEDIrisBarCount {
			f.LEDs.Set(micropanel.LEDIrisBar0+i, i >= from && reach[i])
		}
		// Waiting for pickup: the lever position blinks on the scale.
		if !v.Off && v.Iris.Waiting() && v.Iris.HavePos && v.Blink {
			n := micropanel.LEDIrisBar0 + IrisSegment(s, IrisAt(s, w, v.Iris.Pos))
			f.LEDs.Set(n, !LEDOn(&f.LEDs, n))
		}
	}
	return f
}

// IrisWindowOf and irisCentreOf are the view's iris window and COARSE
// centre, from the knobs when the controller gave none.
func (v State) IrisWindowOf(s core.ControlSpec) Window {
	if v.IrisWin != (Window{}) {
		return v.IrisWin
	}
	return IrisWindow(s, v.Knobs)
}

func (v State) IrisCentreOf(s core.ControlSpec) float64 {
	if v.IrisCentre > 0 {
		return v.IrisCentre
	}
	return IrisCentre(s, v.Knobs)
}

// IrisReach tells which iris scale segments the lever can reach in window
// w: those from the window's open end to its closed end, plus everything
// down to CLS when the window reaches the range's closed end (Max). A
// normalized iris has no window: every segment.
func IrisReach(s core.ControlSpec, w Window) [micropanel.LEDIrisBarCount]bool {
	var r [micropanel.LEDIrisBarCount]bool
	if !IsFStop(s) || w.Lo <= 0 {
		for i := range r {
			r[i] = true
		}
		return r
	}
	lo, hi := IrisSegment(s, w.Lo), IrisSegment(s, w.Hi)
	toCLS := w.Hi >= s.Max-max(s.Step/2, 1e-9)
	for i := range r {
		r[i] = i >= lo && (i <= hi || toCLS)
	}
	return r
}

// IrisSegment is the iris scale segment (0 = top/open .. 15 = CLS) of an
// iris value. f-stop labels 2.0, 2.8, 4 ... 16 sit on segments 1, 3 ... 13.
func IrisSegment(s core.ControlSpec, v float64) int {
	var seg float64
	if IsFStop(s) && v > 0 {
		seg = 1 + 2*2*math.Log2(v/2)
	} else {
		seg = (1 - RangePos(s, v)) * (micropanel.LEDIrisBarCount - 1)
	}
	return min(max(int(math.Round(seg)), 0), micropanel.LEDIrisBarCount-1)
}

// ActivePresets lists the preset numbers stored on the active slot.
func ActivePresets(r app.Rig) []int {
	for _, s := range r.Slots {
		if s.Slot == r.Active && r.Active != 0 {
			return s.Presets
		}
	}
	return nil
}

func ActiveCamera(r app.Rig) *app.CameraView {
	for _, s := range r.Slots {
		if s.Slot == r.Active && r.Active != 0 {
			return s.Camera
		}
	}
	return nil
}

// MatrixFor draws the active slot number, orange or red when on air, or an
// orange dash when no slot is selected.
func MatrixFor(slot int, onAir bool) [32]byte {
	colour := byte(micropanel.PixAmber)
	if onAir {
		colour = micropanel.PixRed
	}
	if slot >= 1 {
		return micropanel.MatrixNumber(slot, colour)
	}
	var m micropanel.Matrix
	for col := 3; col <= 6; col++ {
		m.Set(micropanel.MatrixRows/2, col, colour)
	}
	return m
}

func LEDOn(l *micropanel.LEDBits, n int) bool { return l[n>>3]&(1<<(n&7)) != 0 }
