// Package panelmodel is the pure model of the ATEM Micro Camera Panel as a
// control surface: the active camera as the panel sees it (Camera), what
// the panel shows (State, rendered to a Frame), the display texts, and how
// keys, knobs, the iris lever and the master-black ring map to control
// values (soft takeover included). It does no I/O and knows no time: the
// controller in package panel drives it. The mapping is documented in
// docs/panel-mapping.md.
package panelmodel

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Camera is the active camera as the panel sees it: its declared controls
// and the values known so far (a missing value is unknown).
type Camera struct {
	Slot   int
	Specs  map[core.ControlID]core.ControlSpec
	Values map[core.ControlID]core.Value
	Lens   core.Lens
}

// NewCamera builds the panel view of slot from its control views.
func NewCamera(slot int, views []app.ControlView) Camera {
	c := Camera{Slot: slot, Specs: map[core.ControlID]core.ControlSpec{}, Values: map[core.ControlID]core.Value{}}
	for _, v := range views {
		c.Specs[v.Spec.ID] = v.Spec
		if v.Value != nil {
			c.Values[v.Spec.ID] = *v.Value
		}
	}
	return c
}

// Has reports whether the camera declares id.
func (c Camera) Has(id core.ControlID) bool {
	_, ok := c.Specs[id]
	return ok
}

// Get returns the spec and value of id. declared is false when the camera
// does not have the control; known is false when its value is unknown.
func (c Camera) Get(id core.ControlID) (spec core.ControlSpec, v core.Value, declared, known bool) {
	spec, declared = c.Specs[id]
	if id == core.Iris {
		spec = c.LensIris(spec)
	}
	if declared {
		v, known = c.Values[id]
	}
	return
}

// LensIris narrows an f-stop iris to the lens' own limits when the camera
// reports them, so the lever never asks for an aperture the lens lacks.
func (c Camera) LensIris(s core.ControlSpec) core.ControlSpec {
	l := c.Lens
	if !IsFStop(s) || l.IrisMin <= 0 || l.IrisMax < l.IrisMin {
		return s
	}
	if lo, hi := max(s.Min, l.IrisMin), min(s.Max, l.IrisMax); lo <= hi {
		s.Min, s.Max = lo, hi
	}
	return s
}

// Number returns a range control's known value.
func (c Camera) Number(id core.ControlID) (core.ControlSpec, float64, bool) {
	s, v, declared, known := c.Get(id)
	if !declared || !known || s.Kind != core.KindRange {
		return s, 0, false
	}
	return s, v.Number, true
}

// Pick returns the first of ids whose value is known, or else the first
// declared one (known = false). declared is false when none is declared.
func (c Camera) Pick(ids ...core.ControlID) (id core.ControlID, declared, known bool) {
	for _, i := range ids {
		if _, _, d, k := c.Get(i); d && k {
			return i, true, true
		}
	}
	for _, i := range ids {
		if c.Has(i) {
			return i, true, false
		}
	}
	return "", false, false
}

// The controls behind each panel section, in priority order.
var (
	GainIDs    = []core.ControlID{core.Gain, core.ISO}
	ShutterIDs = []core.ControlID{core.ShutterSpeed, core.ShutterAngle}
)
