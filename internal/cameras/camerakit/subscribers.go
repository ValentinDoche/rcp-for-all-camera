package camerakit

import (
	"maps"
	"slices"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Subscribers is the list of functions a camera notifies of its events
// (Camera.Subscribe). The zero value is ready to use; it is safe for
// concurrent use and must not be copied once used.
type Subscribers struct {
	mu   sync.Mutex
	fns  map[int]func(core.Event)
	next int
}

// Add registers fn and returns the function that removes it.
func (s *Subscribers) Add(fn func(core.Event)) (cancel func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.fns == nil {
		s.fns = map[int]func(core.Event){}
	}
	id := s.next
	s.next++
	s.fns[id] = fn
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.fns, id)
	}
}

// Publish sends each event to every subscriber, in order. It runs the
// callbacks outside its lock, so call it after releasing the camera's own
// lock too: a callback may then read the camera.
func (s *Subscribers) Publish(events ...core.Event) {
	if len(events) == 0 {
		return
	}
	s.mu.Lock()
	fns := slices.Collect(maps.Values(s.fns))
	s.mu.Unlock()
	for _, e := range events {
		for _, fn := range fns {
			fn(e)
		}
	}
}

// Clear removes every subscriber (Camera.Close).
func (s *Subscribers) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	clear(s.fns)
}
