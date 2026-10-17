package panelsim

import (
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// fakeAPI is a rig with cameras on slots 1 and 2, slot 1 active.
type fakeAPI struct {
	mu     sync.Mutex
	active int
	armed  bool
}

func (f *fakeAPI) selected() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.active
}

func (f *fakeAPI) Rig() app.Rig {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.active == 0 {
		f.active = 1
	}
	r := app.Rig{Slots: make([]app.Slot, app.SlotCount), Active: f.active}
	for i := range r.Slots {
		r.Slots[i] = app.Slot{Slot: i + 1, Presets: []int{}}
	}
	for _, n := range []int{1, 2} {
		r.Slots[n-1].Camera = &app.CameraView{Model: "demo", Name: "Cam", Status: core.Status{State: core.StateConnected}}
	}
	return r
}

func (f *fakeAPI) Controls(int) ([]app.ControlView, error)  { return nil, nil }
func (f *fakeAPI) Lens(int) (core.Lens, error)              { return core.Lens{}, nil }
func (f *fakeAPI) SetControl(int, string, core.Value) error { return nil }
func (f *fakeAPI) StorePreset(int, int) error               { return nil }
func (f *fakeAPI) RecallPreset(int, int) error              { return nil }
func (f *fakeAPI) SetStoreArmed(on bool)                    { f.mu.Lock(); f.armed = on; f.mu.Unlock() }
func (f *fakeAPI) StoreArmed() bool                         { f.mu.Lock(); defer f.mu.Unlock(); return f.armed }
func (f *fakeAPI) SelectSlot(n int) error                   { f.mu.Lock(); f.active = n; f.mu.Unlock(); return nil }
