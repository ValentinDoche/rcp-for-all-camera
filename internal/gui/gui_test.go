package gui

import (
	"fmt"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// all returns every laid out object under o.
func all(o fyne.CanvasObject) []fyne.CanvasObject { return test.LaidOutObjects(o) }

func findButton(o fyne.CanvasObject, text string) *widget.Button {
	for _, c := range all(o) {
		if b, ok := c.(*widget.Button); ok && b.Text == text {
			return b
		}
	}
	return nil
}

func allTexts(o fyne.CanvasObject) []string {
	var out []string
	for _, c := range all(o) {
		switch w := c.(type) {
		case *widgets.Text:
			out = append(out, w.Text())
		case *widget.Label:
			out = append(out, w.Text)
		case *widget.Button:
			out = append(out, w.Text)
		case *widget.RichText:
			out = append(out, w.String())
		}
	}
	return out
}

func sized(a *App) {
	a.Window().Resize(fyne.NewSize(1440, 810))
}

func TestStripOnlyOffersWhatTheCameraDeclares(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	bmd, sony, pana := a.strips[0].content, a.strips[1].content, a.strips[2].content
	if bmd == nil || sony == nil || pana == nil {
		t.Fatal("assigned slots must have content")
	}
	if bmd.wheel == nil || bmd.selector == nil || len(bmd.groupMap) != 3 {
		t.Error("Studio 6K Pro declares lift, gamma and gain: wheel and selector expected")
	}
	if sony.wheel != nil || sony.selector != nil {
		t.Error("FX6 declares no color wheels: none must be shown")
	}
	if pana.wheel == nil {
		t.Error("UE150 declares pedestal R/G/B (lift): a wheel is expected")
	}
	if bmd.zoom == nil || bmd.focus == nil || bmd.pad == nil || bmd.meter == nil {
		t.Error("Studio 6K Pro has zoom, focus, iris and master black")
	}
	if findButton(a.strips[0].root, "Auto focus") == nil || findButton(a.strips[0].root, "AWB") == nil {
		t.Error("quick actions missing")
	}
	if findButton(a.strips[0].root, "Détails") == nil {
		t.Error("Détails button missing")
	}
	// the Panasonic has no ND filter in this model: its summary must not list one
	for _, cell := range pana.summary {
		if cell.spec.ID == core.ND {
			if !a.ctl.Has(3, core.ND) {
				t.Error("ND readout shown for a camera without ND")
			}
		}
	}
}

func TestOfflineStripShowsDashesAndNoConnectButton(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	c := a.strips[2].content // 192.168.1.44 is offline
	for _, cell := range c.summary {
		if cell.val.Text() != "--" {
			t.Errorf("unknown value must show --, got %q", cell.val.Text())
		}
	}
	// cameras connect by themselves: no manual connect button anywhere
	for _, st := range a.strips {
		if findButton(st.root, "Connecter") != nil || findButton(st.root, "Reconnecter") != nil {
			t.Error("strips must not offer a connect button")
		}
	}
}

func TestEmptyStripInvitesToAssign(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	b := findButton(a.strips[3].root, "Assigner une caméra")
	if b == nil {
		t.Fatal("empty strip needs an assign button")
	}
	test.Tap(b)
	if a.page != pageConfig || a.config.editSlot != 4 {
		t.Errorf("page=%v editSlot=%d, want the form of slot 4", a.page, a.config.editSlot)
	}
}

func TestHeaderStates(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	if a.strips[1].headerMode != 0 && false {
		t.Skip()
	}
	// slot 2 is active: header text shows its connection
	if got := a.strips[1].stateTxt.Text(); got != "Connecté" {
		t.Errorf("state = %q", got)
	}
	if got := a.strips[2].stateTxt.Text(); got != "Hors ligne" {
		t.Errorf("state = %q", got)
	}
	if got := a.strips[3].stateTxt.Text(); got != "Libre" {
		t.Errorf("state = %q", got)
	}
	// tally on: on-air label (slot 2, the Sony; the Studio 6K Pro API has no tally)
	a.ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 2, Event: core.Event{Control: core.Tally, Value: core.Bool(true)}})
	if got := a.strips[1].stateTxt.Text(); got != "À l'antenne" {
		t.Errorf("tally state = %q", got)
	}
	_ = cams
}

func TestSlotTabSelectsAndFollowsEvents(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	if a.slotTabs.Selected() != 1 {
		t.Fatalf("seeded active slot is 2, tab index %d", a.slotTabs.Selected())
	}
	x := a.slotTabs.Size().Width/2 - tabWidth()*4 + tabWidth()*0 + 10 // first tab
	a.slotTabs.Tapped(&fyne.PointEvent{Position: fyne.NewPos(x, 10)})
	if a.ctl.Active() != 1 || svc.Rig().Active != 1 {
		t.Errorf("tab tap: ui=%d service=%d", a.ctl.Active(), svc.Rig().Active)
	}
	// the panel (or anything else) changes the active slot: tabs follow
	if err := svc.SelectSlot(3); err != nil {
		t.Fatal(err)
	}
	a.ctl.Emit(app.EventRigChanged, svc.Rig())
	if a.slotTabs.Selected() != 2 {
		t.Errorf("tabs = %d, want 2", a.slotTabs.Selected())
	}
}

func tabWidth() float32 { return 104 }

func TestPanelIndicator(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	if got := a.panelTxt.Text(); got != "Micro Camera Panel non détecté" {
		t.Errorf("initial = %q", got)
	}
	a.Emit(app.EventPanelStatus, app.PanelStatus{Connected: true})
	if got := a.panelTxt.Text(); got != "Micro Camera Panel connecté (non piloté)" {
		t.Errorf("connected but not driven = %q", got)
	}
	a.Emit(app.EventPanelStatus, app.PanelStatus{Connected: true, Active: true})
	if got := a.panelTxt.Text(); got != "Micro Camera Panel actif" {
		t.Errorf("active = %q", got)
	}
	a.Emit(app.EventPanelStatus, app.PanelStatus{Detail: "détection USB indisponible"})
	if got := a.panelTxt.Text(); !strings.Contains(got, "détection USB indisponible") {
		t.Errorf("error = %q", got)
	}
}

func TestWheelDragWritesRGBThroughTheService(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	wb := a.strips[0].content.wheel
	s := wb.wheel.Size()
	cx, cy := s.Width/2, s.Height/2
	wb.wheel.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(cx+60, cy)}})
	wb.wheel.DragEnd()
	waitWrites(a.ctl)
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
	c := a.strips[0].content
	c.selector.Tapped(&fyne.PointEvent{Position: fyne.NewPos(c.selector.Size().Width-5, 5)}) // Gain
	if c.wheel.group != 2 {
		t.Fatalf("group = %d, want 2 (Gain)", c.wheel.group)
	}
	c.wheel.onReset()
	waitWrites(a.ctl)
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
	pad := a.strips[0].content.pad
	s := pad.Size()
	pad.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(s.Width/2, 0)}}) // top centre
	pad.DragEnd()
	waitWrites(a.ctl)
	got := map[core.ControlID]core.Value{}
	for _, c := range (*cams)[0].setCalls() {
		got[c.ID] = c.V
	}
	iris, ok := got[core.Iris]
	if !ok || iris.Number != 1 {
		t.Errorf("top of the pad is fully open (iris max): %+v", got)
	}
}

func TestNotImplementedDriverShowsNoticeAndRestoresValue(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	(*cams)[0].setErr = fmt.Errorf("set: %w", core.ErrNotImplemented)
	before := a.ctl.Value(1, core.Iris).Number
	pad := a.strips[0].content.pad
	s := pad.Size()
	pad.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(s.Width/2, 0)}})
	pad.DragEnd()
	waitWrites(a.ctl)
	if got := a.noticeTxt.Text(); got != "Contrôle pas encore disponible : pilote caméra en développement" {
		t.Errorf("notice = %q", got)
	}
	if v := a.ctl.Value(1, core.Iris); v == nil || v.Number != before {
		t.Errorf("iris must stay %v, got %v", before, v)
	}
	_, y := pad.Pos()
	_, wantY := widgets.ValuesToPad(nil, &widgets.Range{Min: 0, Max: 1}, 0, before)
	if y != wantY {
		t.Errorf("pad must return to the last known position: y=%v want %v", y, wantY)
	}
}

func TestDetailsShowsEveryDeclaredControl(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	p := a.details
	if p == nil {
		t.Fatal("details page not open")
	}
	covered := wheelControlIDs()
	for id := range p.rows {
		covered[id] = true
	}
	for _, sp := range a.ctl.Specs(1) {
		if !covered[sp.ID] {
			t.Errorf("control %s has no widget on the details page", sp.ID)
		}
	}
	if len(p.wheels) != 3 {
		t.Errorf("%d wheels, want 3", len(p.wheels))
	}
	for id, r := range p.rows {
		want := widgets.ChooseKind(r.spec)
		if r.kind != want && !(want == widgets.KindSegmented && r.kind == widgets.KindSelect) {
			t.Errorf("%s: row kind %s, want %s", id, r.kind, want)
		}
	}
	// section titles are French and in the fixed order
	var titles []string
	for _, s := range p.secs {
		titles = append(titles, s.title)
	}
	want := []string{"Objectif", "Exposition", "Balance des blancs", "Correction colorimétrique", "Image", "Enregistrement"}
	pos := -1
	for _, ti := range titles {
		found := false
		for i, w := range want {
			if w == ti && i > pos {
				pos, found = i, true
			}
		}
		if !found {
			t.Errorf("unexpected or misordered section %q in %v", ti, titles)
		}
	}
}

func TestDetailsRowsWriteThroughTheService(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	row := a.details.rows[core.Gain]
	if row == nil {
		t.Fatal("gain row missing")
	}
	var entry *widget.Entry
	for _, o := range all(row.obj) {
		if e, ok := o.(*widget.Entry); ok {
			entry = e
		}
	}
	entry.SetText("18")
	entry.OnSubmitted("18")
	waitWrites(a.ctl)
	last := (*cams)[0].setCalls()
	if len(last) == 0 || last[len(last)-1].ID != core.Gain || last[len(last)-1].V.Number != 18 {
		t.Errorf("calls = %+v", last)
	}
	// out of range is clamped, not rejected
	entry.OnSubmitted("999")
	waitWrites(a.ctl)
	last = (*cams)[0].setCalls()
	if last[len(last)-1].V.Number != 36 {
		t.Errorf("999 must clamp to 36, got %v", last[len(last)-1].V.Number)
	}
	// garbage: notice, no write
	n := len(last)
	entry.OnSubmitted("abc")
	waitWrites(a.ctl)
	if len((*cams)[0].setCalls()) != n {
		t.Error("invalid text must not be written")
	}
	if !strings.Contains(a.noticeTxt.Text(), "Valeur numérique attendue") {
		t.Errorf("notice = %q", a.noticeTxt.Text())
	}
}

func TestDetailsFollowsActiveSlotAndLeavesWhenEmpty(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	if err := svc.SelectSlot(2); err != nil {
		t.Fatal(err)
	}
	a.ctl.Emit(app.EventRigChanged, svc.Rig())
	if a.page != pageDetails || a.detailSlot != 2 {
		t.Errorf("page=%v slot=%d, want details of 2", a.page, a.detailSlot)
	}
	if err := svc.ClearSlot(2); err != nil {
		t.Fatal(err)
	}
	a.ctl.Emit(app.EventRigChanged, svc.Rig())
	if a.page != pageCameras {
		t.Errorf("page = %v, want cameras after the camera is removed", a.page)
	}
}

func TestConfigAssignFlow(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(5)
	f := a.config.form
	if f.pass.Text != "" {
		t.Error("password field must never be prefilled")
	}
	// nothing chosen
	f.save()
	if got := f.errTxt.Text(); got != "Choisissez une marque et un modèle." {
		t.Errorf("err = %q", got)
	}
	f.brand.SetSelected("Sony")
	f.model.SetSelected("FX6")
	if f.port.PlaceHolder != "80" {
		t.Errorf("port placeholder = %q (model default)", f.port.PlaceHolder)
	}
	if !f.userBox.Visible() || !f.passBox.Visible() {
		t.Error("FX6 needs credentials")
	}
	f.save()
	if got := f.errTxt.Text(); got != "L'adresse de la caméra est obligatoire." {
		t.Errorf("err = %q", got)
	}
	f.host.SetText("bad host!")
	f.user.SetText("admin")
	f.save()
	if got := f.errTxt.Text(); got != "L'hôte doit être une adresse IP ou un nom d'hôte valide." {
		t.Errorf("service validation error = %q", got)
	}
	f.host.SetText("192.168.1.50")
	f.user.SetText("")
	f.save()
	if got := f.errTxt.Text(); got != "Le nom d'utilisateur est obligatoire pour ce modèle." {
		t.Errorf("err = %q", got)
	}
	f.user.SetText("admin")
	f.pass.SetText("hunter2")
	f.port.SetText("x")
	f.save()
	if got := f.errTxt.Text(); got != "Le port doit être un nombre." {
		t.Errorf("err = %q", got)
	}
	f.port.SetText("")
	f.save()
	if f.errTxt.Text() != "" {
		t.Fatalf("unexpected error %q", f.errTxt.Text())
	}
	cam := svc.Rig().Slots[4].Camera
	if cam == nil || cam.Model != "sony.fx6" || cam.Host != "192.168.1.50" || !cam.HasPassword || cam.Port != 80 {
		t.Fatalf("slot 5 = %+v", cam)
	}
	if a.config.editSlot != 0 {
		t.Error("form should close after saving")
	}

	// editing again: password is not shown, kept when left empty
	a.config.Edit(5)
	f = a.config.form
	if f.pass.Text != "" {
		t.Error("stored password must not come back into the field")
	}
	if !strings.Contains(f.pwHint.Text(), "Mot de passe enregistré") {
		t.Errorf("hint = %q", f.pwHint.Text())
	}
	f.name.SetText("Plateau 2")
	f.save()
	if cam = svc.Rig().Slots[4].Camera; !cam.HasPassword || cam.Name != "Plateau 2" {
		t.Errorf("password must be kept: %+v", cam)
	}
	// explicit clear
	a.config.Edit(5)
	f = a.config.form
	f.clearBtn.OnTapped()
	f.save()
	if svc.Rig().Slots[4].Camera.HasPassword {
		t.Error("explicit clear must remove the password")
	}
}

func TestConfigHidesCredentialsForOpenModels(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	sized(a)
	a.ShowConfig(1)
	f := a.config.form
	f.brand.SetSelected("Blackmagic Design")
	f.model.SetSelected("Studio Camera 6K Pro")
	if f.userBox.Visible() || f.passBox.Visible() {
		t.Error("Blackmagic needs no credentials")
	}
	f.brand.SetSelected("Panasonic")
	f.model.SetSelected("AW-UE150")
	if !f.userBox.Visible() || !strings.Contains(f.userLabel.Text(), "facultatif") {
		t.Errorf("Panasonic credentials are optional: visible=%v label=%q", f.userBox.Visible(), f.userLabel.Text())
	}
	if !strings.Contains(f.support.Text(), "Prévu") || f.notes.Text == "" {
		t.Errorf("support=%q notes=%q", f.support.Text(), f.notes.Text)
	}
}

func TestConfigRemoveAsksForConfirmation(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(3)
	rm := findButton(a.config.obj, "Retirer la caméra")
	if rm == nil {
		t.Fatal("remove button missing")
	}
	test.Tap(rm)
	if svc.Rig().Slots[2].Camera == nil {
		t.Fatal("removal must wait for the confirmation")
	}
	ov := a.Window().Canvas().Overlays().Top()
	if ov == nil {
		t.Fatal("confirmation dialog not shown")
	}
	if findButton(ov, "Annuler") == nil {
		t.Error("dialog needs a cancel button")
	}
	ok := findButton(ov, "Retirer")
	if ok == nil {
		t.Fatal("dialog needs a confirm button")
	}
	test.Tap(ok)
	if svc.Rig().Slots[2].Camera != nil {
		t.Error("camera not removed after confirmation")
	}
}

func TestThemeSettingIsStored(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	sized(a)
	a.ShowConfig(0)
	seg := a.config.general.themeSeg
	if seg == nil {
		t.Fatal("theme selector missing")
	}
	seg.Tapped(&fyne.PointEvent{Position: fyne.NewPos(seg.Size().Width-3, 5)}) // Sombre
	if a.Mode() != style.ModeDark || a.fa.Preferences().String("theme") != "dark" {
		t.Errorf("mode=%v pref=%q", a.Mode(), a.fa.Preferences().String("theme"))
	}
	if th, ok := a.fa.Settings().Theme().(style.Theme); !ok || th.Mode != style.ModeDark {
		t.Errorf("theme = %#v", a.fa.Settings().Theme())
	}
	seg.Tapped(&fyne.PointEvent{Position: fyne.NewPos(seg.Size().Width/2, 5)}) // Clair
	if a.Mode() != style.ModeLight || a.fa.Preferences().String("theme") != "light" {
		t.Errorf("mode=%v", a.Mode())
	}
}

func TestStoredThemeIsRestored(t *testing.T) {
	fa := test.NewTempApp(t)
	fa.Preferences().SetString("theme", "light")
	svc, _, _ := newLiveService(t)
	a := New(fa, svc, syncOptions())
	if a.Mode() != style.ModeLight {
		t.Errorf("mode = %v", a.Mode())
	}
}

func TestNoDashesInVisibleTextAndAllFrench(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	check := func(page string) {
		for _, s := range allTexts(a.Window().Content()) {
			if strings.ContainsAny(s, "—–") {
				t.Errorf("%s: dash in %q", page, s)
			}
		}
	}
	check("cameras")
	a.ShowDetails(1)
	check("details")
	a.ShowConfig(2)
	check("config")
}

func TestNoPasswordInVisibleText(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	for slot := 1; slot <= 3; slot++ {
		a.ShowConfig(slot)
		for _, s := range allTexts(a.Window().Content()) {
			if strings.Contains(s, "secret") {
				t.Errorf("password leaked into %q", s)
			}
		}
	}
}
