package gui

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui/style"
)

// TestScreenshots renders the main pages offscreen in both themes. It only
// runs when RCP_SHOTS_DIR points to an output folder (never inside the repo).
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("RCP_SHOTS_DIR")
	if dir == "" {
		t.Skip("set RCP_SHOTS_DIR to render screenshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	size := fyne.NewSize(1440, 810)
	if os.Getenv("RCP_SHOTS_SMALL") != "" {
		size = fyne.NewSize(1280, 720)
	}
	// RCP_SHOTS_SIZE=WxH overrides the size, e.g. 1440x2400 to see a whole page.
	var w, h float32
	if _, err := fmt.Sscanf(os.Getenv("RCP_SHOTS_SIZE"), "%fx%f", &w, &h); err == nil && w > 0 && h > 0 {
		size = fyne.NewSize(w, h)
	}

	fa := test.NewTempApp(t)
	svc, _, cams := newLiveService(t)
	seedRig(t, svc)
	(*cams)[0].values[core.Tally] = core.Bool(true)
	a := New(fa, svc, syncOptions())
	a.Window().Resize(size)
	save := func(name string, mode style.Mode) {
		img := software.RenderCanvas(a.Window().Canvas(), style.NewTheme(mode))
		writePNG(t, filepath.Join(dir, name+"-"+string(mode)+".png"), img)
	}
	for _, mode := range []style.Mode{style.ModeDark, style.ModeLight} {
		a.SetMode(mode)
		a.ShowCameras()
		a.Window().Resize(size)
		save("1-cameras", mode)

		a.ShowDetails(1)
		a.Window().Resize(size)
		save("2-details", mode)

		a.ShowDetails(2) // FX6: what it does not declare is greyed
		a.Window().Resize(size)
		save("2b-details-greyed", mode)

		a.ShowConfig(2)
		a.Window().Resize(size)
		save("3-config-edit", mode)

		a.ShowConfig(3)
		save("3b-config-edit-offline-cam", mode)
		a.ShowCameras()
	}
	// Configuration > General, in both languages and themes
	a.SetLanguage("fr")
	for _, mode := range []style.Mode{style.ModeDark, style.ModeLight} {
		a.SetMode(mode)
		a.ShowConfig(0)
		a.config.SelectTab(tabGeneral)
		a.Window().Resize(size)
		save("5-general-fr", mode)
	}
	// the whole tab at once: a taller window
	a.SetMode(style.ModeDark)
	a.ShowConfig(0)
	a.Window().Resize(fyne.NewSize(size.Width, 1250))
	save("5b-general-fr-tall", style.ModeDark)
	a.Window().Resize(size)
	a.SetMode(style.ModeDark)
	a.SetLanguage("en")
	a.ShowConfig(0)
	a.config.SelectTab(tabGeneral)
	a.Window().Resize(size)
	save("5-general-en", style.ModeDark)
	a.ShowCameras()
	a.Window().Resize(size)
	save("6-cameras-en", style.ModeDark)
	a.ShowConfig(2)
	a.Window().Resize(size)
	save("7-config-cameras-en", style.ModeDark)
	a.SetLanguage("fr")
	for i, tab := range []int{tabPanel, tabAbout} {
		a.ShowConfig(0)
		a.config.SelectTab(tab)
		a.Window().Resize(size)
		save(fmt.Sprintf("8%c-settings-%d", 'a'+i, tab), style.ModeDark)
	}
	a.ShowCameras()

	if os.Getenv("RCP_SHOTS_EMPTY") != "" {
		// empty rig for the first-run look
		a2, _, _ := newTestApp(t, false)
		a2.Window().Resize(size)
		img := software.RenderCanvas(a2.Window().Canvas(), style.NewTheme(style.ModeDark))
		writePNG(t, filepath.Join(dir, "4-empty-dark.png"), img)
	}
}

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
