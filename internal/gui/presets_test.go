package gui

import (
	"context"
	"strings"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func enabled(b interface{ Disabled() bool }) bool { return !b.Disabled() }

func TestPresetsStartWithEmptyNumbersDisabled(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	if pb == nil {
		t.Fatal("presets card missing")
	}
	for i, btn := range pb.nums {
		if enabled(btn) {
			t.Errorf("preset %d is empty: its button must be disabled", i+1)
		}
	}
	if pb.store.Disabled() || pb.store.Label() != "Enregistrer" {
		t.Errorf("store button: disabled %v, label %q", pb.store.Disabled(), pb.store.Label())
	}
	if !strings.Contains(allTextsJoined(a.details.obj), "Presets") {
		t.Error("presets title missing")
	}
}

func TestStoreArmsThenTheNumberStores(t *testing.T) {
	a, svc, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	cam := (*cams)[0]

	test.Tap(pb.store) // arm: every number can be pressed
	for i, btn := range pb.nums {
		if !enabled(btn) {
			t.Fatalf("armed: number %d must be enabled", i+1)
		}
	}
	test.Tap(pb.nums[1]) // store preset 2
	waitWrites(a.ctl)
	if got := svc.Rig().Slots[0].Presets; len(got) != 1 || got[0] != 2 {
		t.Fatalf("service presets = %v, want [2]", got)
	}
	if pb.armed {
		t.Error("storing disarms Store")
	}
	if !enabled(pb.nums[1]) || enabled(pb.nums[0]) {
		t.Error("only the stored preset can be recalled")
	}
	if !strings.Contains(a.noticeTxt.Text(), "preset 2 enregistré") {
		t.Errorf("notice = %q", a.noticeTxt.Text())
	}

	// a plain press recalls: the stored value comes back
	stored := cam.values[core.Gain]
	_ = cam.Set(context.Background(), core.Gain, core.Number(stored.Number+3))
	before := len(cam.setCalls())
	test.Tap(pb.nums[1])
	waitWrites(a.ctl)
	var wrote bool
	for _, c := range cam.setCalls()[before:] {
		wrote = wrote || (c.ID == core.Gain && c.V == stored)
	}
	if !wrote {
		t.Errorf("gain %v not written back", stored)
	}
	if !strings.Contains(a.noticeTxt.Text(), "preset 2 rappelé") {
		t.Errorf("notice = %q", a.noticeTxt.Text())
	}
}

func TestArmedStoreReplacesWithoutDialogAndCanBeCancelled(t *testing.T) {
	a, svc, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	cam := (*cams)[0]
	test.Tap(pb.store)
	test.Tap(pb.nums[0])
	waitWrites(a.ctl)

	moved := core.Number(cam.values[core.Gain].Number + 5)
	_ = cam.Set(context.Background(), core.Gain, moved)
	test.Tap(pb.store)
	test.Tap(pb.nums[0]) // replace: Store was the confirmation
	waitWrites(a.ctl)
	if a.Window().Canvas().Overlays().Top() != nil {
		t.Fatal("an armed store must not ask again")
	}
	_ = cam.Set(context.Background(), core.Gain, core.Number(0))
	test.Tap(pb.nums[0])
	waitWrites(a.ctl)
	if got := cam.values[core.Gain]; got != moved {
		t.Errorf("replaced preset must restore %v, got %v", moved, got)
	}

	test.Tap(pb.store)
	test.Tap(pb.store) // pressed again: disarmed
	if pb.armed || enabled(pb.nums[3]) {
		t.Error("a second press on Store cancels it")
	}
	if len(svc.Rig().Slots[0].Presets) != 1 {
		t.Errorf("presets = %v", svc.Rig().Slots[0].Presets)
	}
}

func TestPresetErrorsBecomeNotices(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme.cam", core.StateConnected, app.ControlView{Spec: rangeSpec(core.Gain, 0, 36, 1)})
	c := NewController(f, syncOptions())
	c.Load()
	c.RecallPreset(1, 3) // the fake answers nil: success notice
	if !strings.Contains(c.Notice().Text, "preset 3 rappelé") {
		t.Errorf("notice = %q", c.Notice().Text)
	}
	tx := c.Tx()
	if n := tx.NoticeFor(&app.Error{Code: app.CodeEmptyPreset}); n.Text != "Ce preset est vide" || n.Level == 0 {
		t.Errorf("EMPTY_PRESET notice = %+v", n)
	}
	if n := tx.NoticeFor(&app.Error{Code: app.CodeInvalidPreset}); n.Text == "" || n.Text == tx.T("error.internal") {
		t.Errorf("INVALID_PRESET notice = %+v", n)
	}
}
