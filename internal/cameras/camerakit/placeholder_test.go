package camerakit_test

import (
	"context"
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func irisSpec() core.ControlSpec {
	s := core.Std(core.Iris)
	s.Kind, s.Min, s.Max = core.KindRange, 1.8, 22
	return s
}

func TestPlaceholderExposesItsControlsOffline(t *testing.T) {
	c := camerakit.NewPlaceholder([]core.ControlSpec{irisSpec()})
	if got := c.Controls(); len(got) != 1 || got[0].ID != core.Iris {
		t.Fatalf("Controls() = %v, want [iris]", got)
	}
	if c.Status().State != core.StateDisconnected {
		t.Fatalf("Status() = %v, want disconnected", c.Status())
	}
}

func TestPlaceholderControlsReturnsACopy(t *testing.T) {
	c := camerakit.NewPlaceholder([]core.ControlSpec{irisSpec()})
	c.Controls()[0].Max = 99
	if c.Controls()[0].Max != 22 {
		t.Fatal("caller mutated the placeholder's specs")
	}
}

func TestPlaceholderConnectIsNotImplemented(t *testing.T) {
	c := camerakit.NewPlaceholder(nil)
	if err := c.Connect(context.Background()); !errors.Is(err, core.ErrNotImplemented) {
		t.Fatalf("Connect = %v, want ErrNotImplemented", err)
	}
}

func TestPlaceholderSetValidatesBeforeAnything(t *testing.T) {
	c := camerakit.NewPlaceholder([]core.ControlSpec{irisSpec()})
	ctx := context.Background()
	if err := c.Set(ctx, core.Iris, core.Number(99)); !errors.Is(err, core.ErrInvalidValue) {
		t.Errorf("Set(out of range) = %v, want ErrInvalidValue", err)
	}
	if err := c.Set(ctx, core.Zoom, core.Number(0.5)); !errors.Is(err, core.ErrUnknownControl) {
		t.Errorf("Set(unsupported) = %v, want ErrUnknownControl", err)
	}
	if err := c.Set(ctx, core.Iris, core.Number(4)); !errors.Is(err, core.ErrNotImplemented) {
		t.Errorf("Set(valid) = %v, want ErrNotImplemented", err)
	}
}

func TestPlaceholderGetIsNotConnected(t *testing.T) {
	c := camerakit.NewPlaceholder([]core.ControlSpec{irisSpec()})
	if _, err := c.Get(context.Background(), core.Iris); !errors.Is(err, core.ErrNotConnected) {
		t.Fatalf("Get = %v, want ErrNotConnected", err)
	}
}
