// Package panelsource decides where the panel controller takes its panel
// from: the first USB panel, a given USB panel (by serial) or the panel
// simulator. Selector implements app.PanelSources for the GUI and gives
// the controller its Open, Busy and presence probe.
package panelsource

import (
	"fmt"
	"strings"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsim"
)

// USB is how the selector reaches the USB panels. SystemUSB is the real
// one; tests pass fakes.
type USB struct {
	// List returns the serial numbers of the plugged-in panels.
	List func() ([]string, error)
	// Open opens one panel by serial ("" = the first one).
	Open func(serial string) (panel.Device, error)
	// Busy reports whether another program drives the panel.
	Busy func() bool
}

// SystemUSB is the USB panel access of this computer.
func SystemUSB() USB {
	return USB{List: panel.USBPanels, Open: panel.OpenUSBSerial, Busy: panel.OtherOwnerRunning}
}

// Selector holds the selected panel source.
type Selector struct {
	sim *panelsim.Sim
	usb USB

	mu       sync.Mutex
	sel      string
	onSelect func(id string)
}

var _ app.PanelSources = (*Selector)(nil)

// New returns a selector on the first USB panel. sim is the simulator
// offered next to the USB panels; usb reaches the USB panels (SystemUSB).
func New(sim *panelsim.Sim, usb USB) *Selector {
	return &Selector{sim: sim, usb: usb, sel: app.PanelSourceUSB}
}

// OnSelect sets a func called after the source changed (outside the lock):
// cmd/rcp reconnects the controller and shows or hides the simulator.
func (s *Selector) OnSelect(fn func(id string)) {
	s.mu.Lock()
	s.onSelect = fn
	s.mu.Unlock()
}

// Selected returns the selected source id.
func (s *Selector) Selected() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sel
}

// List returns automatic USB, every plugged-in USB panel and the simulator.
func (s *Selector) List() []app.PanelSource {
	out := []app.PanelSource{{ID: app.PanelSourceUSB, Kind: app.PanelKindUSB}}
	serials, _ := s.usb.List() // detection failures already show in the status
	for _, sn := range serials {
		if sn != "" {
			out = append(out, app.PanelSource{ID: app.PanelSourceSerialPrefix + sn, Kind: app.PanelKindUSB, Serial: sn})
		}
	}
	return append(out, app.PanelSource{ID: app.PanelSourceSim, Kind: app.PanelKindSim})
}

// Select switches to source id. Choosing the simulator plugs it in and
// notifies even when it was already selected, so a closed simulator
// window comes back.
func (s *Selector) Select(id string) error {
	if !app.ValidPanelSource(id) {
		return fmt.Errorf("panelsource: unknown source %q", id)
	}
	s.mu.Lock()
	changed := s.sel != id
	s.sel = id
	fn := s.onSelect
	s.mu.Unlock()
	if id == app.PanelSourceSim {
		s.sim.SetPlugged(true)
	}
	if (changed || id == app.PanelSourceSim) && fn != nil {
		fn(id)
	}
	return nil
}

// serial returns the USB serial of the selection ("" = first panel) and
// whether the selection is the simulator.
func (s *Selector) serial() (string, bool) {
	sel := s.Selected()
	if sel == app.PanelSourceSim {
		return "", true
	}
	return strings.TrimPrefix(strings.TrimPrefix(sel, app.PanelSourceUSB), ":"), false
}

// Open is the controller's panel.Options.Open.
func (s *Selector) Open() (panel.Device, error) {
	sn, sim := s.serial()
	if sim {
		return s.sim.Open()
	}
	return s.usb.Open(sn)
}

// Busy is the controller's panel.Options.Busy: ATEM Software Control only
// takes USB panels.
func (s *Selector) Busy() bool {
	if _, sim := s.serial(); sim {
		return false
	}
	return s.usb.Busy()
}

// Present is the app.PanelProbe of the selected source.
func (s *Selector) Present() (bool, error) {
	sn, sim := s.serial()
	if sim {
		return s.sim.Plugged(), nil
	}
	serials, err := s.usb.List()
	if err != nil {
		return false, err
	}
	for _, x := range serials {
		if sn == "" || x == sn {
			return true, nil
		}
	}
	return false, nil
}
