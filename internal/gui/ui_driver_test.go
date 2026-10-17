package gui_test

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// The GUI tests drive the window like a user: they look for what is shown
// (a text, a kind of widget), click at its place on the canvas, type, drag,
// and read what the window shows afterwards. Nothing here reads the GUI's
// internals.

// scope is a part of the window the test looks in.
type scope struct{ roots []fyne.CanvasObject }

// win is the whole window: its content and every overlay (dialogs, menus).
func win(a *gui.App) scope {
	c := a.Window().Canvas()
	return scope{append([]fyne.CanvasObject{c.Content()}, c.Overlays().List()...)}
}

// dialog is the top-most overlay (a confirmation, a menu); it is empty when
// none is open.
func dialog(a *gui.App) scope {
	top := a.Window().Canvas().Overlays().Top()
	if top == nil {
		return scope{}
	}
	return scope{[]fyne.CanvasObject{top}}
}

// dialogOpen reports whether an overlay is shown (Settings is a layer of the
// window, not an overlay).
func dialogOpen(a *gui.App) bool { return a.Window().Canvas().Overlays().Top() != nil }

// objects returns every visible object of the scope, widgets' renderers
// included, in drawing order.
func (s scope) objects() []fyne.CanvasObject {
	var out []fyne.CanvasObject
	for _, r := range s.roots {
		out = append(out, visibleUnder(r)...)
	}
	return out
}

// visibleUnder returns the visible objects under root, root included. A
// hidden container hides everything under it.
func visibleUnder(root fyne.CanvasObject) []fyne.CanvasObject {
	if root == nil || !root.Visible() {
		return nil
	}
	out := []fyne.CanvasObject{root}
	switch o := root.(type) {
	case *fyne.Container:
		for _, c := range o.Objects {
			out = append(out, visibleUnder(c)...)
		}
	case fyne.Widget:
		for _, c := range test.WidgetRenderer(o).Objects() {
			out = append(out, visibleUnder(c)...)
		}
	}
	return out
}

// textOf returns the text an object shows.
func textOf(o fyne.CanvasObject) (string, bool) {
	switch w := o.(type) {
	case *widget.Label:
		return w.Text, true
	case *widget.Button:
		return w.Text, true
	case *widget.Check:
		return w.Text, true
	case *widget.Entry:
		return w.Text, true
	case *widget.Select:
		return w.Selected, true
	case *widget.Hyperlink:
		return w.Text, true
	case *widget.RichText:
		return w.String(), true
	case *widgets.IconButton:
		return w.Label(), true
	case *canvas.Text:
		return w.Text, true
	case interface{ Text() string }:
		return w.Text(), true
	}
	return "", false
}

// texts returns every non-empty text of the scope.
func (s scope) texts() []string {
	var out []string
	for _, o := range s.objects() {
		if t, ok := textOf(o); ok && t != "" {
			out = append(out, t)
		}
	}
	return out
}

// joined is every text of the scope in one string, for error messages.
func (s scope) joined() string { return strings.Join(s.texts(), " | ") }

// shows reports whether the scope shows exactly text.
func (s scope) shows(text string) bool { return s.textObject(text) != nil }

// has reports whether some text of the scope contains part.
func (s scope) has(part string) bool {
	for _, t := range s.texts() {
		if strings.Contains(t, part) {
			return true
		}
	}
	return false
}

// textObject returns the first visible object showing exactly text. Among
// a widget and the canvas text drawing it, the widget comes first.
func (s scope) textObject(text string) fyne.CanvasObject {
	for _, o := range s.objects() {
		if t, ok := textOf(o); ok && t == text {
			return o
		}
	}
	return nil
}

// around returns the smallest container of the scope that shows text and
// for which also is true (nil also: any). It is how a test finds a strip, a
// card or a setting row: by a text it shows.
func (s scope) around(text string, also func(scope) bool) scope {
	var best fyne.CanvasObject
	bestN := -1
	for _, o := range s.objects() {
		if _, ok := o.(*fyne.Container); !ok {
			continue
		}
		in := scope{[]fyne.CanvasObject{o}}
		if !in.shows(text) || (also != nil && !also(in)) {
			continue
		}
		if n := len(in.objects()); bestN < 0 || n < bestN {
			best, bestN = o, n
		}
	}
	if best == nil {
		return scope{}
	}
	return scope{[]fyne.CanvasObject{best}}
}

// withText is an also for around: the part shows text too.
func withText(text string) func(scope) bool {
	return func(s scope) bool { return s.shows(text) }
}

// withA is an also for around: the part holds a T.
func withA[T fyne.CanvasObject]() func(scope) bool {
	return func(s scope) bool { return len(allOf[T](s)) > 0 }
}

// allOf returns every visible T of the scope, in drawing order.
func allOf[T fyne.CanvasObject](s scope) []T {
	var out []T
	for _, o := range s.objects() {
		if w, ok := o.(T); ok {
			out = append(out, w)
		}
	}
	return out
}

// one returns the first visible T of the scope, failing the test if none.
func one[T fyne.CanvasObject](t *testing.T, s scope) T {
	t.Helper()
	all := allOf[T](s)
	if len(all) == 0 {
		var zero T
		t.Fatalf("no %T in a part showing %q", zero, s.joined())
	}
	return all[0]
}

// iconButtons returns the visible icon buttons of the scope with that icon.
func iconButtons(s scope, icon string) []*widgets.IconButton {
	var out []*widgets.IconButton
	for _, b := range allOf[*widgets.IconButton](s) {
		if b.Icon() == icon {
			out = append(out, b)
		}
	}
	return out
}

// --- actions -------------------------------------------------------------------

// centre returns the absolute centre of o on the canvas.
func centre(o fyne.CanvasObject) fyne.Position {
	p := fyne.CurrentApp().Driver().AbsolutePositionForObject(o)
	return p.Add(fyne.NewPos(o.Size().Width/2, o.Size().Height/2))
}

// press scrolls o into view, then clicks at its centre on the canvas:
// whatever is drawn on top there gets the tap, as with a mouse.
func press(a *gui.App, o fyne.CanvasObject) {
	reveal(a, o)
	test.TapCanvas(a.Window().Canvas(), centre(o))
}

// reveal scrolls every scroll holding o so that o is in view, as a user
// scrolls to a setting below the fold.
func reveal(a *gui.App, o fyne.CanvasObject) {
	d := fyne.CurrentApp().Driver()
	for _, sc := range allOf[*container.Scroll](win(a)) {
		if !holds(sc, o) {
			continue
		}
		top := d.AbsolutePositionForObject(sc)
		at := d.AbsolutePositionForObject(o)
		off := sc.Offset
		if y := at.Y - top.Y; y < 0 || y+o.Size().Height > sc.Size().Height {
			off.Y += y - (sc.Size().Height-o.Size().Height)/2
		}
		if x := at.X - top.X; x < 0 || x+o.Size().Width > sc.Size().Width {
			off.X += x - (sc.Size().Width-o.Size().Width)/2
		}
		if off != sc.Offset {
			sc.ScrollToOffset(off)
		}
	}
}

// holds reports whether o is drawn inside root.
func holds(root, o fyne.CanvasObject) bool {
	for _, x := range visibleUnder(root) {
		if x == o && x != root {
			return true
		}
	}
	return false
}

// click clicks the text shown in the window (a button, a tab, a menu item, a
// segment of a selector). It fails the test if the window does not show it.
func click(t *testing.T, a *gui.App, text string) {
	t.Helper()
	clickIn(t, a, win(a), text)
}

// clickIn is click inside part of the window.
func clickIn(t *testing.T, a *gui.App, s scope, text string) {
	t.Helper()
	o := s.textObject(text)
	if o == nil {
		t.Fatalf("no %q to click; shown: %s", text, s.joined())
	}
	press(a, o)
}

// drag drags from o's centre by (dx, dy) and releases.
func drag(a *gui.App, o fyne.CanvasObject, dx, dy float32) {
	test.Drag(a.Window().Canvas(), centre(o), dx, dy)
}

// typeInto replaces an entry's text as a user would (click, clear, type),
// then presses Enter when enter is true.
func typeInto(a *gui.App, e *widget.Entry, text string, enter bool) {
	press(a, e)
	e.SetText("")
	test.Type(e, text)
	if enter {
		e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	}
}

// pick chooses option in a select box, as a user opening it and clicking
// the option.
func pick(t *testing.T, a *gui.App, sel *widget.Select, option string) {
	t.Helper()
	press(a, sel)
	if !dialogOpen(a) {
		t.Fatalf("the select box did not open")
	}
	clickIn(t, a, dialog(a), option)
}

// fieldAfter returns the T that goes with the caption label: the smallest
// part of s showing label and holding a T.
func fieldAfter[T fyne.CanvasObject](t *testing.T, s scope, label string) T {
	t.Helper()
	part := s.around(label, withA[T]())
	if part.roots == nil {
		var zero T
		t.Fatalf("no %T next to %q; shown: %s", zero, label, s.joined())
	}
	return one[T](t, part)
}

// sized gives the window the size of a laptop screen.
func sized(a *gui.App) {
	a.Window().Resize(fyne.NewSize(1440, 810))
}
