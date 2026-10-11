package widgets

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestLeverDeflectsAndSpringsBack(t *testing.T) {
	l := NewLever(true)
	l.Resize(fyne.NewSize(20, 120))
	var got []float64
	l.OnHold = func(d float64) { got = append(got, d) }
	l.Tapped(&fyne.PointEvent{Position: fyne.NewPos(10, 0)}) // top: full zoom in
	if len(got) != 1 || got[0] != 1 {
		t.Fatalf("tap at the top = %v, want [1]", got)
	}
	l.Dragged(&fyne.DragEvent{PointEvent: fyne.PointEvent{Position: fyne.NewPos(10, 90)}})
	if d := l.Deflection(); d >= 0 {
		t.Errorf("below the centre deflects down: %v", d)
	}
	l.DragEnd()
	if l.Deflection() != 0 {
		t.Error("released lever must spring back to the centre")
	}
	l.SetDisabled(true)
	l.Tapped(&fyne.PointEvent{Position: fyne.NewPos(10, 0)})
	if len(got) != 1 {
		t.Error("a disabled lever does nothing")
	}
}
