package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestLoadMissingFileReturnsEmptyRig(t *testing.T) {
	s := FileStore{Path: filepath.Join(t.TempDir(), "rig.json")}
	rig, err := s.Load()
	if err != nil {
		t.Fatalf("Load(missing) = %v, want nil", err)
	}
	if rig.Version != Version || rig.Active != 0 {
		t.Fatalf("Load(missing) = %+v, want empty v%d rig", rig, Version)
	}
}

func TestSaveThenLoadRoundTrips(t *testing.T) {
	s := FileStore{Path: filepath.Join(t.TempDir(), "sub", "rig.json")}
	in := Rig{Version: Version, Active: 2}
	in.Slots[1] = &core.CameraConfig{Model: "sony-fx6", Name: "A", Host: "10.0.0.6", Username: "admin"}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if out.Active != 2 || out.Slots[1] == nil || out.Slots[1].Host != "10.0.0.6" || out.Slots[0] != nil {
		t.Fatalf("round trip = %+v", out)
	}
}

func TestPresetsRoundTrip(t *testing.T) {
	s := FileStore{Path: filepath.Join(t.TempDir(), "rig.json")}
	in := Rig{Version: Version, Presets: []Preset{{
		Slot: 1, Number: 3, Model: "bm-studio-6k-pro",
		Values: map[core.ControlID]core.Value{
			core.Saturation: core.Number(1.34),
			core.ISO:        core.Choice("800"),
			core.Record:     core.Bool(false),
		},
	}}}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Presets) != 1 {
		t.Fatalf("presets = %+v", out.Presets)
	}
	p := out.Presets[0]
	if p.Slot != 1 || p.Number != 3 || p.Model != "bm-studio-6k-pro" ||
		p.Values[core.Saturation] != core.Number(1.34) || p.Values[core.ISO] != core.Choice("800") {
		t.Errorf("preset = %+v", p)
	}
}

func TestRigWithoutPresetsHasNoPresetsKey(t *testing.T) {
	s := FileStore{Path: filepath.Join(t.TempDir(), "rig.json")}
	if err := s.Save(Rig{Version: Version}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.Path)
	if strings.Contains(string(b), "presets") {
		t.Errorf("an empty preset list must not be written: %s", b)
	}
}

func TestSaveNeverWritesPasswords(t *testing.T) {
	s := FileStore{Path: filepath.Join(t.TempDir(), "rig.json")}
	in := Rig{Version: Version}
	in.Slots[0] = &core.CameraConfig{Model: "sony-fx6", Host: "10.0.0.6", Password: "hunter2"}
	if err := s.Save(in); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(s.Path)
	if strings.Contains(string(b), "hunter2") {
		t.Fatalf("password written to config file: %s", b)
	}
}

func TestLoadRejectsUnknownVersion(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rig.json")
	b, _ := json.Marshal(map[string]any{"version": 99})
	_ = os.WriteFile(p, b, 0o600)
	if _, err := (FileStore{Path: p}).Load(); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("Load(v99) = %v, want ErrUnsupportedVersion", err)
	}
}

func TestLoadRejectsMalformedJSON(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rig.json")
	_ = os.WriteFile(p, []byte("{not json"), 0o600)
	if _, err := (FileStore{Path: p}).Load(); err == nil {
		t.Fatal("Load(malformed) = nil, want error")
	}
}

func TestLoadRejectsOversizedFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rig.json")
	_ = os.WriteFile(p, []byte(`{"version":1,"pad":"`+strings.Repeat("x", MaxFileSize)+`"}`), 0o600)
	if _, err := (FileStore{Path: p}).Load(); err == nil {
		t.Fatal("Load(oversized) = nil, want error")
	}
}

func TestLoadClampsActiveSlot(t *testing.T) {
	p := filepath.Join(t.TempDir(), "rig.json")
	_ = os.WriteFile(p, []byte(`{"version":1,"active":42}`), 0o600)
	rig, err := (FileStore{Path: p}).Load()
	if err != nil || rig.Active != 0 {
		t.Fatalf("Load(active 42) = %+v, %v; want active 0", rig, err)
	}
}

func TestMemorySecrets(t *testing.T) {
	m := NewMemorySecrets()
	if _, ok, _ := m.Get("camera/1"); ok {
		t.Fatal("Get on empty store found a value")
	}
	_ = m.Set("camera/1", "pw")
	if v, ok, _ := m.Get("camera/1"); !ok || v != "pw" {
		t.Fatalf("Get = %q, %v", v, ok)
	}
	_ = m.Delete("camera/1")
	_ = m.Delete("camera/1") // idempotent
	if _, ok, _ := m.Get("camera/1"); ok {
		t.Fatal("value survived Delete")
	}
}
