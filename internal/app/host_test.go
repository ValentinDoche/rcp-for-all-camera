package app

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/config"
)

func TestHostPollsPanelUntilStopped(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	var present atomic.Bool
	present.Store(true)
	probe := func() (bool, error) { return present.Load(), nil }

	h := NewHost(f.svc, probe, 5*time.Millisecond)
	h.Start(context.Background(), f.events)
	defer h.Stop()

	waitFor(t, func() bool { return f.svc.PanelStatus().Connected })
	present.Store(false)
	waitFor(t, func() bool { return !f.svc.PanelStatus().Connected })
}

func TestHostReportsProbeErrorAsDisconnected(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	probe := func() (bool, error) { return false, errors.New("setupapi failed") }
	h := NewHost(f.svc, probe, 5*time.Millisecond)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	waitFor(t, func() bool { return f.svc.PanelStatus().Detail != "" })
	if f.svc.PanelStatus().Connected {
		t.Fatal("probe error reported as connected")
	}
}

func TestHostStopIsIdempotent(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	h := NewHost(f.svc, func() (bool, error) { return false, nil }, time.Millisecond)
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
	ran    chan SurfaceAPI
	report chan func(SurfaceStatus)
	ended  chan struct{}
}

func newFakeSurface() *fakeSurface {
	return &fakeSurface{ran: make(chan SurfaceAPI, 1), report: make(chan func(SurfaceStatus), 1), ended: make(chan struct{})}
}

func (s *fakeSurface) Run(ctx context.Context, api SurfaceAPI, report func(SurfaceStatus)) {
	s.ran <- api
	s.report <- report
	<-ctx.Done()
	close(s.ended)
}

func TestHostFansEventsOutToSurfaceAndGUI(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	h := NewHost(f.svc, func() (bool, error) { return false, nil }, time.Hour)
	s := newFakeSurface()
	h.SetSurface(s)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	<-s.ran

	if _, err := f.svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	if f.events.count(EventRigChanged) != 1 || s.count(EventRigChanged) != 1 {
		t.Fatalf("rig:changed GUI=%d surface=%d, want 1 and 1", f.events.count(EventRigChanged), s.count(EventRigChanged))
	}
}

func TestHostSubscribeAndCancel(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	h := NewHost(f.svc, func() (bool, error) { return false, nil }, time.Hour)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	_, _ = f.svc.AssignSlot(2, validInput())
	_, _ = f.svc.AssignSlot(3, validInput())
	f.events.mu.Lock()
	f.events.events = nil // count only the selections below
	f.events.mu.Unlock()
	extra := &recorder{}
	cancel := h.Subscribe(extra)
	f.svc.SelectSlot(2)
	cancel()
	f.svc.SelectSlot(3)
	if extra.count(EventRigChanged) != 1 || f.events.count(EventRigChanged) != 2 {
		t.Fatalf("extra=%d gui=%d, want 1 and 2", extra.count(EventRigChanged), f.events.count(EventRigChanged))
	}
}

func TestHostStopEndsSurface(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	h := NewHost(f.svc, func() (bool, error) { return false, nil }, time.Hour)
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
	f := newFixture(t, config.Rig{Version: config.Version})
	var present atomic.Bool
	present.Store(true)
	h := NewHost(f.svc, func() (bool, error) { return present.Load(), nil }, 5*time.Millisecond)
	s := newFakeSurface()
	h.SetSurface(s)
	h.Start(context.Background(), f.events)
	defer h.Stop()
	<-s.ran
	report := <-s.report

	waitFor(t, func() bool { return f.svc.PanelStatus() == PanelStatus{Connected: true} })
	report(SurfaceStatus{Detail: "utilisé par un autre logiciel"})
	waitFor(t, func() bool {
		return f.svc.PanelStatus() == PanelStatus{Connected: true, Detail: "utilisé par un autre logiciel"}
	})
	report(SurfaceStatus{Active: true, Serial: "ABC"})
	waitFor(t, func() bool { return f.svc.PanelStatus() == PanelStatus{Connected: true, Active: true, Serial: "ABC"} })
}

func TestPanelStatusOf(t *testing.T) {
	for _, tc := range []struct {
		present, probeErr bool
		s                 SurfaceStatus
		want              PanelStatus
	}{
		{false, false, SurfaceStatus{}, PanelStatus{}},
		{true, false, SurfaceStatus{}, PanelStatus{Connected: true}},
		{false, true, SurfaceStatus{}, PanelStatus{Detail: "détection USB indisponible"}},
		// a stale busy detail is hidden once the panel is unplugged
		{false, false, SurfaceStatus{Detail: "busy"}, PanelStatus{}},
		{true, false, SurfaceStatus{Detail: "busy"}, PanelStatus{Connected: true, Detail: "busy"}},
		// driven wins over a probe hiccup
		{false, true, SurfaceStatus{Active: true, Serial: "S"}, PanelStatus{Connected: true, Active: true, Serial: "S"}},
	} {
		if got := panelStatusOf(tc.present, tc.probeErr, tc.s); got != tc.want {
			t.Errorf("panelStatusOf(%v, %v, %+v) = %+v, want %+v", tc.present, tc.probeErr, tc.s, got, tc.want)
		}
	}
}
