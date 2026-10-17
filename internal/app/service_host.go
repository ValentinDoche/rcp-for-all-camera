package app

import (
	"context"
	"sync"
	"time"
)

// PanelProbe reports whether the control panel is plugged in.
type PanelProbe func() (present bool, err error)

// Host owns the Service lifecycle: it attaches the event emitters and runs
// the background loops (panel detection, control surface). It is kept apart
// from Service so that the API surface stays limited to the GUI methods.
type Host struct {
	svc      *Service
	probe    PanelProbe
	interval time.Duration
	events   fanout

	mu      sync.Mutex
	surface Surface
	cancel  context.CancelFunc
	wg      sync.WaitGroup

	stMu     sync.Mutex
	present  bool
	probeErr bool
	surf     SurfaceStatus
}

func NewHost(svc *Service, probe PanelProbe, interval time.Duration) *Host {
	return &Host{svc: svc, probe: probe, interval: interval}
}

// Service returns the API to bind to the GUI.
func (h *Host) Service() *Service { return h.svc }

// SetSurface registers the control surface Start will drive. Call before Start.
func (h *Host) SetSurface(s Surface) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.surface = s
}

// Subscribe adds an extra event listener next to the GUI emitter. Emit must
// not block. Call the returned func to unsubscribe.
func (h *Host) Subscribe(e Emitter) (cancel func()) { return h.events.add(e) }

// Start attaches the emitter, starts panel detection and the control
// surface. Call once.
func (h *Host) Start(ctx context.Context, e Emitter) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cancel != nil {
		return
	}
	if e != nil {
		h.events.add(e)
	}
	if h.surface != nil {
		h.events.add(h.surface)
	}
	h.svc.setEmitter(&h.events)
	ctx, h.cancel = context.WithCancel(ctx)
	h.wg.Add(1)
	go h.watchPanel(ctx)
	h.svc.connectAll()
	if s := h.surface; s != nil {
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			s.Run(ctx, h.svc, h.setSurfaceStatus)
		}()
	}
}

// Stop ends the background loops (the surface clears its device) and
// closes every camera. Idempotent.
func (h *Host) Stop() {
	h.mu.Lock()
	cancel := h.cancel
	h.cancel = nil
	h.mu.Unlock()
	if cancel == nil {
		return
	}
	cancel()
	h.wg.Wait()
	h.svc.closeAll()
}

func (h *Host) watchPanel(ctx context.Context) {
	defer h.wg.Done()
	t := time.NewTicker(h.interval)
	defer t.Stop()
	for {
		present, err := h.probe()
		if err != nil {
			h.svc.log.Debug("panel probe failed", "err", err)
		}
		h.stMu.Lock()
		h.present, h.probeErr = present && err == nil, err != nil
		h.stMu.Unlock()
		h.publishPanel()
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (h *Host) setSurfaceStatus(s SurfaceStatus) {
	h.stMu.Lock()
	h.surf = s
	h.stMu.Unlock()
	h.publishPanel()
}

func (h *Host) publishPanel() {
	// Held across setPanel so two publishers cannot reorder their updates.
	h.stMu.Lock()
	defer h.stMu.Unlock()
	h.svc.setPanel(panelStatusOf(h.present, h.probeErr, h.surf))
}

// panelStatusOf merges USB detection and the surface state. A driven panel
// is plugged in whatever the probe says.
// DetailNoUSBProbe is the panel detail when USB detection itself failed.
const DetailNoUSBProbe = "USB detection unavailable"

func panelStatusOf(present, probeErr bool, s SurfaceStatus) PanelStatus {
	st := PanelStatus{Connected: present || s.Active, Active: s.Active}
	if st.Connected {
		st.Serial = s.Serial
		if !s.Active {
			st.Detail = s.Detail
		}
	} else if probeErr {
		st.Detail = DetailNoUSBProbe
	}
	return st
}
