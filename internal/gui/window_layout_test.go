package gui_test

import (
	"testing"

	"fyne.io/fyne/v2"
)

// edges returns the absolute y of o's top edge and of its bottom edge.
func edges(o fyne.CanvasObject) (top, bottom float32) {
	y := fyne.CurrentApp().Driver().AbsolutePositionForObject(o).Y
	return y, y + o.Size().Height
}

// Regression: after visiting Settings, coming back to Cameras drew the slot
// tabs over the strips because the header was not laid out again.
func TestSlotTabsDoNotOverlapThePageAfterSettings(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	openSettings(t, a)
	clickIn(t, a, settings(a), "Fermer")

	tabs := slotTabs(t, a)
	tabsTop, tabsBottom := edges(tabs)
	stripTop, _ := edges(strip(t, a, "Cam1").roots[0])
	if tabsBottom-tabsTop < 1 {
		t.Fatal("slot tabs have no height after closing Settings")
	}
	if tabsBottom > stripTop+0.5 {
		t.Fatalf("slot tabs end at y=%v but the strips start at y=%v", tabsBottom, stripTop)
	}
}

// Settings is a modal over the page: it starts under the title bar, whose
// window buttons stay usable, and the page is shown again as it was when
// it closes.
func TestSettingsIsAModalUnderTheTitleBar(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	beforeTop, _ := edges(slotTabs(t, a))
	s := openSettings(t, a)
	_, titleBottom := edges(win(a).textObject("RCP For All Camera"))
	if top, _ := edges(s.roots[0]); top < titleBottom {
		t.Fatalf("Settings starts at y=%v, over the title bar (its name ends at y=%v)", top, titleBottom)
	}
	clickIn(t, a, settings(a), "Fermer")
	if settingsOpen(a) {
		t.Fatal("Settings still open")
	}
	if top, _ := edges(slotTabs(t, a)); top != beforeTop {
		t.Fatalf("slot tabs moved from y=%v to y=%v", beforeTop, top)
	}
}
