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

// Selector holds the selected panel source.
type Selector struct {
	sim *panelsim.Sim

	// USB access; tests replace them.
	listUSB func() ([]string, error)
	openUSB func(serial string) (panel.Device, error)
	busyUSB func() bool

	mu       sync.Mutex
	sel      string
	onSelect func(id string)
}

var _ app.PanelSources = (*Selector)(nil)

// New returns a selector on the first USB panel. sim is the simulator
// offered next to the USB panels.
func New(sim *panelsim.Sim) *Selector {
	return &Selector{sim: sim, sel: app.PanelSourceUSB,
		listUSB: panel.USBPanels, openUSB: panel.OpenUSBSerial, busyUSB: panel.OtherOwnerRunning}
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
	serials, _ := s.listUSB() // detection failures already show in the status
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
	return s.openUSB(sn)
}

// Busy is the controller's panel.Options.Busy: ATEM Software Control only
// takes USB panels.
func (s *Selector) Busy() bool {
	if _, sim := s.serial(); sim {
		return false
	}
	return s.busyUSB()
}

// Present is the app.PanelProbe of the selected source.
func (s *Selector) Present() (bool, error) {
	sn, sim := s.serial()
	if sim {
		return s.sim.Plugged(), nil
	}
	serials, err := s.listUSB()
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
