package gui

import (
	"testing"

	"fyne.io/fyne/v2"
)

// pageTop is the y where the page area starts, absolute.
func pageTop(a *App) float32 {
	return fyne.CurrentApp().Driver().AbsolutePositionForObject(a.host).Y
}

func headerHas(a *App, o fyne.CanvasObject) bool {
	for _, x := range a.header.Objects {
		if x == o {
			return true
		}
	}
	return false
}

// Regression: after visiting Configuration, coming back to Cameras drew the
// slot tabs over the strips because the header was not laid out again.
func TestSlotTabsDoNotOverlapPageAfterConfig(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(0)
	a.ShowCameras()

	d := fyne.CurrentApp().Driver()
	tabs := d.AbsolutePositionForObject(a.slotBar)
	if bottom := tabs.Y + a.slotBar.Size().Height; bottom > pageTop(a)+0.5 {
		t.Fatalf("slot tabs end at y=%v but the page starts at y=%v", bottom, pageTop(a))
	}
	if a.slotBar.Size().Height < 1 {
		t.Fatal("slot tabs have no height after returning to Cameras")
	}
}

// Settings is a modal over the page: the page and its slot tabs stay put
// underneath and are shown again as they were when it closes.
func TestSettingsIsAModalOverThePage(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	before := pageTop(a)
	a.ShowConfig(0)
	if a.configPop == nil || !a.configPop.Visible() {
		t.Fatal("Settings not shown as a modal")
	}
	if n := len(a.Window().Canvas().Overlays().List()); n != 1 {
		t.Fatalf("%d overlays, want only Settings", n)
	}
	a.CloseConfig()
	if a.Window().Canvas().Overlays().Top() != nil || pageTop(a) != before || !headerHas(a, a.slotBar) {
		t.Fatal("page not restored after closing Settings")
	}
}
