// Package camerakit holds helpers shared by camera drivers: spec lookup,
// spec builders and Placeholder, the camera used by models whose control
// protocol is not written yet.
package camerakit

import (
	"context"
	"fmt"
	"slices"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// FindSpec returns the spec for id, or ErrUnknownControl.
func FindSpec(specs []core.ControlSpec, id core.ControlID) (core.ControlSpec, error) {
	for _, s := range specs {
		if s.ID == id {
			return s, nil
		}
	}
	return core.ControlSpec{}, fmt.Errorf("%s: %w", id, core.ErrUnknownControl)
}

// Placeholder is a camera that declares its controls but has no protocol
// yet. The GUI can show and validate every control; any I/O returns
// ErrNotImplemented. Replace it with a real Camera when the driver lands.
type Placeholder struct {
	specs []core.ControlSpec
}

var _ core.Camera = (*Placeholder)(nil)

func NewPlaceholder(specs []core.ControlSpec) *Placeholder {
	return &Placeholder{specs: slices.Clone(specs)}
}

func (p *Placeholder) Controls() []core.ControlSpec {
	out := make([]core.ControlSpec, len(p.specs))
	for i, s := range p.specs {
		s.Options = slices.Clone(s.Options)
		out[i] = s
	}
	return out
}

func (p *Placeholder) Connect(context.Context) error {
	return fmt.Errorf("connect: %w", core.ErrNotImplemented)
}

func (p *Placeholder) Close() error { return nil }

func (p *Placeholder) Status() core.Status {
	return core.Status{State: core.StateDisconnected}
}

func (p *Placeholder) Get(_ context.Context, id core.ControlID) (core.Value, error) {
	if _, err := FindSpec(p.specs, id); err != nil {
		return core.Value{}, err
	}
	return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotConnected)
}

func (p *Placeholder) Set(_ context.Context, id core.ControlID, v core.Value) error {
	s, err := FindSpec(p.specs, id)
	if err != nil {
		return err
	}
	if err := s.Validate(v); err != nil {
		return err
	}
	return fmt.Errorf("set %s: %w", id, core.ErrNotImplemented)
}

func (p *Placeholder) Subscribe(func(core.Event)) (cancel func()) { return func() {} }
