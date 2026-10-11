package core

import (
	"errors"
	"strings"
	"testing"
)

func TestRangeSpecAcceptsValueInsideBounds(t *testing.T) {
	s := ControlSpec{ID: Iris, Kind: KindRange, Min: 1.8, Max: 22, Step: 0.1}
	if err := s.Validate(Number(5.6)); err != nil {
		t.Fatalf("Validate(5.6) = %v, want nil", err)
	}
}

func TestRangeSpecRejectsValueOutsideBounds(t *testing.T) {
	s := ControlSpec{ID: Iris, Kind: KindRange, Min: 1.8, Max: 22}
	err := s.Validate(Number(40))
	if !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Validate(40) = %v, want ErrInvalidValue", err)
	}
}

func TestRangeSpecRejectsWrongKind(t *testing.T) {
	s := ControlSpec{ID: Iris, Kind: KindRange, Min: 0, Max: 1}
	if err := s.Validate(Choice("auto")); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Validate(choice) = %v, want ErrInvalidValue", err)
	}
}

func TestRangeSpecRejectsNaN(t *testing.T) {
	s := ControlSpec{ID: Iris, Kind: KindRange, Min: 0, Max: 1}
	nan := 0.0
	nan = nan / nan
	if err := s.Validate(Number(nan)); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Validate(NaN) = %v, want ErrInvalidValue", err)
	}
}

func TestEnumSpecAcceptsKnownOption(t *testing.T) {
	s := ControlSpec{ID: ND, Kind: KindEnum, Options: []Option{{Value: "clear"}, {Value: "1/4"}}}
	if err := s.Validate(Choice("1/4")); err != nil {
		t.Fatalf("Validate(1/4) = %v, want nil", err)
	}
}

func TestEnumSpecRejectsUnknownOption(t *testing.T) {
	s := ControlSpec{ID: ND, Kind: KindEnum, Options: []Option{{Value: "clear"}}}
	if err := s.Validate(Choice("1/64")); !errors.Is(err, ErrInvalidValue) {
		t.Fatalf("Validate(1/64) = %v, want ErrInvalidValue", err)
	}
}

func TestToggleSpecAcceptsBool(t *testing.T) {
	s := ControlSpec{ID: ColorBars, Kind: KindToggle}
	if err := s.Validate(Bool(true)); err != nil {
		t.Fatalf("Validate(true) = %v, want nil", err)
	}
}

func TestActionSpecAcceptsTrigger(t *testing.T) {
	s := ControlSpec{ID: AutoWhiteBalance, Kind: KindAction}
	if err := s.Validate(Trigger()); err != nil {
		t.Fatalf("Validate(trigger) = %v, want nil", err)
	}
}

func TestReadOnlySpecRejectsAnyWrite(t *testing.T) {
	s := ControlSpec{ID: Tally, Kind: KindToggle, ReadOnly: true}
	if err := s.Validate(Bool(true)); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Validate on read-only = %v, want ErrReadOnly", err)
	}
}

func TestSpecCheckRejectsMalformedSpecs(t *testing.T) {
	cases := map[string]ControlSpec{
		"empty id":         {Kind: KindToggle},
		"unknown kind":     {ID: "x", Kind: "dial"},
		"inverted range":   {ID: "x", Kind: KindRange, Min: 5, Max: 1},
		"enum no options":  {ID: "x", Kind: KindEnum},
		"negative step":    {ID: "x", Kind: KindRange, Min: 0, Max: 1, Step: -1},
		"duplicate option": {ID: "x", Kind: KindEnum, Options: []Option{{Value: "a"}, {Value: "a"}}},
	}
	for name, s := range cases {
		if err := s.Check(); err == nil {
			t.Errorf("%s: Check() = nil, want error", name)
		}
	}
}

func TestStdFillsLabelAndGroupFromCatalog(t *testing.T) {
	s := Std(Iris)
	if s.Label == "" || s.Group != GroupLens {
		t.Fatalf("Std(Iris) = %+v, want label and lens group", s)
	}
}

func TestSpecCheckBoundsSection(t *testing.T) {
	ok := ControlSpec{ID: "x", Kind: KindToggle, Section: "Matrice linéaire"}
	if err := ok.Check(); err != nil {
		t.Fatalf("Check(valid section) = %v", err)
	}
	for name, sec := range map[string]string{
		"too long":     strings.Repeat("s", MaxIDLen+1),
		"control char": "a\nb",
	} {
		s := ControlSpec{ID: "x", Kind: KindToggle, Section: sec}
		if err := s.Check(); err == nil {
			t.Errorf("%s: Check() = nil, want error", name)
		}
	}
}
