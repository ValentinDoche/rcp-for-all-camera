package panelsim

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"sync/atomic"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

// Colours of the drawing.
var (
	colBody     = color.NRGBA{0x2b, 0x2b, 0x2e, 0xff}
	colFace     = color.NRGBA{0x1b, 0x1b, 0x1d, 0xff}
	colSilk     = color.NRGBA{0x9a, 0x9a, 0xa0, 0xff}
	colKeyOff   = color.NRGBA{0x3a, 0x3a, 0x3e, 0xff}
	colKeyText  = color.NRGBA{0xc8, 0xc8, 0xcc, 0xff}
	colKeyDark  = color.NRGBA{0x20, 0x20, 0x22, 0xff}
	colWhite    = color.NRGBA{0xf2, 0xf2, 0xf2, 0xff}
	colRed      = color.NRGBA{0xff, 0x3b, 0x30, 0xff}
	colGreen    = color.NRGBA{0x34, 0xc7, 0x59, 0xff}
	colBlue     = color.NRGBA{0x2f, 0x7b, 0xff, 0xff}
	colAmber    = color.NRGBA{0xff, 0xb0, 0x00, 0xff}
	colCamOff   = color.NRGBA{0x8a, 0x8a, 0x8e, 0xff}
	colLEDOff   = color.NRGBA{0x38, 0x38, 0x3a, 0xff}
	colSegOn    = color.NRGBA{0xf5, 0xf5, 0xf5, 0xff}
	colSegOff   = color.NRGBA{0x1c, 0x1c, 0x1d, 0xff}
	colKnob     = color.NRGBA{0x12, 0x12, 0x13, 0xff}
	colPressed  = color.NRGBA{0x60, 0x60, 0x66, 0xff}
	colRingR    = color.NRGBA{0xd0, 0x30, 0x30, 0xff}
	colRingG    = color.NRGBA{0x30, 0xb0, 0x50, 0xff}
	colRingB    = color.NRGBA{0x30, 0x90, 0xe0, 0xff}
	colRingNone = color.NRGBA{0x50, 0x50, 0x55, 0xff}
)

// camColours maps a camera button's RGB LED bits to a colour.
var camColours = map[micropanel.Colour]color.NRGBA{
	micropanel.Off:     colCamOff,
	micropanel.Red:     colRed,
	micropanel.Green:   colGreen,
	micropanel.Amber:   colAmber,
	micropanel.Blue:    colBlue,
	micropanel.Cyan:    {0x30, 0xd0, 0xe0, 0xff},
	micropanel.Magenta: {0xd0, 0x40, 0xd0, 0xff},
	micropanel.White:   {0xff, 0xff, 0xff, 0xff},
}

func pos(x, y float64) fyne.Position {
	return fyne.NewPos(float32((x-originX)*scale), float32(y*scale))
}

func size(w, h float64) fyne.Size { return fyne.NewSize(float32(w*scale), float32(h*scale)) }

// View draws the simulated panel and turns mouse input into panel input:
// click = hold a key, drag up/down or the mouse wheel = turn a knob, drag
// the lever head up/down = iris lever, left/right or the wheel over the
// lever = master black ring.
type View struct {
	widget.BaseWidget
	sim *Sim

	r     *viewRenderer
	held  micropanel.Key // key held by the mouse (0 = none)
	drag  target
	accum float64 // fractional drag counts
	dirty atomic.Bool
}

// target is what a mouse press or scroll acts on.
type target struct {
	kind  targetKind
	key   micropanel.Key
	index int // knob index
}

type targetKind int

const (
	targetNone targetKind = iota
	targetKey
	targetKnob
	targetLever
)

// NewView returns the panel drawing bound to s. It redraws itself when s
// changes.
func NewView(s *Sim) *View {
	v := &View{sim: s}
	v.ExtendBaseWidget(v)
	s.OnChange(func() {
		if !v.dirty.Swap(true) {
			fyne.Do(func() {
				v.dirty.Store(false)
				v.Refresh()
			})
		}
	})
	return v
}

// --- input -----------------------------------------------------------

func (v *View) hit(p fyne.Position) target {
	x := float64(p.X)/scale + originX
	y := float64(p.Y) / scale
	in := func(cx, cy, w, h float64) bool {
		return math.Abs(x-cx) <= w/2 && math.Abs(y-cy) <= h/2
	}
	if in(leverX, (leverTop+leverBottom)/2, 2*leverR+10, leverBottom-leverTop+2*leverR) {
		return target{kind: targetLever}
	}
	for _, k := range knobs {
		if math.Hypot(x-k.x, y-k.y) <= k.r+3 {
			return target{kind: targetKnob, index: k.index}
		}
	}
	for _, b := range buttons {
		if in(b.x, b.y, b.w, b.h) {
			return target{kind: targetKey, key: b.key}
		}
	}
	for _, c := range camButtons {
		if in(c.x+camW/2, c.y+camH/2, camW, camH) {
			return target{kind: targetKey, key: keyCam1 + micropanel.Key(c.n-1)}
		}
	}
	return target{}
}

// MouseDown holds a key, or starts turning a knob or moving the lever.
func (v *View) MouseDown(e *desktop.MouseEvent) {
	if e.Button != desktop.MouseButtonPrimary {
		return
	}
	v.release()
	v.drag, v.accum = v.hit(e.Position), 0
	if v.drag.kind == targetKey {
		v.held = v.drag.key
		v.sim.Press(v.held, true)
	}
}

// MouseUp releases the held key.
func (v *View) MouseUp(*desktop.MouseEvent) { v.release() }

func (v *View) release() {
	if v.held != 0 {
		v.sim.Press(v.held, false)
		v.held = 0
	}
}

// Dragged turns the knob or moves the lever picked by MouseDown.
func (v *View) Dragged(e *fyne.DragEvent) {
	switch v.drag.kind {
	case targetKnob:
		// Up = clockwise; 256 px for the whole 0..1024 travel.
		v.accum += -float64(e.Dragged.DY) * 4 / scale
		if n := int(v.accum); n != 0 {
			v.accum -= float64(n)
			v.sim.TurnKnob(v.drag.index, n)
		}
	case targetLever:
		in := v.sim.Input()
		span := (leverBottom - leverTop) * scale
		dl := -float64(e.Dragged.DY) / span * 2 * micropanel.JoystickMax
		dr := float64(e.Dragged.DX) / (60 * scale) * micropanel.JoystickMax
		v.sim.SetLever(in.Lever + int32(math.Round(dl)))
		if dr != 0 {
			v.sim.SetRing(in.Ring + int32(math.Round(dr)))
		}
	}
}

// DragEnd ends a drag (and releases a key dragged off).
func (v *View) DragEnd() {
	v.release()
	v.drag = target{}
}

// Scrolled turns the knob under the pointer, or the master black ring over
// the lever.
func (v *View) Scrolled(e *fyne.ScrollEvent) {
	dir := 0
	switch {
	case e.Scrolled.DY > 0:
		dir = 1
	case e.Scrolled.DY < 0:
		dir = -1
	}
	if dir == 0 {
		return
	}
	switch t := v.hit(e.Position); t.kind {
	case targetKnob:
		v.sim.TurnKnob(t.index, 8*dir)
	case targetLever:
		v.sim.SetRing(v.sim.Input().Ring + int32(64*dir))
	}
}

var (
	_ desktop.Mouseable = (*View)(nil)
	_ fyne.Draggable    = (*View)(nil)
	_ fyne.Scrollable   = (*View)(nil)
)

// --- drawing -----------------------------------------------------------

// CreateRenderer builds every canvas object once; Refresh only recolours
// and moves them.
func (v *View) CreateRenderer() fyne.WidgetRenderer {
	r := &viewRenderer{view: v, keys: map[micropanel.Key]*keyObj{}}
	r.build()
	v.r = r
	r.update()
	return r
}

type keyObj struct {
	b     button
	bg    *canvas.Rectangle
	texts []*canvas.Text
	glyph []*canvas.Line
}

type knobObj struct {
	k      knob
	needle *canvas.Line
}

type viewRenderer struct {
	view    *View
	objects []fyne.CanvasObject

	keys    map[micropanel.Key]*keyObj
	cams    [8]*canvas.Rectangle
	camText [8]*canvas.Text
	inds    []*canvas.Circle
	segs    [14][8]fyne.CanvasObject // a..g as rectangles, DP as a circle
	pixels  [micropanel.MatrixRows * micropanel.MatrixCols]*canvas.Circle
	irisBar [micropanel.LEDIrisBarCount]*canvas.Rectangle
	knobs   []knobObj
	head    *canvas.Circle
	ring    *canvas.Line
	readout *canvas.Text
}

func (r *viewRenderer) add(o ...fyne.CanvasObject) { r.objects = append(r.objects, o...) }

func (r *viewRenderer) rect(c color.Color, x, y, w, h, radius float64) *canvas.Rectangle {
	o := canvas.NewRectangle(c)
	o.CornerRadius = float32(radius * scale)
	o.Move(pos(x, y))
	o.Resize(size(w, h))
	r.add(o)
	return o
}

func (r *viewRenderer) circle(c color.Color, cx, cy, rad float64) *canvas.Circle {
	o := canvas.NewCircle(c)
	o.Move(pos(cx-rad, cy-rad))
	o.Resize(size(2*rad, 2*rad))
	r.add(o)
	return o
}

// text draws s centred on (cx, cy) with a size in drawing units.
func (r *viewRenderer) text(s string, c color.Color, cx, cy, sz float64, bold bool) *canvas.Text {
	o := canvas.NewText(s, c)
	o.TextSize = float32(sz * scale)
	o.TextStyle.Bold = bold
	o.Alignment = fyne.TextAlignCenter
	h := o.MinSize().Height
	o.Move(fyne.NewPos(pos(cx, cy).X-200, pos(cx, cy).Y-h/2))
	o.Resize(fyne.NewSize(400, h))
	r.add(o)
	return o
}

func (r *viewRenderer) line(c color.Color, x1, y1, x2, y2, width float64) *canvas.Line {
	o := canvas.NewLine(c)
	o.StrokeWidth = float32(width * scale)
	o.Position1, o.Position2 = pos(x1, y1), pos(x2, y2)
	r.add(o)
	return o
}

func (r *viewRenderer) build() {
	r.rect(colBody, originX, 0, bodyW, bodyH, 14)
	r.rect(colFace, 222, 24, 162, 364, 4)
	r.text("Blackmagicdesign", colSilk, 281, 424, 6, false)
	r.line(colKeyOff, 228, 137, 378, 137, 0.6)
	r.line(colKeyOff, 228, 233, 378, 233, 0.6)
	r.text("WHITE", colSilk, 283, 179, 3.6, false)
	r.text("BLACK/FLARE", colSilk, 290, 217, 3.6, false)

	for i, c := range camButtons {
		r.cams[i] = r.rect(colCamOff, c.x, c.y, camW, camH, 2)
		r.camText[i] = r.text(string(rune('0'+c.n)), colKeyDark, c.x+camW/2, c.y+camH/2, 6, true)
	}
	for _, b := range buttons {
		r.buildKey(b)
	}
	for _, in := range indicators {
		r.inds = append(r.inds, r.circle(colLEDOff, in.x, in.y, 1.6))
		t := r.text(in.label, colSilk, 0, in.y, 3.6, false)
		t.Alignment = fyne.TextAlignLeading
		t.Move(fyne.NewPos(pos(in.x+3, 0).X, t.Position().Y))
	}
	for _, g := range digitGroups {
		r.buildDigits(g)
	}
	r.buildMatrix()
	r.buildIris()
	for _, k := range knobs {
		r.buildKnob(k)
	}
	r.buildLever()
}

func (r *viewRenderer) buildKey(b button) {
	o := &keyObj{b: b}
	o.bg = r.rect(colKeyOff, b.x-b.w/2, b.y-b.h/2, b.w, b.h, 1.5)
	switch {
	case b.arrow != 0:
		// Outline triangle, pointing up or down.
		d := float64(b.arrow)
		top := [3][2]float64{{b.x, b.y - 3*d}, {b.x - 3.5, b.y + 2.5*d}, {b.x + 3.5, b.y + 2.5*d}}
		for i := range 3 {
			p, q := top[i], top[(i+1)%3]
			o.glyph = append(o.glyph, r.line(colKeyText, p[0], p[1], q[0], q[1], 0.6))
		}
	case b.label != "":
		lines := strings.Split(b.label, "\n")
		sz := 4.2
		if len(lines) > 1 {
			sz = 3.4
		}
		for i, l := range lines {
			cy := b.y + (float64(i)-float64(len(lines)-1)/2)*sz*1.15
			o.texts = append(o.texts, r.text(l, colKeyText, b.x, cy, sz, true))
		}
	}
	r.keys[b.key] = o
}

func (r *viewRenderer) buildDigits(g digitGroup) {
	total := float64(g.n)*(digitW+digitGap) - digitGap
	x0, y0 := g.cx-total/2, g.cy-digitH/2
	r.rect(color.Black, x0-2.5, y0-2, total+5, digitH+4, 1)
	for i := range g.n {
		x := x0 + float64(i)*(digitW+digitGap)
		t, half := 1.3, digitH/2
		s := [7][4]float64{
			{x + t, y0, digitW - 2*t, t},                       // a
			{x + digitW - t, y0 + t, t, half - 1.5*t},          // b
			{x + digitW - t, y0 + half + t/2, t, half - 1.5*t}, // c
			{x + t, y0 + digitH - t, digitW - 2*t, t},          // d
			{x, y0 + half + t/2, t, half - 1.5*t},              // e
			{x, y0 + t, t, half - 1.5*t},                       // f
			{x + t, y0 + half - t/2, digitW - 2*t, t},          // g
		}
		for j, q := range s {
			r.segs[g.first+i][j] = r.rect(colSegOff, q[0], q[1], q[2], q[3], 0.4)
		}
		r.segs[g.first+i][7] = r.circle(colSegOff, x+digitW+digitGap/2, y0+digitH-t/2, t/2+0.1)
	}
	r.text(g.label, colSilk, g.cx, g.cy+digitH/2+5, 3.6, false)
}

func (r *viewRenderer) buildMatrix() {
	w := float64(micropanel.MatrixCols-1)*matrixPitch + 6
	h := float64(micropanel.MatrixRows-1)*matrixPitch + 6
	r.rect(color.Black, matrixX-3, matrixY-3, w, h, 1)
	for row := range micropanel.MatrixRows {
		for col := range micropanel.MatrixCols {
			r.pixels[row*micropanel.MatrixCols+col] = r.circle(colSegOff,
				matrixX+float64(col)*matrixPitch, matrixY+float64(row)*matrixPitch, 1.7)
		}
	}
	r.text("CAMERA", colSilk, matrixX+w/2-3, matrixY+h+1, 3.6, false)
}

func (r *viewRenderer) buildIris() {
	for i := range micropanel.LEDIrisBarCount {
		y := irisBarY + float64(i)*irisBarPitch
		r.irisBar[i] = r.rect(colLEDOff, irisBarX, y, 6, irisBarPitch-1.6, 0.5)
		if l, ok := irisLabels[i]; ok {
			t := r.text(l, colSilk, 0, y+(irisBarPitch-1.6)/2, 3.4, false)
			t.Alignment = fyne.TextAlignTrailing
			t.Move(fyne.NewPos(pos(irisBarX-1.5, 0).X-400, t.Position().Y))
		}
	}
}

func (r *viewRenderer) buildKnob(k knob) {
	ring := colRingNone
	switch k.ring {
	case "r":
		ring = colRingR
	case "g":
		ring = colRingG
	case "b":
		ring = colRingB
	}
	if k.ring != "" {
		r.circle(ring, k.x, k.y, k.r+2)
	}
	body := r.circle(colKnob, k.x, k.y, k.r)
	body.StrokeColor, body.StrokeWidth = colRingNone, float32(0.5*scale)
	o := knobObj{k: k, needle: r.line(colWhite, k.x, k.y, k.x, k.y-k.r, 0.9)}
	r.knobs = append(r.knobs, o)
	if k.label != "" {
		r.text(k.label, colSilk, k.x, k.y+k.r+3.5, 3.4, false)
	}
}

func (r *viewRenderer) buildLever() {
	r.rect(colKeyDark, leverX-17, leverTop-leverR-3, 34, leverBottom-leverTop+2*leverR+6, 6)
	r.rect(color.Black, leverX-3, leverTop, 6, leverBottom-leverTop, 3)
	r.head = r.circle(colKnob, leverX, leverTop, leverR)
	r.head.StrokeColor, r.head.StrokeWidth = colRingNone, float32(2*scale)
	r.ring = r.line(colWhite, leverX, leverTop, leverX, leverTop-leverR, 1)
	r.readout = r.text("", colSilk, leverX, leverBottom+leverR+8, 3.4, false)
}

// update recolours everything from the sim.
func (r *viewRenderer) update() {
	out, in := r.view.sim.Output(), r.view.sim.Input()
	held := map[micropanel.Key]bool{}
	for _, k := range in.Keys {
		held[k] = true
	}

	for i := range r.cams {
		base := 16 + 3*i
		var c micropanel.Colour
		for ch := range 3 {
			if out.LED(base + ch) {
				c |= 1 << ch
			}
		}
		col := camColours[c]
		if held[keyCam1+micropanel.Key(i)] {
			col = darken(col)
		}
		setFill(r.cams[i], col)
	}
	for k, o := range r.keys {
		bg, fg := colKeyOff, colKeyText
		switch {
		case o.b.red != noLED && out.LED(o.b.red):
			bg, fg = colRed, colWhite
		case o.b.white != noLED && out.LED(o.b.white) && o.b.arrow != 0:
			fg = colWhite // arrows: the glyph lights up
			bg = color.NRGBA{0x55, 0x55, 0x5a, 0xff}
		case o.b.white != noLED && out.LED(o.b.white):
			bg, fg = colWhite, colKeyDark
		}
		if held[k] {
			bg = colPressed
		}
		setFill(o.bg, bg)
		for _, t := range o.texts {
			if t.Color != fg {
				t.Color = fg
				t.Refresh()
			}
		}
		for _, l := range o.glyph {
			if l.StrokeColor != fg {
				l.StrokeColor = fg
				l.Refresh()
			}
		}
	}
	for i, in := range indicators {
		c := colLEDOff
		if out.LED(in.led) {
			c = colWhite
			if in.alarm {
				c = colRed
			}
		}
		if r.inds[i].FillColor != c {
			r.inds[i].FillColor = c
			r.inds[i].Refresh()
		}
	}
	for d := range r.segs {
		for j := range 8 {
			on := out.Digits[d]&(0x80>>j) != 0
			c := colSegOff
			if on {
				c = colSegOn
			}
			switch o := r.segs[d][j].(type) {
			case *canvas.Rectangle:
				setFill(o, c)
			case *canvas.Circle:
				if o.FillColor != c {
					o.FillColor = c
					o.Refresh()
				}
			}
		}
	}
	for p, o := range r.pixels {
		c := colSegOff
		switch (out.Matrix[p>>2] >> (2 * (p & 3))) & 3 {
		case micropanel.PixRed:
			c = colRed
		case micropanel.PixGreen:
			c = colGreen
		case micropanel.PixAmber:
			c = colAmber
		}
		if o.FillColor != c {
			o.FillColor = c
			o.Refresh()
		}
	}
	for i, o := range r.irisBar {
		c := colLEDOff
		if out.LED(micropanel.LEDIrisBar0 + i) {
			c = colAmber
		}
		setFill(o, c)
	}
	for _, o := range r.knobs {
		// The counter (0..1024) maps to -150°..+150°, so a clamped knob shows.
		a := (float64(in.Analog[o.k.index])/micropanel.AnalogMax*300 - 150) * math.Pi / 180
		setLine(o.needle, o.k.x+math.Sin(a)*o.k.r*0.35, o.k.y-math.Cos(a)*o.k.r*0.35,
			o.k.x+math.Sin(a)*o.k.r*0.95, o.k.y-math.Cos(a)*o.k.r*0.95)
	}
	ly := leverTop + (1-(float64(in.Lever)/micropanel.JoystickMax+1)/2)*(leverBottom-leverTop)
	if p := pos(leverX-leverR, ly-leverR); r.head.Position1 != p {
		r.head.Move(p)
		r.head.Refresh()
	}
	a := float64(in.Ring) / micropanel.JoystickMax * 135 * math.Pi / 180
	setLine(r.ring, leverX+math.Sin(a)*leverR*0.3, ly-math.Cos(a)*leverR*0.3,
		leverX+math.Sin(a)*leverR, ly-math.Cos(a)*leverR)
	if s := readout(in); r.readout.Text != s {
		r.readout.Text = s
		r.readout.Refresh()
	}
}

// readout shows the lever and ring positions as the panel reports them.
func readout(in micropanel.State) string {
	return fmt.Sprintf("IRIS %d   MB %d", in.Lever, in.Ring)
}

func darken(c color.NRGBA) color.NRGBA {
	return color.NRGBA{c.R * 3 / 5, c.G * 3 / 5, c.B * 3 / 5, c.A}
}

func setFill(o *canvas.Rectangle, c color.Color) {
	if o.FillColor != c {
		o.FillColor = c
		o.Refresh()
	}
}

func setLine(o *canvas.Line, x1, y1, x2, y2 float64) {
	p1, p2 := pos(x1, y1), pos(x2, y2)
	if o.Position1 != p1 || o.Position2 != p2 {
		o.Position1, o.Position2 = p1, p2
		o.Refresh()
	}
}

func (r *viewRenderer) Layout(fyne.Size) {}

func (r *viewRenderer) MinSize() fyne.Size { return size(bodyW, bodyH) }

func (r *viewRenderer) Refresh() { r.update() }

func (r *viewRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *viewRenderer) Destroy() {}
