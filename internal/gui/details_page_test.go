package gui_test

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// details is the body of the open details page (its scrolled cards and
// sections), without the camera strip beside it.
func details(t *testing.T, a *gui.App) scope {
	t.Helper()
	if sc := detailsScroll(a); sc != nil {
		return scope{[]fyne.CanvasObject{sc}}
	}
	t.Fatalf("no details page open: %s", win(a).joined())
	return scope{}
}

// detailsScroll is the vertical scroll of the details page, nil when the
// page is not open.
func detailsScroll(a *gui.App) *container.Scroll {
	for _, s := range allOf[*container.Scroll](win(a)) {
		if s.Direction == container.ScrollVerticalOnly && (scope{[]fyne.CanvasObject{s}}).shows("Presets") {
			return s
		}
	}
	return nil
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

// card is the card of the details page titled title, with its − / +
// buttons.
func card(t *testing.T, a *gui.App, title string) scope {
	t.Helper()
	c := details(t, a).around(title, func(s scope) bool { return len(iconButtons(s, icons.Plus)) > 0 })
	if c.roots == nil {
		t.Fatalf("no %q card: %s", title, details(t, a).joined())
	}
	return c
}

func TestIrisButtonsMoveByThirdsAndFullStops(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	iris := card(t, a, "Iris")
	press(a, iconButtons(iris, icons.Plus)[0]) // f/4 opens a third to f/3.5
	waitWrites(a.Controller())
	if v := lastSet(t, cams, core.Iris); v.Number != 3.5 {
		t.Errorf("+ from f/4 = %v, want 3.5", v.Number)
	}
	press(a, iconButtons(iris, icons.ChevronsRight)[0]) // a full stop more: the lens' f/2.8
	waitWrites(a.Controller())
	if v := lastSet(t, cams, core.Iris); v.Number != 2.8 {
		t.Errorf("» from f/3.5 = %v, want 2.8 (the lens limit)", v.Number)
	}
	press(a, iconButtons(iris, icons.ChevronsLeft)[0])
	waitWrites(a.Controller())
	if v := lastSet(t, cams, core.Iris); v.Number != 4 {
		t.Errorf("« from f/2.8 = %v, want 4", v.Number)
	}
	if !card(t, a, "Iris").shows("f/4.0") {
		t.Error("the iris card must show the new f-stop")
	}
}

func TestGainButtonsStepByTheSpecStep(t *testing.T) {
	a, _, cams := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	press(a, iconButtons(card(t, a, "Gain"), icons.Plus)[0])
	waitWrites(a.Controller())
	if v := lastSet(t, cams, core.Gain); v.Number != 14 {
		t.Errorf("+ from 12 dB = %v, want 14", v.Number)
	}
	if !card(t, a, "Gain").shows("+14 dB") {
		t.Errorf("gain card: %s", card(t, a, "Gain").joined())
	}
}

func TestExplicitValuesOnTheDetailsPage(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	d := details(t, a)
	for _, want := range []string{"f/4.0", "+12 dB", "1/120 s", "5600 K", "35 mm", "objectif de 0,38 m à ∞", "100 %", "0°", "180,0°"} {
		if !d.has(want) {
			t.Errorf("%q missing in %s", want, d.joined())
		}
	}
}

// zoomLever is the Wide / Tele lever of the lens card.
func zoomLever(t *testing.T, a *gui.App) *widgets.Lever {
	t.Helper()
	return one[*widgets.Lever](t, details(t, a).around("Télé", withA[*widgets.Lever]()))
}

const powerZoomBox = "Optique Power Zoom (zoom motorisé)"

func TestPowerZoomSettingShowsOrHidesTheZoom(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	f := cameraForm(t, a, "Cam1")
	pz := checkBox(t, f, powerZoomBox)
	if !pz.Checked {
		t.Fatal("Studio 6K Pro with a power zoom: the box must be ticked")
	}
	press(a, pz)
	saveForm(t, a)
	if svc.Rig().Slots[0].Camera.PowerZoom {
		t.Fatal("power zoom not saved")
	}
	clickIn(t, a, settings(a), "Fermer")
	detailsOf(t, a, "Cam1")
	if !zoomLever(t, a).Disabled() {
		t.Error("without a power zoom the zoom lever must be greyed")
	}

	f = cameraForm(t, a, "Cam3") // Panasonic PTZ: built-in zoom, no setting
	for _, c := range allOf[*widget.Check](f) {
		if c.Text == powerZoomBox {
			t.Error("a PTZ camera has its zoom built in: no power zoom setting")
		}
	}
}

func TestDetailsShowEachControlOnce(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	d := details(t, a)
	// Exposure and transport controls have their cards: no generic
	// section is left for them.
	for _, title := range []string{"Exposition", "Enregistrement"} {
		if d.shows(title) {
			t.Errorf("section %q left with controls shown elsewhere", title)
		}
	}
	if n := len(allOf[*widgets.ColorWheel](d)); n != 3 {
		t.Errorf("%d colour wheels, want lift, gamma and gain", n)
	}
}

func TestStripShowsTheFStop(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	if s := strip(t, a, "Cam1"); !s.shows("f/4.0") {
		t.Fatalf("strip of Cam1: %s", s.joined())
	}
}

// A maximised window on a 1920 x 1080 screen leaves about 1920 x 990 for the
// canvas (title bar and taskbar).
func TestDetailsFitAMaximisedFullHDWindowWithoutScrolling(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	a.Window().Resize(fyne.NewSize(1920, 990))
	sc := detailsScroll(a)
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
	cameraForm(t, a, "Cam4")
	chooseModel(t, a, "Demo", "")
	f := form(t, a)
	if f.shows("Adresse de la caméra") || f.shows("Port") {
		t.Error("the demo camera has no address: host and port are hidden")
	}
	press(a, checkBox(t, f, powerZoomBox)) // like a Studio 6K Pro
	saveForm(t, a)
	cam := svc.Rig().Slots[3].Camera
	if cam == nil || cam.Host != "" || cam.Status.State != core.StateConnected {
		t.Fatalf("demo slot = %+v", cam)
	}
	clickIn(t, a, settings(a), "Fermer")
	detailsOf(t, a, "Cam4")
	d := details(t, a)
	for _, title := range []string{"Iris", "Objectif", "Gain", "Filtre ND", "Balance des blancs", "Contraste"} {
		if !d.shows(title) {
			t.Errorf("no %q on the demo camera: %s", title, d.joined())
		}
	}
	if zoomLever(t, a).Disabled() {
		t.Error("the demo camera with a power zoom offers the zoom")
	}
	press(a, iconButtons(card(t, a, "Iris"), icons.Plus)[0])
	waitWrites(a.Controller())
	if !card(t, a, "Iris").shows("f/3.5") {
		t.Errorf("demo iris after +: %s", card(t, a, "Iris").joined())
	}
}

// countLabels counts the captions of the scope showing exactly text.
func countLabels(s scope, text string) int {
	n := 0
	for _, w := range allOf[*widgets.Text](s) {
		if w.Text() == text {
			n++
		}
	}
	return n
}

func TestTheModulePageLaysOutTheDetails(t *testing.T) {
	svc, _, _ := newLiveService(t)
	if err := svc.AssignSlot(1, app.SlotInput{Model: "demo", Name: "Demo"}); err != nil {
		t.Fatal(err)
	}
	a := gui.New(test.NewTempApp(t), svc, syncOptions())
	t.Cleanup(func() { waitWrites(a.Controller()) })
	detailsOf(t, a, "Cam1")
	block := details(t, a).around("Tally lamp", withText("Firmware"))
	t.Run("the widgets of ui.go are used", func(t *testing.T) {
		if block.roots == nil {
			t.Fatalf("no Demo block: %s", details(t, a).joined())
		}
		for label, has := range map[string]func(scope) bool{
			"Tally lamp":      withA[*widget.Check](),  // switch
			"Flash the tally": withA[*widget.Button](), // button
			"Firmware":        withText("demo 0.3.0"),  // readout
			"Camera name":     withA[*widget.Entry](),  // text
		} {
			if block.around(label, has).roots == nil {
				t.Errorf("%q has not the widget ui.go asks for: %s", label, block.joined())
			}
		}
	})
	t.Run("controls of the Demo block are not repeated in the generated sections", func(t *testing.T) {
		for _, label := range []string{"Tally lamp", "Flash the tally", "Firmware", "Camera name"} {
			if n := countLabels(details(t, a), label); n != 1 {
				t.Errorf("%q shown %d times", label, n)
			}
		}
	})
}

func TestACustomPageReplacesTheStandardLayout(t *testing.T) {
	svc, _, cams := newLiveService(t)
	if err := svc.AssignSlot(1, app.SlotInput{Model: "bm-studio-6k-pro", Host: "192.168.1.41"}); err != nil {
		t.Fatal(err)
	}
	page := core.Page{Rows: []core.Row{
		{Blocks: []core.Block{{Type: core.BlockGrid, Columns: 2, Blocks: []core.Block{
			{Type: core.BlockIris}, {Type: core.BlockWheel, Wheel: core.WheelLift},
		}}}},
		{Blocks: []core.Block{{Type: core.BlockPresets}}},
		{Blocks: []core.Block{{Type: core.BlockRemaining}}},
	}}
	(*cams)[0].page = &page
	a := gui.New(test.NewTempApp(t), svc, syncOptions())
	t.Cleanup(func() { waitWrites(a.Controller()) })
	detailsOf(t, a, "Cam1")
	d := details(t, a)
	t.Run("one wheel, the lift", func(t *testing.T) {
		if n := len(allOf[*widgets.ColorWheel](d)); n != 1 || !d.shows("Lift") || d.shows("Gamma") {
			t.Errorf("%d wheels: %s", n, d.joined())
		}
	})
	t.Run("the iris card drives the iris", func(t *testing.T) {
		if len(iconButtons(card(t, a, "Iris"), icons.Plus)) == 0 {
			t.Error("no iris card")
		}
	})
	t.Run("everything else is in the remaining sections", func(t *testing.T) {
		for _, label := range []string{"Gain", "Gamma rouge", "Balance des blancs"} {
			if !d.shows(label) {
				t.Errorf("%q is not reachable: %s", label, d.joined())
			}
		}
		if d.shows("Lift rouge") {
			t.Error("lift R is shown twice")
		}
	})
}
