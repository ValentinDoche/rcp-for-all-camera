package style

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

// Mode is the user's theme setting.
type Mode string

const (
	ModeSystem Mode = "system"
	ModeLight  Mode = "light"
	ModeDark   Mode = "dark"
)

// ParseMode turns a stored preference into a Mode (unknown values give system).
func ParseMode(s string) Mode {
	switch Mode(s) {
	case ModeLight, ModeDark:
		return Mode(s)
	}
	return ModeSystem
}

// Theme is the Fyne theme of the application.
type Theme struct{ Mode Mode }

var _ fyne.Theme = Theme{}

// NewTheme builds the theme for a mode.
func NewTheme(m Mode) Theme { return Theme{Mode: m} }

// Palette returns the palette used for the given system variant.
func (t Theme) Palette(system fyne.ThemeVariant) Palette {
	switch t.Mode {
	case ModeLight:
		return Light
	case ModeDark:
		return Dark
	}
	if system == theme.VariantLight {
		return Light
	}
	return Dark
}

func alpha(c color.Color, a uint8) color.Color {
	r, g, b, _ := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: a}
}

func (t Theme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	p := t.Palette(variant)
	if c, ok := p[name]; ok {
		return c
	}
	switch name {
	case theme.ColorNameBackground:
		return p[Bg]
	case theme.ColorNameButton, theme.ColorNameMenuBackground, theme.ColorNameOverlayBackground:
		return p[Raised]
	case theme.ColorNameDisabledButton, theme.ColorNameHeaderBackground:
		return p[Surface]
	case theme.ColorNameDisabled:
		return p[TextFaint]
	case theme.ColorNameError:
		return p[Danger]
	case theme.ColorNameForegroundOnError, theme.ColorNameForegroundOnSuccess, theme.ColorNameForegroundOnWarning:
		return p[Bg]
	case theme.ColorNameFocus:
		return alpha(p[Accent], 0x66)
	case theme.ColorNameForeground:
		return p[Text]
	case theme.ColorNameForegroundOnPrimary:
		return p[OnAccent]
	case theme.ColorNameHover:
		return alpha(p[Text], 0x14)
	case theme.ColorNamePressed:
		return alpha(p[Text], 0x24)
	case theme.ColorNameHyperlink:
		return p[AccentText]
	case theme.ColorNameInputBackground:
		return p[Inset]
	case theme.ColorNameSeparator:
		return p[Border]
	case theme.ColorNameInputBorder,
		theme.ColorNameInnerWindowBorder, theme.ColorNameInnerWindowBorderInactive:
		return p[BorderStrong]
	case theme.ColorNamePlaceHolder:
		return p[TextMuted]
	case theme.ColorNamePrimary:
		return p[Accent]
	case theme.ColorNameScrollBar:
		return p[BorderStrong]
	case theme.ColorNameScrollBarBackground:
		return alpha(p[Border], 0x60)
	case theme.ColorNameSelection:
		return alpha(p[Accent], 0x40)
	case theme.ColorNameShadow:
		return color.NRGBA{A: 0x40}
	case theme.ColorNameSuccess:
		return p[OK]
	case theme.ColorNameWarning:
		return p[Warn]
	}
	return theme.DefaultTheme().Color(name, variant)
}

func (Theme) Font(s fyne.TextStyle) fyne.Resource { return theme.DefaultTheme().Font(s) }

func (Theme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (Theme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNamePadding:
		return 4
	case theme.SizeNameInnerPadding:
		return 8
	case theme.SizeNameText:
		return 13
	case theme.SizeNameCaptionText:
		return 11
	case theme.SizeNameSubHeadingText:
		return 15
	case theme.SizeNameHeadingText:
		return 20
	case theme.SizeNameInputRadius, theme.SizeNameSelectionRadius, theme.SizeNameButtonRadius,
		theme.SizeNameCardRadius, theme.SizeNameDialogRadius, theme.SizeNamePopupRadius,
		theme.SizeNameMenuRadius, theme.SizeNameInnerWindowRadius:
		return Radius
	case theme.SizeNameScrollBarRadius:
		return 3
	case theme.SizeNameScrollBar:
		return 8
	case theme.SizeNameScrollBarSmall:
		return 4
	}
	return theme.DefaultTheme().Size(name)
}

// C returns the current color of a token.
func C(name fyne.ThemeColorName) color.Color { return theme.Color(name) }
