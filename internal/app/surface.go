package app

import (
	"context"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// SurfaceAPI is the part of Service a control surface drives. *Service
// implements it; tests use fakes.
type SurfaceAPI interface {
	Rig() Rig
	Controls(slot int) ([]ControlView, error)
	SetControl(slot int, id string, v core.Value) error
	SelectSlot(slot int) error
}

// SurfaceStatus is what a surface reports about its device.
type SurfaceStatus struct {
	Active bool   // the device is open and driven by the app
	Serial string // device serial, when known
	Detail string // short French reason when present but not driven
}

// Surface is a physical control surface driven by the app (the USB panel
// controller in internal/surfaces/panel). It receives every event the GUI
// receives through Emit, which must not block.
type Surface interface {
	Emitter
	// Run drives the surface until ctx ends, reporting device state changes.
	Run(ctx context.Context, api SurfaceAPI, report func(SurfaceStatus))
}

var _ SurfaceAPI = (*Service)(nil)

// fanout delivers each event to every subscribed emitter, in subscription order.
type fanout struct {
	mu   sync.RWMutex
	next int
	subs []fanoutSub
}

type fanoutSub struct {
	id int
	e  Emitter
}

func (f *fanout) Emit(name string, payload any) {
	f.mu.RLock()
	subs := f.subs
	f.mu.RUnlock()
	for _, s := range subs {
		s.e.Emit(name, payload)
	}
}

// add subscribes e and returns a func that unsubscribes it.
func (f *fanout) add(e Emitter) (cancel func()) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.next++
	id := f.next
	// Copy on write: Emit iterates a snapshot without holding the lock.
	f.subs = append(append([]fanoutSub(nil), f.subs...), fanoutSub{id, e})
	return func() {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := make([]fanoutSub, 0, len(f.subs))
		for _, s := range f.subs {
			if s.id != id {
				out = append(out, s)
			}
		}
		f.subs = out
	}
}
