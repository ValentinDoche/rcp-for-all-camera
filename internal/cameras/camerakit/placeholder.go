// Package camerakit holds the helpers every camera module shares: the
// control set (Specs), the subscriber list (Subscribers), and the module
// and camera used by models whose control protocol is not written yet
// (Planned, Placeholder).
package camerakit

import (
	"context"
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Planned is the module of a camera model whose protocol is not written
// yet: it declares the model and its controls, and builds Placeholders.
// When the protocol lands, the model gets its own module with the same id.
type Planned struct {
	Model core.ModelInfo
	// Specs returns the controls the camera has for a config (a power zoom
	// adds a zoom, for example).
	Specs func(cfg core.CameraConfig) []core.ControlSpec
}

var _ core.Module = Planned{}

// Info is the model declaration.
func (p Planned) Info() core.ModelInfo { return p.Model }

// New builds a Placeholder; it does no I/O.
func (p Planned) New(cfg core.CameraConfig) (core.Camera, error) {
	return NewPlaceholder(p.Specs(cfg)), nil
}

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

func (p *Placeholder) Controls() []core.ControlSpec { return p.specs.All() }

func (p *Placeholder) Connect(context.Context) error {
	return fmt.Errorf("connect: %w", core.ErrNotImplemented)
}

func (p *Placeholder) Close() error { return nil }

func (p *Placeholder) Status() core.Status {
	return core.Status{State: core.StateDisconnected}
}

func (p *Placeholder) Get(_ context.Context, id core.ControlID) (core.Value, error) {
	if _, err := p.specs.Find(id); err != nil {
		return core.Value{}, err
	}
	return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotConnected)
}

func (p *Placeholder) Set(_ context.Context, id core.ControlID, v core.Value) error {
	if _, err := p.specs.Validate(id, v); err != nil {
		return err
	}
	return fmt.Errorf("set %s: %w", id, core.ErrNotImplemented)
}

func (p *Placeholder) Subscribe(func(core.Event)) (cancel func()) { return func() {} }
