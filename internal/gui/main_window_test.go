package gui_test

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// dragTo drags o from its centre by (dx, dy), as a mouse would, and
// releases it.
func dragTo(a *gui.App, o interface {
	fyne.CanvasObject
	fyne.Draggable
}, dx, dy float32) {
	from := fyne.NewPos(o.Size().Width/2, o.Size().Height/2)
	abs := centre(o)
	d := fyne.Delta{DX: dx, DY: dy}
	o.Dragged(&fyne.DragEvent{
		PointEvent: fyne.PointEvent{Position: from.AddXY(dx, dy), AbsolutePosition: abs.AddXY(dx, dy)},
		Dragged:    d,
	})
	o.DragEnd()
}

// lever is the lever of a strip captioned caption (ZOOM, FOCUS).
func lever(t *testing.T, s scope, caption string) *widgets.Lever {
	t.Helper()
	return one[*widgets.Lever](t, s.around(caption, withA[*widgets.Lever]()))
}

// resetButton is the reset button of a colour wheel (an icon, no text).
func resetButton(t *testing.T, s scope) *widget.Button {
	t.Helper()
	for _, b := range allOf[*widget.Button](s) {
		if b.Icon != nil && b.Icon.Name() == theme.ViewRefreshIcon().Name() {
			return b
		}
	}
	t.Fatalf("no reset button: %s", s.joined())
	return nil
}

// Every strip has the same parts; what a camera does not declare is greyed.
func TestStripsLookTheSameAndGreyWhatIsMissing(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	var first [5]int
	for i, slot := range []string{"Cam1", "Cam2", "Cam3"} {
		s := strip(t, a, slot)
		n := [5]int{len(allOf[*widgets.ColorWheel](s)), len(allOf[*widgets.IrisPad](s)),
			len(allOf[*widgets.LevelMeter](s)), len(allOf[*widgets.Lever](s)), len(allOf[*widgets.Slider](s))}
		if i == 0 {
			first = n
		}
		if n[0] != 1 || n[1] != 1 || n[2] != 1 || n[3] != 1 || n[4] == 0 || n != first {
			t.Errorf("%s has %v wheels, pads, meters, levers and sliders; Cam1 has %v", slot, n, first)
		}
		for _, part := range []string{"Lift", "Gamma", "Gain", "ZOOM", "FOCUS", "IRIS", "ND", "GAIN", "OBTURATEUR", "BALANCE"} {
			if !s.shows(part) {
				t.Errorf("%s misses %q", slot, part)
			}
		}
		for _, label := range []string{"Auto focus", "Auto iris", "AWB", "Détails"} {
			if !s.shows(label) {
				t.Errorf("%s misses the %q button", slot, label)
			}
		}
	}
	if !one[*widgets.ColorWheel](t, strip(t, a, "Cam2")).Disabled() {
		t.Error("FX6 declares no color wheels: its wheel must be greyed")
	}
	if one[*widgets.ColorWheel](t, strip(t, a, "Cam1")).Disabled() || one[*widgets.ColorWheel](t, strip(t, a, "Cam3")).Disabled() {
		t.Error("Studio 6K Pro and UE150 have a Lift wheel")
	}
	if lever(t, strip(t, a, "Cam1"), "ZOOM").Disabled() {
		t.Error("Studio 6K Pro with a power zoom: the zoom lever works")
	}
}

func TestOfflineStripShowsDashesAndNoConnectButton(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	s := strip(t, a, "Cam3") // 192.168.1.44 is offline
	for _, cell := range []string{"ND", "GAIN", "OBTURATEUR", "BALANCE"} {
		if s.around(cell, withText("--")).roots == nil {
			t.Errorf("unknown %s must show --", cell)
		}
	}
	// cameras connect by themselves: no manual connect button anywhere
	if win(a).shows("Connecter") || win(a).shows("Reconnecter") {
		t.Error("strips must not offer a connect button")
	}
}

func TestEmptyStripInvitesToAssign(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	clickIn(t, a, strip(t, a, "Cam4"), "Assigner une caméra")
	if !settingsOpen(a) || !form(t, a).shows("Cam4 : assigner une caméra") {
		t.Errorf("the form of slot 4 must open: %s", settings(a).joined())
	}
}

func TestHeaderStates(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	for slot, want := range map[string]string{"Cam2": "Connecté", "Cam3": "Hors ligne", "Cam4": "Libre"} {
		if !strip(t, a, slot).shows(want) {
			t.Errorf("%s: %s", slot, strip(t, a, slot).joined())
		}
	}
	// tally on: on-air label (slot 2, the Sony; the Studio 6K Pro API has no tally)
	a.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Tally, Value: core.Bool(true)}})
	if !strip(t, a, "Cam2").shows("À l'antenne") {
		t.Errorf("tally: %s", strip(t, a, "Cam2").joined())
	}
}

func TestSlotTabSelectsAndFollowsEvents(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	if got := slotTabs(t, a).Selected(); got != 1 {
		t.Fatalf("seeded active slot is 2, tab index %d", got)
	}
	clickTab(t, a, "Cam1")
	if svc.Rig().Active != 1 || slotTabs(t, a).Selected() != 0 {
		t.Errorf("tab click: service %d, tab %d", svc.Rig().Active, slotTabs(t, a).Selected())
	}
	// the panel (or anything else) changes the active slot: tabs follow
	if err := svc.SelectSlot(3); err != nil {
		t.Fatal(err)
	}
	a.Emit(app.EventRigChanged, svc.Rig())
	if got := slotTabs(t, a).Selected(); got != 2 {
		t.Errorf("tabs = %d, want 2", got)
	}
}

func TestPanelIndicator(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	for _, c := range []struct {
		st   app.PanelStatus
		want string
	}{
		{app.PanelStatus{}, "Micro Camera Panel non détecté"},
		{app.PanelStatus{Connected: true}, "Micro Camera Panel connecté (non piloté)"},
		{app.PanelStatus{Connected: true, Active: true}, "Micro Camera Panel actif"},
		{app.PanelStatus{Detail: app.DetailNoUSBProbe}, "Micro Camera Panel : " + app.DetailNoUSBProbe},
	} {
		a.Emit(app.EventPanelStatus, c.st)
		if !win(a).shows(c.want) {
			t.Errorf("%+v: want %q", c.st, c.want)
		}
	}
}

func TestWheelDragWritesRGBThroughTheService(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	dragTo(a, one[*widgets.ColorWheel](t, strip(t, a, "Cam1")), 60, 0)
	waitWrites(a.Controller())
	calls := (*cams)[0].setCalls()
	got := map[core.ControlID]float64{}
	for _, c := range calls {
		got[c.ID] = c.V.Number
	}
	for _, id := range []core.ControlID{core.LiftR, core.LiftG, core.LiftB} {
		if _, ok := got[id]; !ok {
			t.Fatalf("%s not written: %+v", id, calls)
		}
	}
	if !(got[core.LiftR] > got[core.LiftG] && got[core.LiftR] > got[core.LiftB]) {
		t.Errorf("a drag toward red must raise R above G and B: %+v", got)
	}
	if _, wrote := got[core.LiftY]; wrote {
		t.Error("dragging the puck must not touch the master value")
	}
}

func TestWheelSelectorSwitchesGroup(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	s := strip(t, a, "Cam1")
	clickIn(t, a, scope{[]fyne.CanvasObject{one[*widgets.Segmented](t, s)}}, "Gain")
	if !s.shows("1,15") {
		t.Fatalf("the wheel must show the Gain values: %s", s.joined())
	}
	press(a, resetButton(t, s))
	waitWrites(a.Controller())
	got := map[core.ControlID]float64{}
	for _, call := range (*cams)[0].setCalls() {
		got[call.ID] = call.V.Number
	}
	for _, id := range []core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY} {
		if got[id] != 1 {
			t.Errorf("reset must put %s back to 1, got %v", id, got[id])
		}
	}
}

func TestPadDragWritesIrisAndMasterBlack(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	pad := one[*widgets.IrisPad](t, strip(t, a, "Cam1"))
	dragTo(a, pad, 0, -pad.Size().Height/2) // top centre
	waitWrites(a.Controller())
	got := map[core.ControlID]core.Value{}
	for _, c := range (*cams)[0].setCalls() {
		got[c.ID] = c.V
	}
	if iris, ok := got[core.Iris]; !ok || iris.Number != 2.8 {
		t.Errorf("top of the pad is fully open (the lens' widest f-stop, f/2.8): %+v", got)
	}
	if !strip(t, a, "Cam1").shows("f/2.8") {
		t.Errorf("the strip must show f/2.8: %s", strip(t, a, "Cam1").joined())
	}
}

func TestNotImplementedDriverShowsNoticeAndRestoresValue(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	(*cams)[0].setErr = fmt.Errorf("set: %w", core.ErrNotImplemented)
	pad := one[*widgets.IrisPad](t, strip(t, a, "Cam1"))
	_, before := pad.Pos()
	dragTo(a, pad, 0, -pad.Size().Height/2)
	waitWrites(a.Controller())
	if !win(a).shows("Contrôle pas encore disponible : pilote caméra en développement") {
		t.Errorf("no notice: %s", win(a).joined())
	}
	if !strip(t, a, "Cam1").shows("f/4.0") {
		t.Error("the strip must still show f/4.0")
	}
	if _, y := pad.Pos(); y != before {
		t.Errorf("pad must return to the last known position: y=%v want %v", y, before)
	}
}

// cardControls are the controls the details page shows on its cards and
// wheels, with a title of their own instead of the control's label.
var cardControls = map[core.ControlID]bool{
	core.Iris: true, core.IrisNorm: true, core.AutoIris: true, core.Gain: true, core.ISO: true,
	core.ShutterAngle: true, core.ShutterSpeed: true, core.ND: true, core.WhiteBalance: true,
	core.Tint: true, core.AutoWhiteBalance: true, core.Focus: true, core.AutoFocus: true, core.Zoom: true,
	core.Contrast: true, core.Saturation: true, core.Hue: true, core.Record: true, core.MasterBlack: true,
	core.LiftR: true, core.LiftG: true, core.LiftB: true, core.LiftY: true,
	core.GammaR: true, core.GammaG: true, core.GammaB: true, core.GammaY: true,
	core.GainR: true, core.GainG: true, core.GainB: true, core.GainY: true,
}

func TestDetailsShowsEveryDeclaredControl(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	d := details(t, a)
	tx := a.Controller().Tx()
	for _, sp := range a.Controller().Specs(1) {
		if !cardControls[sp.ID] && !d.shows(tx.ControlLabel(sp)) {
			t.Errorf("control %s (%q) has no widget on the details page", sp.ID, tx.ControlLabel(sp))
		}
	}
	// section titles are French and in the fixed order
	want := []string{"Objectif", "Exposition", "Balance des blancs", "Correction colorimétrique", "Image", "Enregistrement"}
	pos := -1
	for _, title := range d.texts() {
		for i, w := range want {
			if w == title {
				if i < pos {
					t.Errorf("section %q is out of order", title)
				}
				pos = i
			}
		}
	}
}

func TestDetailsRowsWriteThroughTheService(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	e := entry(t, details(t, a), "Offset R")
	typeInto(a, e, "1,5", true)
	waitWrites(a.Controller())
	last := (*cams)[0].setCalls()
	if len(last) == 0 || last[len(last)-1].ID != "blackmagic.offsetR" || last[len(last)-1].V.Number != 1.5 {
		t.Errorf("calls = %+v", last)
	}
	// out of range is clamped, not rejected
	typeInto(a, e, "999", true)
	waitWrites(a.Controller())
	last = (*cams)[0].setCalls()
	if last[len(last)-1].V.Number != 8 {
		t.Errorf("999 must clamp to 8, got %v", last[len(last)-1].V.Number)
	}
	// garbage: notice, no write
	n := len(last)
	typeInto(a, e, "abc", true)
	waitWrites(a.Controller())
	if len((*cams)[0].setCalls()) != n {
		t.Error("invalid text must not be written")
	}
	if !win(a).has("Valeur numérique attendue") {
		t.Errorf("no notice: %s", win(a).joined())
	}
}

func TestDetailsFollowsActiveSlotAndLeavesWhenEmpty(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	if err := svc.SelectSlot(2); err != nil {
		t.Fatal(err)
	}
	a.Emit(app.EventRigChanged, svc.Rig())
	if !win(a).shows("Cam2 : détails") {
		t.Errorf("the details must follow the active slot: %s", win(a).joined())
	}
	if err := svc.ClearSlot(2); err != nil {
		t.Fatal(err)
	}
	a.Emit(app.EventRigChanged, svc.Rig())
	if detailsScroll(a) != nil || !strip(t, a, "Cam2").shows("Libre") {
		t.Error("the Cameras page must come back after the camera is removed")
	}
}

// expectError saves the form and checks the error it shows.
func expectError(t *testing.T, a *gui.App, want string) {
	t.Helper()
	saveForm(t, a)
	if !form(t, a).shows(want) {
		t.Errorf("want error %q; the form shows %s", want, form(t, a).joined())
	}
}

func TestConfigAssignFlow(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	f := cameraForm(t, a, "Cam5")
	expectError(t, a, "Choisissez une marque et un modèle.")
	chooseModel(t, a, "Sony", "FX6")
	f = form(t, a)
	if p := entry(t, f, "Port"); p.PlaceHolder != "80" {
		t.Errorf("port placeholder = %q (model default)", p.PlaceHolder)
	}
	if !f.shows("Nom d'utilisateur (obligatoire)") || !f.shows("Mot de passe") {
		t.Error("FX6 needs credentials")
	}
	if entry(t, f, "Mot de passe").Text != "" {
		t.Error("password field must never be prefilled")
	}
	expectError(t, a, "L'adresse de la caméra est obligatoire.")
	typeInto(a, entry(t, f, "Adresse de la caméra"), "bad host!", false)
	typeInto(a, entry(t, f, "Nom d'utilisateur (obligatoire)"), "admin", false)
	expectError(t, a, "L'hôte doit être une adresse IP ou un nom d'hôte valide.")
	typeInto(a, entry(t, f, "Adresse de la caméra"), "192.168.1.50", false)
	typeInto(a, entry(t, f, "Nom d'utilisateur (obligatoire)"), "", false)
	expectError(t, a, "Le nom d'utilisateur est obligatoire pour ce modèle.")
	typeInto(a, entry(t, f, "Nom d'utilisateur (obligatoire)"), "admin", false)
	typeInto(a, entry(t, f, "Mot de passe"), "hunter2", false)
	typeInto(a, entry(t, f, "Port"), "x", false)
	expectError(t, a, "Le port doit être un nombre.")
	typeInto(a, entry(t, f, "Port"), "", false)
	saveForm(t, a)
	if formOpen(a) {
		t.Fatalf("the form must close after saving: %s", settings(a).joined())
	}
	cam := svc.Rig().Slots[4].Camera
	if cam == nil || cam.Model != "sony-fx6" || cam.Host != "192.168.1.50" || !cam.HasPassword || cam.Port != 80 {
		t.Fatalf("slot 5 = %+v", cam)
	}

	// editing again: password is not shown, kept when left empty
	f = cameraForm(t, a, "Cam5")
	if entry(t, f, "Mot de passe").Text != "" {
		t.Error("stored password must not come back into the field")
	}
	if !f.has("Mot de passe enregistré") {
		t.Errorf("no stored password hint: %s", f.joined())
	}
	typeInto(a, entry(t, f, "Nom affiché"), "Plateau 2", false)
	saveForm(t, a)
	if cam = svc.Rig().Slots[4].Camera; !cam.HasPassword || cam.Name != "Plateau 2" {
		t.Errorf("password must be kept: %+v", cam)
	}
	// explicit clear
	f = cameraForm(t, a, "Cam5")
	clickIn(t, a, f, "Effacer le mot de passe enregistré")
	saveForm(t, a)
	if svc.Rig().Slots[4].Camera.HasPassword {
		t.Error("explicit clear must remove the password")
	}
}

func TestConfigHidesCredentialsForOpenModels(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	cameraForm(t, a, "Cam1")
	chooseModel(t, a, "Blackmagic Design", "Studio Camera 6K Pro")
	if f := form(t, a); f.has("Nom d'utilisateur") || f.shows("Mot de passe") {
		t.Error("Blackmagic needs no credentials")
	}
	chooseModel(t, a, "Panasonic", "AW-UE150")
	f := form(t, a)
	if !f.shows("Nom d'utilisateur (facultatif)") {
		t.Errorf("Panasonic credentials are optional: %s", f.joined())
	}
	if !f.has("Prévu") {
		t.Errorf("the support level must show: %s", f.joined())
	}
}

func TestConfigRemoveAsksForConfirmation(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	f := cameraForm(t, a, "Cam3")
	clickIn(t, a, f, "Retirer la caméra")
	if svc.Rig().Slots[2].Camera == nil {
		t.Fatal("removal must wait for the confirmation")
	}
	if !dialogOpen(a) || !dialog(a).shows("Annuler") {
		t.Fatalf("a confirmation dialog with a cancel button is expected: %s", dialog(a).joined())
	}
	clickIn(t, a, dialog(a), "Retirer")
	if svc.Rig().Slots[2].Camera != nil {
		t.Error("camera not removed after confirmation")
	}
}

func TestThemeSettingIsStored(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	category(t, a, "Général")
	choose(t, a, "Thème", "Sombre")
	if th, ok := fyne.CurrentApp().Settings().Theme().(style.Theme); !ok || th.Mode != style.ModeDark || a.Mode() != style.ModeDark {
		t.Errorf("theme = %#v", fyne.CurrentApp().Settings().Theme())
	}
	choose(t, a, "Thème", "Clair")
	if a.Mode() != style.ModeLight {
		t.Errorf("mode=%v", a.Mode())
	}
	choose(t, a, "Thème", "Sombre")
	if m := gui.New(fyne.CurrentApp(), svc, syncOptions()).Mode(); m != style.ModeDark {
		t.Errorf("the next start uses %v", m)
	}
}

func TestStoredThemeIsRestored(t *testing.T) {
	fa := test.NewTempApp(t)
	fa.Preferences().SetString("theme", "light")
	svc, _, _ := newLiveService(t)
	a := gui.New(fa, svc, syncOptions())
	if a.Mode() != style.ModeLight {
		t.Errorf("mode = %v", a.Mode())
	}
}

func TestNoDashesInVisibleText(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	check := func(page string) {
		for _, s := range win(a).texts() {
			if strings.ContainsAny(s, "—–") {
				t.Errorf("%s: dash in %q", page, s)
			}
		}
	}
	check("cameras")
	detailsOf(t, a, "Cam1")
	check("details")
	cameraForm(t, a, "Cam2")
	check("settings")
}

func TestNoPasswordInVisibleText(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	for _, slot := range []string{"Cam1", "Cam2", "Cam3"} {
		cameraForm(t, a, slot)
		if win(a).has("secret") {
			t.Errorf("%s: the password shows", slot)
		}
	}
}
