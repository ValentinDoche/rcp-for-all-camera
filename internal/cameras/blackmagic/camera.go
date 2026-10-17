package blackmagic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

const (
	apiBase   = "/control/api/v1"
	eventPath = apiBase + "/event/websocket"

	requestTimeout = 3 * time.Second
	dialTimeout    = 5 * time.Second
	pingEvery      = 15 * time.Second
	idleTimeout    = 40 * time.Second
	maxBackoff     = 10 * time.Second
	maxBody        = 1 << 20
	subscribeID    = 1
	lensSubID      = 2
)

// Camera drives one camera through the Blackmagic Camera Control REST API.
// Writes are REST PUTs; state comes from the event WebSocket, whose
// subscription answer carries the current value of every watched property.
// Values are cached and every change is pushed to subscribers.
type Camera struct {
	specs    []core.ControlSpec
	byID     map[core.ControlID]core.ControlSpec
	hostport string
	dial     dialFunc
	hc       *http.Client
	log      *slog.Logger

	mu      sync.Mutex
	status  core.Status
	vals    map[core.ControlID]core.Value
	raw     map[string]map[string]any // last object seen per property
	lens    core.Lens
	fps     float64
	subs    map[int]func(core.Event)
	nextSub int
	started bool
	closed  bool
	cancel  context.CancelFunc
	ws      *wsConn

	done      chan struct{}
	firstDone chan struct{}
	firstErr  error
	firstOnce sync.Once
}

var (
	_ core.Camera       = (*Camera)(nil)
	_ core.LensReporter = (*Camera)(nil)
)

// newCamera builds a camera; it does no I/O.
func newCamera(specs []core.ControlSpec, cfg core.CameraConfig, port int) *Camera {
	d := &net.Dialer{Timeout: dialTimeout}
	if ip := net.ParseIP(cfg.BindIP); ip != nil {
		d.LocalAddr = &net.TCPAddr{IP: ip}
	}
	byID := make(map[core.ControlID]core.ControlSpec, len(specs))
	for _, s := range specs {
		byID[s.ID] = s
	}
	return &Camera{
		specs:    slices.Clone(specs),
		byID:     byID,
		hostport: net.JoinHostPort(cfg.Host, strconv.Itoa(port)),
		dial:     d.DialContext,
		hc: &http.Client{
			Timeout:   requestTimeout,
			Transport: &http.Transport{DialContext: d.DialContext, MaxIdleConnsPerHost: 2},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		log:       slog.Default().With("driver", "blackmagic", "host", cfg.Host),
		status:    core.Status{State: core.StateDisconnected},
		vals:      map[core.ControlID]core.Value{},
		raw:       map[string]map[string]any{},
		subs:      map[int]func(core.Event){},
		done:      make(chan struct{}),
		firstDone: make(chan struct{}),
	}
}

func (c *Camera) Controls() []core.ControlSpec {
	out := make([]core.ControlSpec, len(c.specs))
	for i, s := range c.specs {
		s.Options = slices.Clone(s.Options)
		out[i] = s
	}
	return out
}

// Lens is what the camera last reported about its lens.
func (c *Camera) Lens() core.Lens {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lens
}

func (c *Camera) Status() core.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.status
}

func (c *Camera) Subscribe(fn func(core.Event)) (cancel func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id := c.nextSub
	c.nextSub++
	c.subs[id] = fn
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.subs, id)
	}
}

// Connect starts the connection loop and waits for the first attempt. The
// loop keeps reconnecting until Close, whatever the first attempt returned.
func (c *Camera) Connect(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("connect: camera closed: %w", core.ErrNotConnected)
	}
	if !c.started {
		c.started = true
		runCtx, cancel := context.WithCancel(context.Background())
		c.cancel = cancel
		go c.run(runCtx)
	}
	c.mu.Unlock()
	select {
	case <-c.firstDone:
		return c.firstErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *Camera) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	cancel, ws, started := c.cancel, c.ws, c.started
	c.mu.Unlock()
	if started {
		cancel()
		if ws != nil {
			_ = ws.Close()
		}
		<-c.done
	}
	c.hc.CloseIdleConnections()
	c.mu.Lock()
	clear(c.subs)
	c.mu.Unlock()
	return nil
}

func (c *Camera) Get(ctx context.Context, id core.ControlID) (core.Value, error) {
	s, err := camerakit.FindSpec(c.specs, id)
	if err != nil {
		return core.Value{}, err
	}
	if s.Kind == core.KindAction {
		return core.Trigger(), nil
	}
	c.mu.Lock()
	state := c.status.State
	v, ok := c.vals[id]
	c.mu.Unlock()
	if state != core.StateConnected {
		return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotConnected)
	}
	if ok {
		return v, nil
	}
	// Not reported on the socket yet: read the property directly.
	path, known := idPath[id]
	if !known {
		return core.Value{}, fmt.Errorf("get %s: %w", id, core.ErrNotImplemented)
	}
	body, err := c.request(ctx, http.MethodGet, path, nil)
	if err != nil {
		return core.Value{}, fmt.Errorf("get %s: %w", id, err)
	}
	c.applyJSON(path, body)
	c.mu.Lock()
	v, ok = c.vals[id]
	c.mu.Unlock()
	if !ok {
		return core.Value{}, fmt.Errorf("get %s: camera did not report it: %w", id, core.ErrNotConnected)
	}
	return v, nil
}

// Set validates, then sends one PUT. The new value comes back as an event
// on the socket, which updates the cache and notifies subscribers.
func (c *Camera) Set(ctx context.Context, id core.ControlID, v core.Value) error {
	s, err := camerakit.FindSpec(c.specs, id)
	if err != nil {
		return err
	}
	if err := s.Validate(v); err != nil {
		return err
	}
	cmd, err := commandFor(id, v)
	if err != nil {
		return err
	}
	c.mu.Lock()
	if c.status.State != core.StateConnected {
		c.mu.Unlock()
		return fmt.Errorf("set %s: %w", id, core.ErrNotConnected)
	}
	if cmd.composite {
		// Send every key of the property, starting from the latest state we
		// know, and keep the merge so a quick second write builds on it.
		merged := maps.Clone(c.raw[cmd.path])
		if merged == nil {
			merged = map[string]any{}
		}
		maps.Copy(merged, cmd.body)
		c.raw[cmd.path] = merged
		cmd.body = maps.Clone(merged)
	}
	c.mu.Unlock()

	if _, err := c.request(ctx, http.MethodPut, cmd.path, cmd.body); err != nil {
		if cmd.composite {
			c.resync(cmd.path)
		}
		return fmt.Errorf("set %s: %w", id, err)
	}
	return nil
}

// resync re-reads a property after a failed write, so the merge base is the
// camera's real state again.
func (c *Camera) resync(path string) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	if body, err := c.request(ctx, http.MethodGet, path, nil); err == nil {
		c.applyJSON(path, body)
	}
}

func (c *Camera) request(ctx context.Context, method, path string, body map[string]any) ([]byte, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
		if method != http.MethodGet {
			c.log.Debug("command", "method", method, "path", path, "body", string(b))
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+c.hostport+apiBase+path, rd)
	if err != nil {
		return nil, err
	}
	if rd != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s %s: camera answered HTTP %d", method, path, resp.StatusCode)
	}
	return data, nil
}

// --- connection loop -------------------------------------------------------

func (c *Camera) run(ctx context.Context) {
	defer close(c.done)
	backoff := time.Second
	away := false // unreachable on the last attempt: retries stay "offline"
	for ctx.Err() == nil {
		if !away {
			c.setStatus(core.StateConnecting, "")
		}
		connected, err := c.session(ctx)
		if ctx.Err() != nil {
			break
		}
		if connected {
			backoff = time.Second
		}
		c.log.Debug("connection lost", "err", err)
		// A camera that cannot be reached (off, unplugged, wrong address) is
		// offline; error is kept for a camera that answers but refuses us.
		state := core.StateError
		if unreachable(err) {
			state = core.StateDisconnected
		}
		away = state == core.StateDisconnected
		c.setStatus(state, err.Error())
		c.finishFirst(err) // after the status, so Connect's caller sees it
		select {
		case <-ctx.Done():
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, maxBackoff)
	}
	c.finishFirst(ctx.Err())
	c.setStatus(core.StateDisconnected, "")
}

// unreachable reports whether a session failed because the camera could not
// be reached or the link dropped, rather than because it refused us.
func unreachable(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, errWSClosed) || errors.Is(err, os.ErrDeadlineExceeded) || errors.Is(err, syscall.ECONNRESET)
}

func (c *Camera) finishFirst(err error) {
	c.firstOnce.Do(func() {
		c.firstErr = err
		close(c.firstDone)
	})
}

// session runs one event socket connection until it fails. connected reports
// whether the subscription succeeded, which resets the backoff.
func (c *Camera) session(ctx context.Context) (connected bool, err error) {
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	w, err := dialWS(dctx, c.dial, c.hostport, eventPath)
	cancel()
	if err != nil {
		return false, err
	}
	w.idle = idleTimeout
	c.mu.Lock()
	c.ws = w
	c.mu.Unlock()
	sctx, stop := context.WithCancel(ctx)
	defer func() {
		stop()
		_ = w.Close()
		c.mu.Lock()
		c.ws = nil
		c.mu.Unlock()
	}()
	go keepAlive(sctx, w)

	// The lens descriptions go in their own request: firmwares that do not
	// know them refuse the whole subscription they are part of.
	for _, sub := range []struct {
		id    int
		props []string
	}{{subscribeID, watched}, {lensSubID, lensWatched}} {
		req, _ := json.Marshal(map[string]any{
			"type": "request", "id": sub.id,
			"data": map[string]any{"action": "subscribe", "properties": sub.props},
		})
		if err := w.writeText(req); err != nil {
			return false, err
		}
	}
	var gotMain, gotLens bool
	for {
		msg, err := w.readText()
		if err != nil {
			return connected, err
		}
		id, err := c.handle(msg)
		if err != nil {
			return connected, err
		}
		gotMain = gotMain || id == subscribeID
		gotLens = gotLens || id == lensSubID
		if gotMain && gotLens && !connected {
			connected = true
			c.setStatus(core.StateConnected, "")
			c.finishFirst(nil)
		}
	}
}

// keepAlive pings the camera so a dead link fails the idle timeout, and
// closes the socket when the session ends to unblock the reader.
func keepAlive(ctx context.Context, w *wsConn) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			_ = w.Close()
			return
		case <-t.C:
			if w.writePing() != nil {
				_ = w.Close()
				return
			}
		}
	}
}

type wsMessage struct {
	Type string `json:"type"`
	ID   int    `json:"id"`
	Data struct {
		Action   string                     `json:"action"`
		Property string                     `json:"property"`
		Value    json.RawMessage            `json:"value"`
		Success  bool                       `json:"success"`
		Values   map[string]json.RawMessage `json:"values"`
	} `json:"data"`
}

// handle applies one socket message. resp is the id of the subscription it
// answers, 0 for any other message. A refused lens subscription is not an
// error: the camera then reports no lens limits.
func (c *Camera) handle(msg []byte) (resp int, err error) {
	var m wsMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		c.log.Debug("ignored socket message", "err", err)
		return 0, nil
	}
	switch {
	case m.Type == "event" && m.Data.Action == "propertyValueChanged":
		c.applyJSON(m.Data.Property, m.Data.Value)
	case m.Type == "response" && m.ID == subscribeID:
		if !m.Data.Success {
			return 0, errors.New("camera refused the event subscription")
		}
		// The format first: the shutter conversion needs the frame rate.
		if v, ok := m.Data.Values[pathFormat]; ok {
			c.applyJSON(pathFormat, v)
		}
		for path, v := range m.Data.Values {
			if path != pathFormat {
				c.applyJSON(path, v)
			}
		}
		return subscribeID, nil
	case m.Type == "response" && m.ID == lensSubID:
		if !m.Data.Success {
			c.log.Debug("camera has no lens descriptions")
		}
		for path, v := range m.Data.Values {
			c.applyJSON(path, v)
		}
		return lensSubID, nil
	}
	return 0, nil
}

// applyJSON decodes one property object, updates the cache and notifies
// subscribers of the controls that changed.
func (c *Camera) applyJSON(path string, raw json.RawMessage) {
	dec, lensDec := decoders[path], lensDecoders[path]
	if dec == nil && lensDec == nil {
		return
	}
	var o map[string]any
	if err := json.Unmarshal(raw, &o); err != nil || o == nil {
		return
	}
	c.mu.Lock()
	c.raw[path] = o
	var events []core.Event
	if lensDec != nil {
		old := c.lens
		lensDec(&c.lens, o)
		if c.lens != old {
			l := c.lens
			events = append(events, core.Event{Lens: &l})
		}
	}
	if dec == nil {
		dec = func(*Camera, map[string]any) map[core.ControlID]core.Value { return nil }
	}
	for id, v := range dec(c, o) {
		if _, declared := c.byID[id]; !declared {
			continue
		}
		if old, ok := c.vals[id]; ok && old == v {
			continue
		}
		c.vals[id] = v
		events = append(events, core.Event{Control: id, Value: v})
	}
	subs := slices.Collect(maps.Values(c.subs))
	c.mu.Unlock()
	for _, e := range events {
		for _, fn := range subs {
			fn(e)
		}
	}
}

func (c *Camera) setStatus(state core.ConnState, detail string) {
	c.mu.Lock()
	st := core.Status{State: state, Detail: detail}
	if c.status == st {
		c.mu.Unlock()
		return
	}
	c.status = st
	if state != core.StateConnected {
		clear(c.vals) // stale once the link is gone
		c.lens = core.Lens{}
	}
	subs := slices.Collect(maps.Values(c.subs))
	c.mu.Unlock()
	for _, fn := range subs {
		fn(core.Event{Status: &st})
	}
}
