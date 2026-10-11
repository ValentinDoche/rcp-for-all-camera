package gui

import (
	"bytes"
	"errors"
	"io"
	"net/url"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
)

func generalOf(t *testing.T, a *App) *generalTab {
	t.Helper()
	sized(a)
	a.ShowConfig(0)
	if a.config == nil || a.config.general == nil {
		t.Fatal("configuration page missing")
	}
	return a.config.general
}

func TestConfigurationOpensOnGeneralAndRemembersTheTab(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	test.Tap(a.gear)
	if a.config.tab != tabGeneral {
		t.Fatalf("first tab = %d, want General", a.config.tab)
	}
	a.config.SelectTab(tabCameras)
	test.Tap(a.gear) // close
	test.Tap(a.gear) // reopen
	if a.config.tab != tabCameras {
		t.Fatalf("reopened on tab %d, want the last used one (Cameras)", a.config.tab)
	}
	a.ShowCameras()
	a.ShowConfig(2) // assigning from a strip opens the form
	if a.config.tab != tabCameras || a.config.editSlot != 2 {
		t.Fatalf("tab %d slot %d, want Cameras form of slot 2", a.config.tab, a.config.editSlot)
	}
}

func TestAutoStartToggleCallsTheService(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	g := generalOf(t, a)
	g.autoSeg.OnChange(0)
	if !testAuto.on {
		t.Error("Oui must enable start with Windows")
	}
	g.autoSeg.OnChange(1)
	if testAuto.on {
		t.Error("Non must disable it")
	}
}

func TestAutoStartFailureRevertsAndShowsTheError(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	g := generalOf(t, a)
	testAuto.err = errors.New("registry denied")
	g.autoSeg.SetSelected(0)
	g.autoSeg.OnChange(0)
	if g.autoSeg.Selected() != 1 {
		t.Errorf("control must go back to Non, selected %d", g.autoSeg.Selected())
	}
	if g.autoRow.err.Text() == "" || !g.autoRow.err.Visible() {
		t.Error("the error must show inline")
	}
	testAuto.err = nil
	g.autoSeg.OnChange(0)
	if g.autoRow.err.Visible() {
		t.Error("the error must clear after a success")
	}
}

func TestNetworkAdapterList(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	g := generalOf(t, a)
	want := []string{"Par défaut (choisie par Windows)", "Ethernet : 192.168.1.110", "Wi-Fi : 10.0.0.5"}
	if strings.Join(g.nic.Options, "|") != strings.Join(want, "|") {
		t.Fatalf("options = %q", g.nic.Options)
	}
	g.nic.SetSelected("Wi-Fi : 10.0.0.5")
	if got := svc.Settings().BindIP; got != "10.0.0.5" {
		t.Fatalf("BindIP = %q", got)
	}
	g.nic.SetSelectedIndex(0)
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
	g := generalOf(t, a)
	if g.nic.Selected != "10.0.0.5 (indisponible)" {
		t.Fatalf("selected = %q", g.nic.Selected)
	}
	if !strings.Contains(allTextsJoined(g.nicRow.obj), "n'existe plus") {
		t.Error("a warning help text is expected")
	}
	if svc.Settings().BindIP != "10.0.0.5" {
		t.Error("opening the tab must not change the stored address")
	}
}

func TestNetworkAdapterListRefreshesWhenTheTabOpens(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	g := generalOf(t, a)
	testIfaces.set(app.NetInterface{Name: "USB", IP: "172.16.0.2"})
	a.config.SelectTab(tabCameras)
	a.config.SelectTab(tabGeneral)
	if len(g.nic.Options) != 2 || g.nic.Options[1] != "USB : 172.16.0.2" {
		t.Fatalf("options after reopening = %q", g.nic.Options)
	}
}

func TestOperatorLockAndLogLevelCallTheService(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	g := generalOf(t, a)
	if !svc.Settings().OperatorLock || g.lockSeg.Selected() != 0 {
		t.Fatal("the operator lock is on by default and shown as Oui")
	}
	g.lockSeg.OnChange(1)
	if svc.Settings().OperatorLock {
		t.Error("Non must turn the lock off")
	}
	g.lockSeg.OnChange(0)
	if !svc.Settings().OperatorLock {
		t.Error("Oui must turn the lock back on")
	}
	g.logSeg.OnChange(1)
	if svc.Settings().LogLevel != app.LogDebug {
		t.Error("Debug must be stored")
	}
	g.logSeg.OnChange(0)
	if svc.Settings().LogLevel != app.LogNormal {
		t.Error("Normal must be stored")
	}
}

func TestOpenLogsFolderUsesAFileURL(t *testing.T) {
	a, svc, _ := newTestApp(t, false)
	var got *url.URL
	a.openURL = func(u *url.URL) error { got = u; return nil }
	g := generalOf(t, a)
	g.openLogs()
	if got == nil || got.Scheme != "file" {
		t.Fatalf("opened %v", got)
	}
	if !strings.HasSuffix(strings.ReplaceAll(got.Path, "\\", "/"), strings.ReplaceAll(svc.Settings().LogDir, "\\", "/")) &&
		!strings.Contains(got.Path, "/") {
		t.Errorf("path %q does not hold the log dir", got.Path)
	}
	a.openURL = func(*url.URL) error { return errors.New("no explorer") }
	g.openLogs()
	if !g.logRow.err.Visible() {
		t.Error("a failure must show inline")
	}
}

func TestLogDirURL(t *testing.T) {
	if got := logDirURL("C:/Users/x/logs").String(); got != "file:///C:/Users/x/logs" {
		t.Errorf("url = %q", got)
	}
	if got := logDirURL("/var/log/rcp").String(); got != "file:///var/log/rcp" {
		t.Errorf("url = %q", got)
	}
}

func TestImportRejectsABadFileInline(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	g := generalOf(t, a)
	before := svc.Rig()
	g.applyImport([]byte("this is not json"))
	if !g.importRow.err.Visible() || !strings.Contains(g.importRow.err.Text(), "valide") {
		t.Errorf("inline error = %q visible=%v", g.importRow.err.Text(), g.importRow.err.Visible())
	}
	after := svc.Rig()
	for i := range before.Slots {
		if (before.Slots[i].Camera == nil) != (after.Slots[i].Camera == nil) {
			t.Fatal("a rejected file must not change the rig")
		}
	}
}

func TestImportRejectsAFileOverOneMebibyte(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	g := generalOf(t, a)
	g.importFrom(bytes.NewReader(make([]byte, maxImportSize+10)))
	if !g.importRow.err.Visible() {
		t.Fatal("too large file must be reported")
	}
	if a.Window().Canvas().Overlays().Top() != nil {
		t.Error("no confirmation for a refused file")
	}
}

func TestImportAsksBeforeReplacing(t *testing.T) {
	a, svc, _ := newTestApp(t, true)
	g := generalOf(t, a)
	doc, err := svc.ExportConfig()
	if err != nil {
		t.Fatal(err)
	}
	// cancel: nothing happens
	if err := svc.ClearSlot(1); err != nil {
		t.Fatal(err)
	}
	g.importFrom(bytes.NewReader(doc))
	if b := dialogButton(a, "Annuler"); b == nil {
		t.Fatal("a confirmation dialog is expected")
	} else {
		test.Tap(b)
	}
	if svc.Rig().Slots[0].Camera != nil {
		t.Fatal("cancel must not import")
	}
	// confirm: replaces
	g.importFrom(bytes.NewReader(doc))
	b := dialogButton(a, "Remplacer")
	if b == nil {
		t.Fatal("confirm button missing")
	}
	test.Tap(b)
	if svc.Rig().Slots[0].Camera == nil {
		t.Fatal("confirming must import the cameras")
	}
}

func TestExportWritesTheDocument(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	g := generalOf(t, a)
	var buf bytes.Buffer
	g.exportTo(nopCloser{&buf})
	if !strings.Contains(buf.String(), app.ExportFormat) {
		t.Errorf("exported %q", buf.String())
	}
}

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

// --- operator lock confirmation ------------------------------------------------------

func seedGuarded(t *testing.T) (*App, *app.Service, *liveCam) {
	t.Helper()
	a, svc, cams := newTestApp(t, false)
	sized(a)
	if _, err := svc.AssignSlot(1, app.SlotInput{Model: "panasonic.awue150", Host: "192.168.1.46"}); err != nil {
		t.Fatal(err)
	}
	a.ctl.RefreshRig()
	if len(*cams) != 1 {
		t.Fatalf("%d cameras built", len(*cams))
	}
	return a, svc, (*cams)[0]
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

func TestGuardedControlAsksThenSendsOnConfirm(t *testing.T) {
	a, _, cam := seedGuarded(t)
	a.ctl.Write(1, core.ColorBars, core.Bool(true))
	waitWrites(a.ctl)
	if colorBarsSets(cam) != 0 {
		t.Fatal("a guarded control must not be written before the user confirms")
	}
	if a.ctl.Value(1, core.ColorBars) != nil && a.ctl.Value(1, core.ColorBars).On {
		t.Fatal("the widget must show the last known value while asking")
	}
	body := dialogText(a)
	if !strings.Contains(body, "Cam1") || !strings.Contains(body, "Mire de barres") || !strings.Contains(body, "Activé") {
		t.Errorf("dialog text = %q, want slot, control and new value", body)
	}
	b := dialogButton(a, "Confirmer")
	if b == nil {
		t.Fatal("confirm button missing")
	}
	test.Tap(b)
	waitWrites(a.ctl)
	if colorBarsSets(cam) != 1 {
		t.Fatalf("confirm must write once, wrote %d", colorBarsSets(cam))
	}
	if v := a.ctl.Value(1, core.ColorBars); v == nil || !v.On {
		t.Errorf("value after confirm = %v", v)
	}
}

func TestGuardedControlCancelRestoresTheValue(t *testing.T) {
	a, _, cam := seedGuarded(t)
	rec := &recorder{}
	a.ctl.Subscribe(rec.fn)
	a.ctl.Write(1, core.ColorBars, core.Bool(true))
	waitWrites(a.ctl)
	n := rec.count(ChValue)
	b := dialogButton(a, "Annuler")
	if b == nil {
		t.Fatal("cancel button missing")
	}
	test.Tap(b)
	waitWrites(a.ctl)
	if colorBarsSets(cam) != 0 {
		t.Error("cancel must not write")
	}
	if rec.count(ChValue) <= n {
		t.Error("cancel must tell the widget to return to the last known value")
	}
	if v := a.ctl.Value(1, core.ColorBars); v != nil && v.On {
		t.Error("value must stay off")
	}
}

func TestUnguardedControlIsNotConfirmed(t *testing.T) {
	a, _, cam := seedGuarded(t)
	a.ctl.Write(1, core.Tint, core.Number(1))
	waitWrites(a.ctl)
	if a.Window().Canvas().Overlays().Top() != nil {
		t.Error("an ordinary control must not ask")
	}
	_ = cam
}

// --- language ----------------------------------------------------------------------

func TestLanguageSwitchRebuildsInPlace(t *testing.T) {
	a, _, _ := newTestApp(t, true)
	sized(a)
	a.ShowDetails(1)
	a.SetLanguage("en")
	if a.Language() != "en" || a.fa.Preferences().String("language") != "en" {
		t.Fatalf("language %q pref %q", a.Language(), a.fa.Preferences().String("language"))
	}
	if a.page != pageDetails || a.detailSlot != 1 {
		t.Fatalf("page %v slot %d, want the same details page", a.page, a.detailSlot)
	}
	if got := a.strips[3].stateTxt.Text(); got != "Free" {
		t.Errorf("free strip = %q", got)
	}
	if a.panelTxt.Text() != "Micro Camera Panel not detected" {
		t.Errorf("panel = %q", a.panelTxt.Text())
	}
	joined := allTextsJoined(a.Window().Content())
	for _, fr := range []string{"Caméras", "Détails", "Libre", "Connecté"} {
		if strings.Contains(joined, fr) {
			t.Errorf("French text %q left on the English window", fr)
		}
	}
	a.SetLanguage("fr")
	if a.strips[3].stateTxt.Text() != "Libre" {
		t.Error("switching back must restore French")
	}
}

func TestLanguageSwitchFromTheGeneralTab(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	g := generalOf(t, a)
	if len(g.env.langs) < 2 {
		t.Fatalf("languages = %v", g.env.langs)
	}
	g.langSeg.OnChange(0) // English (languages are sorted by code: en, fr)
	if a.Language() != "en" || a.page != pageConfig {
		t.Fatalf("language %q page %v", a.Language(), a.page)
	}
	if got := a.config.tabs.Selected(); got != tabGeneral {
		t.Errorf("tab = %d", got)
	}
	if !strings.Contains(allTextsJoined(a.config.obj), "Network adapter") {
		t.Error("General tab must be in English")
	}
}

func TestStatusBarUsesTheLanguageUnits(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	a.SetLanguage("en")
	a.setStats(sysinfoStats())
	got := a.statsText()
	for _, want := range []string{"22.3 / 63.3 GB", "5.2 KB/s"} {
		if !strings.Contains(got, want) {
			t.Errorf("status bar %q lacks %q", got, want)
		}
	}
}

func TestLanguageSurvivesRebuildOfStats(t *testing.T) {
	a, _, _ := newTestApp(t, false)
	a.setStats(sysinfoStats())
	a.SetLanguage("en")
	if !strings.Contains(a.statsText(), "KB/s") {
		t.Errorf("stats lost on rebuild: %q", a.statsText())
	}
}

// --- helpers ----------------------------------------------------------------------

func allTextsJoined(o fyne.CanvasObject) string { return strings.Join(allTexts(o), " | ") }

func sysinfoStats() sysinfo.Stats {
	return sysinfo.Stats{CPU: 12.4, MemUsed: 23915626496, MemTotal: 67939266560,
		DownBps: 5329.3, UpBps: 1608.8, IP: "192.168.1.110", HasRates: true}
}

func dialogButton(a *App, text string) *widget.Button {
	ov := a.Window().Canvas().Overlays().Top()
	if ov == nil {
		return nil
	}
	return findButton(ov, text)
}

func dialogText(a *App) string {
	ov := a.Window().Canvas().Overlays().Top()
	if ov == nil {
		return ""
	}
	return strings.Join(allTexts(ov), " | ")
}
