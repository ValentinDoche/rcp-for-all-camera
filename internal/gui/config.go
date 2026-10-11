package gui

import (
	"fmt"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/fr"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// configPage lists the 8 slots and edits one at a time, and holds the theme
// setting.
type configPage struct {
	ctl    *Controller
	win    fyne.Window
	mode   style.Mode
	setTh  func(style.Mode)
	obj    fyne.CanvasObject
	list   *fyne.Container
	right  *fyne.Container
	cancel func()

	editSlot int
	form     *slotForm
}

func newConfigPage(ctl *Controller, win fyne.Window, mode style.Mode, setTheme func(style.Mode)) *configPage {
	p := &configPage{ctl: ctl, win: win, mode: mode, setTh: setTheme}
	p.list = container.NewVBox()
	p.right = container.NewVBox()
	title := widgets.NewText("Emplacements", style.Text, 16).Bold()
	left := container.NewBorder(container.New(layout.NewCustomPaddedLayout(0, 6, 2, 0), title), nil, nil, nil,
		container.NewVScroll(p.list))
	p.obj = container.NewBorder(nil, nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 12), widgets.Sized(440, 0, left)), nil,
		container.New(layout.NewCustomPaddedLayout(0, 0, 0, 8), container.NewVScroll(p.right)))
	p.cancel = ctl.Subscribe(func(ch Change) {
		if ch.Kind == ChRig {
			p.rebuildList()
		}
	})
	p.rebuildList()
	p.rebuildRight()
	return p
}

// Close stops listening.
func (p *configPage) Close() {
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// Edit opens the form for a slot.
func (p *configPage) Edit(slot int) {
	p.editSlot = slot
	p.rebuildList()
	p.rebuildRight()
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
		mid = widgets.NewText("Emplacement libre", style.TextMuted, 13)
		btn = widget.NewButton("Assigner", func() { p.Edit(slot) })
	} else {
		title := cam.Name
		if title == "" {
			title = cam.ModelName
		}
		line2 := fmt.Sprintf("%s %s, %s:%d", cam.Brand, cam.ModelName, cam.Host, cam.Port)
		mid = container.NewVBox(
			widgets.NewText(title, style.Text, 13).Bold().Clip(),
			widgets.NewText(line2, style.TextMuted, 11).Clip(),
		)
		btn = widget.NewButton("Modifier", func() { p.Edit(slot) })
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
		state = widgets.NewText(fr.ConnLabel(cam.Status.State), tok, 11).Align(fyne.TextAlignTrailing)
	}
	right := container.NewHBox(widgets.Sized(70, 0, state), btn)
	row := container.NewBorder(nil, nil, widgets.Sized(54, 0, name), right, mid)
	return widgets.NewPanel(fill, style.Border, 8, row)
}

func (p *configPage) rebuildRight() {
	p.right.Objects = nil
	if p.editSlot == 0 {
		hint := widget.NewLabel("Choisissez un emplacement à gauche pour assigner ou modifier une caméra.")
		hint.Wrapping = fyne.TextWrapWord
		p.right.Add(widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(
			widgets.NewText("Aucun emplacement sélectionné", style.Text, 14).Bold(), hint)))
	} else {
		p.form = newSlotForm(p, p.editSlot)
		p.right.Add(widgets.NewPanel(style.Surface, style.Border, 12, p.form.obj))
	}
	p.right.Add(widgets.NewPanel(style.Surface, style.Border, 12, p.appearance()))
	p.right.Refresh()
}

func (p *configPage) appearance() fyne.CanvasObject {
	modes := []style.Mode{style.ModeSystem, style.ModeLight, style.ModeDark}
	seg := widgets.NewSegmented([]string{"Système", "Clair", "Sombre"}, func(i int) {
		p.mode = modes[i]
		if p.setTh != nil {
			p.setTh(modes[i])
		}
	})
	for i, m := range modes {
		if m == p.mode {
			seg.SetSelected(i)
		}
	}
	return container.NewVBox(
		widgets.NewText("Apparence", style.Text, 14).Bold(),
		widgets.Sized(0, 1, widgets.NewHairline()),
		widgets.NewText("Thème de l'application", style.TextMuted, 12),
		container.NewHBox(seg),
	)
}

// --- slot form ------------------------------------------------------------------------

type slotForm struct {
	p    *configPage
	slot int
	cam  *app.CameraView
	obj  fyne.CanvasObject

	brand, model           *widget.Select
	name, host, port, user *widget.Entry
	pass                   *widget.Entry

	userBox, passBox fyne.CanvasObject
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
	f := &slotForm{p: p, slot: slot, cam: p.ctl.Camera(slot)}
	all := p.ctl.Models()
	var brands []string
	seen := map[string]bool{}
	for _, m := range all {
		if !seen[m.Brand] {
			seen[m.Brand] = true
			brands = append(brands, m.Brand)
		}
	}

	verb := "Assigner une caméra"
	if f.cam != nil {
		verb = "Modifier la caméra"
	}
	f.title = widgets.NewText(fmt.Sprintf("Cam%d : %s", slot, strings.ToLower(verb[:1])+verb[1:]), style.Text, 16).Bold()

	f.brand = widget.NewSelect(brands, nil)
	f.brand.PlaceHolder = "Choisir une marque"
	f.model = widget.NewSelect(nil, nil)
	f.model.PlaceHolder = "Choisir un modèle"
	f.name = widget.NewEntry()
	f.name.PlaceHolder = fmt.Sprintf("Cam%d", slot)
	f.host = widget.NewEntry()
	f.host.PlaceHolder = "192.168.1.20 ou camera.local"
	f.port = widget.NewEntry()
	f.port.PlaceHolder = "80"
	f.user = widget.NewEntry()
	f.pass = widget.NewPasswordEntry()
	f.pass.PlaceHolder = "Mot de passe"

	f.userLabel = widgets.NewText("Nom d'utilisateur", style.TextMuted, 12)
	f.userBox = container.NewVBox(f.userLabel, f.user)
	f.pwHint = widgets.NewText("", style.TextMuted, 11)
	f.clearBtn = widget.NewButton("Effacer le mot de passe enregistré", func() {
		f.clearPw = true
		f.pass.SetText("")
		f.updatePwHint()
	})
	f.clearBtn.Importance = widget.LowImportance
	f.passBox = container.NewVBox(widgets.NewText("Mot de passe", style.TextMuted, 12), f.pass, f.pwHint, container.NewHBox(f.clearBtn))
	f.pass.OnChanged = func(string) {
		if f.pass.Text != "" {
			f.clearPw = false
		}
		f.updatePwHint()
	}

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

	save := widget.NewButton("Enregistrer", f.save)
	save.Importance = widget.HighImportance
	cancel := widget.NewButton("Annuler", func() { p.Edit(0) })
	buttons := []fyne.CanvasObject{save, cancel}
	if f.cam != nil {
		rm := widget.NewButton("Retirer la caméra", f.remove)
		rm.Importance = widget.DangerImportance
		buttons = append(buttons, layout.NewSpacer(), rm)
	}

	f.obj = container.NewVBox(spaced([]fyne.CanvasObject{
		f.title,
		widgets.Sized(0, 1, widgets.NewHairline()),
		container.NewGridWithColumns(2, fieldBlock("Marque", f.brand), fieldBlock("Modèle", f.model)),
		container.NewVBox(container.NewHBox(f.support), f.notes, f.supportHelp),
		fieldBlock("Nom affiché", f.name),
		container.NewGridWithColumns(2, fieldBlock("Adresse de la caméra", f.host), fieldBlock("Port", f.port)),
		container.NewGridWithColumns(2, f.userBox, f.passBox),
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
	f.support.SetText("Prise en charge : " + fr.SupportLabel(m.Support))
	switch m.Support {
	case core.SupportSupported:
		f.support.SetToken(style.OK)
	case core.SupportExperimental:
		f.support.SetToken(style.Warn)
	default:
		f.support.SetToken(style.TextMuted)
	}
	f.supportHelp.SetText(fr.SupportHelp(m.Support))
	f.updateAuth()
}

// updateAuth shows the credential fields only when the model uses them.
func (f *slotForm) updateAuth() {
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
		f.userLabel.SetText("Nom d'utilisateur (obligatoire)")
	} else {
		f.userLabel.SetText("Nom d'utilisateur (facultatif)")
	}
	f.updatePwHint()
}

func (f *slotForm) updatePwHint() {
	switch {
	case f.clearPw:
		f.pwHint.SetText("Le mot de passe enregistré sera effacé.")
		f.clearBtn.Hide()
	case f.cam != nil && f.cam.HasPassword && f.pass.Text == "":
		f.pwHint.SetText("Mot de passe enregistré. Laissez vide pour le conserver.")
		f.clearBtn.Show()
	default:
		f.pwHint.SetText("Stocké hors du fichier de configuration.")
		f.clearBtn.Hide()
	}
}

// input builds the SlotInput or returns a French validation message.
func (f *slotForm) input() (app.SlotInput, string) {
	var in app.SlotInput
	if f.current == nil {
		return in, "Choisissez une marque et un modèle."
	}
	m := f.current
	host := strings.TrimSpace(f.host.Text)
	if host == "" {
		return in, "L'adresse de la caméra est obligatoire."
	}
	port := 0
	if t := strings.TrimSpace(f.port.Text); t != "" {
		n, err := strconv.Atoi(t)
		if err != nil {
			return in, "Le port doit être un nombre."
		}
		port = n
	}
	in = app.SlotInput{
		Model: m.ID, Name: strings.TrimSpace(f.name.Text), Host: host, Port: port,
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
		f.errTxt.SetText(fr.ConfigError(err))
		return
	}
	f.errTxt.SetText("")
	f.p.ctl.SetNotice(fr.Notice{Text: fmt.Sprintf("Cam%d enregistrée", f.slot), Level: fr.LevelInfo})
	f.p.Edit(0)
}

func (f *slotForm) remove() {
	body := widget.NewLabel(fmt.Sprintf("La caméra de Cam%d sera retirée de la configuration, son mot de passe enregistré sera effacé.", f.slot))
	body.Wrapping = fyne.TextWrapWord
	d := dialog.NewCustomConfirm(fmt.Sprintf("Retirer Cam%d", f.slot), "Retirer", "Annuler",
		widgets.Sized(360, 0, body), func(ok bool) {
			if !ok {
				return
			}
			if err := f.p.ctl.Clear(f.slot); err != nil {
				f.errTxt.SetText(fr.ConfigError(err))
				return
			}
			f.p.ctl.SetNotice(fr.Notice{Text: fmt.Sprintf("Cam%d retirée", f.slot), Level: fr.LevelInfo})
			f.p.Edit(0)
		}, f.p.win)
	d.Show()
}
