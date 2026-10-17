package app_test

import (
	"context"
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"sync/atomic"
	"testing"
	"time"
)

func TestHostPollsPanelUntilStopped(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	var present atomic.Bool
	present.Store(true)
	probe := func() (bool, error) { return present.Load(), nil }

	h := app.NewHost(f.svc, probe, 5*time.Millisecond)
	h.Start(context.Background(), f.events)
	defer h.Stop()

	waitFor(t, func() bool { return f.svc.PanelStatus().Connected })
	present.Store(false)
	waitFor(t, func() bool { return !f.svc.PanelStatus().Connected })
}

func TestHostReportsProbeErrorAsDisconnected(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	probe := func() (bool, error) { return false, errors.New("setupapi failed") }
	h := app.NewHost(f.svc, probe, 5*time.Millisecond)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	waitFor(t, func() bool { return f.svc.PanelStatus().Detail != "" })
	if f.svc.PanelStatus().Connected {
		t.Fatal("probe error reported as connected")
	}
}

func TestHostStopIsIdempotent(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	h := app.NewHost(f.svc, func() (bool, error) { return false, nil }, time.Millisecond)
	h.Start(context.Background(), f.events)
	h.Stop()
	h.Stop()
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 2s")
		}
		time.Sleep(time.Millisecond)
	}
}

type fakeSurface struct {
	recorder
	ran    chan app.SurfaceAPI
	report chan func(app.SurfaceStatus)
	ended  chan struct{}
}

func newFakeSurface() *fakeSurface {
	return &fakeSurface{ran: make(chan app.SurfaceAPI, 1), report: make(chan func(app.SurfaceStatus), 1), ended: make(chan struct{})}
}

func (s *fakeSurface) Run(ctx context.Context, api app.SurfaceAPI, report func(app.SurfaceStatus)) {
	s.ran <- api
	s.report <- report
	<-ctx.Done()
	close(s.ended)
}

func TestHostFansEventsOutToSurfaceAndGUI(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	h := app.NewHost(f.svc, func() (bool, error) { return false, nil }, time.Hour)
	s := newFakeSurface()
	h.SetSurface(s)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	<-s.ran

	if err := f.svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	if f.events.count(app.EventRigChanged) != 1 || s.count(app.EventRigChanged) != 1 {
		t.Fatalf("rig:changed GUI=%d surface=%d, want 1 and 1", f.events.count(app.EventRigChanged), s.count(app.EventRigChanged))
	}
}

func TestHostSubscribeAndCancel(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	h := app.NewHost(f.svc, func() (bool, error) { return false, nil }, time.Hour)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	_ = f.svc.AssignSlot(2, validInput())
	_ = f.svc.AssignSlot(3, validInput())
	f.events.mu.Lock()
	f.events.events = nil // count only the selections below
	f.events.mu.Unlock()
	extra := &recorder{}
	cancel := h.Subscribe(extra)
	f.svc.SelectSlot(2)
	cancel()
	f.svc.SelectSlot(3)
	if extra.count(app.EventRigChanged) != 1 || f.events.count(app.EventRigChanged) != 2 {
		t.Fatalf("extra=%d gui=%d, want 1 and 2", extra.count(app.EventRigChanged), f.events.count(app.EventRigChanged))
	}
}

func TestHostStopEndsSurface(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	h := app.NewHost(f.svc, func() (bool, error) { return false, nil }, time.Hour)
	s := newFakeSurface()
	h.SetSurface(s)
	h.Start(context.Background(), f.events)
	<-s.ran
	h.Stop()
	select {
	case <-s.ended:
	default:
		t.Fatal("Stop returned before the surface ended")
	}
}

func TestHostMergesSurfaceStatus(t *testing.T) {
	f := newFixture(t, app.SavedRig{})
	var present atomic.Bool
	present.Store(true)
	h := app.NewHost(f.svc, func() (bool, error) { return present.Load(), nil }, 5*time.Millisecond)
	s := newFakeSurface()
	h.SetSurface(s)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	<-s.ran
	report := <-s.report

	waitFor(t, func() bool { return f.svc.PanelStatus() == app.PanelStatus{Connected: true} })
	report(app.SurfaceStatus{Detail: "used by another program"})
	waitFor(t, func() bool {
		return f.svc.PanelStatus() == app.PanelStatus{Connected: true, Detail: "used by another program"}
	})
	report(app.SurfaceStatus{Active: true, Serial: "ABC"})
	waitFor(t, func() bool {
		return f.svc.PanelStatus() == app.PanelStatus{Connected: true, Active: true, Serial: "ABC"}
	})
}

func TestPanelStatusOf(t *testing.T) {
	for _, tc := range []struct {
		present, probeErr bool
		s                 app.SurfaceStatus
		want              app.PanelStatus
	}{
		{false, false, app.SurfaceStatus{}, app.PanelStatus{}},
		{true, false, app.SurfaceStatus{}, app.PanelStatus{Connected: true}},
		{false, true, app.SurfaceStatus{}, app.PanelStatus{Detail: app.DetailNoUSBProbe}},
		// a stale busy detail is hidden once the panel is unplugged
		{false, false, app.SurfaceStatus{Detail: "busy"}, app.PanelStatus{}},
		{true, false, app.SurfaceStatus{Detail: "busy"}, app.PanelStatus{Connected: true, Detail: "busy"}},
		// driven wins over a probe hiccup
		{false, true, app.SurfaceStatus{Active: true, Serial: "S"}, app.PanelStatus{Connected: true, Active: true, Serial: "S"}},
	} {
		if got := app.PanelStatusOf(tc.present, tc.probeErr, tc.s); got != tc.want {
			t.Errorf("app.PanelStatusOf(%v, %v, %+v) = %+v, want %+v", tc.present, tc.probeErr, tc.s, got, tc.want)
		}
	}
}
