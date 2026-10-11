package gui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// Configuration tabs.
const (
	tabGeneral = iota
	tabCameras
)

// configPage is Configuration: a General tab (app settings) and a Cameras
// tab (the 8 slots and the form of one).
type configPage struct {
	ctl *Controller
	win fyne.Window
	env *configEnv
	tx  texts.Texts

	obj     fyne.CanvasObject
	tabs    *widgets.Tabs
	body    *fyne.Container
	tab     int
	general *generalTab
	cameras fyne.CanvasObject

	list   *fyne.Container
	right  *fyne.Container
	cancel func()

	editSlot int
	form     *slotForm
}

func newConfigPage(ctl *Controller, env *configEnv, tab int) *configPage {
	p := &configPage{ctl: ctl, win: env.win, env: env, tx: ctl.Tx()}
	p.list = container.NewVBox()
	p.right = container.NewVBox()
	title := widgets.NewText(p.tx.T("config.slots"), style.Text, 16).Bold()
	left := container.NewBorder(container.New(layout.NewCustomPaddedLayout(0, 6, 2, 0), title), nil, nil, nil,
		container.NewVScroll(p.list))
	p.cameras = container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), widgets.Sized(440, 0, left)), nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), container.NewVScroll(p.right)))
	p.general = newGeneralTab(ctl, env)

	p.tabs = widgets.NewTabs([]widgets.TabItem{
		{Text: p.tx.T("config.tab.general")}, {Text: p.tx.T("config.tab.cameras")},
	}, func(i int) { p.SelectTab(i) })
	p.body = container.NewStack()
	p.obj = container.NewBorder(p.tabs, nil, nil, nil, container.New(layout.NewCustomPaddedLayout(8, 0, 0, 0), p.body))

	p.cancel = ctl.Subscribe(func(ch Change) {
		if ch.Kind == ChRig {
			p.rebuildList()
		}
	})
	p.rebuildList()
	p.rebuildRight()
	p.SelectTab(tab)
	return p
}

// SelectTab shows one tab; the General tab re-reads the settings each time.
func (p *configPage) SelectTab(i int) {
	p.tab = i
	p.tabs.SetSelected(i)
	if i == tabCameras {
		p.body.Objects = []fyne.CanvasObject{p.cameras}
	} else {
		p.general.Refresh()
		p.body.Objects = []fyne.CanvasObject{p.general.obj}
	}
	p.body.Refresh()
	if p.env.setTab != nil {
		p.env.setTab(i)
	}
}

// Close stops listening.
func (p *configPage) Close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// Edit opens the form for a slot (on the Cameras tab).
func (p *configPage) Edit(slot int) {
	p.editSlot = slot
	p.rebuildList()
	p.rebuildRight()
	if slot > 0 && p.tab != tabCameras {
		p.SelectTab(tabCameras)
	}
}

func (p *configPage) rebuildList() {
	p.list.Objects = nil
	for i := 1; i <= app.SlotCount; i++ {
		p.list.Add(p.slotRow(i))
	}
	p.list.Refresh()
}

func (p *configPage) slotRow(slot int) fyne.CanvasObject {
	cam := p.ctl.Camera(slot)
	name := widgets.NewText(fmt.Sprintf("Cam%d", slot), style.Text, 14).Bold()
	var mid fyne.CanvasObject
	var btn *widget.Button
	if cam == nil {
		mid = widgets.NewText(p.tx.T("config.freeSlot"), style.TextMuted, 13)
		btn = widget.NewButton(p.tx.T("config.assign"), func() { p.Edit(slot) })
	} else {
		title := cam.Name
		if title == "" {
			title = cam.ModelName
		}
		line2 := fmt.Sprintf("%s %s, %s:%d", cam.Brand, cam.ModelName, cam.Host, cam.Port)
		if cam.Host == "" {
			line2 = fmt.Sprintf("%s %s", cam.Brand, cam.ModelName)
		}
		mid = container.NewVBox(
			widgets.NewText(title, style.Text, 13).Bold().Clip(),
			widgets.NewText(line2, style.TextMuted, 11).Clip(),
		)
		btn = widget.NewButton(p.tx.T("config.edit"), func() { p.Edit(slot) })
	}
	fill := style.Surface
	if p.editSlot == slot {
		fill = style.Raised
	}
	var state fyne.CanvasObject = widgets.Sized(0, 0, layout.NewSpacer())
	if cam != nil {
		tok := style.TextMuted
		switch cam.Status.State {
		case core.StateConnected:
			tok = style.OK
		case core.StateError:
			tok = style.Danger
		}
		state = widgets.NewText(p.tx.ConnLabel(cam.Status.State), tok, 11).Align(fyne.TextAlignTrailing)
	}
	right := container.NewHBox(widgets.Sized(70, 0, state), btn)
	row := container.NewBorder(nil, nil, widgets.Sized(54, 0, name), right, mid)
	return widgets.NewPanel(fill, style.Border, 8, row)
}

func (p *configPage) rebuildRight() {
	p.right.Objects = nil
	if p.editSlot == 0 {
		hint := widget.NewLabel(p.tx.T("config.pickHint"))
		hint.Wrapping = fyne.TextWrapWord
		p.right.Add(widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(
			widgets.NewText(p.tx.T("config.noneSelected"), style.Text, 14).Bold(), hint)))
	} else {
		p.form = newSlotForm(p, p.editSlot)
		p.right.Add(widgets.NewPanel(style.Surface, style.Border, 12, p.form.obj))
	}
	p.right.Refresh()
}

// --- slot form ------------------------------------------------------------------------

type slotForm struct {
	p    *configPage
	slot int
	tx   texts.Texts
	cam  *app.CameraView
	obj  fyne.CanvasObject

	brand, model           *widget.Select
	name, host, port, user *widget.Entry
	pass                   *widget.Entry

	userBox, passBox fyne.CanvasObject
	powerZoom        *widget.Check
	powerZoomBox     fyne.CanvasObject
	addrBox          fyne.CanvasObject // host and port, hidden for the demo camera
	userLabel        *widgets.Text
	pwHint           *widgets.Text
	clearBtn         *widget.Button
	notes            *widget.Label
	support          *widgets.Text
	supportHelp      *widget.Label
	errTxt           *widgets.Text
	title            *widgets.Text

	models  []core.ModelInfo // models of the selected brand
	current *core.ModelInfo
	clearPw bool
}

func fieldBlock(label string, in fyne.CanvasObject) fyne.CanvasObject {
	return container.NewVBox(widgets.NewText(label, style.TextMuted, 12), in)
}

func newSlotForm(p *configPage, slot int) *slotForm {
	f := &slotForm{p: p, slot: slot, cam: p.ctl.Camera(slot), tx: p.tx}
	all := p.ctl.Models()
	var brands []string
	seen := map[string]bool{}
	for _, m := range all {
		if !seen[m.Brand] {
			seen[m.Brand] = true
			brands = append(brands, m.Brand)
		}
	}

	titleKey := "config.title.assign"
	if f.cam != nil {
		titleKey = "config.title.edit"
	}
	f.title = widgets.NewText(f.tx.T(titleKey, "slot", slot), style.Text, 16).Bold()

	f.brand = widget.NewSelect(brands, nil)
	f.brand.PlaceHolder = f.tx.T("config.pickBrand")
	f.model = widget.NewSelect(nil, nil)
	f.model.PlaceHolder = f.tx.T("config.pickModel")
	f.name = widget.NewEntry()
	f.name.PlaceHolder = fmt.Sprintf("Cam%d", slot)
	f.host = widget.NewEntry()
	f.host.PlaceHolder = f.tx.T("config.hostHint")
	f.port = widget.NewEntry()
	f.port.PlaceHolder = "80"
	f.user = widget.NewEntry()
	f.pass = widget.NewPasswordEntry()
	f.pass.PlaceHolder = f.tx.T("config.password")

	f.userLabel = widgets.NewText(f.tx.T("config.user.optional"), style.TextMuted, 12)
	f.userBox = container.NewVBox(f.userLabel, f.user)
	f.pwHint = widgets.NewText("", style.TextMuted, 11)
	f.clearBtn = widget.NewButton(f.tx.T("config.pw.clear"), func() {
		f.clearPw = true
		f.pass.SetText("")
		f.updatePwHint()
	})
	f.clearBtn.Importance = widget.LowImportance
	f.passBox = container.NewVBox(widgets.NewText(f.tx.T("config.password"), style.TextMuted, 12), f.pass, f.pwHint, container.NewHBox(f.clearBtn))
	f.pass.OnChanged = func(string) {
		if f.pass.Text != "" {
			f.clearPw = false
		}
		f.updatePwHint()
	}

	f.addrBox = container.NewGridWithColumns(2, fieldBlock(f.tx.T("config.host"), f.host), fieldBlock(f.tx.T("config.port"), f.port))
	f.powerZoom = widget.NewCheck(f.tx.T("config.powerZoom"), nil)
	pzHint := widget.NewLabel(f.tx.T("config.powerZoomHint"))
	pzHint.Wrapping = fyne.TextWrapWord
	pzHint.Importance = widget.LowImportance
	f.powerZoomBox = container.NewVBox(
		container.NewHBox(widgets.NewIcon(icons.ZoomIn, style.TextMuted, 16), f.powerZoom), pzHint)

	f.notes = widget.NewLabel("")
	f.notes.Wrapping = fyne.TextWrapWord
	f.support = widgets.NewText("", style.Text, 12).Bold()
	f.supportHelp = widget.NewLabel("")
	f.supportHelp.Wrapping = fyne.TextWrapWord
	f.errTxt = widgets.NewText("", style.Danger, 12).Bold()

	f.brand.OnChanged = func(b string) { f.setBrand(b, "") }
	f.model.OnChanged = func(label string) {
		for i := range f.models {
			if f.models[i].Name == label {
				f.setModel(&f.models[i])
				return
			}
		}
	}

	save := widget.NewButton(f.tx.T("config.save"), f.save)
	save.Importance = widget.HighImportance
	cancel := widget.NewButton(f.tx.T("config.cancel"), func() { p.Edit(0) })
	buttons := []fyne.CanvasObject{save, cancel}
	if f.cam != nil {
		rm := widget.NewButton(f.tx.T("config.remove"), f.remove)
		rm.Importance = widget.DangerImportance
		buttons = append(buttons, layout.NewSpacer(), rm)
	}

	f.obj = container.NewVBox(spaced([]fyne.CanvasObject{
		f.title,
		widgets.Sized(0, 1, widgets.NewHairline()),
		container.NewGridWithColumns(2, fieldBlock(f.tx.T("config.brand"), f.brand), fieldBlock(f.tx.T("config.model"), f.model)),
		container.NewVBox(container.NewHBox(f.support), f.notes, f.supportHelp),
		fieldBlock(f.tx.T("config.name"), f.name),
		f.addrBox,
		container.NewGridWithColumns(2, f.userBox, f.passBox),
		f.powerZoomBox,
		f.errTxt,
		container.NewHBox(buttons...),
	})...)

	if f.cam != nil {
		if m, ok := f.lookup(f.cam.Model); ok {
			f.brand.SetSelected(m.Brand)
			f.setBrand(m.Brand, m.Name)
		}
		f.name.SetText(f.cam.Name)
		f.host.SetText(f.cam.Host)
		if f.current != nil && f.cam.Port != f.current.DefaultPort {
			f.port.SetText(strconv.Itoa(f.cam.Port))
		}
		f.user.SetText(f.cam.Username)
		f.powerZoom.SetChecked(f.cam.PowerZoom)
	} else {
		f.updateAuth()
	}
	f.updateAuth()
	return f
}

func (f *slotForm) lookup(id core.ModelID) (core.ModelInfo, bool) {
	for _, m := range f.p.ctl.Models() {
		if m.ID == id {
			return m, true
		}
	}
	return core.ModelInfo{}, false
}

// setBrand fills the model list of a brand; pick selects a model by name.
func (f *slotForm) setBrand(brand, pick string) {
	f.models = f.models[:0]
	var names []string
	for _, m := range f.p.ctl.Models() {
		if m.Brand == brand {
			f.models = append(f.models, m)
			names = append(names, m.Name)
		}
	}
	f.model.Options = names
	f.model.ClearSelected()
	f.current = nil
	f.model.Refresh()
	if pick != "" {
		f.model.SetSelected(pick)
	} else if len(names) == 1 {
		f.model.SetSelected(names[0])
	}
	f.updateAuth()
}

func (f *slotForm) setModel(m *core.ModelInfo) {
	f.current = m
	f.port.PlaceHolder = strconv.Itoa(m.DefaultPort)
	f.port.Refresh()
	f.notes.SetText(m.Notes)
	f.support.SetText(f.tx.T("support.line", "level", f.tx.SupportLabel(m.Support)))
	switch m.Support {
	case core.SupportSupported:
		f.support.SetToken(style.OK)
	case core.SupportExperimental:
		f.support.SetToken(style.Warn)
	default:
		f.support.SetToken(style.TextMuted)
	}
	f.supportHelp.SetText(f.tx.SupportHelp(m.Support))
	f.updateAuth()
}

// updateAuth shows the credential fields only when the model uses them, and
// the power zoom box only for a model with an interchangeable lens.
func (f *slotForm) updateAuth() {
	if f.current != nil && f.current.NoAddress {
		f.addrBox.Hide()
	} else {
		f.addrBox.Show()
	}
	if f.current != nil && f.current.InterchangeableLens {
		f.powerZoomBox.Show()
	} else {
		f.powerZoomBox.Hide()
	}
	auth := core.AuthNone
	if f.current != nil {
		auth = f.current.Auth
	}
	show := auth != core.AuthNone
	if show {
		f.userBox.Show()
		f.passBox.Show()
	} else {
		f.userBox.Hide()
		f.passBox.Hide()
	}
	if auth == core.AuthRequired {
		f.userLabel.SetText(f.tx.T("config.user.required"))
	} else {
		f.userLabel.SetText(f.tx.T("config.user.optional"))
	}
	f.updatePwHint()
}

func (f *slotForm) updatePwHint() {
	switch {
	case f.clearPw:
		f.pwHint.SetText(f.tx.T("config.pw.willClear"))
		f.clearBtn.Hide()
	case f.cam != nil && f.cam.HasPassword && f.pass.Text == "":
		f.pwHint.SetText(f.tx.T("config.pw.stored"))
		f.clearBtn.Show()
	default:
		f.pwHint.SetText(f.tx.T("config.pw.elsewhere"))
		f.clearBtn.Hide()
	}
}

// input builds the SlotInput or returns a French validation message.
func (f *slotForm) input() (app.SlotInput, string) {
	var in app.SlotInput
	if f.current == nil {
		return in, f.tx.T("config.needModel")
	}
	m := f.current
	host := strings.TrimSpace(f.host.Text)
	if m.NoAddress {
		host, f.port.Text = "", ""
	} else if host == "" {
		return in, f.tx.T("config.needHost")
	}
	port := 0
	if t := strings.TrimSpace(f.port.Text); t != "" {
		n, err := strconv.Atoi(t)
		if err != nil {
			return in, f.tx.T("config.portNumber")
		}
		port = n
	}
	in = app.SlotInput{
		Model: m.ID, Name: strings.TrimSpace(f.name.Text), Host: host, Port: port,
		PowerZoom: m.InterchangeableLens && f.powerZoom.Checked,
	}
	if m.Auth != core.AuthNone {
		in.Username = strings.TrimSpace(f.user.Text)
		switch {
		case f.pass.Text != "":
			pw := f.pass.Text
			in.Password = &pw
		case f.clearPw:
			empty := ""
			in.Password = &empty
		}
	} else if f.cam != nil && f.cam.HasPassword {
		empty := ""
		in.Password = &empty
	}
	return in, ""
}

func (f *slotForm) save() {
	in, msg := f.input()
	if msg != "" {
		f.errTxt.SetText(msg)
		return
	}
	if err := f.p.ctl.Assign(f.slot, in); err != nil {
		f.errTxt.SetText(f.tx.ConfigError(err))
		return
	}
	f.errTxt.SetText("")
	f.p.ctl.SetNotice(texts.Notice{Text: f.tx.T("config.saved", "slot", f.slot), Level: texts.LevelInfo})
	f.p.Edit(0)
}

func (f *slotForm) remove() {
	showConfirm(f.p.win, f.tx.T("config.removeTitle", "slot", f.slot), f.tx.T("config.removeBody", "slot", f.slot),
		f.tx.T("config.removeConfirm"), f.tx.T("config.cancel"), func(ok bool) {
			if !ok {
				return
			}
			if err := f.p.ctl.Clear(f.slot); err != nil {
				f.errTxt.SetText(f.tx.ConfigError(err))
				return
			}
			f.p.ctl.SetNotice(texts.Notice{Text: f.tx.T("config.removed", "slot", f.slot), Level: texts.LevelInfo})
			f.p.Edit(0)
		})
}
