package panelsim_test

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsim"
	"image/png"
	"os"
	"slices"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/driver/software"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/theme"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
)

func TestMouseDrivesInputs(t *testing.T) {
	test.NewTempApp(t)
	s := panelsim.New()
	v := panelsim.NewView(s)
	test.NewTempWindow(t, v)

	at := panelsim.CamButtonCentre(2)
	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: at}, Button: desktop.MouseButtonPrimary})
	if keys := s.Input().Keys; !slices.Equal(keys, []micropanel.Key{panelsim.KeyCam1 + 2}) {
		t.Fatalf("held keys = %v, want CAM3", keys)
	}
	v.MouseUp(&desktop.MouseEvent{})
	if keys := s.Input().Keys; len(keys) != 0 {
		t.Fatalf("keys after release = %v", keys)
	}

	v.Scrolled(&fyne.ScrollEvent{PointEvent: fyne.PointEvent{Position: panelsim.KnobCentre(8)}, Scrolled: fyne.Delta{DY: 10}})
	if got := s.Input().Analog[8]; got != 512+8 {
		t.Fatalf("COARSE = %d after one wheel notch", got)
	}

	v.MouseDown(&desktop.MouseEvent{PointEvent: fyne.PointEvent{Position: panelsim.LeverTopCentre()}, Button: desktop.MouseButtonPrimary})
	v.Dragged(&fyne.DragEvent{Dragged: fyne.Delta{DY: -1000}})
	v.DragEnd()
	if got := s.Input().Lever; got != micropanel.JoystickMax {
		t.Fatalf("lever = %d after dragging fully forward", got)
	}
}

// TestShot renders the panel with sample output to $PANELSIM_SHOT (a PNG
// path outside the repo) to check the drawing.
func TestShot(t *testing.T) {
	path := os.Getenv("PANELSIM_SHOT")
	if path == "" {
		t.Skip("set PANELSIM_SHOT to render the panel")
	}
	test.NewTempApp(t)
	s := panelsim.New()
	var d [14]byte
	micropanel.EncodeText(&d, micropanel.DispND, "1")
	micropanel.EncodeText(&d, micropanel.DispGain, "06")
	micropanel.EncodeText(&d, micropanel.DispShutter, "50")
	micropanel.EncodeText(&d, micropanel.DispIris, "F5.6")
	micropanel.EncodeText(&d, micropanel.DispMasterBlack, "-5")
	_ = s.SetDigitsRaw(d)
	_ = s.SetMatrixRaw(micropanel.MatrixNumber(1, micropanel.PixAmber))
	var l micropanel.LEDBits
	l.SetCamera(1, micropanel.Amber)
	l.SetCamera(2, micropanel.Blue)
	l.SetCamera(5, micropanel.Red)
	l.SetIrisBar(7)
	for _, n := range []int{micropanel.LEDScene1, micropanel.LEDScene1 + 2, micropanel.LEDNDUp, micropanel.LEDGainUp,
		micropanel.LEDGainDown, micropanel.LEDShutterUp, micropanel.LEDPanelRed, micropanel.LEDIrisMBRed,
		micropanel.LEDNetwork, micropanel.LEDBars, micropanel.LEDBarsInd} {
		l.Set(n, true)
	}
	_ = s.WriteReport(l.Report(0))
	s.SetLever(1500)
	s.SetRing(-1200)
	s.TurnKnob(0, 200)

	w := test.NewTempWindow(t, panelsim.NewView(s))
	w.Resize(panelsim.ViewSize().AddWidthHeight(16, 16))
	img := software.RenderCanvas(w.Canvas(), theme.DarkTheme())
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}
