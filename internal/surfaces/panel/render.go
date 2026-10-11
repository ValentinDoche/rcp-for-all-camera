package panel

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
	colourAssigned = micropanel.Blue
	colourActive   = micropanel.Amber // orange, the GUI selection colour
	colourTally    = micropanel.Red   // on air, wins over active
)

// maxLED is the highest LED index the panel may receive (trap 3).
const maxLED = micropanel.LEDPreview

// pickHint is the soft-takeover state of the iris lever or the ring.
type pickHint struct {
	declared bool    // the camera has the control
	known    bool    // its value is known
	picked   bool    // the physical control drives it
	pos      float64 // physical position 0..1 (havePos)
	havePos  bool
}

func (h pickHint) waiting() bool { return h.declared && h.known && !h.picked }

// view is everything the panel shows.
type view struct {
	rig   app.Rig
	cam   camState // the active camera (cam.slot == rig.Active)
	tally [app.SlotCount + 1]bool
	iris  pickHint
	mb    pickHint
	held  map[string]bool
	blink bool // blink phase, toggles a few times a second
	alarm bool // a command failed a moment ago
	knobs irisKnobs
	// coarse and sens: the knob moved a moment ago, preview its effect.
	coarse, sens bool
	// wb: white balance mode, GAIN shows the tint and SHUTTER the Kelvin.
	wb bool
	// armed: Store is armed (the next SCENE key stores).
	armed bool
}

// frame is one complete panel image.
type frame struct {
	digits [14]byte
	matrix [32]byte
	leds   micropanel.LEDBits
}

func render(v view) frame {
	var f frame
	for _, s := range v.rig.Slots {
		if s.Slot < 1 || s.Slot > app.SlotCount || s.Camera == nil {
			continue
		}
		c := colourAssigned
		switch {
		case v.tally[s.Slot]:
			c = colourTally
		case s.Slot == v.rig.Active:
			c = colourActive
		}
		f.leds.SetCamera(s.Slot, c)
	}
	f.leds.Set(micropanel.LEDPanelRed, true)
	// STORE: red while armed, off at rest.
	f.leds.Set(micropanel.LEDStoreRed, v.armed)

	// ALARM blinks while the active camera is in error, and for a moment
	// after any command failed.
	active := activeCamera(v.rig)
	camErr := active != nil && v.cam.slot == v.rig.Active && active.Status.State == core.StateError
	f.leds.Set(micropanel.LEDAlarm, (camErr || v.alarm) && v.blink)

	f.matrix = matrixFor(v.rig.Active, v.tally[min(max(v.rig.Active, 0), app.SlotCount)])
	if active == nil || v.cam.slot != v.rig.Active {
		return f
	}
	if active.Status.State == core.StateConnected {
		f.leds.Set(micropanel.LEDNetwork, true)
	}
	// SCENE 1-5: lit for the presets stored on the active slot; all
	// blinking while Store is armed, to invite a choice.
	presets := activePresets(v.rig)
	for n := 1; n <= app.PresetCount; n++ {
		lit := slices.Contains(presets, n)
		if v.armed {
			lit = v.blink
		}
		f.leds.Set(micropanel.LEDScene1+n-1, lit)
	}

	c := v.cam
	micropanel.EncodeText(&f.digits, micropanel.DispND, sectionText(c, micropanel.DispND, []core.ControlID{core.ND}, ndText))
	gainSection, gainFmt := gainIDs, gainText
	shutterSection, shutterFmt := shutterIDs, shutterText
	if v.wb {
		gainSection, gainFmt = []core.ControlID{core.Tint}, tintText
		shutterSection, shutterFmt = []core.ControlID{core.WhiteBalance}, kelvinText
	}
	micropanel.EncodeText(&f.digits, micropanel.DispGain, sectionText(c, micropanel.DispGain, gainSection, gainFmt))
	irisSpec, _, irisDeclared, _ := c.get(core.Iris)
	coarseView := v.coarse && irisDeclared && irisSpec.Kind == core.KindRange && isFStop(irisSpec) && irisSpec.Min > 0
	if coarseView {
		// COARSE preview: the window centre blinks on the SHUTTER display.
		text := ""
		if v.blink {
			text = fstopText(irisCentre(irisSpec, v.knobs))
		}
		micropanel.EncodeText(&f.digits, micropanel.DispShutter, text)
	} else {
		micropanel.EncodeText(&f.digits, micropanel.DispShutter, sectionText(c, micropanel.DispShutter, shutterSection, shutterFmt))
	}
	micropanel.EncodeText(&f.digits, micropanel.DispIris, sectionText(c, micropanel.DispIris, []core.ControlID{core.Iris}, irisText))
	micropanel.EncodeText(&f.digits, micropanel.DispMasterBlack, sectionText(c, micropanel.DispMasterBlack, []core.ControlID{core.MasterBlack}, mbText))

	arrows := func(ids []core.ControlID, up, down int) {
		id, _, known := c.pick(ids...)
		if !known {
			return
		}
		s, val, _, _ := c.get(id)
		f.leds.Set(up, canStep(s, val, +1))
		f.leds.Set(down, canStep(s, val, -1))
	}
	arrows([]core.ControlID{core.ND}, micropanel.LEDNDUp, micropanel.LEDNDDown)
	arrows(gainSection, micropanel.LEDGainUp, micropanel.LEDGainDown)
	if !coarseView {
		arrows(shutterSection, micropanel.LEDShutterUp, micropanel.LEDShutterDown)
	}

	if _, bars, _, known := c.get(core.ColorBars); known && bars.On {
		f.leds.Set(micropanel.LEDBars, true)
		f.leds.Set(micropanel.LEDBarsInd, true)
	}
	f.leds.Set(micropanel.LEDWB, v.wb)
	f.leds.Set(micropanel.LEDAutoIris, v.held["AUTO_IRIS"] && c.has(core.AutoIris))

	// IRIS/MB ACTIVE: red = the lever/ring drive the camera, white = waiting
	// for pickup (or value unknown), off = neither control exists.
	if v.iris.declared || v.mb.declared {
		drives := (v.iris.picked || !v.iris.declared) && (v.mb.picked || !v.mb.declared)
		f.leds.Set(micropanel.LEDIrisMBRed, drives)
		f.leds.Set(micropanel.LEDIrisMBWhite, !drives)
	}

	switch s, iris, ok := c.number(core.Iris); {
	case irisDeclared && v.sens && irisSpec.Kind == core.KindRange && isFStop(irisSpec):
		// SENS preview: the window itself, every reachable segment lit.
		reach := irisReach(irisSpec, irisWindow(irisSpec, v.knobs))
		for i := range micropanel.LEDIrisBarCount {
			f.leds.Set(micropanel.LEDIrisBar0+i, reach[i])
		}
	case ok:
		// The camera iris as a bar down to CLS, masked to the window.
		w := irisWindow(s, v.knobs)
		reach := irisReach(s, w)
		from := irisSegment(s, iris)
		for i := range micropanel.LEDIrisBarCount {
			f.leds.Set(micropanel.LEDIrisBar0+i, i >= from && reach[i])
		}
		// Waiting for pickup: the lever position blinks on the scale.
		if v.iris.waiting() && v.iris.havePos && v.blink {
			n := micropanel.LEDIrisBar0 + irisSegment(s, irisAt(s, w, v.iris.pos))
			f.leds.Set(n, !ledOn(&f.leds, n))
		}
	}
	return f
}

// irisReach tells which iris scale segments the lever can reach in window
// w: those from the window's open end to its closed end, plus everything
// down to CLS when the window reaches the range's closed end (Max). A
// normalized iris has no window: every segment.
func irisReach(s core.ControlSpec, w window) [micropanel.LEDIrisBarCount]bool {
	var r [micropanel.LEDIrisBarCount]bool
	if !isFStop(s) || w.lo <= 0 {
		for i := range r {
			r[i] = true
		}
		return r
	}
	lo, hi := irisSegment(s, w.lo), irisSegment(s, w.hi)
	toCLS := w.hi >= s.Max-max(s.Step/2, 1e-9)
	for i := range r {
		r[i] = i >= lo && (i <= hi || toCLS)
	}
	return r
}

// irisSegment is the iris scale segment (0 = top/open .. 15 = CLS) of an
// iris value. f-stop labels 2.0, 2.8, 4 ... 16 sit on segments 1, 3 ... 13.
func irisSegment(s core.ControlSpec, v float64) int {
	var seg float64
	if isFStop(s) && v > 0 {
		seg = 1 + 2*2*math.Log2(v/2)
	} else {
		seg = (1 - rangePos(s, v)) * (micropanel.LEDIrisBarCount - 1)
	}
	return min(max(int(math.Round(seg)), 0), micropanel.LEDIrisBarCount-1)
}

// activePresets lists the preset numbers stored on the active slot.
func activePresets(r app.Rig) []int {
	for _, s := range r.Slots {
		if s.Slot == r.Active && r.Active != 0 {
			return s.Presets
		}
	}
	return nil
}

func activeCamera(r app.Rig) *app.CameraView {
	for _, s := range r.Slots {
		if s.Slot == r.Active && r.Active != 0 {
			return s.Camera
		}
	}
	return nil
}

// matrixFor draws the active slot number, orange or red when on air, or an
// orange dash when no slot is selected.
func matrixFor(slot int, onAir bool) [32]byte {
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

func ledOn(l *micropanel.LEDBits, n int) bool { return l[n>>3]&(1<<(n&7)) != 0 }
