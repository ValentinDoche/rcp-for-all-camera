package style

import (
	"testing"

	"fyne.io/fyne/v2"
)

func TestPaletteContrastAA(t *testing.T) {
	type pair struct {
		fg, bg fyne.ThemeColorName
		min    float64
	}
	pairs := []pair{
		{Text, Bg, 4.5}, {Text, Surface, 4.5}, {Text, Raised, 4.5}, {Text, Inset, 4.5},
		{TextMuted, Bg, 4.5}, {TextMuted, Surface, 4.5}, {TextMuted, Raised, 4.5}, {TextMuted, Inset, 4.5},
		{OnAccent, Accent, 4.5}, {AccentText, Surface, 4.5}, {AccentText, Bg, 4.5},
		{OnTally, Tally, 4.5},
		{OK, Surface, 4.5}, {Warn, Surface, 4.5}, {Danger, Surface, 4.5},
		{Danger, Bg, 4.5}, {OK, Bg, 4.5}, {Warn, Bg, 4.5},
		{ChanR, Inset, 3}, {ChanG, Inset, 3}, {ChanB, Inset, 3},
		{BorderStrong, Surface, 3}, // control outlines, WCAG 1.4.11
	}
	for name, p := range map[string]Palette{"dark": Dark, "light": Light} {
		for _, c := range pairs {
			if got := Contrast(p[c.fg], p[c.bg]); got < c.min {
				t.Errorf("%s: %s on %s = %.2f, want >= %.1f", name, c.fg, c.bg, got, c.min)
			}
		}
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]Mode{"light": ModeLight, "dark": ModeDark, "system": ModeSystem, "": ModeSystem, "x": ModeSystem} {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}
