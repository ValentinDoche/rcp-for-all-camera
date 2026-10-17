package gui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

func TestGearTogglesConfiguration(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	test.Tap(a.gear)
	if a.page != pageConfig {
		t.Fatalf("after gear: page %v, want config", a.page)
	}
	test.Tap(a.gear)
	if a.page != pageCameras {
		t.Fatalf("after second gear: page %v, want cameras", a.page)
	}
}

func TestSlotTabFromConfigReturnsToCameras(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(0)
	a.onSlotTab(2)
	if a.page != pageCameras || a.ctl.Active() != 2 {
		t.Fatalf("page %v active %d, want cameras and slot 2", a.page, a.ctl.Active())
	}
}

func TestStatusBarShowsHostInfo(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	a.setStats(HostStats{CPU: 12.4, MemUsed: 23915626496, MemTotal: 67939266560,
		DownBps: 5329.3, UpBps: 1608.8, IP: "192.168.1.110", HasRates: true})
	got := a.statsText()
	for _, want := range []string{"CPU 12 %", "RAM 22,3 / 63,3 Go", "5,2 ko/s", "1,6 ko/s", "IP 192.168.1.110"} {
		if !strings.Contains(got, want) {
			t.Errorf("status bar %q lacks %q", got, want)
		}
	}
}

func TestStatusBarBeforeFirstRates(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	a.setStats(HostStats{MemUsed: 1 << 30, MemTotal: 2 << 30})
	if got := a.statsText(); !strings.Contains(got, "CPU --") || !strings.Contains(got, "IP --") {
		t.Fatalf("status bar = %q, want placeholders", got)
	}
}

func TestEmptySlotTabIsDisabled(t *testing.T) {
	a, _, _ := newTestApp(t, true) // slots 1 to 3 assigned, 2 active
	sized(a)
	before := a.ctl.Active()
	a.onSlotTab(6)
	waitWrites(a.ctl)
	if a.ctl.Active() != before {
		t.Fatalf("empty slot 6 got selected (active %d)", a.ctl.Active())
	}
	if !a.slotTabs.Disabled(5) || a.slotTabs.Disabled(0) {
		t.Fatal("tab 6 must be disabled and tab 1 enabled")
	}
}

// Regression (0.2.0): the stats were centred in the space between the gear
// and the notice, so they sat left of the window's centre.
func TestStatsAreCentredInTheWindow(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.setStats(sysinfoStats())
	d := fyne.CurrentApp().Driver()
	box := d.AbsolutePositionForObject(a.statsBox).X + a.statsBox.Size().Width/2
	win := a.Window().Canvas().Size().Width / 2
	if diff := box - win; diff < -2 || diff > 2 {
		t.Fatalf("stats centred at x=%v, window centre %v", box, win)
	}
}

func TestStatsCanBeHiddenFromSettings(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	g := generalOf(t, a)
	g.statsSeg.OnChange(1) // No
	if a.ShowStats() || a.statsBox.Visible() {
		t.Fatal("stats still shown")
	}
	if a.fa.Preferences().BoolWithFallback(statsPref, true) {
		t.Fatal("choice not stored")
	}
	g.statsSeg.OnChange(0)
	if !a.ShowStats() || !a.statsBox.Visible() {
		t.Fatal("stats not shown again")
	}
}
