package app

import (
	"context"
	"sync"
	"time"
)

// PanelProbe reports whether the control panel is plugged in.
type PanelProbe func() (present bool, err error)

// Host owns the Service lifecycle: it attaches the event emitter and runs
// the background loops. It is kept apart from Service so that the API
// surface stays limited to the GUI methods.
type Host struct {
	svc      *Service
	probe    PanelProbe
	interval time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

func NewHost(svc *Service, probe PanelProbe, interval time.Duration) *Host {
	return &Host{svc: svc, probe: probe, interval: interval}
}

// Service returns the API to bind to the GUI.
func (h *Host) Service() *Service { return h.svc }

// Start attaches the emitter and starts panel detection. Call once.
func (h *Host) Start(ctx context.Context, e Emitter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		return
	}
	h.svc.setEmitter(e)
	ctx, h.cancel = context.WithCancel(ctx)
	h.done = make(chan struct{})
	go h.watchPanel(ctx)
}

// Stop ends the background loops and closes every camera. Idempotent.
func (h *Host) Stop() {
	h.mu.Lock()
	cancel, done := h.cancel, h.done
	h.cancel = nil
	h.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	<-done
	h.svc.closeAll()
}

func (h *Host) watchPanel(ctx context.Context) {
	defer close(h.done)
	t := time.NewTicker(h.interval)
	defer t.Stop()
	for {
		present, err := h.probe()
		st := PanelStatus{Connected: present && err == nil}
		if err != nil {
			st.Detail = "détection USB indisponible"
			h.svc.log.Debug("panel probe failed", "err", err)
		}
		h.svc.setPanel(st)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}
