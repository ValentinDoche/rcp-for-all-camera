package core

// Standard control IDs. Every driver maps its camera's parameters onto these
// where the meaning matches, so the panel and the GUI treat all cameras alike.
const (
	// Lens
	Iris      ControlID = "iris"      // f-stop, or normalized 0..1 when the camera only reports that
	AutoIris  ControlID = "autoIris"  // action: one-shot auto exposure
	Focus     ControlID = "focus"     // normalized 0 (near) .. 1 (far)
	AutoFocus ControlID = "autoFocus" // action: one-shot autofocus
	Zoom      ControlID = "zoom"      // normalized 0 (wide) .. 1 (tele)

	// Exposure
	Gain         ControlID = "gain"         // dB
	ISO          ControlID = "iso"          // enum of the camera's ISO steps
	ShutterAngle ControlID = "shutterAngle" // degrees
	ShutterSpeed ControlID = "shutterSpeed" // enum of the camera's 1/x steps
	ND           ControlID = "nd"           // enum of filter positions
	MasterBlack  ControlID = "masterBlack"  // pedestal / black level

	// White balance
	WhiteBalance     ControlID = "whiteBalance"     // kelvin
	Tint             ControlID = "tint"             // green/magenta
	AutoWhiteBalance ControlID = "autoWhiteBalance" // action

	// Color correction (lift = blacks, gamma = mids, gain = whites)
	LiftR      ControlID = "liftR"
	LiftG      ControlID = "liftG"
	LiftB      ControlID = "liftB"
	LiftY      ControlID = "liftY"
	GammaR     ControlID = "gammaR"
	GammaG     ControlID = "gammaG"
	GammaB     ControlID = "gammaB"
	GammaY     ControlID = "gammaY"
	GainR      ControlID = "gainR"
	GainG      ControlID = "gainG"
	GainB      ControlID = "gainB"
	GainY      ControlID = "gainY"
	Contrast   ControlID = "contrast"
	Saturation ControlID = "saturation"
	Hue        ControlID = "hue"

	// Image
	Detail    ControlID = "detail"
	ColorBars ControlID = "colorBars"

	// PTZ
	Pan          ControlID = "pan"          // speed -1..1
	Tilt         ControlID = "tilt"         // speed -1..1
	PresetRecall ControlID = "presetRecall" // enum of preset numbers

	// Transport / status
	Record ControlID = "record"
	Tally  ControlID = "tally"
)

// guarded are the standard controls that change recording or what goes on
// air; the operator lock asks for confirmation before they are written.
var guarded = map[ControlID]bool{Record: true, ColorBars: true}

type catalogEntry struct {
	label string
	group ControlGroup
}

var catalog = map[ControlID]catalogEntry{
	Iris:             {"Iris", GroupLens},
	AutoIris:         {"Auto iris", GroupLens},
	Focus:            {"Focus", GroupLens},
	AutoFocus:        {"Autofocus", GroupLens},
	Zoom:             {"Zoom", GroupLens},
	Gain:             {"Gain", GroupExposure},
	ISO:              {"ISO", GroupExposure},
	ShutterAngle:     {"Shutter angle", GroupExposure},
	ShutterSpeed:     {"Shutter speed", GroupExposure},
	ND:               {"ND", GroupExposure},
	MasterBlack:      {"Master black", GroupExposure},
	WhiteBalance:     {"White balance", GroupWhite},
	Tint:             {"Tint", GroupWhite},
	AutoWhiteBalance: {"Auto white balance", GroupWhite},
	LiftR:            {"Lift R", GroupColor},
	LiftG:            {"Lift G", GroupColor},
	LiftB:            {"Lift B", GroupColor},
	LiftY:            {"Lift Y", GroupColor},
	GammaR:           {"Gamma R", GroupColor},
	GammaG:           {"Gamma G", GroupColor},
	GammaB:           {"Gamma B", GroupColor},
	GammaY:           {"Gamma Y", GroupColor},
	GainR:            {"Gain R", GroupColor},
	GainG:            {"Gain G", GroupColor},
	GainB:            {"Gain B", GroupColor},
	GainY:            {"Gain Y", GroupColor},
	Contrast:         {"Contrast", GroupColor},
	Saturation:       {"Saturation", GroupColor},
	Hue:              {"Hue", GroupColor},
	Detail:           {"Detail", GroupImage},
	ColorBars:        {"Color bars", GroupImage},
	Pan:              {"Pan", GroupPTZ},
	Tilt:             {"Tilt", GroupPTZ},
	PresetRecall:     {"Preset", GroupPTZ},
	Record:           {"Record", GroupTransport},
	Tally:            {"Tally", GroupTransport},
}

// Std returns a spec pre-filled with the standard label and group for id.
// Drivers then set Kind and the range or options for their camera. Unknown
// ids get the id as label and GroupMisc.
func Std(id ControlID) ControlSpec {
	e, ok := catalog[id]
	if !ok {
		return ControlSpec{ID: id, Label: string(id), Group: GroupMisc}
	}
	return ControlSpec{ID: id, Label: e.label, Group: e.group, Guarded: guarded[id]}
}
