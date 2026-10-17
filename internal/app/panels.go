package app

import "strings"

// Panel source ids (PanelSource.ID).
const (
	PanelSourceUSB = "usb" // the first USB panel found (default)
	PanelSourceSim = "sim" // the panel simulator window
	// PanelSourceSerialPrefix + serial: one USB panel among several.
	PanelSourceSerialPrefix = "usb:"
)

// Panel source kinds (PanelSource.Kind).
const (
	PanelKindUSB = "usb"
	PanelKindSim = "sim"
)

// PanelSource is one place the panel controller can take its panel from:
// the first USB panel, a given USB panel (Serial), or the simulator.
type PanelSource struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Serial string `json:"serial,omitempty"`
}

// PanelSources lists and switches the panel sources. cmd/rcp implements it
// with the USB enumeration and the simulator, and switches the surface.
type PanelSources interface {
	// List returns the sources available now: automatic USB, every
	// plugged-in USB panel and the simulator.
	List() []PanelSource
	Selected() string
	// Select switches the panel. It accepts every valid id, even a USB
	// serial not plugged in now (the panel is then driven when it appears).
	Select(id string) error
}

// ValidPanelSource reports whether id is a panel source id.
func ValidPanelSource(id string) bool {
	switch {
	case id == PanelSourceUSB, id == PanelSourceSim:
		return true
	case strings.HasPrefix(id, PanelSourceSerialPrefix):
		return len(id) > len(PanelSourceSerialPrefix)
	}
	return false
}

// PanelSources returns the available panel sources and the selected id.
// Without a selector (tests, other platforms) it returns nothing.
func (s *Service) PanelSources() ([]PanelSource, string) {
	if s.panels == nil {
		return nil, ""
	}
	return s.panels.List(), s.panels.Selected()
}

// SelectPanelSource switches the panel to source id and remembers the choice
// in the rig file, so the app starts on it next time.
func (s *Service) SelectPanelSource(id string) error {
	if !ValidPanelSource(id) {
		return &Error{Code: CodeInvalidConfig, Message: "unknown panel source"}
	}
	if s.panels == nil {
		return &Error{Code: CodeNotImplemented, Message: "panel selection not available"}
	}
	if err := s.panels.Select(id); err != nil {
		return &Error{Code: CodeInternal, Message: err.Error()}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.panelSource == id {
		return nil
	}
	s.panelSource = id
	if err := s.saveLocked(); err != nil {
		s.log.Warn("panel choice not saved", "err", err)
	}
	return nil
}

// restorePanelSource applies the saved panel choice at start.
func (s *Service) restorePanelSource(id string) {
	if id == "" || s.panels == nil {
		return
	}
	if !ValidPanelSource(id) {
		s.log.Warn("saved panel source ignored", "source", id)
		return
	}
	if err := s.panels.Select(id); err != nil {
		s.log.Warn("saved panel source not applied", "source", id, "err", err)
		return
	}
	s.panelSource = id
}
