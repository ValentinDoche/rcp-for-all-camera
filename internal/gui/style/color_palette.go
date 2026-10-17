// Package style holds the visual tokens of the application: two palettes
// (graphite dark, soft grey light), one radius system and the Fyne theme that
// serves them. Widgets never hardcode a color: they ask for a token.
package style

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
)

// Token names. They extend Fyne's own color names.
const (
	Bg           fyne.ThemeColorName = "rcp.bg"           // window background
	Surface      fyne.ThemeColorName = "rcp.surface"      // strips, panels
	Raised       fyne.ThemeColorName = "rcp.raised"       // buttons, headers, chips
	Inset        fyne.ThemeColorName = "rcp.inset"        // wells, inputs, meters
	Border       fyne.ThemeColorName = "rcp.border"       // hairlines
	BorderStrong fyne.ThemeColorName = "rcp.borderStrong" // control outlines
	Text         fyne.ThemeColorName = "rcp.text"
	TextMuted    fyne.ThemeColorName = "rcp.textMuted"
	TextFaint    fyne.ThemeColorName = "rcp.textFaint" // decoration and disabled only
	Accent       fyne.ThemeColorName = "rcp.accent"    // selection fill (orange)
	OnAccent     fyne.ThemeColorName = "rcp.onAccent"
	AccentText   fyne.ThemeColorName = "rcp.accentText" // accent used as text color
	Tally        fyne.ThemeColorName = "rcp.tally"      // on-air, the only saturated red
	OnTally      fyne.ThemeColorName = "rcp.onTally"
	OK           fyne.ThemeColorName = "rcp.ok"
	Warn         fyne.ThemeColorName = "rcp.warn"
	Danger       fyne.ThemeColorName = "rcp.danger"
	ChanR        fyne.ThemeColorName = "rcp.chanR"
	ChanG        fyne.ThemeColorName = "rcp.chanG"
	ChanB        fyne.ThemeColorName = "rcp.chanB"
)

// Radius is the single corner radius used by every control and panel.
const Radius float32 = 6

// Palette maps every token to a color.
type Palette map[fyne.ThemeColorName]color.Color

func hex(rgb uint32) color.Color {
	return color.NRGBA{R: uint8(rgb >> 16), G: uint8(rgb >> 8), B: uint8(rgb), A: 0xFF}
}

// Dark is the default palette: graphite surfaces, orange selection.
var Dark = Palette{
	Bg: hex(0x17181A), Surface: hex(0x212225), Raised: hex(0x2B2D31), Inset: hex(0x131416),
	Border: hex(0x34363B), BorderStrong: hex(0x7A7E88),
	Text: hex(0xE8E9EC), TextMuted: hex(0xA3A7B0), TextFaint: hex(0x6B6F78),
	Accent: hex(0xF47B20), OnAccent: hex(0x17181A), AccentText: hex(0xF59A4E),
	Tally: hex(0xCC2621), OnTally: hex(0xFFF4F3),
	OK: hex(0x4CC27A), Warn: hex(0xE6B13A), Danger: hex(0xF58882),
	ChanR: hex(0xE5575B), ChanG: hex(0x4CC27A), ChanB: hex(0x5B9BF8),
}

// Light keeps the same hierarchy on soft grey surfaces.
var Light = Palette{
	Bg: hex(0xE2E4E8), Surface: hex(0xF3F4F6), Raised: hex(0xFFFFFF), Inset: hex(0xE8EAED),
	Border: hex(0xCBCED4), BorderStrong: hex(0x7A7F89),
	Text: hex(0x1B1D21), TextMuted: hex(0x535862), TextFaint: hex(0x8A8F99),
	Accent: hex(0xF47B20), OnAccent: hex(0x1B1D21), AccentText: hex(0x9E4400),
	Tally: hex(0xCC2621), OnTally: hex(0xFFF7F6),
	OK: hex(0x16703A), Warn: hex(0x7F5900), Danger: hex(0xB3261E),
	ChanR: hex(0xD13A3E), ChanG: hex(0x1F8A4B), ChanB: hex(0x2D6BD6),
}

// Luminance is the WCAG relative luminance of c.
func Luminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	lin := func(v uint32) float64 {
		x := float64(v) / 65535
		if x <= 0.03928 {
			return x / 12.92
		}
		return math.Pow((x+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(r) + 0.7152*lin(g) + 0.0722*lin(b)
}

// Contrast is the WCAG contrast ratio between two colors (1 to 21).
func Contrast(a, b color.Color) float64 {
	la, lb := Luminance(a), Luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}
