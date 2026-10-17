package gui_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
)

// fakePanels is the panel enumeration: what is plugged in, what is chosen.
type fakePanels struct {
	list []app.PanelSource
	sel  string
}

func (f *fakePanels) List() []app.PanelSource { return f.list }
func (f *fakePanels) Selected() string        { return f.sel }
func (f *fakePanels) Select(id string) error  { f.sel = id; return nil }

// menuItem is one line of an open pop-up menu.
type menuItem struct {
	label   string
	checked bool
}

// openPanelMenu clicks the panel status in the title bar and reads the menu.
func openPanelMenu(t *testing.T, a *gui.App) []menuItem {
	t.Helper()
	status := "Micro Camera Panel non détecté"
	if !win(a).shows(status) {
		status = "Simulateur fermé (cliquer pour choisir le panneau)"
	}
	click(t, a, status)
	if !dialogOpen(a) {
		t.Fatal("clicking the panel status must open the source menu")
	}
	var items []menuItem
	for _, o := range dialog(a).objects() {
		if _, ok := o.(fyne.Tappable); !ok {
			continue
		}
		in := scope{[]fyne.CanvasObject{o}}
		labels := allOf[*canvas.Text](in)
		if len(labels) != 1 {
			continue // the menu itself, not one of its lines
		}
		items = append(items, menuItem{label: labels[0].Text, checked: len(allOf[*canvas.Image](in)) > 0})
	}
	return items
}

func labelsOf(items []menuItem) (labels []string, checked string) {
	for _, m := range items {
		labels = append(labels, m.label)
		if m.checked {
			checked = m.label
		}
	}
	return labels, checked
}

func newPanelApp(t *testing.T, p *fakePanels) *gui.App {
	t.Helper()
	svc, _, _ := newLiveServiceWith(t, func(o *app.Options) { o.Panels = p })
	a := gui.New(fyne.CurrentApp(), svc, syncOptions())
	t.Cleanup(func() { waitWrites(a.Controller()) })
	sized(a)
	return a
}

func TestPanelMenuListsSources(t *testing.T) {
	usb := app.PanelSource{ID: app.PanelSourceUSB, Kind: app.PanelKindUSB}
	sim := app.PanelSource{ID: app.PanelSourceSim, Kind: app.PanelKindSim}
	one := app.PanelSource{ID: "usb:A1", Kind: app.PanelKindUSB, Serial: "A1"}
	two := app.PanelSource{ID: "usb:B2", Kind: app.PanelKindUSB, Serial: "B2"}
	newTestApp(t, false) // the test driver app

	t.Run("one panel", func(t *testing.T) {
		a := newPanelApp(t, &fakePanels{list: []app.PanelSource{usb, one, sim}, sel: app.PanelSourceSim})
		labels, checked := labelsOf(openPanelMenu(t, a))
		if len(labels) != 2 || labels[0] != "USB : A1" || checked != "Simulateur" {
			t.Fatalf("%v checked %q", labels, checked)
		}
	})
	t.Run("two panels", func(t *testing.T) {
		a := newPanelApp(t, &fakePanels{list: []app.PanelSource{usb, one, two, sim}, sel: "usb:B2"})
		labels, checked := labelsOf(openPanelMenu(t, a))
		if len(labels) != 4 || checked != "USB : B2" {
			t.Fatalf("%v checked %q", labels, checked)
		}
	})
	t.Run("the chosen panel is not plugged in", func(t *testing.T) {
		a := newPanelApp(t, &fakePanels{list: []app.PanelSource{usb, sim}, sel: "usb:ZZ"})
		labels, _ := labelsOf(openPanelMenu(t, a))
		if len(labels) != 3 || labels[1] != "USB : ZZ (non branché)" {
			t.Fatalf("%v", labels)
		}
	})
}

func TestPanelMenuSwitchesTheSource(t *testing.T) {
	newTestApp(t, false)
	p := &fakePanels{list: []app.PanelSource{
		{ID: app.PanelSourceUSB, Kind: app.PanelKindUSB}, {ID: app.PanelSourceSim, Kind: app.PanelKindSim},
	}, sel: app.PanelSourceUSB}
	a := newPanelApp(t, p)
	openPanelMenu(t, a)
	clickIn(t, a, dialog(a), "Simulateur")
	if p.sel != app.PanelSourceSim {
		t.Fatalf("selected %q, want the simulator", p.sel)
	}
}
