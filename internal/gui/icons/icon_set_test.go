package icons_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/icons"
	"image/color"
	"strings"
	"testing"
)

func TestEveryIconIsEmbeddedAndRecolored(t *testing.T) {
	for _, n := range []string{icons.Aperture, icons.Brightness, icons.Stopwatch, icons.Temperature, icons.Focus, icons.FocusAuto, icons.ZoomIn, icons.ZoomOut,
		icons.Wand, icons.Minus, icons.Plus, icons.ChevronLeft, icons.ChevronRight, icons.ChevronsLeft, icons.ChevronsRight, icons.Palette, icons.Bookmarks, icons.Save,
		icons.Trash, icons.Reset, icons.Record, icons.Broadcast, icons.Wifi, icons.WifiOff, icons.Video, icons.Back, icons.Camera, icons.Adjustments, icons.Filter, icons.Sun, icons.Close, icons.Maximize, icons.Restore, icons.Info} {
		res := icons.Res(n, color.NRGBA{R: 0xF4, G: 0x7B, B: 0x20, A: 0xFF})
		if res == nil {
			t.Errorf("%s not embedded", n)
			continue
		}
		svg := string(res.Content())
		if !strings.Contains(svg, `stroke="#f47b20"`) || strings.Contains(svg, `stroke="currentColor"`) && !strings.Contains(svg, "<svg") {
			t.Errorf("%s not recolored", n)
		}
	}
	if icons.Res("nope", color.Black) != nil {
		t.Error("unknown icon must give nil")
	}
}
