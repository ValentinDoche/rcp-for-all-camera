package gui_test

import (
	"testing"

	"fyne.io/fyne/v2"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
)

func TestGearOpensSettingsAndCloseClosesIt(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	press(a, gear(t, a))
	if !settingsOpen(a) {
		t.Fatal("the gear must open Settings")
	}
	clickIn(t, a, settings(a), "Fermer")
	if settingsOpen(a) {
		t.Fatal("Fermer must close Settings")
	}
}

func TestStatusBarShowsHostInfo(t *testing.T) {
	a, _ := newStatsApp(t, someStats())
	for _, want := range []string{"CPU 12 %", "RAM 22,3 / 63,3 Go", "↓ 5,2 ko/s", "↑ 1,6 ko/s", "IP 192.168.1.110"} {
		if !win(a).shows(want) {
			t.Errorf("status bar lacks %q", want)
		}
	}
}

func TestStatusBarBeforeFirstRates(t *testing.T) {
	a, _ := newStatsApp(t, gui.HostStats{MemUsed: 1 << 30, MemTotal: 2 << 30})
	for _, want := range []string{"CPU --", "IP --", "RAM 1,0 / 2,0 Go"} {
		if !win(a).shows(want) {
			t.Errorf("status bar lacks %q: %s", want, win(a).joined())
		}
	}
}

func TestEmptySlotTabIsDisabled(t *testing.T) {
	a, _, _ := newTestApp(t, true) // slots 1 to 3 assigned, 2 active
	sized(a)
	clickTab(t, a, "Cam6")
	waitWrites(a.Controller())
	if got := a.Controller().Active(); got != 2 {
		t.Fatalf("clicking the empty slot 6 selected slot %d", got)
	}
	if tabs := slotTabs(t, a); !tabs.Disabled(5) || tabs.Disabled(0) {
		t.Fatal("tab 6 must be disabled and tab 1 enabled")
	}
}

// Regression (0.2.0): the stats were centred in the space between the gear
// and the notice, so they sat left of the window's centre.
func TestStatsAreCentredInTheWindow(t *testing.T) {
	a, _ := newStatsApp(t, someStats())
	sized(a)
	d := fyne.CurrentApp().Driver()
	first, last := win(a).textObject("CPU 12 %"), win(a).textObject("IP 192.168.1.110")
	left := d.AbsolutePositionForObject(first).X
	right := d.AbsolutePositionForObject(last).X + last.Size().Width
	mid, centre := (left+right)/2, a.Window().Canvas().Size().Width/2
	if diff := mid - centre; diff < -12 || diff > 12 {
		t.Fatalf("stats centred at x=%v, window centre %v", mid, centre)
	}
}

func TestStatsCanBeHiddenFromSettings(t *testing.T) {
	a, svc := newStatsApp(t, someStats())
	category(t, a, "Général")
	choose(t, a, "Statistiques système", "Non")
	if win(a).has("CPU") || a.ShowStats() {
		t.Fatal("stats still shown")
	}
	choose(t, a, "Statistiques système", "Oui")
	if !win(a).shows("CPU 12 %") {
		t.Fatalf("stats not shown again: %s", win(a).joined())
	}
	choose(t, a, "Statistiques système", "Non")
	if gui.New(fyne.CurrentApp(), svc, syncOptions()).ShowStats() {
		t.Fatal("the choice is not remembered for the next start")
	}
}
