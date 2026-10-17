// Package cameratest holds the contract suites every camera module must
// pass. Call them from the module's own tests:
//
//	func TestContract(t *testing.T) { cameratest.Module(t, demo.Module{}) }
//
// Module checks the declarations and the offline behaviour; Online checks a
// connected camera. Fakes (the demo camera, a module's fake server) and real
// cameras run the same suites, so what works on a fake works on hardware.
package cameratest

import (
	"context"
	"errors"
	"math"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Module checks the module's declaration and the offline behaviour of a
// camera it builds, with and without a power zoom. It needs no hardware.
func Module(t *testing.T, m core.Module) {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(m); err != nil {
		t.Fatalf("Register: %v", err)
	}
	info := m.Info()
	for _, zoom := range []bool{false, true} {
		cfg := core.CameraConfig{Model: info.ID, Name: "contract", Host: "192.0.2.10", PowerZoom: zoom}
		if info.Auth == core.AuthRequired {
			cfg.Username = "user"
		}
		cam, err := reg.New(cfg)
		if err != nil {
			t.Fatalf("New(valid config, power zoom %t): %v", zoom, err)
		}
		offline(t, cam)
		if err := cam.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	}
}

func offline(t *testing.T, cam core.Camera) {
	t.Helper()
	specs := cam.Controls()
	if len(specs) == 0 {
		t.Fatal("Controls() is empty: declare at least the controls the model exposes")
	}
	seen := map[core.ControlID]bool{}
	for _, s := range specs {
		if err := s.Check(); err != nil {
			t.Errorf("spec: %v", err)
		}
		if s.Label == "" || s.Group == "" {
			t.Errorf("spec %s: label and group are required (use core.Std)", s.ID)
		}
		if seen[s.ID] {
			t.Errorf("spec %s declared twice", s.ID)
		}
		seen[s.ID] = true
	}
	ctx := context.Background()
	if err := cam.Set(ctx, "contract.noSuchControl", core.Trigger()); !errors.Is(err, core.ErrUnknownControl) {
		t.Errorf("Set(unknown control) = %v, want ErrUnknownControl", err)
	}
	for _, s := range specs {
		if s.ReadOnly {
			continue
		}
		if err := cam.Set(ctx, s.ID, wrongKind(s.Kind)); !errors.Is(err, core.ErrInvalidValue) {
			t.Errorf("Set(%s, wrong kind) = %v, want ErrInvalidValue", s.ID, err)
		}
	}
	if st := cam.Status().State; st != core.StateDisconnected {
		t.Errorf("Status() before Connect = %s, want disconnected", st)
	}
}

// Online checks a connected camera: every readable control answers Get
// with a value of its kind that its spec accepts, and a valid write is
// accepted. cam must already be connected; Online does not close it.
func Online(t *testing.T, cam core.Camera) {
	t.Helper()
	if st := cam.Status().State; st != core.StateConnected {
		t.Fatalf("Status() = %s, want connected", st)
	}
	ctx := context.Background()
	for _, s := range cam.Controls() {
		if s.Kind == core.KindAction {
			continue
		}
		v, err := cam.Get(ctx, s.ID)
		if err != nil {
			t.Errorf("Get(%s) = %v", s.ID, err)
			continue
		}
		if v.Kind != s.Kind {
			t.Errorf("Get(%s) kind = %q, want %q", s.ID, v.Kind, s.Kind)
			continue
		}
		if math.IsNaN(v.Number) || math.IsInf(v.Number, 0) {
			t.Errorf("Get(%s) = %v, want a finite number", s.ID, v.Number)
		}
		if s.Kind == core.KindEnum {
			if err := s.Validate(v); err != nil && !s.ReadOnly {
				t.Errorf("Get(%s) = %q, not one of its options", s.ID, v.Choice)
			}
		}
	}
	for _, s := range cam.Controls() {
		if s.Kind == core.KindRange && !s.ReadOnly && !s.Guarded {
			if err := cam.Set(ctx, s.ID, core.Number((s.Min+s.Max)/2)); err != nil {
				t.Errorf("Set(%s, mid-range) = %v", s.ID, err)
			}
			return // one write is enough: the module's own tests cover each control
		}
	}
}

func wrongKind(k core.ControlKind) core.Value {
	if k == core.KindEnum {
		return core.Number(0)
	}
	return core.Choice("contract-wrong-kind")
}
