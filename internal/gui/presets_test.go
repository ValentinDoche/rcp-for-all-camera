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

func TestPresetsPanelStartsWithEmptyPresetsDisabled(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	if pb == nil {
		t.Fatal("presets panel missing")
	}
	for i := 0; i < app.PresetCount; i++ {
		if enabled(pb.recall[i]) || enabled(pb.del[i]) {
			t.Errorf("preset %d is empty: recall and delete must be disabled", i+1)
		}
		if !enabled(pb.store[i]) {
			t.Errorf("store %d must always be enabled", i+1)
		}
	}
	texts := allTexts(a.details.obj)
	for _, want := range []string{"Presets", "Rappeler", "Enregistrer", "Effacer"} {
		found := false
		for _, s := range texts {
			if s == want {
				found = true
			}
		}
		if !found {
			t.Errorf("text %q missing on the details page: %v", want, texts)
		}
	}
	// the panel comes first, above the wheels
	if len(a.details.body.Objects) == 0 {
		t.Fatal("empty body")
	}
}

func TestStoreEnablesRecallAndRecallWritesValuesBack(t *testing.T) {
	a, svc, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	cam := (*cams)[0]

	test.Tap(pb.store[1]) // preset 2, empty: stored without asking
	waitWrites(a.ctl)
	if a.Window().Canvas().Overlays().Top() != nil {
		t.Fatal("storing in an empty preset must not ask for confirmation")
	}
	if got := svc.Rig().Slots[0].Presets; len(got) != 1 || got[0] != 2 {
		t.Fatalf("service presets = %v, want [2]", got)
	}
	if !enabled(pb.recall[1]) || !enabled(pb.del[1]) {
		t.Fatal("recall and delete of preset 2 must be enabled after the store")
	}
	if enabled(pb.recall[0]) {
		t.Error("preset 1 is still empty")
	}
	if !strings.Contains(a.noticeTxt.Text(), "preset 2 enregistré") {
		t.Errorf("notice = %q", a.noticeTxt.Text())
	}

	// change the camera, then recall: the stored value comes back
	stored := cam.values[core.Gain]
	if err := cam.Set(context.Background(), core.Gain, core.Number(stored.Number+3)); err != nil {
		t.Fatal(err)
	}
	before := len(cam.setCalls())
	test.Tap(pb.recall[1])
	waitWrites(a.ctl)
	calls := cam.setCalls()
	if len(calls) <= before {
		t.Fatal("recall did not write anything to the camera")
	}
	var wrote bool
	for _, c := range calls[before:] {
		if c.ID == core.Gain && c.V == stored {
			wrote = true
		}
	}
	if !wrote {
		t.Errorf("gain %v not written back: %+v", stored, calls[before:])
	}
	if v := a.ctl.Value(1, core.Gain); v == nil || *v != stored {
		t.Errorf("controller gain = %v, want %v", v, stored)
	}
	if !strings.Contains(a.noticeTxt.Text(), "preset 2 rappelé") {
		t.Errorf("notice = %q", a.noticeTxt.Text())
	}
}

func TestStoreOnExistingPresetAsksConfirmation(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	cam := (*cams)[0]
	test.Tap(pb.store[0])
	waitWrites(a.ctl)

	// the camera moves on; a declined replace keeps the old snapshot
	old := cam.values[core.Gain]
	_ = cam.Set(context.Background(), core.Gain, core.Number(old.Number+5))
	test.Tap(pb.store[0])
	ov := a.Window().Canvas().Overlays().Top()
	if ov == nil {
		t.Fatal("replacing a stored preset must ask for confirmation")
	}
	if !strings.Contains(strings.Join(allTexts(ov), "|"), "Remplacer le preset 1 ?") {
		t.Errorf("dialog texts = %v", allTexts(ov))
	}
	test.Tap(findButton(ov, "Annuler"))
	_ = cam.Set(context.Background(), core.Gain, core.Number(old.Number+9))
	test.Tap(pb.recall[0])
	waitWrites(a.ctl)
	if got := cam.values[core.Gain]; got != old {
		t.Errorf("declined replace must keep the old preset: gain %v, want %v", got, old)
	}

	// confirmed: the new snapshot wins
	moved := core.Number(old.Number + 5)
	_ = cam.Set(context.Background(), core.Gain, moved)
	test.Tap(pb.store[0])
	ov = a.Window().Canvas().Overlays().Top()
	if ov == nil {
		t.Fatal("confirmation dialog not shown")
	}
	test.Tap(findButton(ov, "Remplacer"))
	waitWrites(a.ctl)
	_ = cam.Set(context.Background(), core.Gain, old)
	test.Tap(pb.recall[0])
	waitWrites(a.ctl)
	if got := cam.values[core.Gain]; got != moved {
		t.Errorf("replaced preset must restore %v, got %v", moved, got)
	}
}

func TestDeletePresetAsksConfirmationAndDisablesRecall(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	pb := a.details.presets
	test.Tap(pb.store[2])
	waitWrites(a.ctl)
	if !enabled(pb.del[2]) {
		t.Fatal("delete must be enabled for a stored preset")
	}

	test.Tap(pb.del[2])
	ov := a.Window().Canvas().Overlays().Top()
	if ov == nil {
		t.Fatal("delete must ask for confirmation")
	}
	test.Tap(findButton(ov, "Annuler"))
	if len(svc.Rig().Slots[0].Presets) != 1 || !enabled(pb.recall[2]) {
		t.Fatal("a cancelled delete must keep the preset")
	}

	test.Tap(pb.del[2])
	ov = a.Window().Canvas().Overlays().Top()
	if ov == nil {
		t.Fatal("delete must ask for confirmation")
	}
	if !strings.Contains(strings.Join(allTexts(ov), "|"), "Effacer le preset 3 ?") {
		t.Errorf("dialog texts = %v", allTexts(ov))
	}
	test.Tap(findButton(ov, "Effacer"))
	waitWrites(a.ctl)
	if len(svc.Rig().Slots[0].Presets) != 0 {
		t.Errorf("presets = %v, want none", svc.Rig().Slots[0].Presets)
	}
	if enabled(pb.recall[2]) || enabled(pb.del[2]) {
		t.Error("recall and delete must be disabled again")
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
