package gui

import (
	"testing"

	"fyne.io/fyne/v2"
)

// Regression: after visiting Configuration (where the slot tabs are hidden),
// coming back to Cameras drew the slot tabs over the strips because the
// header was not laid out again.
func TestSlotTabsDoNotOverlapPageAfterConfig(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(0)
	a.ShowCameras()

	d := fyne.CurrentApp().Driver()
	tabs := d.AbsolutePositionForObject(a.slotBar)
	page := d.AbsolutePositionForObject(a.host)
	if bottom := tabs.Y + a.slotBar.Size().Height; bottom > page.Y+0.5 {
		t.Fatalf("slot tabs end at y=%v but the page starts at y=%v", bottom, page.Y)
	}
	if a.slotBar.Size().Height < 1 {
		t.Fatal("slot tabs have no height after returning to Cameras")
	}
}
