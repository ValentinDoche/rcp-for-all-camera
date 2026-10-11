package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

const stripWidth = 300

// stripActions is what a strip can ask the app to do.
type stripActions struct {
	Details func(slot int) // nil hides the Détails button
	Assign  func(slot int)
}

// strip is the dense control column of one camera (header, color wheel,
// readouts, iris pad, zoom and focus, quick actions).
type strip struct {
	ctl     *Controller
	slot    int
	actions stripActions

	root    *widgets.Panel
	stack   *fyne.Container
	cancel  func()
	content *stripContent
	sig     string
	empty   fyne.CanvasObject

	// header
	head       *widgets.Panel
	slotTxt    *widgets.Text
	stateTxt   *widgets.Text
	nameTxt    *widgets.Text
	headerMode int // 0 normal, 1 active, 2 on air
}

// stripContent holds the widgets of the control part, rebuilt whenever the
// camera or its declared controls change.
type stripContent struct {
	obj fyne.CanvasObject

	selector *widgets.Segmented
	groupMap []int // selector index -> wheelGroups index
	wheel    *wheelBlock

	summary []summaryCell
	pad     *widgets.IrisPad
	meter   *widgets.LevelMeter
	iris    *widgets.Text // the f-stop above the pad
	zoom    *widgets.Slider
	focus   *widgets.Slider
	quiet   bool
}

type summaryCell struct {
	spec core.ControlSpec
	val  *widgets.Text
}

func newStrip(ctl *Controller, slot int, act stripActions) *strip {
	s := &strip{ctl: ctl, slot: slot, actions: act}
	s.slotTxt = widgets.NewText(fmt.Sprintf("Cam%d", slot), style.Text, 16).Bold()
	s.stateTxt = widgets.NewText("", style.TextMuted, 12).Align(fyne.TextAlignTrailing)
	s.nameTxt = widgets.NewText("", style.TextMuted, 12).Clip()
	s.head = widgets.NewPanel(style.Raised, "", 8, container.NewVBox(
		container.NewBorder(nil, nil, s.slotTxt, s.stateTxt),
		s.nameTxt,
	))
	s.stack = container.NewStack()
	s.root = widgets.NewPanel(style.Surface, style.Border, 6, container.NewBorder(s.head, nil, nil, nil, s.stack))
	s.cancel = ctl.Subscribe(s.onChange)
	s.refresh(true)
	return s
}

// Object is the widget to place in a layout.
func (s *strip) Object() fyne.CanvasObject { return widgets.Sized(stripWidth, 0, s.root) }

// Close stops listening to the controller.
func (s *strip) Close() {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
}

func (s *strip) onChange(ch Change) {
	switch ch.Kind {
	case ChRig:
		s.refresh(false)
	case ChSlot:
		if ch.Slot == s.slot {
			s.refresh(false)
		}
	case ChValue, ChLens:
		if ch.Slot == s.slot && s.content != nil {
			s.applyContent()
		}
	}
}

// signature identifies the layout of the content: same camera and same set of
// declared controls means the widgets can be reused.
func (s *strip) signature() string {
	cam := s.ctl.Camera(s.slot)
	if cam == nil || !s.ctl.Loaded(s.slot) {
		return ""
	}
	var b strings.Builder
	b.WriteString(string(cam.Model))
	for _, sp := range s.ctl.Specs(s.slot) {
		b.WriteString("," + string(sp.ID))
	}
	return b.String()
}

func (s *strip) refresh(force bool) {
	cam := s.ctl.Camera(s.slot)
	s.applyHeader(cam)
	switch {
	case cam == nil:
		s.content, s.sig = nil, ""
		s.stack.Objects = []fyne.CanvasObject{s.emptyView()}
		s.stack.Refresh()
	default:
		sig := s.signature()
		if sig == "" {
			// camera known, controls still loading
			if s.sig != "" || force || s.content == nil {
				s.content, s.sig = nil, ""
				s.stack.Objects = []fyne.CanvasObject{s.loadingView()}
				s.stack.Refresh()
			}
			return
		}
		if sig != s.sig || s.content == nil {
			s.content = s.buildContent()
			s.sig = sig
			s.stack.Objects = []fyne.CanvasObject{s.content.obj}
			s.stack.Refresh()
		}
		s.applyContent()
	}
}

// --- header --------------------------------------------------------------------

func (s *strip) applyHeader(cam *app.CameraView) {
	live := s.onAir()
	mode := 0
	switch {
	case live:
		mode = 2
	case s.ctl.Active() == s.slot:
		mode = 1
	}
	fill, main, sub := style.Raised, style.Text, style.TextMuted
	switch mode {
	case 1:
		fill, main, sub = style.Accent, style.OnAccent, style.OnAccent
	case 2:
		fill, main, sub = style.Tally, style.OnTally, style.OnTally
	}
	s.head.SetFill(fill)
	s.slotTxt.SetToken(main)
	s.nameTxt.SetToken(sub)
	if cam == nil {
		s.stateTxt.SetText(s.ctl.tx.T("strip.free"))
		s.stateTxt.SetToken(sub)
		s.nameTxt.SetText(s.ctl.tx.T("strip.noCamera"))
		return
	}
	text := s.ctl.tx.ConnLabel(cam.Status.State)
	tok := sub
	if mode == 0 {
		switch cam.Status.State {
		case core.StateConnected:
			tok = style.OK
		case core.StateError:
			tok = style.Danger
		}
	}
	if live {
		text = s.ctl.tx.T("strip.onAir")
	}
	s.stateTxt.SetText(text)
	s.stateTxt.SetToken(tok)
	name := cam.Name
	switch {
	case name == "":
		name = cam.ModelName
	case name != cam.ModelName:
		name += " - " + cam.ModelName
	}
	s.nameTxt.SetText(name)
}

func (s *strip) onAir() bool {
	v := s.ctl.Value(s.slot, core.Tally)
	return v != nil && v.On
}

// --- empty and loading views --------------------------------------------------------

func (s *strip) emptyView() fyne.CanvasObject {
	if s.empty != nil {
		return s.empty
	}
	title := widgets.NewText(s.ctl.tx.T("strip.emptyTitle"), style.Text, 14).Bold().Align(fyne.TextAlignCenter)
	hint := widget.NewLabel(s.ctl.tx.T("strip.emptyHint"))
	hint.Wrapping = fyne.TextWrapWord
	hint.Alignment = fyne.TextAlignCenter
	btn := widget.NewButton(s.ctl.tx.T("strip.assign"), func() {
		if s.actions.Assign != nil {
			s.actions.Assign(s.slot)
		}
	})
	btn.Importance = widget.HighImportance
	s.empty = container.NewCenter(container.NewVBox(title, hint, container.NewCenter(btn)))
	return s.empty
}

func (s *strip) loadingView() fyne.CanvasObject {
	return container.NewCenter(widgets.NewText(s.ctl.tx.T("strip.reading"), style.TextMuted, 13))
}

// --- content ---------------------------------------------------------------------------

func (s *strip) buildContent() *stripContent {
	c := &stripContent{}
	slot, ctl := s.slot, s.ctl
	has := func(id core.ControlID) bool { return ctl.Has(slot, id) }

	var top []fyne.CanvasObject

	// Lift / Gamma / Gain selector and wheel
	var avail []int
	var titles []string
	for g := range wheelGroups {
		if wheelAvailable(ctl, slot, g) {
			avail = append(avail, g)
			titles = append(titles, ctl.tx.T(wheelGroups[g].Key))
		}
	}
	if len(avail) > 0 {
		c.groupMap = avail
		c.wheel = newWheelBlock(ctl, slot, 168)
		c.wheel.setGroup(avail[0])
		c.selector = widgets.NewSegmented(titles, func(i int) {
			c.wheel.setGroup(c.groupMap[i])
		})
		c.selector.SetSelected(0)
		top = append(top, container.New(layout.NewCustomPaddedLayout(6, 2, 4, 4), c.selector), c.wheel.obj)
	}

	// ND / GAIN / OBTURATEUR / BALANCE
	var cells []fyne.CanvasObject
	addCell := func(caption string, id core.ControlID) {
		sp, ok := ctl.Spec(slot, id)
		if !ok {
			return
		}
		val := widgets.NewText(texts.Missing, style.Text, 13).Bold().Mono().Align(fyne.TextAlignCenter)
		c.summary = append(c.summary, summaryCell{spec: sp, val: val})
		cells = append(cells, container.NewVBox(
			widgets.NewText(caption, style.TextMuted, 10).Align(fyne.TextAlignCenter), val))
	}
	if has(core.ND) {
		addCell(ctl.tx.T("strip.nd"), core.ND)
	}
	switch {
	case has(core.Gain):
		addCell(ctl.tx.T("strip.gain"), core.Gain)
	case has(core.ISO):
		addCell(ctl.tx.T("strip.iso"), core.ISO)
	}
	switch {
	case has(core.ShutterSpeed):
		addCell(ctl.tx.T("strip.shutter"), core.ShutterSpeed)
	case has(core.ShutterAngle):
		addCell(ctl.tx.T("strip.shutter"), core.ShutterAngle)
	}
	if has(core.WhiteBalance) {
		addCell(ctl.tx.T("strip.balance"), core.WhiteBalance)
	}
	if len(cells) > 0 {
		top = append(top, container.New(layout.NewCustomPaddedLayout(6, 0, 0, 0),
			widgets.NewPanel(style.Inset, style.Border, 4, container.NewGridWithColumns(len(cells), cells...))))
	}

	// iris pad, meter, zoom and focus
	hasIris, hasMB := has(core.Iris), has(core.MasterBlack)
	hasZoom, hasFocus := has(core.Zoom), has(core.Focus)
	var center fyne.CanvasObject = layout.NewSpacer()
	if hasIris || hasMB {
		c.pad = widgets.NewIrisPad(fyne.NewSize(120, 110))
		c.pad.SetAxes(hasMB, hasIris)
		c.pad.OnChanged = func(x, y float64, _, _ bool) { s.onPad(c, x, y) }
		center = c.pad
	}
	row := container.NewBorder(nil, nil, nil, nil, center)
	if hasIris {
		c.meter = widgets.NewLevelMeter(20, fyne.NewSize(12, 100))
		row = container.NewBorder(nil, nil,
			container.New(layout.NewCustomPaddedLayout(14, 14, 0, 4), c.meter), nil, center)
	}
	var side []fyne.CanvasObject
	if hasZoom {
		c.zoom = s.vslider(c, core.Zoom)
		side = append(side, labeledVertical(ctl.tx.T("strip.zoom"), c.zoom))
	}
	if hasFocus {
		c.focus = s.vslider(c, core.Focus)
		side = append(side, labeledVertical(ctl.tx.T("strip.focus"), c.focus))
	}
	if len(side) > 0 {
		row = container.NewBorder(nil, nil, nil, container.NewHBox(side...), row)
	}
	if hasIris {
		c.iris = widgets.NewText(texts.Missing, style.Text, 15).Bold().Mono()
		row = container.NewBorder(container.NewHBox(
			widgets.NewText(ctl.tx.T("strip.iris"), style.TextMuted, 10), c.iris), nil, nil, nil, row)
	}
	padArea := container.New(layout.NewCustomPaddedLayout(6, 0, 0, 0), row)

	// quick actions
	var acts []fyne.CanvasObject
	for _, a := range []struct {
		id    core.ControlID
		label string
	}{{core.AutoFocus, ctl.tx.T("strip.autoFocus")}, {core.AutoIris, ctl.tx.T("strip.autoIris")}, {core.AutoWhiteBalance, ctl.tx.T("strip.awb")}} {
		if !has(a.id) {
			continue
		}
		id := a.id
		acts = append(acts, widget.NewButton(a.label, func() { ctl.Write(slot, id, core.Trigger()) }))
	}
	var bottom []fyne.CanvasObject
	if len(acts) > 0 {
		bottom = append(bottom, container.NewGridWithColumns(len(acts), acts...))
	}
	if s.actions.Details != nil {
		d := widget.NewButton(ctl.tx.T("strip.details"), func() { s.actions.Details(slot) })
		d.Importance = widget.MediumImportance
		bottom = append(bottom, d)
	}

	c.obj = container.NewBorder(container.NewVBox(top...), container.NewVBox(bottom...), nil, nil, padArea)
	return c
}

// vslider builds a vertical zoom or focus slider that writes through the
// controller.
func (s *strip) vslider(c *stripContent, id core.ControlID) *widgets.Slider {
	sp, _ := s.ctl.Spec(s.slot, id)
	sl := widgets.NewSlider(sp.Min, sp.Max)
	sl.Vertical = true
	sl.Step = stepOf(sp)
	sl.OnChanged = func(v float64) {
		s.ctl.Write(s.slot, id, core.Number(widgets.Quantize(v, widgets.RangeOf(sp))))
	}
	return sl
}

func labeledVertical(caption string, sl *widgets.Slider) fyne.CanvasObject {
	return widgets.Sized(38, 0, container.NewBorder(
		widgets.NewText(caption, style.TextMuted, 10).Align(fyne.TextAlignCenter), nil, nil, nil, sl))
}

// irisScale is the iris scale of the slot's camera and lens.
func (s *strip) irisScale() (widgets.IrisScale, bool) {
	sp, ok := s.ctl.Spec(s.slot, core.Iris)
	if !ok {
		return widgets.IrisScale{}, false
	}
	return widgets.NewIrisScale(sp, s.ctl.Lens(s.slot)), true
}

// onPad: up opens the iris, right raises the master black.
func (s *strip) onPad(c *stripContent, x, y float64) {
	if k, ok := s.irisScale(); ok {
		v := k.At((y + 1) / 2)
		s.ctl.Write(s.slot, core.Iris, core.Number(v))
		if c.meter != nil {
			c.meter.SetLevel(k.Open(v), true)
		}
	}
	var mbR *widgets.Range
	if sp, ok := s.ctl.Spec(s.slot, core.MasterBlack); ok {
		r := widgets.RangeOf(sp)
		mbR = &r
	}
	_, mv := widgets.PadToValues(x, y, nil, mbR)
	if mv != nil {
		s.ctl.Write(s.slot, core.MasterBlack, core.Number(*mv))
	}
}

// applyContent pushes the controller's values into the widgets.
func (s *strip) applyContent() {
	c := s.content
	if c == nil {
		return
	}
	cam := s.ctl.Camera(s.slot)
	if cam != nil {
		s.applyHeader(cam)
	}
	c.quiet = true
	defer func() { c.quiet = false }()
	if c.wheel != nil {
		c.wheel.apply()
	}
	for _, cell := range c.summary {
		cell.val.SetText(texts.ShortLabel(s.ctl.tx.FormatValue(cell.spec, s.ctl.Value(s.slot, cell.spec.ID))))
	}
	if c.pad != nil {
		c.pad.SetActive(s.ctl.Active() == s.slot)
		var mbR *widgets.Range
		var mbV float64
		if sp, ok := s.ctl.Spec(s.slot, core.MasterBlack); ok {
			r := widgets.RangeOf(sp)
			mbR = &r
			if v := s.ctl.Value(s.slot, core.MasterBlack); v != nil {
				mbV = v.Number
			} else {
				mbV = (sp.Min + sp.Max) / 2
			}
		}
		x, _ := widgets.ValuesToPad(mbR, nil, mbV, 0)
		y := 0.0
		if k, ok := s.irisScale(); ok {
			if v := s.ctl.Value(s.slot, core.Iris); v != nil {
				y = k.Open(v.Number)*2 - 1
			}
		}
		c.pad.SetPos(x, y)
	}
	if c.iris != nil {
		if sp, ok := s.ctl.Spec(s.slot, core.Iris); ok {
			c.iris.SetText(s.ctl.tx.FormatValue(sp, s.ctl.Value(s.slot, core.Iris)))
		}
	}
	if c.meter != nil {
		if k, ok := s.irisScale(); ok {
			if v := s.ctl.Value(s.slot, core.Iris); v != nil {
				c.meter.SetLevel(k.Open(v.Number), true)
			} else {
				c.meter.SetLevel(0, false)
			}
		}
	}
	for _, p := range []struct {
		sl *widgets.Slider
		id core.ControlID
	}{{c.zoom, core.Zoom}, {c.focus, core.Focus}} {
		if p.sl == nil {
			continue
		}
		sp, _ := s.ctl.Spec(s.slot, p.id)
		if v := s.ctl.Value(s.slot, p.id); v != nil {
			p.sl.Set(v.Number)
		} else {
			p.sl.Set(sp.Min)
		}
	}
}
