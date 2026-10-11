package gui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
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
	a.setStats(sysinfo.Stats{CPU: 12.4, MemUsed: 23915626496, MemTotal: 67939266560,
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
	a.setStats(sysinfo.Stats{MemUsed: 1 << 30, MemTotal: 2 << 30})
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
