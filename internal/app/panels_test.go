package app

import (
	"errors"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

type fakePanels struct {
	sel  string
	fail bool
}

func (f *fakePanels) List() []PanelSource {
	return []PanelSource{{ID: PanelSourceUSB, Kind: PanelKindUSB}, {ID: PanelSourceSim, Kind: PanelKindSim}}
}
func (f *fakePanels) Selected() string { return f.sel }
func (f *fakePanels) Select(id string) error {
	if f.fail {
		return errors.New("boom")
	}
	f.sel = id
	return nil
}

func TestPanelSourceIsSavedAndRestored(t *testing.T) {
	store := &memStore{rig: config.Rig{Version: config.Version}}
	p := &fakePanels{sel: PanelSourceUSB}
	svc, err := NewService(Options{Registry: core.NewRegistry(), Store: store, Secrets: config.NewMemorySecrets(), Panels: p})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SelectPanelSource("usb:ABC"); err != nil {
		t.Fatal(err)
	}
	if p.sel != "usb:ABC" || store.rig.Panel != "usb:ABC" {
		t.Fatalf("selected %q, saved %q", p.sel, store.rig.Panel)
	}
	if list, sel := svc.PanelSources(); len(list) != 2 || sel != "usb:ABC" {
		t.Fatalf("PanelSources = %v, %q", list, sel)
	}

	p2 := &fakePanels{sel: PanelSourceUSB}
	if _, err := NewService(Options{Registry: core.NewRegistry(), Store: store, Secrets: config.NewMemorySecrets(), Panels: p2}); err != nil {
		t.Fatal(err)
	}
	if p2.sel != "usb:ABC" {
		t.Fatalf("restored source = %q", p2.sel)
	}
}

func TestSelectPanelSourceRejectsBadIDs(t *testing.T) {
	p := &fakePanels{sel: PanelSourceUSB}
	svc, _ := NewService(Options{Registry: core.NewRegistry(), Store: &memStore{rig: config.Rig{Version: config.Version}},
		Secrets: config.NewMemorySecrets(), Panels: p})
	for _, id := range []string{"", "usb:", "hdmi"} {
		if err := svc.SelectPanelSource(id); err == nil {
			t.Errorf("SelectPanelSource(%q) = nil", id)
		}
	}
	p.fail = true
	if err := svc.SelectPanelSource(PanelSourceSim); err == nil {
		t.Error("a failing selector must report an error")
	}
	noSel, _ := NewService(Options{Registry: core.NewRegistry(), Store: &memStore{rig: config.Rig{Version: config.Version}},
		Secrets: config.NewMemorySecrets()})
	if list, sel := noSel.PanelSources(); list != nil || sel != "" {
		t.Errorf("without selector: %v, %q", list, sel)
	}
}
