package app_test

import (
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

type fakePanels struct {
	sel  string
	fail bool
}

func (f *fakePanels) List() []app.PanelSource {
	return []app.PanelSource{{ID: app.PanelSourceUSB, Kind: app.PanelKindUSB}, {ID: app.PanelSourceSim, Kind: app.PanelKindSim}}
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
	store := &memStore{rig: app.SavedRig{}}
	p := &fakePanels{sel: app.PanelSourceUSB}
	svc, err := app.NewService(app.Options{Registry: core.NewRegistry(), Store: store, Secrets: newMemSecrets(), Panels: p})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SelectPanelSource("usb:ABC"); err != nil {
		t.Fatal(err)
	}
	if p.sel != "usb:ABC" || store.rig.Panel != "usb:ABC" {
		t.Fatalf("selected %q, saved %q", p.sel, store.rig.Panel)
	}
	if svc.SelectedPanelSource() != "usb:ABC" {
		t.Fatal("SelectedPanelSource not updated")
	}
	if list, sel := svc.PanelSources(); len(list) != 2 || sel != "usb:ABC" {
		t.Fatalf("PanelSources = %v, %q", list, sel)
	}

	p2 := &fakePanels{sel: app.PanelSourceUSB}
	if _, err := app.NewService(app.Options{Registry: core.NewRegistry(), Store: store, Secrets: newMemSecrets(), Panels: p2}); err != nil {
		t.Fatal(err)
	}
	if p2.sel != "usb:ABC" {
		t.Fatalf("restored source = %q", p2.sel)
	}
}

func TestSelectPanelSourceRejectsBadIDs(t *testing.T) {
	p := &fakePanels{sel: app.PanelSourceUSB}
	svc, _ := app.NewService(app.Options{Registry: core.NewRegistry(), Store: &memStore{rig: app.SavedRig{}},
		Secrets: newMemSecrets(), Panels: p})
	for _, id := range []string{"", "usb:", "hdmi"} {
		if err := svc.SelectPanelSource(id); err == nil {
			t.Errorf("SelectPanelSource(%q) = nil", id)
		}
	}
	p.fail = true
	if err := svc.SelectPanelSource(app.PanelSourceSim); err == nil {
		t.Error("a failing selector must report an error")
	}
	noSel, _ := app.NewService(app.Options{Registry: core.NewRegistry(), Store: &memStore{rig: app.SavedRig{}},
		Secrets: newMemSecrets()})
	if list, sel := noSel.PanelSources(); list != nil || sel != "" {
		t.Errorf("without selector: %v, %q", list, sel)
	}
}
