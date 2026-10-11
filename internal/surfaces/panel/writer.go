package panel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// op is one Service call requested by the panel.
type op struct {
	select_ bool // SelectSlot(slot); otherwise SetControl(slot, id, v)
	slot    int
	id      core.ControlID
	v       core.Value
}

type opKey struct {
	slot int
	id   core.ControlID
}

// writer runs every Service call on one goroutine, so a slow camera never
// blocks the panel. Discrete ops (keys) run in order; continuous ones
// (lever, ring, knobs) are coalesced per control and flushed at most once
// per throttle period, so the latest value is always sent last.
type writer struct {
	api      app.SurfaceAPI
	log      *slog.Logger
	throttle time.Duration
	onError  func(op, error) // called after a failed call (refresh the cache)

	ops  chan op
	mu   sync.Mutex
	cont map[opKey]core.Value
	// order keeps continuous flushes deterministic.
	order []opKey

	logged map[string]bool // dedupe of expected errors
}

func newWriter(api app.SurfaceAPI, log *slog.Logger, throttle time.Duration, onError func(op, error)) *writer {
	return &writer{api: api, log: log, throttle: throttle, onError: onError,
		ops: make(chan op, 64), cont: map[opKey]core.Value{}, logged: map[string]bool{}}
}

// do queues a discrete op. It drops the op (logged) if the queue is full.
func (w *writer) do(o op) {
	select {
	case w.ops <- o:
	default:
		w.log.Warn("panel: command queue full, dropped", "slot", o.slot, "control", o.id)
	}
}

// set queues a continuous write; a newer value replaces a pending one.
func (w *writer) set(slot int, id core.ControlID, v core.Value) {
	w.mu.Lock()
	defer w.mu.Unlock()
	k := opKey{slot, id}
	if _, ok := w.cont[k]; !ok {
		w.order = append(w.order, k)
	}
	w.cont[k] = v
}

// run executes ops until ctx ends. extra is called on the writer goroutine
// whenever it receives from wake (cache refreshes).
func (w *writer) run(ctx context.Context, wake <-chan struct{}, extra func()) {
	t := time.NewTicker(w.throttle)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case o := <-w.ops:
			w.exec(o)
		case <-t.C:
			w.flush()
		case <-wake:
			extra()
		}
	}
}

func (w *writer) flush() {
	w.mu.Lock()
	order, cont := w.order, w.cont
	w.order, w.cont = nil, map[opKey]core.Value{}
	w.mu.Unlock()
	for _, k := range order {
		w.exec(op{slot: k.slot, id: k.id, v: cont[k]})
	}
}

func (w *writer) exec(o op) {
	var err error
	if o.select_ {
		err = w.api.SelectSlot(o.slot)
	} else {
		err = w.api.SetControl(o.slot, string(o.id), o.v)
	}
	if err == nil {
		return
	}
	w.logError(o, err)
	if w.onError != nil {
		w.onError(o, err)
	}
}

// logError logs failures once per (control, error code): NOT_IMPLEMENTED,
// NOT_CONNECTED and CONFIRM_REQUIRED are expected and logged at debug.
func (w *writer) logError(o op, err error) {
	code := app.CodeInternal
	var ae *app.Error
	if errors.As(err, &ae) {
		code = ae.Code
	}
	key := fmt.Sprintf("%d/%s/%v/%s", o.slot, o.id, o.select_, code)
	if w.logged[key] {
		return
	}
	w.logged[key] = true
	level := slog.LevelWarn
	switch code {
	case app.CodeNotImplemented, app.CodeNotConnected, app.CodeConfirmRequired, app.CodeEmptySlot:
		level = slog.LevelDebug
	}
	w.log.Log(context.Background(), level, "panel command failed (further identical errors not logged)",
		"slot", o.slot, "control", o.id, "select", o.select_, "code", code, "err", err)
}
