package camerakit

import (
	"fmt"
	"slices"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Specs is the immutable set of controls one camera declares. It answers
// the questions every module asks: which controls exist, what one control
// accepts, and whether a value may be written. The zero value has no
// control.
type Specs struct {
	list []core.ControlSpec
	byID map[core.ControlID]int
}

// NewSpecs copies list, so the caller may reuse it.
func NewSpecs(list []core.ControlSpec) Specs {
	s := Specs{list: cloneSpecs(list), byID: make(map[core.ControlID]int, len(list))}
	for i, c := range s.list {
		s.byID[c.ID] = i
	}
	return s
}

// All returns a copy of every spec, in declaration order. Callers may
// modify it freely.
func (s Specs) All() []core.ControlSpec { return cloneSpecs(s.list) }

// Has reports whether the control is declared.
func (s Specs) Has(id core.ControlID) bool {
	_, ok := s.byID[id]
	return ok
}

// Find returns the spec for id, or ErrUnknownControl.
func (s Specs) Find(id core.ControlID) (core.ControlSpec, error) {
	i, ok := s.byID[id]
	if !ok {
		return core.ControlSpec{}, fmt.Errorf("%s: %w", id, core.ErrUnknownControl)
	}
	c := s.list[i]
	c.Options = slices.Clone(c.Options)
	return c, nil
}

// Validate finds the spec for id and checks that v may be written to it:
// ErrUnknownControl, ErrReadOnly or ErrInvalidValue otherwise. It is the
// check Camera.Set runs before any I/O.
func (s Specs) Validate(id core.ControlID, v core.Value) (core.ControlSpec, error) {
	c, err := s.Find(id)
	if err != nil {
		return core.ControlSpec{}, err
	}
	if err := c.Validate(v); err != nil {
		return core.ControlSpec{}, err
	}
	return c, nil
}

// Without returns the set minus the given controls (a camera without a
// power zoom has no zoom).
func (s Specs) Without(ids ...core.ControlID) Specs {
	return NewSpecs(slices.DeleteFunc(s.All(), func(c core.ControlSpec) bool { return slices.Contains(ids, c.ID) }))
}

func cloneSpecs(list []core.ControlSpec) []core.ControlSpec {
	out := make([]core.ControlSpec, len(list))
	for i, c := range list {
		c.Options = slices.Clone(c.Options)
		out[i] = c
	}
	return out
}
