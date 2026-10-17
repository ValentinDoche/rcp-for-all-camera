package gui_test

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// fakeFiles is the file picker: the file the user opens, the file they
// save to.
type fakeFiles struct {
	open  []byte
	saved bytes.Buffer
}

func (f *fakeFiles) Open(_ fyne.Window, done func(io.ReadCloser, error)) {
	done(io.NopCloser(bytes.NewReader(f.open)), nil)
}

func (f *fakeFiles) Save(_ fyne.Window, _ string, done func(io.WriteCloser, error)) {
	done(nopCloser{&f.saved}, nil)
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// general opens Settings > Général.
func general(t *testing.T, a *gui.App) scope {
	t.Helper()
	return category(t, a, "Général")
}

// segment is the selector of a setting.
func segment(t *testing.T, a *gui.App, caption string) *widgets.Segmented {
	t.Helper()
	return one[*widgets.Segmented](t, setting(t, a, caption))
}

func TestSettingsCategoriesAndAbout(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	about := category(t, a, "À propos")
	for _, want := range []string{"Valentin Doche", "github.com/ValentinDoche/rcp-for-all-camera", "© 2026"} {
		if !about.has(want) {
			t.Errorf("About lacks %q", want)
		}
	}
	category(t, a, "Panneau")
	if got := one[*widgets.SideNav](t, settings(a)).Selected(); got != 2 {
		t.Fatalf("side list on %d, want Panneau", got)
	}
	clickIn(t, a, settings(a), "Fermer")
	if settingsOpen(a) {
		t.Fatal("Fermer must close Settings")
	}
}

func TestSettingsOpensOnGeneralAndRemembersTheCategory(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	if s := openSettings(t, a); !s.shows("Carte réseau") {
		t.Fatalf("first category: %s", s.joined())
	}
	category(t, a, "Caméras")
	clickIn(t, a, settings(a), "Fermer")
	if s := openSettings(t, a); !s.shows("Emplacements") {
		t.Fatalf("reopened on %s, want the last used category (Caméras)", s.joined())
	}
	clickIn(t, a, settings(a), "Fermer")
	clickIn(t, a, strip(t, a, "Cam4"), "Assigner une caméra") // assigning from a strip opens the form
	if !form(t, a).shows("Cam4 : assigner une caméra") || one[*widgets.SideNav](t, settings(a)).Selected() != 1 {
		t.Fatalf("want the Caméras form of slot 4: %s", settings(a).joined())
	}
}

func TestAutoStartToggleCallsTheService(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	general(t, a)
	choose(t, a, "Lancer au démarrage de Windows", "Oui")
	if !testAuto.on {
		t.Error("Oui must enable start with Windows")
	}
	choose(t, a, "Lancer au démarrage de Windows", "Non")
	if testAuto.on {
		t.Error("Non must disable it")
	}
}

func TestAutoStartFailureRevertsAndShowsTheError(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	general(t, a)
	const caption = "Lancer au démarrage de Windows"
	quiet := len(setting(t, a, caption).texts())
	testAuto.err = errors.New("registry denied")
	choose(t, a, caption, "Oui")
	if got := segment(t, a, caption).Selected(); got != 1 {
		t.Errorf("the control must go back to Non, selected %d", got)
	}
	if len(setting(t, a, caption).texts()) <= quiet {
		t.Error("the error must show inline")
	}
	testAuto.err = nil
	choose(t, a, caption, "Oui")
	if len(setting(t, a, caption).texts()) != quiet {
		t.Errorf("the error must clear after a success: %s", setting(t, a, caption).joined())
	}
}

func nicBox(t *testing.T, a *gui.App) *widget.Select {
	t.Helper()
	return selectBox(t, settings(a), "Carte réseau")
}

func TestNetworkAdapterList(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	general(t, a)
	want := []string{"Par défaut (choisie par Windows)", "Ethernet : 192.168.1.110", "Wi-Fi : 10.0.0.5"}
	if got := nicBox(t, a).Options; strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("options = %q", got)
	}
	pick(t, a, nicBox(t, a), "Wi-Fi : 10.0.0.5")
	if got := svc.Settings().BindIP; got != "10.0.0.5" {
		t.Fatalf("BindIP = %q", got)
	}
	pick(t, a, nicBox(t, a), "Par défaut (choisie par Windows)")
	if got := svc.Settings().BindIP; got != "" {
		t.Fatalf("default must clear BindIP, got %q", got)
	}
}

func TestStoredAdapterThatDisappearedIsMarkedUnavailable(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	if err := svc.SetBindIP("10.0.0.5"); err != nil {
		t.Fatal(err)
	}
	testIfaces.set(app.NetInterface{Name: "Ethernet", IP: "192.168.1.110"}) // Wi-Fi gone
	general(t, a)
	if got := nicBox(t, a).Selected; got != "10.0.0.5 (indisponible)" {
		t.Fatalf("selected = %q", got)
	}
	if !setting(t, a, "Carte réseau").has("n'existe plus") {
		t.Error("a warning help text is expected")
	}
	if svc.Settings().BindIP != "10.0.0.5" {
		t.Error("opening the category must not change the stored address")
	}
}

func TestNetworkAdapterListRefreshesWhenTheCategoryOpens(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	general(t, a)
	testIfaces.set(app.NetInterface{Name: "USB", IP: "172.16.0.2"})
	category(t, a, "Caméras")
	general(t, a)
	if got := nicBox(t, a).Options; len(got) != 2 || got[1] != "USB : 172.16.0.2" {
		t.Fatalf("options after reopening = %q", got)
	}
}

func TestOperatorLockAndLogLevelCallTheService(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	general(t, a)
	if !svc.Settings().OperatorLock || segment(t, a, "Verrou opérateur").Selected() != 0 {
		t.Fatal("the operator lock is on by default and shown as Oui")
	}
	choose(t, a, "Verrou opérateur", "Non")
	if svc.Settings().OperatorLock {
		t.Error("Non must turn the lock off")
	}
	choose(t, a, "Verrou opérateur", "Oui")
	if !svc.Settings().OperatorLock {
		t.Error("Oui must turn the lock back on")
	}
	choose(t, a, "Journaux", "Debug")
	if svc.Settings().LogLevel != app.LogDebug {
		t.Error("Debug must be stored")
	}
	choose(t, a, "Journaux", "Normal")
	if svc.Settings().LogLevel != app.LogNormal {
		t.Error("Normal must be stored")
	}
}

func TestOpenLogsFolderUsesAFileURL(t *testing.T) {
	var got *url.URL
	var fail error
	a, svc, _ := newTestAppWith(t, false, func(o *gui.Options) {
		o.OpenURL = func(u *url.URL) error { got = u; return fail }
	})
	general(t, a)
	clickIn(t, a, setting(t, a, "Journaux"), "Ouvrir le dossier")
	if got == nil || got.Scheme != "file" {
		t.Fatalf("opened %v", got)
	}
	if want := filepath.ToSlash(svc.Settings().LogDir); !strings.HasSuffix(got.Path, want) {
		t.Errorf("opened %q, want the log folder %q", got.Path, want)
	}
	fail = errors.New("no explorer")
	clickIn(t, a, setting(t, a, "Journaux"), "Ouvrir le dossier")
	if !setting(t, a, "Journaux").shows("Impossible d'ouvrir le dossier des journaux.") {
		t.Error("a failure must show inline")
	}
}

// importFile opens Settings > Général and imports data as the chosen file.
func importFile(t *testing.T, data []byte) (*gui.App, *app.Service) {
	t.Helper()
	files := &fakeFiles{open: data}
	a, svc, _ := newTestAppWith(t, true, func(o *gui.Options) { o.Files = files })
	general(t, a)
	clickIn(t, a, setting(t, a, "Importer une configuration"), "Importer")
	return a, svc
}

func TestImportRejectsABadFileInline(t *testing.T) {
	a, svc := importFile(t, []byte("this is not json"))
	clickIn(t, a, dialog(a), "Remplacer") // the file is read once confirmed
	if !setting(t, a, "Importer une configuration").has("valide") {
		t.Errorf("inline error: %s", setting(t, a, "Importer une configuration").joined())
	}
	if svc.Rig().Slots[0].Camera == nil {
		t.Fatal("a rejected file must not change the rig")
	}
}

func TestImportRejectsAFileOverOneMebibyte(t *testing.T) {
	a, _ := importFile(t, make([]byte, 1<<20+10))
	if !setting(t, a, "Importer une configuration").shows("Fichier trop volumineux (1 Mo maximum).") {
		t.Fatal("too large file must be reported")
	}
	if dialogOpen(a) {
		t.Error("no confirmation for a refused file")
	}
}

func TestImportAsksBeforeReplacing(t *testing.T) {
	svc, _, _ := newLiveService(t)
	seedRig(t, svc)
	doc, err := svc.ExportConfig()
	if err != nil {
		t.Fatal(err)
	}
	a, svc := importFile(t, doc)
	if err := svc.ClearSlot(1); err != nil {
		t.Fatal(err)
	}
	if !dialog(a).shows("Annuler") {
		t.Fatalf("a confirmation dialog is expected: %s", dialog(a).joined())
	}
	clickIn(t, a, dialog(a), "Annuler")
	if svc.Rig().Slots[0].Camera != nil {
		t.Fatal("cancel must not import")
	}
	clickIn(t, a, setting(t, a, "Importer une configuration"), "Importer")
	clickIn(t, a, dialog(a), "Remplacer")
	if svc.Rig().Slots[0].Camera == nil {
		t.Fatal("confirming must import the cameras")
	}
}

func TestExportWritesTheDocument(t *testing.T) {
	files := &fakeFiles{}
	a, _, _ := newTestAppWith(t, true, func(o *gui.Options) { o.Files = files })
	general(t, a)
	clickIn(t, a, setting(t, a, "Exporter la configuration"), "Exporter")
	if !strings.Contains(files.saved.String(), app.ExportFormat) {
		t.Errorf("exported %q", files.saved.String())
	}
	if !win(a).shows("Configuration exportée") {
		t.Error("no notice after the export")
	}
}

// --- operator lock confirmation ------------------------------------------------------

// seedGuarded assigns a UE150 (it has colour bars) to slot 1 and opens its
// details page.
func seedGuarded(t *testing.T) (*gui.App, *liveCam) {
	t.Helper()
	a, svc, cams := newTestApp(t, false)
	if err := svc.AssignSlot(1, app.SlotInput{Model: "panasonic-aw-ue150", Host: "192.168.1.46"}); err != nil {
		t.Fatal(err)
	}
	a.Emit(app.EventRigChanged, svc.Rig())
	if len(*cams) != 1 {
		t.Fatalf("%d cameras built", len(*cams))
	}
	detailsOf(t, a, "Cam1")
	return a, (*cams)[0]
}

func colorBarsSets(c *liveCam) int {
	n := 0
	for _, s := range c.setCalls() {
		if s.ID == core.ColorBars {
			n++
		}
	}
	return n
}

// barsSwitch is the colour bars switch of the details page.
func barsSwitch(t *testing.T, a *gui.App) *widget.Check {
	t.Helper()
	return one[*widget.Check](t, details(t, a).around("Mire de barres", withA[*widget.Check]()))
}

func TestGuardedControlAsksThenSendsOnConfirm(t *testing.T) {
	a, cam := seedGuarded(t)
	press(a, barsSwitch(t, a))
	waitWrites(a.Controller())
	if colorBarsSets(cam) != 0 {
		t.Fatal("a guarded control must not be written before the user confirms")
	}
	body := dialog(a).joined()
	if !strings.Contains(body, "Cam1") || !strings.Contains(body, "Mire de barres") || !strings.Contains(body, "Activé") {
		t.Errorf("dialog text = %q, want slot, control and new value", body)
	}
	clickIn(t, a, dialog(a), "Confirmer")
	waitWrites(a.Controller())
	if colorBarsSets(cam) != 1 {
		t.Fatalf("confirm must write once, wrote %d", colorBarsSets(cam))
	}
	if !barsSwitch(t, a).Checked {
		t.Error("the switch must show the bars on after the confirmation")
	}
}

func TestGuardedControlCancelRestoresTheValue(t *testing.T) {
	a, cam := seedGuarded(t)
	press(a, barsSwitch(t, a))
	waitWrites(a.Controller())
	clickIn(t, a, dialog(a), "Annuler")
	waitWrites(a.Controller())
	if colorBarsSets(cam) != 0 {
		t.Error("cancel must not write")
	}
	if barsSwitch(t, a).Checked {
		t.Error("the switch must return to the last known value")
	}
}

func TestUnguardedControlIsNotConfirmed(t *testing.T) {
	a, cam := seedGuarded(t)
	row := details(t, a).around("Focus mode", withA[*widgets.Segmented]())
	clickIn(t, a, scope{[]fyne.CanvasObject{one[*widgets.Segmented](t, row)}}, "Manual")
	waitWrites(a.Controller())
	if dialogOpen(a) {
		t.Error("an ordinary control must not ask")
	}
	if len(cam.setCalls()) == 0 {
		t.Error("the focus mode must be written at once")
	}
}

// --- language ----------------------------------------------------------------------

func TestLanguageSwitchRebuildsInPlace(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	detailsOf(t, a, "Cam1")
	general(t, a)
	choose(t, a, "Langue", "English")
	if a.Language() != "en" {
		t.Fatalf("language %q", a.Language())
	}
	s := settings(a)
	if !s.shows("Network adapter") || one[*widgets.SideNav](t, s).Selected() != 0 {
		t.Errorf("Settings must stay open on General, in English: %s", s.joined())
	}
	clickIn(t, a, s, "Close")
	if !win(a).has("details") || detailsScroll(a) == nil {
		t.Fatalf("the details page must stay open: %s", win(a).joined())
	}
	if !win(a).shows("Micro Camera Panel not detected") {
		t.Error("the panel status must be in English")
	}
	for _, fr := range []string{"Caméras", "Détails", "Libre", "Connecté"} {
		if win(a).has(fr) {
			t.Errorf("French text %q left on the English window", fr)
		}
	}
	clickIn(t, a, win(a), "Cameras")
	if !strip(t, a, "Cam4").shows("Free") {
		t.Errorf("free strip: %s", strip(t, a, "Cam4").joined())
	}
	press(a, gear(t, a))
	choose(t, a, "Language", "Français")
	clickIn(t, a, settings(a), "Fermer")
	if !strip(t, a, "Cam4").shows("Libre") {
		t.Error("switching back must restore French")
	}
}

func TestLanguagePersistsForTheNextStart(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	general(t, a)
	choose(t, a, "Langue", "English")
	if got := gui.New(fyne.CurrentApp(), svc, syncOptions()).Language(); got != "en" {
		t.Errorf("the next start is in %q", got)
	}
}

func TestStatusBarUsesTheLanguageUnits(t *testing.T) {
	a, _ := newStatsApp(t, someStats())
	general(t, a)
	choose(t, a, "Langue", "English")
	for _, want := range []string{"RAM 22.3 / 63.3 GB", "↓ 5.2 KB/s"} {
		if !win(a).shows(want) {
			t.Errorf("status bar lacks %q: %s", want, win(a).joined())
		}
	}
}
