package icons

import (
	"image/color"
	"strings"
	"testing"
)

func TestEveryIconIsEmbeddedAndRecolored(t *testing.T) {
	for _, n := range []string{Aperture, Brightness, Stopwatch, Temperature, Focus, FocusAuto, ZoomIn, ZoomOut,
		Wand, Minus, Plus, ChevronLeft, ChevronRight, ChevronsLeft, ChevronsRight, Palette, Bookmarks, Save,
		Trash, Reset, Record, Broadcast, Wifi, WifiOff, Video, Back, Camera, Adjustments, Filter, Sun, Close, Maximize, Restore, Info} {
		res := Res(n, color.NRGBA{R: 0xF4, G: 0x7B, B: 0x20, A: 0xFF})
		if res == nil {
			t.Errorf("%s not embedded", n)
			continue
		}
		svg := string(res.Content())
		if !strings.Contains(svg, `stroke="#f47b20"`) || strings.Contains(svg, `stroke="currentColor"`) && !strings.Contains(svg, "<svg") {
			t.Errorf("%s not recolored", n)
		}
	}
	if Res("nope", color.Black) != nil {
		t.Error("unknown icon must give nil")
	}
}
