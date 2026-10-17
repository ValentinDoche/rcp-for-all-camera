// Package panelsim is a software ATEM Micro Camera Panel. Sim implements
// panel.Device, so the real panel controller drives it exactly like the USB
// panel; a Fyne window (NewWindow) draws its displays and LEDs and turns
// mouse input into the panel's input reports.
//
// The simulated panel follows the hardware where the controller relies on
// it: knob counters clamp at 0 and 1024, the 0x0E/0x0F feature reports give
// the current positions, feature 0x0E can be written, LED layers are ORed,
// and the lever and ring stay where they are left.
package panelsim

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"slices"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
)

// Serial is the serial number the simulated panel reports.
const Serial = "SIMULATEUR"

// errUnplugged ends a session when the simulated panel is unplugged.
var errUnplugged = errors.New("panelsim: unplugged")

// Output is a snapshot of what the panel shows.
type Output struct {
	Digits [14]byte
	Matrix [32]byte
	LEDs   micropanel.LEDBits // OR of the six layers
}

// LED reports whether LED n is lit.
func (o Output) LED(n int) bool {
	return n >= 0 && n < len(o.LEDs)*8 && o.LEDs[n>>3]&(1<<(n&7)) != 0
}

// Sim is the simulated panel. It is safe for concurrent use: the controller
// calls the Device methods, the window calls the input methods.
type Sim struct {
	mu      sync.Mutex
	plugged bool
	state   micropanel.State
	digits  [14]byte
	matrix  [32]byte
	layers  [6]micropanel.LEDBits

	// session is the input queue of the running session (nil when none).
	session chan micropanel.State
	gone    chan struct{} // closed when unplugged during a session

	changed func() // output or input changed (redraw)
}

// New returns a plugged-in simulated panel with every knob centred.
func New() *Sim {
	s := &Sim{plugged: true}
	for i := range s.state.Analog {
		s.state.Analog[i] = micropanel.AnalogMax / 2
	}
	s.state.AnalogRead = true
	return s
}

// OnChange sets a func called (from any goroutine, without the lock held)
// after the outputs or inputs changed.
func (s *Sim) OnChange(fn func()) {
	s.mu.Lock()
	s.changed = fn
	s.mu.Unlock()
}

func (s *Sim) notify() {
	s.mu.Lock()
	fn := s.changed
	s.mu.Unlock()
	if fn != nil {
		fn()
	}
}

// --- plugging ----------------------------------------------------------

// Plugged reports whether the simulated panel is plugged in.
func (s *Sim) Plugged() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.plugged
}

// Present is an app.PanelProbe.
func (s *Sim) Present() (bool, error) { return s.Plugged(), nil }

// SetPlugged plugs or unplugs the panel. Unplugging ends the running
// session with an error, like a USB read failure, and blanks the panel.
func (s *Sim) SetPlugged(on bool) {
	s.mu.Lock()
	if s.plugged == on {
		s.mu.Unlock()
		return
	}
	s.plugged = on
	if !on {
		if s.gone != nil {
			close(s.gone)
			s.gone = nil
		}
		s.digits, s.matrix, s.layers = [14]byte{}, [32]byte{}, [6]micropanel.LEDBits{}
	}
	s.mu.Unlock()
	s.notify()
}

// Open is a panel.Options.Open: it returns the simulated panel, or
// panel.ErrNoDevice while it is unplugged. Like micropanel.Open, it clears
// every LED layer.
func (s *Sim) Open() (panel.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.plugged {
		return nil, panel.ErrNoDevice
	}
	s.layers = [6]micropanel.LEDBits{}
	return s, nil
}

// --- panel.Device ------------------------------------------------------

var _ panel.Device = (*Sim)(nil)

// Serial returns Serial.
func (s *Sim) Serial() string { return Serial }

// Run delivers the current input state, then every input change, until ctx
// ends or the panel is unplugged.
func (s *Sim) Run(ctx context.Context, fn func(micropanel.State), raw func([]byte)) error {
	s.mu.Lock()
	if !s.plugged {
		s.mu.Unlock()
		return errUnplugged
	}
	q := make(chan micropanel.State, 256)
	gone := make(chan struct{})
	s.session, s.gone = q, gone
	first := s.snapshotLocked()
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		if s.session == q {
			s.session = nil
			if s.gone == gone {
				s.gone = nil
			}
		}
		s.mu.Unlock()
	}()

	fn(first)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-gone:
			return errUnplugged
		case st := <-q:
			if raw != nil {
				raw(inputReport(st))
			}
			fn(st)
		}
	}
}

// inputReport is the 0x03 key report of a state, for the raw callback.
func inputReport(st micropanel.State) []byte {
	b := make([]byte, 13)
	b[0] = 0x03
	for i, k := range st.Keys {
		if i < 6 {
			binary.LittleEndian.PutUint16(b[1+2*i:], uint16(k))
		}
	}
	return b
}

// SetDigitsRaw sets the 14 seven-segment digits.
func (s *Sim) SetDigitsRaw(seg [14]byte) error {
	return s.output(func() { s.digits = seg })
}

// SetMatrixRaw sets the camera dot matrix.
func (s *Sim) SetMatrixRaw(px [32]byte) error {
	return s.output(func() { s.matrix = px })
}

// WriteReport accepts the output reports 0x02 (LEDs), 0x10 and 0x11.
func (s *Sim) WriteReport(b []byte) error {
	if len(b) == 0 {
		return errors.New("panelsim: empty report")
	}
	switch b[0] {
	case 0x02:
		if len(b) < 32 || b[31] > 5 {
			return fmt.Errorf("panelsim: bad LED report (%d bytes)", len(b))
		}
		var l micropanel.LEDBits
		copy(l[:], b[1:31])
		return s.output(func() { s.layers[b[31]] = l })
	case 0x10:
		var d [14]byte
		copy(d[:], b[1:])
		return s.SetDigitsRaw(d)
	case 0x11:
		var m [32]byte
		copy(m[:], b[1:])
		return s.SetMatrixRaw(m)
	}
	return fmt.Errorf("panelsim: output report 0x%02X not simulated", b[0])
}

// ClearLEDs clears every LED layer except keep (-1 clears all).
func (s *Sim) ClearLEDs(keep int) error {
	return s.output(func() {
		for i := range s.layers {
			if i != keep {
				s.layers[i] = micropanel.LEDBits{}
			}
		}
	})
}

// SetFeature writes feature 0x0E (knob counters, clamped to 0..1024).
func (s *Sim) SetFeature(b []byte) error {
	pos, ok := micropanel.ParseAnalog(b)
	if !ok || len(b) != 19 {
		return fmt.Errorf("panelsim: feature report not simulated (% x)", b)
	}
	s.mu.Lock()
	if !s.plugged {
		s.mu.Unlock()
		return errUnplugged
	}
	for i, v := range pos {
		s.state.Analog[i] = min(v, micropanel.AnalogMax)
	}
	s.mu.Unlock()
	s.notify()
	return nil
}

// GetFeature reads feature 0x0E (19 bytes) or 0x0F (13 bytes).
func (s *Sim) GetFeature(id byte, size int) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.plugged {
		return nil, errUnplugged
	}
	switch {
	case id == 0x0E && size == 19:
		return micropanel.AnalogReport(s.state.Analog), nil
	case id == 0x0F && size == 13:
		b := make([]byte, 13)
		b[0] = 0x0F
		binary.LittleEndian.PutUint32(b[1:], 1)
		binary.LittleEndian.PutUint32(b[5:], uint32(s.state.Lever))
		binary.LittleEndian.PutUint32(b[9:], uint32(s.state.Ring))
		return b, nil
	}
	return nil, fmt.Errorf("panelsim: feature 0x%02X (%d bytes) not simulated", id, size)
}

// Close does nothing: the simulated panel stays available.
func (s *Sim) Close() error { return nil }

func (s *Sim) output(set func()) error {
	s.mu.Lock()
	if !s.plugged {
		s.mu.Unlock()
		return errUnplugged
	}
	set()
	s.mu.Unlock()
	s.notify()
	return nil
}

// --- inputs (from the window) ------------------------------------------

// Output returns what the panel currently shows.
func (s *Sim) Output() Output {
	s.mu.Lock()
	defer s.mu.Unlock()
	o := Output{Digits: s.digits, Matrix: s.matrix}
	for _, l := range s.layers {
		for i := range o.LEDs {
			o.LEDs[i] |= l[i]
		}
	}
	return o
}

// Input returns the current input state.
func (s *Sim) Input() micropanel.State {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.snapshotLocked()
}

// Press holds key k down (down) or releases it. The panel reports at most
// six held keys.
func (s *Sim) Press(k micropanel.Key, down bool) {
	s.input(func() bool {
		held := slices.Contains(s.state.Keys, k)
		switch {
		case down && !held && len(s.state.Keys) < 6:
			s.state.Keys = append(s.state.Keys, k)
		case !down && held:
			s.state.Keys = slices.DeleteFunc(s.state.Keys, func(x micropanel.Key) bool { return x == k })
		default:
			return false
		}
		return true
	})
}

// TurnKnob moves knob i (micropanel.AnalogNames order) by delta counts; the
// counter clamps at 0 and 1024 like the panel's.
func (s *Sim) TurnKnob(i, delta int) {
	if i < 0 || i >= len(s.state.Analog) {
		return
	}
	s.input(func() bool {
		v := min(max(int(s.state.Analog[i])+delta, 0), micropanel.AnalogMax)
		if uint16(v) == s.state.Analog[i] {
			return false
		}
		s.state.Analog[i] = uint16(v)
		return true
	})
}

// SetLever sets the iris lever, -4096 (pulled back, closed) .. 4096.
func (s *Sim) SetLever(v int32) {
	v = clampJoy(v)
	s.input(func() bool {
		if s.state.Lever == v {
			return false
		}
		s.state.Lever = v
		return true
	})
}

// SetRing sets the master black ring, -4096 .. 4096.
func (s *Sim) SetRing(v int32) {
	v = clampJoy(v)
	s.input(func() bool {
		if s.state.Ring == v {
			return false
		}
		s.state.Ring = v
		return true
	})
}

func clampJoy(v int32) int32 {
	return min(max(v, -micropanel.JoystickMax), micropanel.JoystickMax)
}

// input applies change and, when it changed something, queues the new
// state for the running session. A full queue drops the report (a real
// panel would not report faster than the host reads either).
func (s *Sim) input(change func() bool) {
	s.mu.Lock()
	if !change() {
		s.mu.Unlock()
		return
	}
	if s.plugged && s.session != nil {
		select {
		case s.session <- s.snapshotLocked():
		default:
		}
	}
	s.mu.Unlock()
	s.notify()
}

func (s *Sim) snapshotLocked() micropanel.State {
	st := s.state
	st.Keys = slices.Clone(s.state.Keys)
	return st
}
