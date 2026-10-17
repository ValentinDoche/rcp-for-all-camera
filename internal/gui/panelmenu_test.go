package gui

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
)

func menuLabels(a *App, list []app.PanelSource, sel string) (labels []string, checked string) {
	for _, m := range a.panelMenuItems(list, sel, func(string) {}) {
		if m.IsSeparator {
			continue
		}
		labels = append(labels, m.Label)
		if m.Checked {
			checked = m.Label
		}
	}
	return labels, checked
}

func TestPanelMenuListsSources(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	usb := app.PanelSource{ID: app.PanelSourceUSB, Kind: app.PanelKindUSB}
	sim := app.PanelSource{ID: app.PanelSourceSim, Kind: app.PanelKindSim}
	one := app.PanelSource{ID: "usb:A1", Kind: app.PanelKindUSB, Serial: "A1"}
	two := app.PanelSource{ID: "usb:B2", Kind: app.PanelKindUSB, Serial: "B2"}

	labels, checked := menuLabels(a, []app.PanelSource{usb, one, sim}, app.PanelSourceSim)
	if len(labels) != 2 || labels[0] != "USB : A1" || checked != "Simulateur" {
		t.Fatalf("one panel: %v checked %q", labels, checked)
	}
	labels, checked = menuLabels(a, []app.PanelSource{usb, one, two, sim}, "usb:B2")
	if len(labels) != 4 || checked != "USB : B2" {
		t.Fatalf("two panels: %v checked %q", labels, checked)
	}
	labels, _ = menuLabels(a, []app.PanelSource{usb, sim}, "usb:ZZ")
	if len(labels) != 3 || labels[1] != "USB : ZZ (non branché)" {
		t.Fatalf("absent chosen panel: %v", labels)
	}
}
