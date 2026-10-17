package core_test

import (
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"strings"
	"testing"
)

func TestRangeSpecAcceptsValueInsideBounds(t *testing.T) {
	s := core.ControlSpec{ID: core.Iris, Kind: core.KindRange, Min: 1.8, Max: 22, Step: 0.1}
	if err := s.Validate(core.Number(5.6)); err != nil {
		t.Fatalf("Validate(5.6) = %v, want nil", err)
	}
}

func TestRangeSpecRejectsValueOutsideBounds(t *testing.T) {
	s := core.ControlSpec{ID: core.Iris, Kind: core.KindRange, Min: 1.8, Max: 22}
	err := s.Validate(core.Number(40))
	if !errors.Is(err, core.ErrInvalidValue) {
		t.Fatalf("Validate(40) = %v, want ErrInvalidValue", err)
	}
}

func TestRangeSpecRejectsWrongKind(t *testing.T) {
	s := core.ControlSpec{ID: core.Iris, Kind: core.KindRange, Min: 0, Max: 1}
	if err := s.Validate(core.Choice("auto")); !errors.Is(err, core.ErrInvalidValue) {
		t.Fatalf("Validate(choice) = %v, want ErrInvalidValue", err)
	}
}

func TestRangeSpecRejectsNaN(t *testing.T) {
	s := core.ControlSpec{ID: core.Iris, Kind: core.KindRange, Min: 0, Max: 1}
	nan := 0.0
	nan = nan / nan
	if err := s.Validate(core.Number(nan)); !errors.Is(err, core.ErrInvalidValue) {
		t.Fatalf("Validate(NaN) = %v, want ErrInvalidValue", err)
	}
}

func TestEnumSpecAcceptsKnownOption(t *testing.T) {
	s := core.ControlSpec{ID: core.ND, Kind: core.KindEnum, Options: []core.Option{{Value: "clear"}, {Value: "1/4"}}}
	if err := s.Validate(core.Choice("1/4")); err != nil {
		t.Fatalf("Validate(1/4) = %v, want nil", err)
	}
}

func TestEnumSpecRejectsUnknownOption(t *testing.T) {
	s := core.ControlSpec{ID: core.ND, Kind: core.KindEnum, Options: []core.Option{{Value: "clear"}}}
	if err := s.Validate(core.Choice("1/64")); !errors.Is(err, core.ErrInvalidValue) {
		t.Fatalf("Validate(1/64) = %v, want ErrInvalidValue", err)
	}
}

func TestToggleSpecAcceptsBool(t *testing.T) {
	s := core.ControlSpec{ID: core.ColorBars, Kind: core.KindToggle}
	if err := s.Validate(core.Bool(true)); err != nil {
		t.Fatalf("Validate(true) = %v, want nil", err)
	}
}

func TestActionSpecAcceptsTrigger(t *testing.T) {
	s := core.ControlSpec{ID: core.AutoWhiteBalance, Kind: core.KindAction}
	if err := s.Validate(core.Trigger()); err != nil {
		t.Fatalf("Validate(trigger) = %v, want nil", err)
	}
}

func TestReadOnlySpecRejectsAnyWrite(t *testing.T) {
	s := core.ControlSpec{ID: core.Tally, Kind: core.KindToggle, ReadOnly: true}
	if err := s.Validate(core.Bool(true)); !errors.Is(err, core.ErrReadOnly) {
		t.Fatalf("Validate on read-only = %v, want ErrReadOnly", err)
	}
}

func TestSpecCheckRejectsMalformedSpecs(t *testing.T) {
	cases := map[string]core.ControlSpec{
		"empty id":         {Kind: core.KindToggle},
		"unknown kind":     {ID: "x", Kind: "dial"},
		"inverted range":   {ID: "x", Kind: core.KindRange, Min: 5, Max: 1},
		"enum no options":  {ID: "x", Kind: core.KindEnum},
		"negative step":    {ID: "x", Kind: core.KindRange, Min: 0, Max: 1, Step: -1},
		"duplicate option": {ID: "x", Kind: core.KindEnum, Options: []core.Option{{Value: "a"}, {Value: "a"}}},
	}
	for name, s := range cases {
		if err := s.Check(); err == nil {
			t.Errorf("%s: Check() = nil, want error", name)
		}
	}
}

func TestStdFillsLabelAndGroupFromCatalog(t *testing.T) {
	s := core.Std(core.Iris)
	if s.Label == "" || s.Group != core.GroupLens {
		t.Fatalf("Std(Iris) = %+v, want label and lens group", s)
	}
}

func TestSpecCheckBoundsSection(t *testing.T) {
	ok := core.ControlSpec{ID: "x", Kind: core.KindToggle, Section: "Matrice linéaire"}
	if err := ok.Check(); err != nil {
		t.Fatalf("Check(valid section) = %v", err)
	}
	for name, sec := range map[string]string{
		"too long":     strings.Repeat("s", core.MaxIDLen+1),
		"control char": "a\nb",
	} {
		s := core.ControlSpec{ID: "x", Kind: core.KindToggle, Section: sec}
		if err := s.Check(); err == nil {
			t.Errorf("%s: Check() = nil, want error", name)
		}
	}
}
