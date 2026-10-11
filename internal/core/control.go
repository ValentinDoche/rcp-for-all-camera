package core

import (
	"fmt"
	"math"
)

// ControlID names a normalized camera parameter. The standard IDs are in
// catalog.go; a driver may add its own IDs prefixed with its driver name
// (for example "sony.slowShutter") when no standard ID fits.
type ControlID string

// ControlKind says how a control is driven and which Value field it uses.
type ControlKind string

const (
	KindRange  ControlKind = "range"  // continuous number in [Min, Max], Value.Number
	KindEnum   ControlKind = "enum"   // one of Options, Value.Choice
	KindToggle ControlKind = "toggle" // on/off, Value.On
	KindAction ControlKind = "action" // one-shot trigger, no payload
)

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
	GroupMisc      ControlGroup = "misc"
)

// Option is one choice of an enum control.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// ControlSpec describes one parameter a camera exposes. Drivers return one
// spec per supported control; a control that is absent is not supported.
type ControlSpec struct {
	ID       ControlID    `json:"id"`
	Label    string       `json:"label"`
	Group    ControlGroup `json:"group"`
	Kind     ControlKind  `json:"kind"`
	Unit     string       `json:"unit,omitempty"`
	Min      float64      `json:"min,omitempty"`
	Max      float64      `json:"max,omitempty"`
	Step     float64      `json:"step,omitempty"`
	Options  []Option     `json:"options,omitempty"`
	ReadOnly bool         `json:"readOnly,omitempty"`
}

// Value carries a control value. Only the field matching Kind is meaningful:
// Number for range, Choice for enum, On for toggle, nothing for action.
// Build values with Number, Choice, Bool and Trigger.
type Value struct {
	Kind   ControlKind `json:"kind"`
	Number float64     `json:"number,omitempty"`
	Choice string      `json:"choice,omitempty"`
	On     bool        `json:"on,omitempty"`
}

func Number(v float64) Value { return Value{Kind: KindRange, Number: v} }
func Choice(v string) Value  { return Value{Kind: KindEnum, Choice: v} }
func Bool(v bool) Value      { return Value{Kind: KindToggle, On: v} }
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
	case KindRange, KindEnum, KindToggle, KindAction:
	default:
		return fmt.Errorf("unknown value kind: %w", ErrInvalidValue)
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
			if o.Value == "" || len(o.Value) > MaxIDLen || seen[o.Value] {
				return fmt.Errorf("control %s: empty or duplicate option %q", s.ID, o.Value)
			}
			seen[o.Value] = true
		}
	case KindToggle, KindAction:
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
	}
	return nil
}
