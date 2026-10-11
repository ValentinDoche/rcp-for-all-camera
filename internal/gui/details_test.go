package gui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// iconButtons returns the details page's icon buttons with that icon, in
// layout order (iris card first, then gain, shutter...).
func iconButtons(a *App, icon string) []*widgets.IconButton {
	var out []*widgets.IconButton
	for _, o := range all(a.details.body) {
		if b, ok := o.(*widgets.IconButton); ok && b.Icon() == icon && b.Visible() {
			out = append(out, b)
		}
	}
	return out
}

func lastSet(t *testing.T, cams *[]*liveCam, id core.ControlID) core.Value {
	t.Helper()
	calls := (*cams)[0].setCalls()
	for i := len(calls) - 1; i >= 0; i-- {
		if calls[i].ID == id {
			return calls[i].V
		}
	}
	t.Fatalf("no %s written: %+v", id, calls)
	return core.Value{}
}

func TestIrisButtonsMoveByThirdsAndFullStops(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	plus := iconButtons(a, icons.Plus)
	if len(plus) == 0 {
		t.Fatal("no + button")
	}
	test.Tap(plus[0]) // iris card comes first: f/4 opens a third to f/3.5
	waitWrites(a.ctl)
	if v := lastSet(t, cams, core.Iris); v.Number != 3.5 {
		t.Errorf("+ from f/4 = %v, want 3.5", v.Number)
	}
	test.Tap(iconButtons(a, icons.ChevronsRight)[0]) // a full stop more: the lens' f/2.8
	waitWrites(a.ctl)
	if v := lastSet(t, cams, core.Iris); v.Number != 2.8 {
		t.Errorf("» from f/3.5 = %v, want 2.8 (the lens limit)", v.Number)
	}
	test.Tap(iconButtons(a, icons.ChevronsLeft)[0])
	waitWrites(a.ctl)
	if v := lastSet(t, cams, core.Iris); v.Number != 4 {
		t.Errorf("« from f/2.8 = %v, want 4", v.Number)
	}
}

func TestGainButtonsStepByTheSpecStep(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	test.Tap(iconButtons(a, icons.Plus)[1]) // gain card
	waitWrites(a.ctl)
	if v := lastSet(t, cams, core.Gain); v.Number != 14 {
		t.Errorf("+ from 12 dB = %v, want 14", v.Number)
	}
	if got := a.ctl.Value(1, core.Gain); got == nil || got.Number != 14 {
		t.Errorf("controller gain %v", got)
	}
}

func TestExplicitValuesOnTheDetailsPage(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	texts := allTextsJoined(a.details.obj)
	for _, want := range []string{"f/4.0", "+12 dB", "1/120 s", "5600 K", "35 mm", "objectif de 0,38 m à ∞", "100 %", "0°", "180,0°"} {
		if !containsText(a, want) {
			t.Errorf("%q missing in %s", want, texts)
		}
	}
}

// containsText looks through every text, readouts and captions included.
func containsText(a *App, want string) bool {
	for _, o := range all(a.details.obj) {
		var s string
		switch w := o.(type) {
		case *widgets.Readout:
			s = w.Text.Text()
		case *widgets.Text:
			s = w.Text()
		default:
			continue
		}
		if s == want || (len(s) > len(want) && contains(s, want)) {
			return true
		}
	}
	return false
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestPowerZoomSettingShowsOrHidesTheZoom(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(1)
	f := a.config.form
	if !f.powerZoomBox.Visible() || !f.powerZoom.Checked {
		t.Fatal("Studio 6K Pro with a power zoom: the box must be shown and ticked")
	}
	f.powerZoom.SetChecked(false)
	f.save()
	if svc.Rig().Slots[0].Camera.PowerZoom {
		t.Fatal("power zoom not saved")
	}
	if a.ctl.Has(1, core.Zoom) {
		t.Error("without a power zoom the camera must not declare a zoom")
	}

	a.ShowConfig(3) // Panasonic PTZ: built-in zoom, no setting
	if a.config.form.powerZoomBox.Visible() {
		t.Error("a PTZ camera has its zoom built in: no power zoom setting")
	}
}

func TestDetailsShowEachControlOnce(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	for _, id := range []core.ControlID{core.ShutterAngle, core.MasterBlack, core.Record, core.ISO} {
		if a.details.rows[id] != nil {
			t.Errorf("%s has a generic row: it is already shown elsewhere", id)
		}
	}
	for _, s := range a.details.secs {
		if s.group == core.GroupTransport || s.group == core.GroupExposure {
			t.Errorf("section %q left with controls shown elsewhere", s.title)
		}
	}
}

func TestStripShowsTheFStop(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	c := a.strips[0].content
	if c.iris == nil || c.iris.Text() != "f/4.0" {
		t.Fatalf("strip iris = %v", c.iris)
	}
}

// A maximised window on a 1920 x 1080 screen leaves about 1920 x 990 for the
// canvas (title bar and taskbar).
func TestDetailsFitAMaximisedFullHDWindowWithoutScrolling(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	a.Window().Resize(fyne.NewSize(1920, 990))
	a.ShowDetails(1)
	a.Window().Resize(fyne.NewSize(1920, 990))
	var sc *container.Scroll
	for _, o := range all(a.details.obj) {
		if s, ok := o.(*container.Scroll); ok && s.Content != nil && s.Direction == container.ScrollVerticalOnly {
			sc = s
			break
		}
	}
	if sc == nil {
		t.Fatal("details scroll not found")
	}
	if need, have := sc.Content.MinSize().Height, sc.Size().Height; need > have {
		t.Errorf("details need %v px of height, the window gives %v", need, have)
	}
	if need, have := sc.Content.MinSize().Width, sc.Size().Width; need > have {
		t.Errorf("details need %v px of width, the window gives %v", need, have)
	}
}

func TestDemoCameraNeedsNoAddressAndDrivesEverything(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	sized(a)
	a.ShowConfig(4)
	f := a.config.form
	f.brand.SetSelected("Démo")
	if f.current == nil || !f.current.NoAddress {
		t.Fatal("the demo model must be picked")
	}
	if f.addrBox.Visible() {
		t.Error("the demo camera has no address: host and port are hidden")
	}
	f.save()
	cam := svc.Rig().Slots[3].Camera
	if cam == nil || cam.Host != "" || cam.Status.State != core.StateConnected {
		t.Fatalf("demo slot = %+v", cam)
	}
	a.ShowDetails(4)
	for _, id := range []core.ControlID{core.Iris, core.Zoom, core.Focus, core.Gain, core.ND, core.WhiteBalance, core.Contrast} {
		if !a.details.cards[id] {
			t.Errorf("%s must be live on the demo camera", id)
		}
	}
	test.Tap(iconButtons(a, icons.Plus)[0]) // iris +
	waitWrites(a.ctl)
	if v := a.ctl.Value(4, core.Iris); v == nil || v.Number != 3.5 {
		t.Errorf("demo iris after + = %v, want 3.5", v)
	}
}
