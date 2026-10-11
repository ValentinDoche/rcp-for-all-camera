package app

import (
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// pushCamera lets a test push events as if they came from the camera.
type pushCamera struct {
	*camerakit.Placeholder
	mu        sync.Mutex
	fn        func(core.Event)
	cancelled bool
}

func (c *pushCamera) Subscribe(fn func(core.Event)) func() {
	c.mu.Lock()
	c.fn = fn
	c.mu.Unlock()
	return func() { c.mu.Lock(); c.cancelled = true; c.mu.Unlock() }
}

func (c *pushCamera) push(e core.Event) {
	c.mu.Lock()
	fn := c.fn
	c.mu.Unlock()
	if fn != nil {
		fn(e)
	}
}

type pushDriver struct {
	testDriver
	mu   sync.Mutex
	cams []*pushCamera
}

func (d *pushDriver) New(core.ModelInfo, core.CameraConfig) (core.Camera, error) {
	c := &pushCamera{Placeholder: camerakit.NewPlaceholder(nil)}
	d.mu.Lock()
	d.cams = append(d.cams, c)
	d.mu.Unlock()
	return c, nil
}

type eventLog struct {
	mu  sync.Mutex
	got []CameraEvent
}

func (l *eventLog) Emit(name string, p any) {
	if name != EventCamera {
		return
	}
	l.mu.Lock()
	l.got = append(l.got, p.(CameraEvent))
	l.mu.Unlock()
}

func newPushService(t *testing.T) (*Service, *pushDriver, *eventLog) {
	t.Helper()
	reg := core.NewRegistry()
	drv := &pushDriver{}
	if err := reg.Register(drv); err != nil {
		t.Fatal(err)
	}
	svc, err := NewService(Options{Registry: reg, Store: &memStore{rig: config.Rig{Version: config.Version}},
		Secrets: config.NewMemorySecrets(), Go: func(f func()) { f() }})
	if err != nil {
		t.Fatal(err)
	}
	log := &eventLog{}
	svc.setEmitter(log)
	return svc, drv, log
}

func TestCameraEventsAreForwardedWithTheirSlot(t *testing.T) {
	svc, drv, log := newPushService(t)
	_, _ = svc.AssignSlot(4, validInput())
	drv.cams[0].push(core.Event{Control: core.Iris, Value: core.Number(5.6)})
	if len(log.got) != 1 || log.got[0].Slot != 4 || log.got[0].Event.Control != core.Iris {
		t.Fatalf("camera events = %+v, want one iris event on slot 4", log.got)
	}
}

func TestEventsFromAReplacedCameraAreDropped(t *testing.T) {
	svc, drv, log := newPushService(t)
	_, _ = svc.AssignSlot(1, validInput())
	_, _ = svc.AssignSlot(1, validInput()) // replaces the camera
	old := drv.cams[0]
	if !old.cancelled {
		t.Fatal("subscription of the replaced camera not cancelled")
	}
	old.push(core.Event{Control: core.Iris, Value: core.Number(2)})
	if len(log.got) != 0 {
		t.Fatalf("stale camera event forwarded: %+v", log.got)
	}
}

func TestEventsAfterClearSlotAreDropped(t *testing.T) {
	svc, drv, log := newPushService(t)
	_, _ = svc.AssignSlot(2, validInput())
	_ = svc.ClearSlot(2)
	drv.cams[0].push(core.Event{Status: &core.Status{State: core.StateConnected}})
	if len(log.got) != 0 {
		t.Fatalf("event of a cleared slot forwarded: %+v", log.got)
	}
}
