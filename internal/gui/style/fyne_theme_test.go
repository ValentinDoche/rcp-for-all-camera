package style_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"testing"

	"fyne.io/fyne/v2"
)

func TestPaletteContrastAA(t *testing.T) {
	type pair struct {
		fg, bg fyne.ThemeColorName
		min    float64
	}
	pairs := []pair{
		{style.Text, style.Bg, 4.5}, {style.Text, style.Surface, 4.5}, {style.Text, style.Raised, 4.5}, {style.Text, style.Inset, 4.5},
		{style.TextMuted, style.Bg, 4.5}, {style.TextMuted, style.Surface, 4.5}, {style.TextMuted, style.Raised, 4.5}, {style.TextMuted, style.Inset, 4.5},
		{style.OnAccent, style.Accent, 4.5}, {style.AccentText, style.Surface, 4.5}, {style.AccentText, style.Bg, 4.5},
		{style.OnTally, style.Tally, 4.5},
		{style.OK, style.Surface, 4.5}, {style.Warn, style.Surface, 4.5}, {style.Danger, style.Surface, 4.5},
		{style.Danger, style.Bg, 4.5}, {style.OK, style.Bg, 4.5}, {style.Warn, style.Bg, 4.5},
		{style.ChanR, style.Inset, 3}, {style.ChanG, style.Inset, 3}, {style.ChanB, style.Inset, 3},
		{style.BorderStrong, style.Surface, 3}, // control outlines, WCAG 1.4.11
	}
	for name, p := range map[string]style.Palette{"dark": style.Dark, "light": style.Light} {
		for _, c := range pairs {
			if got := style.Contrast(p[c.fg], p[c.bg]); got < c.min {
				t.Errorf("%s: %s on %s = %.2f, want >= %.1f", name, c.fg, c.bg, got, c.min)
			}
		}
	}
}

func TestParseMode(t *testing.T) {
	for in, want := range map[string]style.Mode{"light": style.ModeLight, "dark": style.ModeDark, "system": style.ModeSystem, "": style.ModeSystem, "x": style.ModeSystem} {
		if got := style.ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}
