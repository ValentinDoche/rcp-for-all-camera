package camerakit_test

import (
	"errors"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func sampleSpecs() camerakit.Specs {
	return camerakit.NewSpecs([]core.ControlSpec{
		speckit.Range(core.Iris, 1, 22, 0.1, "f"),
		speckit.Enum(core.ISO, speckit.Opts("400", "800")),
		speckit.ReadOnly(speckit.Toggle(core.Tally)),
		speckit.Range(core.Zoom, 0, 1, 0.001, ""),
	})
}

func TestSpecsValidate(t *testing.T) {
	s := sampleSpecs()
	t.Run("accepts a value inside the spec", func(t *testing.T) {
		if _, err := s.Validate(core.Iris, core.Number(4)); err != nil {
			t.Errorf("Validate(iris 4) = %v", err)
		}
	})
	for name, tc := range map[string]struct {
		id   core.ControlID
		v    core.Value
		want error
	}{
		"unknown control": {"acme.nothing", core.Number(1), core.ErrUnknownControl},
		"out of range":    {core.Iris, core.Number(40), core.ErrInvalidValue},
		"unknown option":  {core.ISO, core.Choice("12"), core.ErrInvalidValue},
		"read-only":       {core.Tally, core.Bool(true), core.ErrReadOnly},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := s.Validate(tc.id, tc.v); !errors.Is(err, tc.want) {
				t.Errorf("Validate(%s) = %v, want %v", tc.id, err, tc.want)
			}
		})
	}
}

func TestSpecsAreCopies(t *testing.T) {
	s := sampleSpecs()
	all := s.All()
	all[1].Options[0].Value = "changed"
	got, _ := s.Find(core.ISO)
	if got.Options[0].Value != "400" {
		t.Errorf("All() shares its options with the set: %q", got.Options[0].Value)
	}
}

func TestSpecsWithout(t *testing.T) {
	s := sampleSpecs().Without(core.Zoom)
	if s.Has(core.Zoom) {
		t.Error("Without(zoom) still has zoom")
	}
	if len(s.All()) != 3 {
		t.Errorf("Without(zoom) has %d controls, want 3", len(s.All()))
	}
}

func TestSubscribersPublishInOrder(t *testing.T) {
	var subs camerakit.Subscribers
	var got []core.ControlID
	subs.Add(func(e core.Event) { got = append(got, e.Control) })
	subs.Publish(core.Event{Control: core.Iris}, core.Event{Control: core.Gain})
	if len(got) != 2 || got[0] != core.Iris || got[1] != core.Gain {
		t.Errorf("events received %v, want [iris gain]", got)
	}
}

func TestSubscribersCancel(t *testing.T) {
	var subs camerakit.Subscribers
	calls := 0
	cancel := subs.Add(func(core.Event) { calls++ })
	cancel()
	subs.Publish(core.Event{Control: core.Iris})
	if calls != 0 {
		t.Errorf("a cancelled subscriber was called %d times", calls)
	}
}

func TestSubscribersClear(t *testing.T) {
	var subs camerakit.Subscribers
	calls := 0
	subs.Add(func(core.Event) { calls++ })
	subs.Clear()
	subs.Publish(core.Event{Control: core.Iris})
	if calls != 0 {
		t.Errorf("a cleared subscriber was called %d times", calls)
	}
}
