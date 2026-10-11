package gui

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
	"github.com/valentindoche/rcp-for-all-camera/internal/i18n"
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
)

// AppID is the Fyne application id (preferences are stored under it).
const AppID = "com.valentindoche.rcpforallcamera"

// WindowTitle is the title of the main window.
const WindowTitle = "RCP For All Camera"

// MinWindow is the smallest supported window.
var MinWindow = fyne.NewSize(1280, 720)

const (
	themePref = "theme"
	langPref  = "language"
)

type pageKind int

const (
	pageCameras pageKind = iota
	pageDetails
	pageConfig
)

// App is the main window and its navigation. It implements app.Emitter.
type App struct {
	fa  fyne.App
	win fyne.Window
	ctl *Controller
	api AppAPI

	mode      style.Mode
	bundle    *i18n.Bundle
	lang      string
	configTab int
	lastStats sysinfo.Stats
	openURL   func(*url.URL) error
	unsub     func()

	root      *fyne.Container
	host      *fyne.Container
	slotTabs  *widgets.Tabs
	slotBar   fyne.CanvasObject
	gear      *widget.Button
	stats     []*widgets.Text // CPU, RAM, down, up, IP
	panelDot  *widgets.Dot
	panelTxt  *widgets.Text
	noticeTxt *widgets.Text

	page       pageKind
	detailSlot int

	cameras *fyne.Container
	scroll  *container.Scroll
	strips  []*strip
	details *detailsPage
	config  *configPage
}

var _ app.Emitter = (*App)(nil)

// New builds the window around api. Pass Options{} in production.
func New(fa fyne.App, api AppAPI, o Options) *App {
	a := &App{fa: fa, api: api, ctl: NewController(api, o), bundle: i18n.Default()}
	a.openURL = fa.OpenURL
	a.lang = fa.Preferences().StringWithFallback(langPref, "fr")
	if !a.bundle.Has(a.lang) {
		a.lang = i18n.FallbackLang
	}
	a.ctl.SetTexts(texts.New(a.bundle.Translator(a.lang)))
	a.mode = style.ParseMode(fa.Preferences().StringWithFallback(themePref, string(style.ModeSystem)))
	fa.Settings().SetTheme(style.NewTheme(a.mode))
	a.win = fa.NewWindow(WindowTitle)
	a.build()
	a.ctl.Load()
	a.syncChrome()
	a.unsub = a.ctl.Subscribe(a.onChange)
	a.ctl.OnConfirm(a.askConfirm)
	a.win.SetContent(a.root)
	a.win.SetPadded(false)
	a.win.Resize(fyne.NewSize(1600, 900))
	return a
}

// Window returns the main window.
func (a *App) Window() fyne.Window { return a.win }

// Controller returns the UI model (tests).
func (a *App) Controller() *Controller { return a.ctl }

// Emit implements app.Emitter: service events reach the UI on the Fyne thread.
func (a *App) Emit(name string, payload any) { a.ctl.Emit(name, payload) }

// Run shows the window and blocks until it is closed. ctx stops the poller.
func (a *App) Run(ctx context.Context) {
	a.ctl.Poll(ctx, 2*time.Second)
	go a.sampleHost(ctx, time.Second)
	a.win.ShowAndRun()
}

// Mode returns the current theme setting.
func (a *App) Mode() style.Mode { return a.mode }

// SetMode applies and stores the theme setting.
func (a *App) SetMode(m style.Mode) {
	a.mode = m
	a.fa.Preferences().SetString(themePref, string(m))
	a.fa.Settings().SetTheme(style.NewTheme(m))
}

// --- construction ------------------------------------------------------------------

func (a *App) build() {
	// top bar
	name := widgets.NewText(WindowTitle, style.Text, 15).Bold()
	a.panelDot = widgets.NewDot(style.TextFaint)
	a.panelTxt = widgets.NewText("", style.TextMuted, 12)
	status := container.NewHBox(a.panelDot, a.panelTxt)
	top := widgets.NewPanel(style.Surface, "", 0, container.New(layout.NewCustomPaddedLayout(10, 10, 16, 16),
		container.NewBorder(nil, nil, name, status))).Flat()

	// slot tabs
	items := make([]widgets.TabItem, app.SlotCount)
	a.slotTabs = widgets.NewTabs(items, func(i int) { a.onSlotTab(i + 1) })
	a.slotBar = a.slotTabs

	// bottom bar: the gear toggles Configuration, the centre shows host stats
	a.gear = widget.NewButtonWithIcon("", theme.SettingsIcon(), func() {
		if a.page == pageConfig {
			a.ShowCameras()
		} else {
			a.ShowConfig(0)
		}
	})
	a.strips = nil
	a.gear.Importance = widget.LowImportance
	var statObjs []fyne.CanvasObject
	for i := 0; i < 5; i++ {
		t := widgets.NewText("", style.TextMuted, 12).Mono()
		a.stats = append(a.stats, t)
		statObjs = append(statObjs, container.New(layout.NewCustomPaddedLayout(0, 0, 10, 10), t))
	}
	a.setStats(sysinfo.Stats{})
	a.noticeTxt = widgets.NewText("", style.TextMuted, 12).Clip().Align(fyne.TextAlignTrailing)
	bottom := widgets.NewPanel(style.Surface, "", 0, container.New(layout.NewCustomPaddedLayout(2, 2, 10, 14),
		container.NewBorder(nil, nil, a.gear, widgets.Sized(460, 0, a.noticeTxt),
			container.NewCenter(container.NewHBox(statObjs...))))).Flat()

	// pages
	for i := 1; i <= app.SlotCount; i++ {
		a.strips = append(a.strips, newStrip(a.ctl, i, stripActions{
			Details: func(slot int) { a.ShowDetails(slot) },
			Assign:  func(slot int) { a.ShowConfig(slot) },
		}))
	}
	var objs []fyne.CanvasObject
	for _, s := range a.strips {
		objs = append(objs, container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), s.Object()))
	}
	a.cameras = container.NewHBox(objs...)
	a.scroll = container.NewScroll(container.New(layout.NewCustomPaddedLayout(8, 8, 12, 4), a.cameras))
	a.host = container.NewStack(a.scroll)

	bg := widgets.NewPanel(style.Bg, "", 0, nil)
	body := container.NewBorder(container.NewVBox(top, a.slotBar), bottom, nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 8, 8), a.host))
	a.root = container.NewStack(bg, widgets.Sized(MinWindow.Width, MinWindow.Height, body))
}

// --- navigation -------------------------------------------------------------------------

func (a *App) setPage(obj fyne.CanvasObject) {
	a.host.Objects = []fyne.CanvasObject{obj}
	a.host.Refresh()
}

func (a *App) closeSubpages() {
	if a.details != nil {
		a.details.Close()
		a.details = nil
	}
	if a.config != nil {
		a.config.Close()
		a.config = nil
	}
}

// ShowCameras shows the strips page.
func (a *App) ShowCameras() {
	a.closeSubpages()
	a.page = pageCameras
	a.setPage(a.scroll)
	a.gear.Importance = widget.LowImportance
	a.gear.Refresh()
	a.scrollToActive()
}

// ShowDetails shows the full page of one assigned camera.
func (a *App) ShowDetails(slot int) {
	if a.ctl.Camera(slot) == nil {
		a.ShowCameras()
		return
	}
	a.closeSubpages()
	a.page = pageDetails
	a.detailSlot = slot
	if a.ctl.Active() != slot {
		a.ctl.Select(slot)
	}
	a.details = newDetailsPage(a.ctl, slot, a.ShowCameras)
	a.setPage(a.details.obj)
	a.gear.Importance = widget.LowImportance
	a.gear.Refresh()
}

// ShowConfig shows the configuration page; slot > 0 opens its form.
func (a *App) ShowConfig(slot int) {
	a.closeSubpages()
	a.page = pageConfig
	tab := a.configTab
	if slot > 0 {
		tab = tabCameras
	}
	a.config = newConfigPage(a.ctl, a.configEnv(), tab)
	if slot > 0 {
		a.config.Edit(slot)
	}
	a.setPage(a.config.obj)
	a.gear.Importance = widget.HighImportance
	a.gear.Refresh()
}

func (a *App) configEnv() *configEnv {
	return &configEnv{
		win: a.win, api: a.api, mode: a.mode, setMode: a.SetMode,
		lang: a.lang, langs: a.bundle.Languages(), setLang: a.SetLanguage,
		openURL: func(u *url.URL) error { return a.openURL(u) }, setTab: func(t int) { a.configTab = t },
	}
}

// Language returns the code of the current language.
func (a *App) Language() string { return a.lang }

// SetLanguage switches the interface language: the choice is stored and the
// window content is rebuilt in place, keeping the current page.
func (a *App) SetLanguage(code string) {
	if !a.bundle.Has(code) || code == a.lang {
		return
	}
	a.lang = code
	a.fa.Preferences().SetString(langPref, code)
	a.ctl.SetTexts(texts.New(a.bundle.Translator(code)))
	a.rebuild()
}

// rebuild recreates every widget (after a language switch) and returns to
// the page that was open.
func (a *App) rebuild() {
	page, slot, stats := a.page, a.detailSlot, a.lastStats
	a.closeSubpages()
	for _, s := range a.strips {
		s.Close()
	}
	if a.unsub != nil {
		a.unsub()
	}
	a.build()
	a.syncChrome()
	a.setStats(stats)
	a.unsub = a.ctl.Subscribe(a.onChange)
	a.win.SetContent(a.root)
	switch page {
	case pageDetails:
		a.ShowDetails(slot)
	case pageConfig:
		a.ShowConfig(0)
	default:
		a.page = pageCameras
	}
}

// askConfirm shows the operator-lock dialog for a guarded write. The write is
// sent only if the user confirms; otherwise the control returns to its value.
func (a *App) askConfirm(r ConfirmRequest) {
	tx := a.ctl.Tx()
	control := tx.ControlLabel(r.Spec)
	body := tx.T("confirm.actionBody", "slot", r.Slot, "control", control)
	if r.Spec.Kind != core.KindAction {
		body = tx.T("confirm.valueBody", "slot", r.Slot, "control", control, "value", tx.FormatValue(r.Spec, &r.Value))
	}
	showConfirm(a.win, tx.T("confirm.title"), body, tx.T("confirm.yes"), tx.T("confirm.no"), func(ok bool) {
		if ok {
			a.ctl.Confirm(r)
		} else {
			a.ctl.Decline(r)
		}
	})
}

func (a *App) scrollToActive() {
	slot := a.ctl.Active()
	if slot < 1 {
		return
	}
	left := float32(slot-1) * (stripWidth + 8)
	view := a.scroll.Size().Width
	x := a.scroll.Offset.X
	switch {
	case left < x:
		x = left
	case left+stripWidth+16 > x+view:
		x = left + stripWidth + 16 - view
	}
	if x < 0 {
		x = 0
	}
	a.scroll.Offset = fyne.NewPos(x, 0)
	a.scroll.Refresh()
}

func (a *App) onSlotTab(slot int) {
	a.ctl.Select(slot)
	switch a.page {
	case pageCameras:
		a.scrollToActive()
	case pageDetails:
		a.ShowDetails(slot)
	case pageConfig:
		a.ShowCameras()
	}
}

// --- reacting to the model ----------------------------------------------------------------

func (a *App) onChange(ch Change) {
	switch ch.Kind {
	case ChRig, ChSlot, ChPanel, ChNotice:
		a.syncChrome()
	case ChValue:
		if ch.ID == core.Tally {
			a.syncChrome()
		}
	}
	if ch.Kind == ChRig && a.page == pageDetails {
		if a.ctl.Camera(a.detailSlot) == nil {
			a.ShowCameras()
		} else if act := a.ctl.Active(); act != a.detailSlot && act > 0 && a.ctl.Camera(act) != nil {
			a.ShowDetails(act)
		}
	}
}

// syncChrome refreshes the top bar, the slot tabs and the notice.
func (a *App) syncChrome() {
	p := a.ctl.Panel()
	tx := a.ctl.Tx()
	switch {
	case p.Connected && p.Active:
		a.panelDot.SetToken(style.OK)
		a.panelTxt.SetToken(style.Text)
		a.panelTxt.SetText(tx.T("app.panel.active"))
	case p.Connected:
		a.panelDot.SetToken(style.Warn)
		a.panelTxt.SetToken(style.Text)
		if p.Detail != "" {
			a.panelTxt.SetText(tx.T("app.panel.idleDetail", "detail", p.Detail))
		} else {
			a.panelTxt.SetText(tx.T("app.panel.idle"))
		}
	case p.Detail != "":
		a.panelDot.SetToken(style.Warn)
		a.panelTxt.SetToken(style.Text)
		a.panelTxt.SetText(tx.T("app.panel.detail", "detail", p.Detail))
	default:
		a.panelDot.SetToken(style.TextFaint)
		a.panelTxt.SetToken(style.TextMuted)
		a.panelTxt.SetText(tx.T("app.panel.absent"))
	}

	items := make([]widgets.TabItem, app.SlotCount)
	for i := range items {
		slot := i + 1
		v := a.ctl.Value(slot, core.Tally)
		items[i] = widgets.TabItem{
			Text: fmt.Sprintf("Cam%d", slot),
			Dim:  a.ctl.Camera(slot) == nil,
			Live: v != nil && v.On,
		}
	}
	a.slotTabs.SetItems(items)
	a.slotTabs.SetSelected(a.ctl.Active() - 1)

	n := a.ctl.Notice()
	switch {
	case n.Text == "":
		a.noticeTxt.SetText("")
	case n.Level == texts.LevelError:
		a.noticeTxt.SetToken(style.Danger)
		a.noticeTxt.SetText(n.Text)
	default:
		a.noticeTxt.SetToken(style.TextMuted)
		a.noticeTxt.SetText(n.Text)
	}
}

// --- host statistics ------------------------------------------------------------------

// sampleHost refreshes the status bar every interval until ctx ends. Errors
// leave the placeholders: the bar is informative only.
func (a *App) sampleHost(ctx context.Context, interval time.Duration) {
	s := sysinfo.New()
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if st, err := s.Sample(); err == nil {
			fyne.Do(func() { a.setStats(st) })
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (a *App) setStats(st sysinfo.Stats) {
	a.lastStats = st
	tx := a.ctl.Tx()
	lab := func(k string) string { return tx.T(k) }
	cpu, down, up, ip := lab("status.cpu")+" --", "↓ --", "↑ --", lab("status.ip")+" --"
	if st.HasRates {
		cpu = lab("status.cpu") + " " + a.ctl.tx.FormatNumber(st.CPU, 0) + " %"
		down = "↓ " + a.ctl.tx.FormatRate(st.DownBps)
		up = "↑ " + a.ctl.tx.FormatRate(st.UpBps)
	}
	ram := lab("status.ram") + " --"
	if st.MemTotal > 0 {
		ram = lab("status.ram") + " " + a.ctl.tx.FormatMemory(st.MemUsed, st.MemTotal)
	}
	if st.IP != "" {
		ip = lab("status.ip") + " " + st.IP
	}
	for i, txt := range []string{cpu, ram, down, up, ip} {
		a.stats[i].SetText(txt)
	}
}

// statsText is the status bar content as one string (tests).
func (a *App) statsText() string {
	parts := make([]string, len(a.stats))
	for i, t := range a.stats {
		parts[i] = t.Text()
	}
	return strings.Join(parts, "  ")
}
