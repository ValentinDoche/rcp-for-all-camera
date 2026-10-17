package gui_test

import (
	"context"
	"strconv"
	"testing"

	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// presetsCard is the Presets card of the details page: Store and 1 to 5.
type presetsCard struct {
	store *widgets.IconButton
	nums  []*widget.Button
}

func openPresets(t *testing.T, a *gui.App) presetsCard {
	t.Helper()
	card := win(a).around("Presets", func(s scope) bool { return s.shows("Enregistrer") && s.shows("5") })
	if card.roots == nil {
		t.Fatalf("no Presets card; shown: %s", win(a).joined())
	}
	var p presetsCard
	for _, b := range allOf[*widgets.IconButton](card) {
		if b.Label() == "Enregistrer" {
			p.store = b
		}
	}
	for _, b := range allOf[*widget.Button](card) {
		if b.Text == strconv.Itoa(len(p.nums)+1) {
			p.nums = append(p.nums, b)
		}
	}
	if p.store == nil || len(p.nums) != 5 {
		t.Fatalf("Presets card: store %v, %d numbers", p.store != nil, len(p.nums))
	}
	return p
}

// detailsOf opens the details page of slot from its strip.
func detailsOf(t *testing.T, a *gui.App, slot string) {
	t.Helper()
	sized(a)
	clickIn(t, a, strip(t, a, slot), "Détails")
	if !win(a).shows(slot + " : détails") {
		t.Fatalf("details of %s not shown: %s", slot, win(a).joined())
	}
}

func TestPresetsStartWithEmptyNumbersDisabled(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	p := openPresets(t, a)
	for i, b := range p.nums {
		if !b.Disabled() {
			t.Errorf("preset %d is empty: its button must be disabled", i+1)
		}
	}
	if p.store.Disabled() {
		t.Error("Store must be enabled")
	}
}

func TestStoreArmsThenTheNumberStores(t *testing.T) {
	a, svc, cams := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	p := openPresets(t, a)
	cam := (*cams)[0]

	press(a, p.store) // arm: every number can be pressed
	for i, b := range p.nums {
		if b.Disabled() {
			t.Fatalf("armed: number %d must be enabled", i+1)
		}
	}
	press(a, p.nums[1]) // store preset 2
	waitWrites(a.Controller())
	if got := svc.Rig().Slots[0].Presets; len(got) != 1 || got[0] != 2 {
		t.Fatalf("service presets = %v, want [2]", got)
	}
	if svc.StoreArmed() || p.store.Selected() {
		t.Error("storing disarms Store")
	}
	if p.nums[1].Disabled() || !p.nums[0].Disabled() {
		t.Error("only the stored preset can be recalled")
	}
	if !win(a).has("preset 2 enregistré") {
		t.Errorf("no notice: %s", win(a).joined())
	}

	// a plain press recalls: the stored value comes back
	stored := cam.values[core.Gain]
	_ = cam.Set(context.Background(), core.Gain, core.Number(stored.Number+3))
	before := len(cam.setCalls())
	press(a, p.nums[1])
	waitWrites(a.Controller())
	var wrote bool
	for _, c := range cam.setCalls()[before:] {
		wrote = wrote || (c.ID == core.Gain && c.V == stored)
	}
	if !wrote {
		t.Errorf("gain %v not written back", stored)
	}
	if !win(a).has("preset 2 rappelé") {
		t.Errorf("no notice: %s", win(a).joined())
	}
}

func TestArmedStoreReplacesWithoutDialogAndCanBeCancelled(t *testing.T) {
	a, svc, cams := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	p := openPresets(t, a)
	cam := (*cams)[0]
	press(a, p.store)
	press(a, p.nums[0])
	waitWrites(a.Controller())

	moved := core.Number(cam.values[core.Gain].Number + 5)
	_ = cam.Set(context.Background(), core.Gain, moved)
	press(a, p.store)
	press(a, p.nums[0]) // replace: Store was the confirmation
	waitWrites(a.Controller())
	if dialogOpen(a) {
		t.Fatal("an armed store must not ask again")
	}
	_ = cam.Set(context.Background(), core.Gain, core.Number(0))
	press(a, p.nums[0])
	waitWrites(a.Controller())
	if got := cam.values[core.Gain]; got != moved {
		t.Errorf("replaced preset must restore %v, got %v", moved, got)
	}

	press(a, p.store)
	press(a, p.store) // pressed again: disarmed
	if svc.StoreArmed() || p.store.Selected() || !p.nums[3].Disabled() {
		t.Error("a second press on Store cancels it")
	}
	if len(svc.Rig().Slots[0].Presets) != 1 {
		t.Errorf("presets = %v", svc.Rig().Slots[0].Presets)
	}
}

func TestPresetErrorsBecomeNotices(t *testing.T) {
	f := newFakeAPI()
	f.assign(1, "acme-cam", core.StateConnected, app.ControlView{Spec: rangeSpec(core.Gain, 0, 36, 1)})
	c := gui.NewController(f, syncOptions())
	c.Load()
	c.RecallPreset(1, 3) // the fake answers nil: success notice
	if got := c.Notice().Text; got != "Cam1 : preset 3 rappelé" && !contains(got, "preset 3 rappelé") {
		t.Errorf("notice = %q", got)
	}
	tx := c.Tx()
	if n := tx.NoticeFor(&app.Error{Code: app.CodeEmptyPreset}); n.Text != "Ce preset est vide" || n.Level == 0 {
		t.Errorf("EMPTY_PRESET notice = %+v", n)
	}
	if n := tx.NoticeFor(&app.Error{Code: app.CodeInvalidPreset}); n.Text == "" || n.Text == tx.T("error.internal") {
		t.Errorf("INVALID_PRESET notice = %+v", n)
	}
}

func TestStoreArmedFromTheServiceLightsTheButton(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	p := openPresets(t, a)
	if p.store.Selected() || !p.nums[0].Disabled() {
		t.Fatal("Store starts disarmed")
	}
	// The panel's STORE key arms the service; the event reaches the GUI.
	svc.SetStoreArmed(true)
	a.Emit(app.EventStoreArmed, app.StoreArmedState{Armed: true})
	if !p.store.Selected() || p.nums[0].Disabled() {
		t.Fatal("an armed service must light the Store button and enable the numbers")
	}
	// Pressing Store in the app disarms the service too.
	press(a, p.store)
	if svc.StoreArmed() || p.store.Selected() {
		t.Fatal("tapping Store while armed must disarm everywhere")
	}
	// And the other way round.
	press(a, p.store)
	if !svc.StoreArmed() || !p.store.Selected() {
		t.Fatal("tapping Store must arm the service")
	}
	svc.SetStoreArmed(false)
	a.Emit(app.EventStoreArmed, app.StoreArmedState{Armed: false})
	if p.store.Selected() {
		t.Fatal("disarming from the service must clear the button")
	}
}

// contains reports whether s holds sub.
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
