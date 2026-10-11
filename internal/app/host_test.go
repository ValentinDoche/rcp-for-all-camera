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
