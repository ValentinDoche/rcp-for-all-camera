package rcpmodtest

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// Module checks a module's declaration and the offline behaviour of the
// cameras it builds, with and without a power zoom. The application runs
// stricter checks when it loads the module; this suite catches mistakes
// while writing it.
func Module(t *testing.T, m rcpmod.Module) {
	t.Helper()
	info := m.Info()
	t.Run("info", func(t *testing.T) { checkInfo(t, info) })
	for _, zoom := range []bool{false, true} {
		cfg := rcpmod.Config{Model: info.ID, Name: "contract", Host: "192.0.2.10", Port: info.DefaultPort, PowerZoom: zoom}
		if info.Auth == rcpmod.AuthRequired {
			cfg.Username = "user"
		}
		cam, err := m.New(cfg, NewHost())
		if err != nil {
			t.Fatalf("New(power zoom %t): %v", zoom, err)
		}
		offline(t, cam)
		page := rcpmod.StandardPage()
		if d, ok := cam.(rcpmod.Designer); ok {
			page = d.Page()
		}
		if err := page.Check(cam.Controls()); err != nil {
			t.Errorf("power zoom %t: %v", zoom, err)
		}
		cam.Close()
	}
}

func checkInfo(t *testing.T, info rcpmod.Info) {
	if !validID(info.ID) {
		t.Errorf("id %q: want lowercase words joined by single hyphens", info.ID)
	}
	if info.Brand == "" || info.Name == "" {
		t.Error("brand and name are required")
	}
	if len(info.Transports) == 0 {
		t.Error("declare at least one transport")
	}
	if info.DefaultPort < 0 || info.DefaultPort > 65535 {
		t.Errorf("default port %d out of range", info.DefaultPort)
	}
	switch info.Auth {
	case rcpmod.AuthNone, rcpmod.AuthOptional, rcpmod.AuthRequired:
	default:
		t.Errorf("auth %q unknown", info.Auth)
	}
	switch info.Support {
	case rcpmod.SupportPlanned, rcpmod.SupportExperimental, rcpmod.SupportSupported:
	default:
		t.Errorf("support %q unknown", info.Support)
	}
}

func validID(id string) bool {
	if id == "" || len(id) > 64 || strings.HasPrefix(id, "-") || strings.HasSuffix(id, "-") || strings.Contains(id, "--") {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func offline(t *testing.T, cam rcpmod.Camera) {
	t.Helper()
	specs := cam.Controls()
	if len(specs) == 0 {
		t.Fatal("Controls() is empty: declare the controls the model exposes")
	}
	seen := map[string]bool{}
	for _, s := range specs {
		checkSpec(t, s)
		if seen[s.ID] {
			t.Errorf("control %s declared twice", s.ID)
		}
		seen[s.ID] = true
	}
	if err := cam.Set("contract.noSuchControl", rcpmod.Trigger()); !errors.Is(err, rcpmod.ErrUnknownControl) {
		t.Errorf("Set(unknown control) = %v, want ErrUnknownControl", err)
	}
	for _, s := range specs {
		if s.ReadOnly {
			continue
		}
		if err := cam.Set(s.ID, wrongKind(s.Kind)); !errors.Is(err, rcpmod.ErrInvalidValue) {
			t.Errorf("Set(%s, wrong kind) = %v, want ErrInvalidValue", s.ID, err)
		}
	}
}

func checkSpec(t *testing.T, s rcpmod.ControlSpec) {
	t.Helper()
	if s.ID == "" || len(s.ID) > 64 {
		t.Errorf("control id %q: 1 to 64 characters", s.ID)
	}
	if strings.Contains(s.ID, ".") && (s.Label == "" || s.Group == "") {
		t.Errorf("control %s: a brand-specific control needs a label and a group (rcpmod.Labeled)", s.ID)
	}
	switch s.Kind {
	case rcpmod.KindRange:
		if math.IsNaN(s.Min) || math.IsNaN(s.Max) || s.Min > s.Max || s.Step < 0 {
			t.Errorf("control %s: bad range [%v, %v] step %v", s.ID, s.Min, s.Max, s.Step)
		}
	case rcpmod.KindEnum:
		if len(s.Options) == 0 {
			t.Errorf("control %s: enum without options", s.ID)
		}
		seen := map[string]bool{}
		for _, o := range s.Options {
			if o.Value == "" || seen[o.Value] {
				t.Errorf("control %s: empty or duplicate option %q", s.ID, o.Value)
			}
			seen[o.Value] = true
		}
	case rcpmod.KindToggle, rcpmod.KindAction, rcpmod.KindText:
	default:
		t.Errorf("control %s: unknown kind %q", s.ID, s.Kind)
	}
}

// Online checks a connected camera: every readable control answers Get
// with a value of its kind, finite, and one of its options for an enum.
func Online(t *testing.T, cam rcpmod.Camera) {
	t.Helper()
	for _, s := range cam.Controls() {
		if s.Kind == rcpmod.KindAction {
			continue
		}
		v, err := cam.Get(s.ID)
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
		if s.Kind == rcpmod.KindEnum && !s.ReadOnly && s.Validate(v) != nil {
			t.Errorf("Get(%s) = %q, not one of its options", s.ID, v.Choice)
		}
	}
}

func wrongKind(k string) rcpmod.Value {
	if k == rcpmod.KindEnum || k == rcpmod.KindText {
		return rcpmod.Number(0)
	}
	return rcpmod.Choice("contract-wrong-kind")
}
