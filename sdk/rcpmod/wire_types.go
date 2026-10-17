package rcpmod

// The types below cross the boundary between a module and the application
// as JSON: they are the data transfer objects of the contract. The
// application checks everything it receives (ids, ranges, sizes).

// Info declares a module: one camera model.
type Info struct {
	// ID is lowercase words joined by hyphens, starting with the brand
	// ("bm-studio-6k-pro"). The module file is named <ID>.wasm.
	ID    string `json:"id"`
	Brand string `json:"brand"`
	Name  string `json:"name"`
	// Transports are the physical links the camera supports
	// (TransportEthernet...). Informational.
	Transports []string `json:"transports"`
	// DefaultPort is used when the user leaves the port empty.
	DefaultPort int `json:"defaultPort"`
	// Auth is AuthNone, AuthOptional or AuthRequired.
	Auth string `json:"auth"`
	// Support is SupportPlanned, SupportExperimental or SupportSupported.
	Support string `json:"support"`
	Notes   string `json:"notes,omitempty"`
	// InterchangeableLens: the lens is the user's choice, so whether it has
	// a power zoom is a per-camera setting (Config.PowerZoom).
	InterchangeableLens bool `json:"interchangeableLens,omitempty"`
	// NoAddress: the camera is not on the network (the demo camera).
	NoAddress bool `json:"noAddress,omitempty"`
}

// Transports, authentication modes and support levels of Info.
const (
	TransportEthernet    = "ethernet"
	TransportWiFi        = "wifi"
	TransportUSBEthernet = "usbEthernet"

	AuthNone     = "none"
	AuthOptional = "optional"
	AuthRequired = "required"

	SupportPlanned      = "planned"
	SupportExperimental = "experimental"
	SupportSupported    = "supported"
)

// Config is what the user entered for one camera, validated by the
// application.
type Config struct {
	Model    string `json:"model"`
	Name     string `json:"name"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	// Password is only ever given to the camera's own module.
	Password string `json:"password,omitempty"`
	// PowerZoom: the mounted lens has a motorised zoom (models with an
	// interchangeable lens only).
	PowerZoom bool `json:"powerZoom,omitempty"`
}

// ControlSpec describes one camera parameter. With a standard ID, Label
// and Group may stay empty: the application fills them in.
type ControlSpec struct {
	ID      string   `json:"id"`
	Label   string   `json:"label,omitempty"`
	Group   string   `json:"group,omitempty"`
	Kind    string   `json:"kind"`
	Unit    string   `json:"unit,omitempty"`
	Min     float64  `json:"min,omitempty"`
	Max     float64  `json:"max,omitempty"`
	Step    float64  `json:"step,omitempty"`
	Options []Option `json:"options,omitempty"`
	// ReadOnly: the value can be read, not written.
	ReadOnly bool `json:"readOnly,omitempty"`
	// Section sub-groups controls inside a group ("Knee").
	Section string `json:"section,omitempty"`
	// Guarded: changes recording, power or what goes on air; the operator
	// lock asks for confirmation before writing it.
	Guarded bool `json:"guarded,omitempty"`
	// Hidden: for control surfaces only (IrisNorm); the GUI and presets
	// ignore it.
	Hidden bool `json:"hidden,omitempty"`
	// Block puts the control in a custom block of that title, shown at the
	// end of the camera page, in declaration order. Use it for what the
	// standard layout has no place for. Empty: the standard layout.
	Block string `json:"block,omitempty"`
	// Widget chooses how a custom block shows the control (WidgetSlider...).
	// Empty: the default widget of the control's kind.
	Widget string `json:"widget,omitempty"`
}

// Widgets of a custom block, and the kinds each one accepts.
const (
	WidgetSlider  = "slider"  // range: a slider with its value
	WidgetStepper = "stepper" // range: − and + buttons around the value
	WidgetButtons = "buttons" // enum: one button per option
	WidgetSelect  = "select"  // enum: a drop-down list
	WidgetSwitch  = "switch"  // toggle: an on/off switch
	WidgetButton  = "button"  // action: one button
	WidgetReadout = "readout" // any read-only kind: the value, large
	WidgetText    = "text"    // text: a text field (read-only: a label)
)

// Custom places a control in a custom block with a widget.
func Custom(s ControlSpec, block, widget string) ControlSpec {
	s.Block, s.Widget = block, widget
	return s
}

// Option is one choice of an enum control.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Control kinds.
const (
	KindRange  = "range"  // a number in [Min, Max]: Value.Number
	KindEnum   = "enum"   // one of Options: Value.Choice
	KindToggle = "toggle" // on/off: Value.On
	KindAction = "action" // a one-shot trigger, no payload
	KindText   = "text"   // free text, at most MaxTextLen bytes: Value.Text
)

// MaxTextLen bounds a text value.
const MaxTextLen = 256

// Groups lay controls out in sections. A standard control id has its own.
const (
	GroupLens      = "lens"
	GroupExposure  = "exposure"
	GroupWhite     = "white"
	GroupColor     = "color"
	GroupImage     = "image"
	GroupPTZ       = "ptz"
	GroupTransport = "transport"
	GroupAudio     = "audio"
	GroupOutput    = "output"
	GroupSystem    = "system"
	GroupMisc      = "misc"
)

// Value is a control value. Only the field matching Kind is meaningful.
type Value struct {
	Kind   string  `json:"kind"`
	Number float64 `json:"number,omitempty"`
	Choice string  `json:"choice,omitempty"`
	On     bool    `json:"on,omitempty"`
	Text   string  `json:"text,omitempty"`
}

// Number, Choice, Bool, Text and Trigger build values.
func Number(v float64) Value { return Value{Kind: KindRange, Number: v} }
func Choice(v string) Value  { return Value{Kind: KindEnum, Choice: v} }
func Bool(v bool) Value      { return Value{Kind: KindToggle, On: v} }
func Text(v string) Value    { return Value{Kind: KindText, Text: v} }
func Trigger() Value         { return Value{Kind: KindAction} }

// Connection states.
const (
	StateDisconnected = "disconnected"
	StateConnecting   = "connecting"
	StateConnected    = "connected"
	StateError        = "error"
)

// Status is the camera's connection state. Detail is a short reason for
// StateError or StateDisconnected, never with credentials.
type Status struct {
	State  string `json:"state"`
	Detail string `json:"detail,omitempty"`
}

// Lens is what the camera knows of its lens; a zero field is unknown.
// F-stops, millimetres and metres; FocusFar is FocusInfinity when the lens
// focuses to infinity.
type Lens struct {
	IrisMin       float64 `json:"irisMin,omitempty"`
	IrisMax       float64 `json:"irisMax,omitempty"`
	FocalLength   float64 `json:"focalLength,omitempty"`
	FocalMin      float64 `json:"focalMin,omitempty"`
	FocalMax      float64 `json:"focalMax,omitempty"`
	FocusDistance float64 `json:"focusDistance,omitempty"`
	FocusNear     float64 `json:"focusNear,omitempty"`
	FocusFar      float64 `json:"focusFar,omitempty"`
}

// FocusInfinity stands for a focus distance at infinity.
const FocusInfinity = 1e9

// Event is what a camera signals. Exactly one of these is set: Status;
// Control and Value; Lens.
type Event struct {
	Status  *Status `json:"status,omitempty"`
	Control string  `json:"control,omitempty"`
	Value   Value   `json:"value"`
	Lens    *Lens   `json:"lens,omitempty"`
}

// StatusEvent, ControlEvent and LensEvent build events.
func StatusEvent(state, detail string) Event {
	return Event{Status: &Status{State: state, Detail: detail}}
}
func ControlEvent(id string, v Value) Event { return Event{Control: id, Value: v} }
func LensEvent(l Lens) Event                { return Event{Lens: &l} }

// Standard control ids. Use them whenever the meaning matches, so the
// panel and the GUI treat every camera alike; a control no standard id
// fits gets an id prefixed with the brand ("blackmagic.contrastPivot").
const (
	Iris             = "iris"     // f-stop
	AutoIris         = "autoIris" // action
	IrisNorm         = "irisNorm" // iris over the lens' range, 0 open .. 1 closed (hidden)
	Focus            = "focus"    // 0 near .. 1 far
	AutoFocus        = "autoFocus"
	Zoom             = "zoom" // 0 wide .. 1 tele
	Gain             = "gain" // dB
	ISO              = "iso"
	ShutterAngle     = "shutterAngle" // degrees
	ShutterSpeed     = "shutterSpeed" // "1/50"
	ND               = "nd"
	MasterBlack      = "masterBlack"
	WhiteBalance     = "whiteBalance" // kelvin
	Tint             = "tint"
	AutoWhiteBalance = "autoWhiteBalance"
	LiftR            = "liftR"
	LiftG            = "liftG"
	LiftB            = "liftB"
	LiftY            = "liftY"
	GammaR           = "gammaR"
	GammaG           = "gammaG"
	GammaB           = "gammaB"
	GammaY           = "gammaY"
	GainR            = "gainR"
	GainG            = "gainG"
	GainB            = "gainB"
	GainY            = "gainY"
	Contrast         = "contrast"
	Saturation       = "saturation"
	Hue              = "hue"
	Detail           = "detail"
	ColorBars        = "colorBars"
	Pan              = "pan"  // speed -1..1
	Tilt             = "tilt" // speed -1..1
	PresetRecall     = "presetRecall"
	Record           = "record"
	Tally            = "tally"
)
