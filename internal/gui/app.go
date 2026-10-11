package gui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/fr"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
)

// AppID is the Fyne application id (preferences are stored under it).
const AppID = "com.valentindoche.rcpforallcamera"

// WindowTitle is the title of the main window.
const WindowTitle = "RCP For All Camera"

// MinWindow is the smallest supported window.
var MinWindow = fyne.NewSize(1280, 720)

const themePref = "theme"

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

	mode style.Mode

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
func New(fa fyne.App, api API, o Options) *App {
	a := &App{fa: fa, ctl: NewController(api, o)}
	a.mode = style.ParseMode(fa.Preferences().StringWithFallback(themePref, string(style.ModeSystem)))
	fa.Settings().SetTheme(style.NewTheme(a.mode))
	a.win = fa.NewWindow(WindowTitle)
	a.build()
	a.ctl.Load()
	a.syncChrome()
	a.ctl.Subscribe(a.onChange)
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
	a.config = newConfigPage(a.ctl, a.win, a.mode, a.SetMode)
	if slot > 0 {
		a.config.Edit(slot)
	}
	a.setPage(a.config.obj)
	a.gear.Importance = widget.HighImportance
	a.gear.Refresh()
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
	switch {
	case p.Connected:
		a.panelDot.SetToken(style.OK)
		a.panelTxt.SetToken(style.Text)
		a.panelTxt.SetText("Micro Camera Panel connecté")
	case p.Detail != "":
		a.panelDot.SetToken(style.Warn)
		a.panelTxt.SetToken(style.Text)
		a.panelTxt.SetText("Micro Camera Panel : " + p.Detail)
	default:
		a.panelDot.SetToken(style.TextFaint)
		a.panelTxt.SetToken(style.TextMuted)
		a.panelTxt.SetText("Micro Camera Panel non détecté")
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
	case n.Level == fr.LevelError:
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
	cpu, down, up, ip := "CPU --", "↓ --", "↑ --", "IP --"
	if st.HasRates {
		cpu = "CPU " + fr.FormatNumber(st.CPU, 0) + " %"
		down = "↓ " + fr.FormatRate(st.DownBps)
		up = "↑ " + fr.FormatRate(st.UpBps)
	}
	ram := "RAM --"
	if st.MemTotal > 0 {
		ram = "RAM " + fr.FormatMemory(st.MemUsed, st.MemTotal)
	}
	if st.IP != "" {
		ip = "IP " + st.IP
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
