// Package cameratest is the contract suite every camera driver must pass.
// Call it from the driver's own test:
//
//	func TestContract(t *testing.T) { cameratest.Driver(t, sony.Driver{}) }
package cameratest

import (
	"context"
	"errors"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Driver checks the driver's declarations and the offline behaviour of a
// camera built for each of its models. It needs no hardware.
func Driver(t *testing.T, d core.Driver) {
	t.Helper()
	reg := core.NewRegistry()
	if err := reg.Register(d); err != nil {
		t.Fatalf("Register(%s): %v", d.Name(), err)
	}
	if len(d.Models()) == 0 {
		t.Fatalf("driver %s declares no model", d.Name())
	}
	for _, m := range d.Models() {
		t.Run(string(m.ID), func(t *testing.T) { model(t, reg, m) })
	}
}

func model(t *testing.T, reg *core.Registry, m core.ModelInfo) {
	cfg := core.CameraConfig{Model: m.ID, Name: "contract", Host: "192.0.2.10"}
	if m.Auth == core.AuthRequired {
		cfg.Username = "user"
	}
	cam, err := reg.New(cfg)
	if err != nil {
		t.Fatalf("New(valid config): %v", err)
	}
	defer cam.Close()

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

func wrongKind(k core.ControlKind) core.Value {
	if k == core.KindEnum {
		return core.Number(0)
	}
	return core.Choice("contract-wrong-kind")
}
