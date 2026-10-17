package brand

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"
)

func TestLogosAreValidSVG(t *testing.T) {
	for _, r := range []fyne.Resource{Logo, Mark(true), Mark(false)} {
		if len(r.Content()) == 0 {
			t.Fatalf("%s is empty", r.Name())
		}
	}
}

// TestRender renders the logos to $BRAND_SHOTS_DIR (outside the repo).
func TestRender(t *testing.T) {
	dir := os.Getenv("BRAND_SHOTS_DIR")
	if dir == "" {
		t.Skip("set BRAND_SHOTS_DIR to render the logos")
	}
	test.NewTempApp(t)
	img := func(r fyne.Resource, px float32) fyne.CanvasObject {
		i := canvas.NewImageFromResource(r)
		i.SetMinSize(fyne.NewSize(px, px))
		i.FillMode = canvas.ImageFillContain
		return i
	}
	row := container.NewHBox(img(Logo, 256), img(Logo, 64), img(Logo, 32), img(Logo, 16), img(Mark(true), 128))
	w := test.NewTempWindow(t, row)
	w.Resize(row.MinSize().AddWidthHeight(16, 16))
	f, err := os.Create(filepath.Join(dir, "logo.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_ = png.Encode(f, software.RenderCanvas(w.Canvas(), theme.DarkTheme()))

}
