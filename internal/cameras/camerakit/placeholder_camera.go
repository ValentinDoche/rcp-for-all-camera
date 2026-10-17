// Package camerakit holds helpers for core.Camera implementations: the
// control set (Specs), the subscriber list (Subscribers) and Placeholder,
// a camera that declares controls without any protocol (tests).
package camerakit

import (
	"context"
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Placeholder is a camera that declares its controls but has no protocol
// yet. The GUI can show and validate every control; any I/O returns
// ErrNotImplemented.
type Placeholder struct {
	specs Specs
}

var _ core.Camera = (*Placeholder)(nil)

// NewPlaceholder builds a placeholder with a copy of specs.
func NewPlaceholder(specs []core.ControlSpec) *Placeholder {
	return &Placeholder{specs: NewSpecs(specs)}
}

// Controls returns the declared controls.
func (p *Placeholder) Controls() []core.ControlSpec { return p.specs.All() }

// Connect fails: a placeholder never reaches a camera.
func (p *Placeholder) Connect(context.Context) error {
	return fmt.Errorf("connect: %w", core.ErrNotImplemented)
}

// Close does nothing.
func (p *Placeholder) Close() error { return nil }

// Status reports the camera as not implemented.
func (p *Placeholder) Status() core.Status {
	return core.Status{State: core.StateDisconnected}
}

// Get fails with ErrNotImplemented for a declared control.
func (p *Placeholder) Get(_ context.Context, id core.ControlID) (core.Value, error) {
	if _, err := p.specs.Find(id); err != nil {
		return core.Value{}, err
	}
	return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotConnected)
}

// Set fails with ErrNotImplemented for a declared control.
func (p *Placeholder) Set(_ context.Context, id core.ControlID, v core.Value) error {
	if _, err := p.specs.Validate(id, v); err != nil {
		return err
	}
	return fmt.Errorf("set %s: %w", id, core.ErrNotImplemented)
}

// Subscribe never reports anything.
func (p *Placeholder) Subscribe(func(core.Event)) (cancel func()) { return func() {} }
