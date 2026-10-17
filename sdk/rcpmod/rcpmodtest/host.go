// Package rcpmodtest tests camera modules without WebAssembly and without a
// camera. Host plays the application: it answers HTTP requests with a
// handler you give, lets the test push WebSocket messages, runs timers on a
// virtual clock and records the events the module emits. Module and Online
// are the contract every module must pass.
package rcpmodtest

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// Host is a fake application for module tests. It is not safe for
// concurrent use: like the real host, it calls the module one call at a
// time, from the test goroutine.
type Host struct {
	// HTTPHandler answers HTTP requests; nil answers 404.
	HTTPHandler func(method, path string, body []byte) (status int, resp []byte)
	// WSHandler accepts (nil) or refuses a WebSocket; nil accepts all.
	WSHandler func(path string) error

	now     time.Duration
	timers  []*timer
	nextID  int
	sockets []*Socket
	events  []rcpmod.Event
	logs    []string
}

var _ rcpmod.Host = (*Host)(nil)

// NewHost returns a host with no handler and the clock at zero.
func NewHost() *Host { return &Host{} }

// HTTP calls HTTPHandler.
func (h *Host) HTTP(method, path string, body []byte) (int, []byte, error) {
	if h.HTTPHandler == nil {
		return 404, nil, nil
	}
	status, resp := h.HTTPHandler(method, path, slices.Clone(body))
	return status, resp, nil
}

// OpenWebSocket opens a fake socket the test drives with Push and Drop.
func (h *Host) OpenWebSocket(path string, onMessage func([]byte), onClose func(error)) (rcpmod.WebSocket, error) {
	if h.WSHandler != nil {
		if err := h.WSHandler(path); err != nil {
			return nil, err
		}
	}
	h.nextID++
	s := &Socket{Path: path, host: h, id: h.nextID, onMessage: onMessage, onClose: onClose, open: true}
	h.sockets = append(h.sockets, s)
	return s, nil
}

// After schedules fn on the virtual clock (see Advance).
func (h *Host) After(d time.Duration, fn func()) (cancel func()) {
	h.nextID++
	t := &timer{due: h.now + max(d, 0), id: h.nextID, fn: fn}
	h.timers = append(h.timers, t)
	return func() { t.fn = nil }
}

// Emit records the event.
func (h *Host) Emit(e rcpmod.Event) { h.events = append(h.events, e) }

// Log records the line.
func (h *Host) Log(level rcpmod.Level, msg string) {
	h.logs = append(h.logs, fmt.Sprintf("%d %s", level, msg))
}

// Advance moves the virtual clock forward by d and runs, in time order,
// every timer that falls due (including timers they schedule).
func (h *Host) Advance(d time.Duration) {
	end := h.now + d
	for {
		sort.SliceStable(h.timers, func(i, j int) bool { return h.timers[i].due < h.timers[j].due })
		if len(h.timers) == 0 || h.timers[0].due > end {
			break
		}
		t := h.timers[0]
		h.timers = h.timers[1:]
		h.now = t.due
		if t.fn != nil {
			t.fn()
		}
	}
	h.now = end
}

// Pending reports how many timers are armed.
func (h *Host) Pending() int {
	n := 0
	for _, t := range h.timers {
		if t.fn != nil {
			n++
		}
	}
	return n
}

// Events returns every event emitted so far.
func (h *Host) Events() []rcpmod.Event { return slices.Clone(h.events) }

// Last returns the last value emitted for a control.
func (h *Host) Last(id string) (rcpmod.Value, bool) {
	for i := len(h.events) - 1; i >= 0; i-- {
		if h.events[i].Control == id {
			return h.events[i].Value, true
		}
	}
	return rcpmod.Value{}, false
}

// Status returns the last status emitted (StateDisconnected before any).
func (h *Host) Status() rcpmod.Status {
	for i := len(h.events) - 1; i >= 0; i-- {
		if h.events[i].Status != nil {
			return *h.events[i].Status
		}
	}
	return rcpmod.Status{State: rcpmod.StateDisconnected}
}

// Lens returns the last lens emitted.
func (h *Host) Lens() (rcpmod.Lens, bool) {
	for i := len(h.events) - 1; i >= 0; i-- {
		if h.events[i].Lens != nil {
			return *h.events[i].Lens, true
		}
	}
	return rcpmod.Lens{}, false
}

// Logs returns the lines logged so far.
func (h *Host) Logs() []string { return slices.Clone(h.logs) }

// Socket returns the last socket opened on path that is still open, or nil.
func (h *Host) Socket(path string) *Socket {
	for i := len(h.sockets) - 1; i >= 0; i-- {
		if s := h.sockets[i]; s.Path == path && s.open {
			return s
		}
	}
	return nil
}

// Connected builds a camera of m on h, connects it and fails the test
// unless the camera reports StateConnected.
func (h *Host) Connected(t testing.TB, m rcpmod.Module, cfg rcpmod.Config) rcpmod.Camera {
	t.Helper()
	cam, err := m.New(cfg, h)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := cam.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if st := h.Status(); st.State != rcpmod.StateConnected {
		t.Fatalf("status after Connect = %+v, want connected", st)
	}
	return cam
}

type timer struct {
	due time.Duration
	id  int
	fn  func()
}

// Socket is a fake WebSocket. Sent holds what the module sent.
type Socket struct {
	Path      string
	Sent      [][]byte
	host      *Host
	id        int
	onMessage func([]byte)
	onClose   func(error)
	open      bool
}

// Send records the message.
func (s *Socket) Send(text []byte) error {
	if !s.open {
		return errors.New("socket closed")
	}
	s.Sent = append(s.Sent, slices.Clone(text))
	return nil
}

// Close closes the socket from the module's side (onClose is not called).
func (s *Socket) Close() { s.open = false }

// Open reports whether the socket is open.
func (s *Socket) Open() bool { return s.open }

// Push delivers a message from the camera.
func (s *Socket) Push(msg []byte) {
	if s.open && s.onMessage != nil {
		s.onMessage(msg)
	}
}

// Drop closes the socket from the camera's side, as a cable pull would.
func (s *Socket) Drop(err error) {
	if !s.open {
		return
	}
	s.open = false
	if s.onClose != nil {
		s.onClose(err)
	}
}
