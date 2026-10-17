// Package speckit has small builders for core.ControlSpec, shared by the
// camera modules so each model declares its controls in a few readable lines.
package speckit

import (
	"strconv"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Range builds a continuous control based on the standard spec for id.
func Range(id core.ControlID, min, max, step float64, unit string) core.ControlSpec {
	s := core.Std(id)
	s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, min, max, step, unit
	return s
}

// Enum builds a choice control.
func Enum(id core.ControlID, opts []core.Option) core.ControlSpec {
	s := core.Std(id)
	s.Kind, s.Options = core.KindEnum, opts
	return s
}

// Toggle builds an on/off control.
func Toggle(id core.ControlID) core.ControlSpec {
	s := core.Std(id)
	s.Kind = core.KindToggle
	return s
}

// Action builds a one-shot trigger.
func Action(id core.ControlID) core.ControlSpec {
	s := core.Std(id)
	s.Kind = core.KindAction
	return s
}

// Color labels a module-specific color correction control (no standard
// id, so no standard label or group).
func Color(s core.ControlSpec, label string) core.ControlSpec {
	s.Label, s.Group = label, core.GroupColor
	return s
}

// ReadOnly marks a spec as not writable.
func ReadOnly(s core.ControlSpec) core.ControlSpec {
	s.ReadOnly = true
	return s
}

// Opts builds options whose label equals their value.
func Opts(values ...string) []core.Option {
	out := make([]core.Option, len(values))
	for i, v := range values {
		out[i] = core.Option{Value: v, Label: v}
	}
	return out
}

// Labeled builds options from value/label pairs: "v1", "L1", "v2", "L2"...
// It panics on an odd count (programming error in a static table).
func Labeled(pairs ...string) []core.Option {
	if len(pairs)%2 != 0 {
		panic("speckit.Labeled: odd number of arguments")
	}
	out := make([]core.Option, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, core.Option{Value: pairs[i], Label: pairs[i+1]})
	}
	return out
}

// Numbered builds options "1".."n" (presets, for example).
func Numbered(n int) []core.Option {
	out := make([]core.Option, n)
	for i := range out {
		v := strconv.Itoa(i + 1)
		out[i] = core.Option{Value: v, Label: v}
	}
	return out
}
