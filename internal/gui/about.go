package gui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/valentindoche/rcp-for-all-camera/internal/gui/brand"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/texts"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/widgets"
)

// Author and project page shown in Settings > About.
const (
	Author     = "Valentin Doche"
	ProjectURL = "https://github.com/ValentinDoche/rcp-for-all-camera"
	Copyright  = "© 2026 Valentin Doche"
)

// centred puts a fixed-width column in the middle of a scrollable page.
func centred(width float32, objs ...fyne.CanvasObject) fyne.CanvasObject {
	col := widgets.Sized(width, 0, container.NewVBox(spaced(objs)...))
	return container.NewVScroll(container.New(layout.NewCustomPaddedLayout(0, 8, 0, 8),
		container.NewHBox(layout.NewSpacer(), col, layout.NewSpacer())))
}

// newAboutTab is Settings > About: logo, name, version, author, project
// page and copyright.
func newAboutTab(tx texts.Texts, env *configEnv) fyne.CanvasObject {
	logo := canvas.NewImageFromResource(brand.Logo)
	logo.FillMode = canvas.ImageFillContain
	logo.SetMinSize(fyne.NewSize(96, 96))

	link := widget.NewHyperlink(ProjectURL, nil)
	if u, err := url.Parse(ProjectURL); err == nil {
		link.URL = u
		if env.openURL != nil {
			link.OnTapped = func() { _ = env.openURL(u) }
		}
	}
	tagline := newHelp(tx.T("about.tagline"), style.TextMuted)
	credits := newHelp(tx.T("about.credits"), style.TextMuted)

	row := func(label string, value fyne.CanvasObject) fyne.CanvasObject {
		return container.NewBorder(nil, nil, widgets.Sized(130, 0, widgets.NewText(label, style.TextMuted, 13)), nil, value)
	}
	head := container.NewHBox(container.NewCenter(logo), container.New(layout.NewCustomPaddedLayout(0, 0, 12, 0),
		container.NewVBox(layout.NewSpacer(),
			widgets.NewText(WindowTitle, style.Text, 22).Bold(),
			widgets.NewText(tx.T("about.version", "version", env.version), style.TextMuted, 13),
			layout.NewSpacer())))
	info := widgets.NewPanel(style.Surface, style.Border, 12, container.NewVBox(
		row(tx.T("about.author"), widgets.NewText(Author, style.Text, 13).Bold()),
		row(tx.T("about.source"), link),
		row(tx.T("about.license"), widgets.NewText(Copyright, style.Text, 13)),
	))
	return centred(620, head, tagline, info, credits)
}
