package rcpmod

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"unicode/utf8"
)

// Range builds a continuous control.
func Range(id string, min, max, step float64, unit string) ControlSpec {
	return ControlSpec{ID: id, Kind: KindRange, Min: min, Max: max, Step: step, Unit: unit}
}

// Enum builds a choice control.
func Enum(id string, opts []Option) ControlSpec {
	return ControlSpec{ID: id, Kind: KindEnum, Options: opts}
}

// Toggle builds an on/off control.
func Toggle(id string) ControlSpec { return ControlSpec{ID: id, Kind: KindToggle} }

// Action builds a one-shot trigger.
func Action(id string) ControlSpec { return ControlSpec{ID: id, Kind: KindAction} }

// ReadOnly marks a spec as not writable.
func ReadOnly(s ControlSpec) ControlSpec {
	s.ReadOnly = true
	return s
}

// Labeled gives a brand-specific control its label and group (a standard
// id needs neither).
func Labeled(s ControlSpec, label, group string) ControlSpec {
	s.Label, s.Group = label, group
	return s
}

// Opts builds options whose label equals their value.
func Opts(values ...string) []Option {
	out := make([]Option, len(values))
	for i, v := range values {
		out[i] = Option{Value: v, Label: v}
	}
	return out
}

// Pairs builds options from value/label pairs: "v1", "L1", "v2", "L2"...
// It panics on an odd count: that is a programming error in a static table.
func Pairs(pairs ...string) []Option {
	if len(pairs)%2 != 0 {
		panic("rcpmod.Pairs: odd number of arguments")
	}
	out := make([]Option, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, Option{Value: pairs[i], Label: pairs[i+1]})
	}
	return out
}

// Numbered builds options "1".."n".
func Numbered(n int) []Option {
	out := make([]Option, n)
	for i := range out {
		v := strconv.Itoa(i + 1)
		out[i] = Option{Value: v, Label: v}
	}
	return out
}

// Validate reports whether v may be written to the control: ErrReadOnly or
// ErrInvalidValue otherwise.
func (s ControlSpec) Validate(v Value) error {
	if s.ReadOnly {
		return fmt.Errorf("%s: %w", s.ID, ErrReadOnly)
	}
	if v.Kind != s.Kind {
		return fmt.Errorf("%s: got a %q value for a %q control: %w", s.ID, v.Kind, s.Kind, ErrInvalidValue)
	}
	switch s.Kind {
	case KindRange:
		if math.IsNaN(v.Number) || v.Number < s.Min || v.Number > s.Max {
			return fmt.Errorf("%s: %v outside [%v, %v]: %w", s.ID, v.Number, s.Min, s.Max, ErrInvalidValue)
		}
	case KindEnum:
		if !slices.ContainsFunc(s.Options, func(o Option) bool { return o.Value == v.Choice }) {
			return fmt.Errorf("%s: unknown option: %w", s.ID, ErrInvalidValue)
		}
	case KindText:
		if len(v.Text) > MaxTextLen || !utf8.ValidString(v.Text) {
			return fmt.Errorf("%s: text longer than %d bytes or not UTF-8: %w", s.ID, MaxTextLen, ErrInvalidValue)
		}
	}
	return nil
}

// Specs is the immutable set of controls a camera declares. The zero value
// has no control.
type Specs struct {
	list []ControlSpec
	byID map[string]int
}

// NewSpecs copies list.
func NewSpecs(list []ControlSpec) Specs {
	s := Specs{list: cloneSpecs(list), byID: make(map[string]int, len(list))}
	for i, c := range s.list {
		s.byID[c.ID] = i
	}
	return s
}

// All returns a copy of every spec, in declaration order.
func (s Specs) All() []ControlSpec { return cloneSpecs(s.list) }

// Has reports whether the control is declared.
func (s Specs) Has(id string) bool {
	_, ok := s.byID[id]
	return ok
}

// Find returns the spec for id, or ErrUnknownControl.
func (s Specs) Find(id string) (ControlSpec, error) {
	i, ok := s.byID[id]
	if !ok {
		return ControlSpec{}, fmt.Errorf("%s: %w", id, ErrUnknownControl)
	}
	c := s.list[i]
	c.Options = slices.Clone(c.Options)
	return c, nil
}

// Validate finds the spec for id and checks v against it: the check
// Camera.Set runs before anything else.
func (s Specs) Validate(id string, v Value) (ControlSpec, error) {
	c, err := s.Find(id)
	if err != nil {
		return ControlSpec{}, err
	}
	return c, c.Validate(v)
}

// Without returns the set minus the given controls.
func (s Specs) Without(ids ...string) Specs {
	return NewSpecs(slices.DeleteFunc(s.All(), func(c ControlSpec) bool { return slices.Contains(ids, c.ID) }))
}

func cloneSpecs(list []ControlSpec) []ControlSpec {
	out := make([]ControlSpec, len(list))
	for i, c := range list {
		c.Options = slices.Clone(c.Options)
		out[i] = c
	}
	return out
}
