// Package icons serves the GUI's line icons: a subset of Tabler Icons
// (outline, MIT, see LICENSE), embedded as SVG and recolored on demand.
//
// Tabler draws with strokes in currentColor; Fyne only recolors fills. Res
// therefore rewrites each shape with an explicit stroke color, so an icon can
// follow any style token in both themes.
package icons

import (
	"embed"
	"fmt"
	"image/color"
	"regexp"
	"strings"
	"sync"

	"fyne.io/fyne/v2"
)

//go:embed svg/*.svg
var files embed.FS

// Names of the embedded icons.
const (
	Aperture      = "aperture"
	Brightness    = "brightness-up"
	Stopwatch     = "stopwatch"
	Temperature   = "temperature"
	Focus         = "focus-2"
	FocusAuto     = "focus-auto"
	ZoomIn        = "zoom-in"
	ZoomOut       = "zoom-out"
	Wand          = "wand"
	Minus         = "minus"
	Plus          = "plus"
	ChevronLeft   = "chevron-left"
	ChevronRight  = "chevron-right"
	ChevronsLeft  = "chevrons-left"
	ChevronsRight = "chevrons-right"
	Palette       = "palette"
	Bookmarks     = "bookmarks"
	Save          = "device-floppy"
	Trash         = "trash"
	Reset         = "refresh"
	Record        = "player-record"
	Broadcast     = "broadcast"
	Wifi          = "wifi"
	WifiOff       = "wifi-off"
	Video         = "video"
	Back          = "arrow-left"
	Camera        = "camera"
	Adjustments   = "adjustments-horizontal"
	Filter        = "filter"
	Sun           = "sun-high"
	Close         = "x"
	Maximize      = "square"
	Restore       = "window-restore" // drawn for this app in Tabler's style
	Info          = "info-circle"
)

var (
	mu    sync.Mutex
	cache = map[string]fyne.Resource{}

	// shapes without their own stroke get the icon's stroke and no fill.
	shapeRe = regexp.MustCompile(`<(path|circle|rect|line|polyline|polygon|ellipse)\b([^>]*?)(/?)>`)
)

// Res returns icon name drawn in c. Unknown names give nil.
func Res(name string, c color.Color) fyne.Resource {
	r, g, b, _ := c.RGBA()
	hex := fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
	key := name + hex
	mu.Lock()
	defer mu.Unlock()
	if res, ok := cache[key]; ok {
		return res
	}
	src, err := files.ReadFile("svg/" + name + ".svg")
	if err != nil {
		return nil
	}
	svg := shapeRe.ReplaceAllStringFunc(string(src), func(tag string) string {
		m := shapeRe.FindStringSubmatch(tag)
		if strings.Contains(m[2], "stroke=") {
			return tag
		}
		return "<" + m[1] + m[2] + ` fill="none" stroke="` + hex +
			`" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"` + m[3] + ">"
	})
	res := fyne.NewStaticResource(name+hex+".svg", []byte(svg))
	cache[key] = res
	return res
}

// Has reports whether name is embedded.
func Has(name string) bool {
	_, err := files.ReadFile("svg/" + name + ".svg")
	return err == nil
}
