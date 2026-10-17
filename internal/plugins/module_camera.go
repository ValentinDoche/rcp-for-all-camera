package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// camera is a core.Camera backed by one instance of a module. Every call
// into the instance goes through call, which serializes them, collects the
// events the module emitted and publishes them once the instance is free,
// so a subscriber may call the camera back.
type camera struct {
	in    *instance
	svc   *services
	specs camerakit.Specs
	subs  camerakit.Subscribers
	log   *slog.Logger

	mu        sync.Mutex // guards the fields below
	status    core.Status
	lens      core.Lens
	closed    bool
	dead      bool          // the module crashed or hung: no more calls
	attempted chan struct{} // closed by the first status after a connect
	attempt   bool          // a connect waits for its first status
}

var (
	_ core.Camera       = (*camera)(nil)
	_ core.LensReporter = (*camera)(nil)
)

func newCamera(m *module, cfg core.CameraConfig) (*camera, error) {
	ctx := context.Background()
	c := &camera{status: core.Status{State: core.StateDisconnected},
		log: m.host.log.With("module", string(m.info.ID), "host", cfg.Host)}
	c.svc = newServices(c, cfg)
	in, err := m.host.instantiate(ctx, m.compiled, c.svc)
	if err != nil {
		return nil, err
	}
	c.in = in
	wcfg := configToWire(cfg)
	resp, err := in.invoke(ctx, rcpmod.Request{Op: rcpmod.OpNew, Config: &wcfg})
	if err == nil && resp.Error != nil {
		err = wireError(resp.Error)
	}
	var specs []rcpmod.ControlSpec
	if err == nil {
		err = in.callResult(ctx, rcpmod.Request{Op: rcpmod.OpControls}, &specs)
	}
	if err != nil {
		in.close(ctx)
		return nil, fmt.Errorf("%s: %w", m.info.ID, err)
	}
	list := make([]core.ControlSpec, 0, len(specs))
	seen := map[core.ControlID]bool{}
	for _, s := range specs {
		cs, err := specToCore(s)
		if err == nil && seen[cs.ID] {
			err = fmt.Errorf("control %s declared twice", cs.ID)
		}
		if err != nil {
			in.close(ctx)
			return nil, fmt.Errorf("%s: %w", m.info.ID, err)
		}
		seen[cs.ID] = true
		list = append(list, cs)
	}
	c.specs = camerakit.NewSpecs(list)
	return c, nil
}

// call runs one request in the instance. A module error is returned as
// the matching core error; a crash or a hang stops the camera for good.
func (c *camera) call(ctx context.Context, req rcpmod.Request) (json.RawMessage, error) {
	c.in.mu.Lock()
	c.mu.Lock()
	if c.closed || c.dead {
		c.mu.Unlock()
		c.in.mu.Unlock()
		return nil, fmt.Errorf("%s: camera stopped: %w", req.Op, core.ErrNotConnected)
	}
	c.mu.Unlock()
	resp, err := c.in.invoke(ctx, req)
	events := c.svc.events
	c.svc.events = nil
	if err != nil {
		c.svc.shutdown()
	}
	c.in.mu.Unlock()
	c.publish(events)
	if err != nil {
		c.die(err)
		return nil, fmt.Errorf("%s: %w", req.Op, err)
	}
	if resp.Error != nil {
		return nil, wireError(resp.Error)
	}
	return resp.Result, nil
}

// die marks the camera stopped after the module crashed or hung.
func (c *camera) die(err error) {
	c.log.Error("camera module stopped", "err", err)
	c.mu.Lock()
	c.dead = true
	c.mu.Unlock()
	c.publish([]core.Event{{Status: &core.Status{State: core.StateError, Detail: "camera module stopped"}}})
}

// publish updates the cached status and lens, then notifies subscribers.
func (c *camera) publish(events []core.Event) {
	if len(events) == 0 {
		return
	}
	c.mu.Lock()
	for _, e := range events {
		switch {
		case e.Status != nil:
			c.status = *e.Status
			if c.attempt && e.Status.State != core.StateConnecting {
				c.attempt = false
				close(c.attempted)
			}
		case e.Lens != nil:
			c.lens = *e.Lens
		}
	}
	c.mu.Unlock()
	c.subs.Publish(events...)
}

func (c *camera) Controls() []core.ControlSpec { return c.specs.All() }

func (c *camera) Status() core.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *camera) Lens() core.Lens {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lens
}

func (c *camera) Subscribe(fn func(core.Event)) (cancel func()) { return c.subs.Add(fn) }

// Connect asks the module to connect, then waits for the first status it
// reports: Connect returns once the first attempt is done, as core.Camera
// requires. The module keeps reconnecting by itself until Close.
func (c *camera) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("connect: camera closed: %w", core.ErrNotConnected)
	}
	done := make(chan struct{})
	c.attempted, c.attempt = done, true
	c.mu.Unlock()
	if _, err := c.call(ctx, rcpmod.Request{Op: rcpmod.OpConnect}); err != nil {
		c.mu.Lock()
		c.attempt = false
		c.mu.Unlock()
		return fmt.Errorf("connect: %w", err)
	}
	select {
	case <-done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if st := c.Status(); st.State != core.StateConnected {
		return fmt.Errorf("connect: %s: %w", st.Detail, core.ErrNotConnected)
	}
	return nil
}

func (c *camera) Close() error {
	ctx := context.Background()
	c.in.mu.Lock()
	c.mu.Lock()
	alreadyClosed, dead := c.closed, c.dead
	c.closed = true
	c.mu.Unlock()
	if !alreadyClosed {
		if !dead {
			_, _ = c.in.invoke(ctx, rcpmod.Request{Op: rcpmod.OpClose})
		}
		c.svc.events = nil
		c.svc.shutdown()
		c.in.close(ctx)
	}
	c.in.mu.Unlock()
	if !alreadyClosed {
		c.publish([]core.Event{{Status: &core.Status{State: core.StateDisconnected}}})
	}
	c.subs.Clear()
	return nil
}

func (c *camera) Get(ctx context.Context, id core.ControlID) (core.Value, error) {
	s, err := c.specs.Find(id)
	if err != nil {
		return core.Value{}, err
	}
	if s.Kind == core.KindAction {
		return core.Trigger(), nil
	}
	raw, err := c.call(ctx, rcpmod.Request{Op: rcpmod.OpGet, ID: string(id)})
	if err != nil {
		return core.Value{}, fmt.Errorf("get %s: %w", id, err)
	}
	var wv rcpmod.Value
	if err := json.Unmarshal(raw, &wv); err != nil {
		return core.Value{}, fmt.Errorf("get %s: module answer malformed", id)
	}
	v, err := valueToCore(wv)
	if err != nil || v.Kind != s.Kind {
		return core.Value{}, fmt.Errorf("get %s: module returned a bad value", id)
	}
	return v, nil
}

// Set checks the value against the spec before the module sees it: the
// module checks it again (two lines of defence).
func (c *camera) Set(ctx context.Context, id core.ControlID, v core.Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	wv := valueToWire(v)
	if _, err := c.call(ctx, rcpmod.Request{Op: rcpmod.OpSet, ID: string(id), Value: &wv}); err != nil {
		return fmt.Errorf("set %s: %w", id, err)
	}
	return nil
}

// readSocket delivers a socket's messages to the module, in order, then
// its closing, unless the module closed it itself.
func (c *camera) readSocket(id int, w *wsConn) {
	for {
		msg, err := w.readText()
		if err != nil {
			c.deliverClosed(id, w, err)
			return
		}
		if _, err := c.call(context.Background(), rcpmod.Request{Op: rcpmod.OpWSMessage, Socket: id, Text: string(msg)}); err != nil {
			c.log.Debug("socket message refused", "err", err)
		}
	}
}

func (c *camera) deliverClosed(id int, w *wsConn, cause error) {
	c.in.mu.Lock()
	open := c.svc.sockets[id] == w
	if open {
		delete(c.svc.sockets, id)
	}
	c.in.mu.Unlock()
	_ = w.Close()
	if !open {
		return // closed by the module or by Close
	}
	if _, err := c.call(context.Background(), rcpmod.Request{Op: rcpmod.OpWSClosed, Socket: id, Error: cause.Error()}); err != nil {
		c.log.Debug("socket close not delivered", "err", err)
	}
}

func (c *camera) fireTimer(id int) {
	c.in.mu.Lock()
	_, armed := c.svc.timers[id]
	delete(c.svc.timers, id)
	c.in.mu.Unlock()
	if !armed {
		return
	}
	if _, err := c.call(context.Background(), rcpmod.Request{Op: rcpmod.OpTimer, Timer: id}); err != nil {
		c.log.Debug("timer not delivered", "err", err)
	}
}
