package gui_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// The parts of the main window, found the way a user sees them.

// gear is the Settings button at the bottom left (an icon, no text).
func gear(t *testing.T, a *gui.App) *widget.Button {
	t.Helper()
	for _, b := range allOf[*widget.Button](win(a)) {
		if b.Icon != nil && b.Icon.Name() == theme.SettingsIcon().Name() {
			return b
		}
	}
	t.Fatal("no Settings gear in the bottom bar")
	return nil
}

// settings is the Settings window, empty when it is closed.
func settings(a *gui.App) scope {
	if s := win(a).around("Fermer", withA[*widgets.SideNav]()); s.roots != nil {
		return s
	}
	return win(a).around("Close", withA[*widgets.SideNav]()) // English
}

// settingsOpen reports whether the Settings window is shown.
func settingsOpen(a *gui.App) bool { return settings(a).roots != nil }

// openSettings opens Settings with the gear, on its last used category.
func openSettings(t *testing.T, a *gui.App) scope {
	t.Helper()
	sized(a)
	if !settingsOpen(a) {
		press(a, gear(t, a))
	}
	s := settings(a)
	if s.roots == nil {
		t.Fatal("the gear did not open Settings")
	}
	return s
}

// category shows a Settings category (Général, Caméras, Panneau, À propos).
func category(t *testing.T, a *gui.App, name string) scope {
	t.Helper()
	s := openSettings(t, a)
	clickIn(t, a, scope{[]fyne.CanvasObject{one[*widgets.SideNav](t, s)}}, name)
	return settings(a)
}

// setting is the row of a setting in Settings, found by its caption.
func setting(t *testing.T, a *gui.App, caption string) scope {
	t.Helper()
	r := settings(a).around(caption, func(s scope) bool {
		return len(allOf[*widgets.Segmented](s))+len(allOf[*widget.Select](s))+len(allOf[*widget.Button](s)) > 0
	})
	if r.roots == nil {
		t.Fatalf("no setting %q; Settings shows: %s", caption, settings(a).joined())
	}
	return r
}

// choose clicks option on the selector of the setting caption.
func choose(t *testing.T, a *gui.App, caption, option string) {
	t.Helper()
	row := setting(t, a, caption)
	clickIn(t, a, scope{[]fyne.CanvasObject{one[*widgets.Segmented](t, row)}}, option)
}

// slotTabs is the bar of camera tabs under the title bar.
func slotTabs(t *testing.T, a *gui.App) *widgets.Tabs {
	t.Helper()
	return one[*widgets.Tabs](t, win(a))
}

// clickTab clicks the camera tab named name ("Cam3").
func clickTab(t *testing.T, a *gui.App, name string) {
	t.Helper()
	clickIn(t, a, scope{[]fyne.CanvasObject{slotTabs(t, a)}}, name)
}

// strip is the strip of the camera named name on the Cameras page ("Plateau"
// for the Studio 6K Pro of the seeded rig, "Cam4" for a free slot).
func strip(t *testing.T, a *gui.App, slot string) scope {
	t.Helper()
	s := win(a).around(slot, func(s scope) bool {
		return s.shows("Détails") || s.shows("Assigner une caméra") || s.shows("Details") || s.shows("Assign a camera")
	})
	if s.roots == nil {
		t.Fatalf("no strip for %q; shown: %s", slot, win(a).joined())
	}
	return s
}

// cameraForm opens Settings > Caméras and the form of slot ("Cam4"): Modifier
// on an assigned slot, Assigner on a free one.
func cameraForm(t *testing.T, a *gui.App, slot string) scope {
	t.Helper()
	list := category(t, a, "Caméras")
	row := list.around(slot, func(s scope) bool { return s.shows("Modifier") || s.shows("Assigner") })
	if row.roots == nil {
		t.Fatalf("no row for %s in Settings > Caméras: %s", slot, list.joined())
	}
	if row.shows("Modifier") {
		clickIn(t, a, row, "Modifier")
	} else {
		clickIn(t, a, row, "Assigner")
	}
	return form(t, a)
}

// form is the camera form open in Settings > Caméras.
func form(t *testing.T, a *gui.App) scope {
	t.Helper()
	f := settings(a).around("Marque", func(s scope) bool { return s.shows("Enregistrer") && s.shows("Annuler") })
	if f.roots == nil {
		t.Fatalf("no camera form open: %s", settings(a).joined())
	}
	return f
}

// formOpen reports whether a camera form is open.
func formOpen(a *gui.App) bool {
	return settings(a).around("Marque", withText("Annuler")).roots != nil
}

// entry is the text field under caption in s.
func entry(t *testing.T, s scope, caption string) *widget.Entry {
	t.Helper()
	return fieldAfter[*widget.Entry](t, s, caption)
}

// selectBox is the select box under caption in s.
func selectBox(t *testing.T, s scope, caption string) *widget.Select {
	t.Helper()
	return fieldAfter[*widget.Select](t, s, caption)
}

// checkBox is the check box labelled text in s.
func checkBox(t *testing.T, s scope, text string) *widget.Check {
	t.Helper()
	for _, c := range allOf[*widget.Check](s) {
		if c.Text == text {
			return c
		}
	}
	t.Fatalf("no check box %q: %s", text, s.joined())
	return nil
}

// chooseModel picks brand then model in the open camera form.
func chooseModel(t *testing.T, a *gui.App, brand, model string) {
	t.Helper()
	pick(t, a, selectBox(t, form(t, a), "Marque"), brand)
	if model != "" {
		pick(t, a, selectBox(t, form(t, a), "Modèle"), model)
	}
}

// saveForm clicks Enregistrer in the camera form.
func saveForm(t *testing.T, a *gui.App) {
	t.Helper()
	clickIn(t, a, form(t, a), "Enregistrer")
}
