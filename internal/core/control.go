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

// Check reports whether the spec itself is well formed. Driver tests call it
// on every spec they declare.
func (s ControlSpec) Check() error {
	if s.ID == "" {
		return fmt.Errorf("control spec: empty id")
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
			if o.Value == "" || seen[o.Value] {
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
