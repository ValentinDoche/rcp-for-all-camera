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

// Owner request: no slot tabs in Configuration, and the page moves up.
func TestConfigurationHasNoSlotTabs(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	withTabs := pageTop(a)
	a.ShowConfig(0)
	if headerHas(a, a.slotBar) {
		t.Fatal("slot tabs still in the header on the Configuration page")
	}
	if got := pageTop(a); got >= withTabs {
		t.Fatalf("page top %v, want above %v once the tabs are gone", got, withTabs)
	}
	a.ShowCameras()
	if !headerHas(a, a.slotBar) || pageTop(a) != withTabs {
		t.Fatal("slot tabs not restored on the Cameras page")
	}
}
