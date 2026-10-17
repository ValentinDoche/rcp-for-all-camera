package panelsource_test

import (
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsource"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsim"
)

func newTest(serials ...string) (*panelsource.Selector, *string) {
	opened := new(string)
	s := panelsource.New(panelsim.New(), panelsource.USB{
		List: func() ([]string, error) { return serials, nil },
		Open: func(sn string) (panel.Device, error) { *opened = "usb[" + sn + "]"; return nil, panel.ErrNoDevice },
		Busy: func() bool { return true },
	})
	return s, opened
}

func TestListOffersEveryPanelAndTheSimulator(t *testing.T) {
	s, _ := newTest("A1", "B2")
	var ids []string
	for _, p := range s.List() {
		ids = append(ids, p.ID)
	}
	want := []string{"usb", "usb:A1", "usb:B2", "sim"}
	if len(ids) != len(want) {
		t.Fatalf("ids = %v", ids)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ids = %v, want %v", ids, want)
		}
	}
}

func TestSelectRoutesOpenBusyAndPresence(t *testing.T) {
	s, opened := newTest("A1", "B2")
	var notified []string
	s.OnSelect(func(id string) { notified = append(notified, id) })

	_, _ = s.Open()
	if *opened != "usb[]" || !s.Busy() {
		t.Fatalf("auto USB: opened %q busy %v", *opened, s.Busy())
	}
	if err := s.Select("usb:B2"); err != nil {
		t.Fatal(err)
	}
	_, _ = s.Open()
	if *opened != "usb[B2]" {
		t.Fatalf("opened %q, want B2", *opened)
	}
	if err := s.Select("usb:ZZ"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.Present(); ok {
		t.Fatal("an absent serial must not be present")
	}
	if err := s.Select(app.PanelSourceSim); err != nil {
		t.Fatal(err)
	}
	dev, err := s.Open()
	if err != nil || dev.Serial() != panelsim.Serial || s.Busy() {
		t.Fatalf("sim: %v %v busy %v", dev, err, s.Busy())
	}
	if ok, _ := s.Present(); !ok {
		t.Fatal("simulator not present")
	}
	_ = s.Select(app.PanelSourceSim) // again: the window must come back
	if len(notified) != 4 {
		t.Fatalf("notified %v", notified)
	}
	if err := s.Select("bogus"); err == nil {
		t.Fatal("bogus id accepted")
	}
}

func TestPresentReportsDetectionErrors(t *testing.T) {
	s := panelsource.New(panelsim.New(), panelsource.USB{
		List: func() ([]string, error) { return nil, errors.New("setupapi") },
		Open: func(string) (panel.Device, error) { return nil, panel.ErrNoDevice },
		Busy: func() bool { return false },
	})
	if _, err := s.Present(); err == nil {
		t.Fatal("detection error hidden")
	}
}
