package core

import (
	"fmt"
	"math"
)

// ControlID names a normalized camera parameter. The standard IDs are in
// control_catalog.go; a driver may add its own IDs prefixed with its driver name
// (for example "sony.slowShutter") when no standard ID fits.
type ControlID string

// ControlKind says how a control is driven and which Value field it uses.
type ControlKind string

const (
	KindRange  ControlKind = "range"  // continuous number in [Min, Max], Value.Number
	KindEnum   ControlKind = "enum"   // one of Options, Value.Choice
	KindToggle ControlKind = "toggle" // on/off, Value.On
	KindAction ControlKind = "action" // one-shot trigger, no payload
	KindText   ControlKind = "text"   // free text, at most MaxTextLen bytes, Value.Text
)

// MaxTextLen bounds a text value.
const MaxTextLen = 256

// Widget is how a BlockControls panel shows a control (Item.Widget).
type Widget string

const (
	WidgetSlider  Widget = "slider"  // range
	WidgetStepper Widget = "stepper" // range: − and + around the value
	WidgetButtons Widget = "buttons" // enum: one button per option
	WidgetSelect  Widget = "select"  // enum: drop-down list
	WidgetSwitch  Widget = "switch"  // toggle
	WidgetButton  Widget = "button"  // action
	WidgetReadout Widget = "readout" // any read-only kind but action
	WidgetText    Widget = "text"    // text
)

// Fits reports whether the widget can show a control of that kind.
func (w Widget) Fits(k ControlKind, readOnly bool) bool {
	switch w {
	case WidgetSlider, WidgetStepper:
		return k == KindRange
	case WidgetButtons, WidgetSelect:
		return k == KindEnum
	case WidgetSwitch:
		return k == KindToggle
	case WidgetButton:
		return k == KindAction
	case WidgetReadout:
		return readOnly && k != KindAction
	case WidgetText:
		return k == KindText
	}
	return false
}

// ControlGroup is used by the GUI to lay controls out in sections.
type ControlGroup string

const (
	GroupLens      ControlGroup = "lens"
	GroupExposure  ControlGroup = "exposure"
	GroupWhite     ControlGroup = "white"
	GroupColor     ControlGroup = "color"
	GroupImage     ControlGroup = "image"
	GroupPTZ       ControlGroup = "ptz"
	GroupTransport ControlGroup = "transport"
	GroupAudio     ControlGroup = "audio"
	GroupOutput    ControlGroup = "output" // video outputs, formats, HDR
	GroupSystem    ControlGroup = "system" // power, installation, tally lamps, OSD
	GroupMisc      ControlGroup = "misc"
)

// Option is one choice of an enum control.
type Option struct {
	Value string
	Label string
}

// ControlSpec describes one parameter a camera exposes. Drivers return one
// spec per supported control; a control that is absent is not supported.
type ControlSpec struct {
	ID       ControlID
	Label    string
	Group    ControlGroup
	Kind     ControlKind
	Unit     string
	Min      float64
	Max      float64
	Step     float64
	Options  []Option
	ReadOnly bool
	// Section optionally sub-groups controls inside a group (e.g. "Knee",
	// "Matrix - Phase"), for cameras that expose many parameters.
	Section string
	// Guarded marks a control that changes recording, power or what goes on
	// air (record, color bars, output format...). While the operator lock is
	// on, the app asks for confirmation before writing it.
	Guarded bool
	// Hidden marks a control meant for control surfaces only (IrisNorm):
	// the GUI does not show it and presets do not store it, as it doubles a
	// visible control.
	Hidden bool
}

// Value carries a control value. Only the field matching Kind is meaningful:
// Number for range, Choice for enum, On for toggle, Text for text, nothing
// for action. Build values with Number, Choice, Bool, Text and Trigger.
type Value struct {
	Kind   ControlKind
	Number float64
	Choice string
	On     bool
	Text   string
}

func Number(v float64) Value { return Value{Kind: KindRange, Number: v} }
func Choice(v string) Value  { return Value{Kind: KindEnum, Choice: v} }
func Bool(v bool) Value      { return Value{Kind: KindToggle, On: v} }
func Text(v string) Value    { return Value{Kind: KindText, Text: v} }
func Trigger() Value         { return Value{Kind: KindAction} }

// MaxIDLen bounds control ids and enum choice values. Both cross the GUI
// boundary and end up in logs, errors and camera commands.
const MaxIDLen = 64

// CheckControlID reports whether id is shaped like a control id: 1 to
// MaxIDLen ASCII letters, digits, '.', '_' or '-'. It is the boundary check
// for ids coming from the GUI; the error never echoes the id.
func CheckControlID(id ControlID) error {
	if id == "" || len(id) > MaxIDLen {
		return fmt.Errorf("control id must be 1 to %d characters: %w", MaxIDLen, ErrUnknownControl)
	}
	for i := 0; i < len(id); i++ {
		if !isIDChar(id[i]) {
			return fmt.Errorf("control id has a forbidden character: %w", ErrUnknownControl)
		}
	}
	return nil
}

func isIDChar(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '.' || c == '_' || c == '-'
}

// Check reports whether v is well formed on its own (known kind, bounded
// printable choice), before it is matched against a spec. The error never
// echoes the value.
func (v Value) Check() error {
	switch v.Kind {
	case KindRange, KindEnum, KindToggle, KindAction, KindText:
	default:
		return fmt.Errorf("unknown value kind: %w", ErrInvalidValue)
	}
	if len(v.Text) > MaxTextLen || !printable(v.Text) {
		return fmt.Errorf("text must be at most %d printable characters: %w", MaxTextLen, ErrInvalidValue)
	}
	if len(v.Choice) > MaxIDLen || !printable(v.Choice) {
		return fmt.Errorf("choice must be at most %d printable characters: %w", MaxIDLen, ErrInvalidValue)
	}
	return nil
}

// Check reports whether the spec itself is well formed. Driver tests call it
// on every spec they declare.
func (s ControlSpec) Check() error {
	if err := CheckControlID(s.ID); err != nil {
		return fmt.Errorf("control spec %q: %w", s.ID, err)
	}
	if len(s.Section) > MaxIDLen || !printable(s.Section) {
		return fmt.Errorf("control %s: section must be at most %d printable characters", s.ID, MaxIDLen)
	}
	if len(s.Label) > MaxIDLen || !printable(s.Label) || len(s.Unit) > 16 || !printable(s.Unit) {
		return fmt.Errorf("control %s: label or unit too long or not printable", s.ID)
	}
	switch s.Kind {
	case KindRange:
		if math.IsNaN(s.Min) || math.IsNaN(s.Max) || s.Min > s.Max {
			return fmt.Errorf("control %s: bad range [%v, %v]", s.ID, s.Min, s.Max)
		}
		if math.IsNaN(s.Step) || s.Step < 0 {
			return fmt.Errorf("control %s: bad step %v", s.ID, s.Step)
		}
	case KindEnum:
		if len(s.Options) == 0 {
			return fmt.Errorf("control %s: enum without options", s.ID)
		}
		seen := make(map[string]bool, len(s.Options))
		for _, o := range s.Options {
			if o.Value == "" || len(o.Value) > MaxIDLen || seen[o.Value] || !printable(o.Value) ||
				len(o.Label) > MaxIDLen || !printable(o.Label) {
				return fmt.Errorf("control %s: empty or duplicate option %q", s.ID, o.Value)
			}
			seen[o.Value] = true
		}
	case KindToggle, KindAction, KindText:
	default:
		return fmt.Errorf("control %s: unknown kind %q", s.ID, s.Kind)
	}
	return nil
}

// Validate reports whether v may be written to this control.
func (s ControlSpec) Validate(v Value) error {
	if s.ReadOnly {
		return fmt.Errorf("%s: %w", s.ID, ErrReadOnly)
	}
	if v.Kind != s.Kind {
		return fmt.Errorf("%s: got %q value for %q control: %w", s.ID, v.Kind, s.Kind, ErrInvalidValue)
	}
	switch s.Kind {
	case KindRange:
		if math.IsNaN(v.Number) || v.Number < s.Min || v.Number > s.Max {
			return fmt.Errorf("%s: %v outside [%v, %v]: %w", s.ID, v.Number, s.Min, s.Max, ErrInvalidValue)
		}
	case KindEnum:
		for _, o := range s.Options {
			if o.Value == v.Choice {
				return nil
			}
		}
		return fmt.Errorf("%s: unknown option %q: %w", s.ID, v.Choice, ErrInvalidValue)
	case KindText:
		if len(v.Text) > MaxTextLen || !printable(v.Text) {
			return fmt.Errorf("%s: text must be at most %d printable characters: %w", s.ID, MaxTextLen, ErrInvalidValue)
		}
	}
	return nil
}
