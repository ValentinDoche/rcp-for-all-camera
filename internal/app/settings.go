package app

import (
	"fmt"
	"net"
)

// NetInterface is one local IPv4 address the app can bind to.
type NetInterface struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// Settings are the app-wide options shown in Configuration > General.
// Language and theme are GUI preferences and live in the GUI.
type Settings struct {
	// BindIP is the local address used to reach cameras; "" = OS default.
	BindIP string `json:"bindIp"`
	// BindIPAvailable is false when BindIP is set but no interface has it
	// right now (cable unplugged, other network).
	BindIPAvailable bool `json:"bindIpAvailable"`
	// AutoStart says whether the app starts with the Windows session.
	AutoStart bool `json:"autoStart"`
	// OperatorLock asks for confirmation before guarded controls are written.
	OperatorLock bool `json:"operatorLock"`
	// LogLevel is LogNormal or LogDebug; LogDir holds the log file.
	LogLevel string `json:"logLevel"`
	LogDir   string `json:"logDir"`
}

// AutoStarter turns "start with Windows" on and off (internal/autostart).
type AutoStarter interface {
	Enabled() (bool, error)
	SetEnabled(on bool) error
}

// Settings returns the current app-wide options.
func (s *Service) Settings() Settings {
	s.mu.Lock()
	bind, lock, level := s.bindIP, s.lock, s.logLevel
	s.mu.Unlock()
	st := Settings{BindIP: bind, BindIPAvailable: bind == "" || s.hasLocalIP(bind),
		OperatorLock: lock, LogLevel: level, LogDir: s.logDir}
	if s.autoStart != nil {
		on, err := s.autoStart.Enabled()
		if err != nil {
			s.log.Warn("auto start state not readable", "err", err)
		}
		st.AutoStart = on
	}
	return st
}

// NetworkInterfaces lists the local IPv4 addresses that can be bound.
func (s *Service) NetworkInterfaces() []NetInterface {
	if s.interfaces == nil {
		return nil
	}
	return s.interfaces()
}

// SetBindIP selects the local address cameras are reached from; "" restores
// the OS default. The address must belong to this PC. Cameras are rebuilt so
// their next connection uses it.
func (s *Service) SetBindIP(ip string) error {
	if ip != "" {
		if net.ParseIP(ip) == nil || !s.hasLocalIP(ip) {
			return &Error{Code: CodeInvalidConfig, Message: "bind address is not an address of this PC"}
		}
	}
	s.mu.Lock()
	if s.bindIP == ip {
		s.mu.Unlock()
		return nil
	}
	prev := s.bindIP
	s.bindIP = ip
	if err := s.saveLocked(); err != nil {
		s.bindIP = prev
		s.mu.Unlock()
		return toAPIError(err)
	}
	var stale []*slotState
	for i, st := range s.slots {
		if st == nil {
			continue
		}
		fresh, err := s.build(st.cfg)
		if err != nil {
			s.log.Warn("camera not rebuilt for new bind address", "slot", i+1, "err", err)
			continue
		}
		stale = append(stale, st)
		s.slots[i] = fresh
	}
	rig, emit := s.rigLocked(), s.emit
	s.mu.Unlock()
	for _, st := range stale {
		st.cam.Close()
	}
	s.log.Info("bind address changed", "ip", ip)
	emit.Emit(EventRigChanged, rig)
	return nil
}

// SetAutoStart turns "start with Windows" on or off.
func (s *Service) SetAutoStart(on bool) error {
	if s.autoStart == nil {
		return &Error{Code: CodeNotImplemented, Message: "auto start is not available on this system"}
	}
	if err := s.autoStart.SetEnabled(on); err != nil {
		return toAPIError(fmt.Errorf("auto start: %w", err))
	}
	return nil
}

func (s *Service) hasLocalIP(ip string) bool {
	for _, n := range s.NetworkInterfaces() {
		if n.IP == ip {
			return true
		}
	}
	return false
}
