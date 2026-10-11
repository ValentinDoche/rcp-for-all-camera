// Package panel drives the ATEM Micro Camera Panel as a control surface of
// the app: it renders the active camera on the panel and turns panel input
// into app.Service calls. The mapping is documented in docs/panel-mapping.md.
package panel

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// camState is the active camera as the panel sees it: its declared controls
// and the values known so far (a missing value is unknown).
type camState struct {
	slot   int
	specs  map[core.ControlID]core.ControlSpec
	values map[core.ControlID]core.Value
}

func newCamState(slot int, views []app.ControlView) camState {
	c := camState{slot: slot, specs: map[core.ControlID]core.ControlSpec{}, values: map[core.ControlID]core.Value{}}
	for _, v := range views {
		c.specs[v.Spec.ID] = v.Spec
		if v.Value != nil {
			c.values[v.Spec.ID] = *v.Value
		}
	}
	return c
}

// has reports whether the camera declares id.
func (c camState) has(id core.ControlID) bool {
	_, ok := c.specs[id]
	return ok
}

// get returns the spec and value of id. declared is false when the camera
// does not have the control; known is false when its value is unknown.
func (c camState) get(id core.ControlID) (spec core.ControlSpec, v core.Value, declared, known bool) {
	spec, declared = c.specs[id]
	if declared {
		v, known = c.values[id]
	}
	return
}

// number returns a range control's known value.
func (c camState) number(id core.ControlID) (core.ControlSpec, float64, bool) {
	s, v, declared, known := c.get(id)
	if !declared || !known || s.Kind != core.KindRange {
		return s, 0, false
	}
	return s, v.Number, true
}

// pick returns the first of ids whose value is known, or else the first
// declared one (known = false). declared is false when none is declared.
func (c camState) pick(ids ...core.ControlID) (id core.ControlID, declared, known bool) {
	for _, i := range ids {
		if _, _, d, k := c.get(i); d && k {
			return i, true, true
		}
	}
	for _, i := range ids {
		if c.has(i) {
			return i, true, false
		}
	}
	return "", false, false
}

// The controls behind each panel section, in priority order.
var (
	gainIDs    = []core.ControlID{core.Gain, core.ISO}
	shutterIDs = []core.ControlID{core.ShutterSpeed, core.ShutterAngle}
)
