package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

const (
	apiBase     = "/control/api/v1"
	eventPath   = apiBase + "/event/websocket"
	minBackoff  = time.Second
	maxBackoff  = 10 * time.Second
	subscribeID = 1
	lensSubID   = 2
)

// camera drives one camera. The state comes from the event socket, whose
// subscription answer carries the current value of every watched property;
// writes are REST PUTs whose result comes back as socket events. The host
// calls it one call at a time, so it needs no lock.
type camera struct {
	host  rcpmod.Host
	specs rcpmod.Specs

	closed   bool
	ws       rcpmod.WebSocket
	retry    func() // cancels the pending reconnection
	backoff  time.Duration
	status   string
	away     bool // unreachable on the last attempt: retries stay "disconnected"
	gotMain  bool
	gotLens  bool
	vals     map[string]rcpmod.Value
	raw      map[string]map[string]any // last object seen per property
	lens     rcpmod.Lens
	irisDesc bool    // the lens limits come from /lens/iris/description
	fps      float64 // for the shutter speed <-> angle conversion
}

func newCamera(host rcpmod.Host, specs []rcpmod.ControlSpec) *camera {
	return &camera{host: host, specs: rcpmod.NewSpecs(specs), backoff: minBackoff,
		status: rcpmod.StateDisconnected, vals: map[string]rcpmod.Value{}, raw: map[string]map[string]any{}}
}

func (c *camera) Controls() []rcpmod.ControlSpec { return c.specs.All() }

// Connect opens the event socket and subscribes. The camera reports
// "connected" once both subscriptions are answered; on any failure it
// reports why and retries by itself, with a growing delay.
func (c *camera) Connect() error {
	if c.closed {
		return rcpmod.ErrNotConnected
	}
	c.dial()
	return nil
}

func (c *camera) dial() {
	c.retry = nil
	if !c.away {
		c.setStatus(rcpmod.StateConnecting, "")
	}
	ws, err := c.host.OpenWebSocket(eventPath, c.onMessage, c.onClose)
	if err != nil {
		state := rcpmod.StateDisconnected // off, unplugged, wrong address
		if strings.Contains(err.Error(), "upgrade refused") {
			state = rcpmod.StateError // it answers, but is not this camera's API
		}
		c.lost(state, err.Error())
		return
	}
	c.ws, c.gotMain, c.gotLens = ws, false, false
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
		if err := ws.Send(req); err != nil {
			ws.Close()
			c.lost(rcpmod.StateDisconnected, err.Error())
			return
		}
	}
}

// lost reports a failed or dropped connection and schedules the next try.
func (c *camera) lost(state, detail string) {
	c.ws = nil
	c.away = state == rcpmod.StateDisconnected
	clear(c.vals) // stale once the link is gone
	c.lens = rcpmod.Lens{}
	c.setStatus(state, detail)
	if c.closed || c.retry != nil {
		return
	}
	c.retry = c.host.After(c.backoff, c.dial)
	c.backoff = min(c.backoff*2, maxBackoff)
}

func (c *camera) onClose(err error) {
	detail := "connection closed"
	if err != nil {
		detail = err.Error()
	}
	c.lost(rcpmod.StateDisconnected, detail)
}

func (c *camera) setStatus(state, detail string) {
	if c.status == state && state != rcpmod.StateDisconnected {
		return
	}
	c.status = state
	c.host.Emit(rcpmod.StatusEvent(state, detail))
}

func (c *camera) Close() {
	c.closed = true
	if c.retry != nil {
		c.retry()
		c.retry = nil
	}
	if c.ws != nil {
		c.ws.Close()
		c.ws = nil
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

// onMessage applies one socket message. A refused lens subscription is not
// an error: the camera then reports no lens limits.
func (c *camera) onMessage(msg []byte) {
	var m wsMessage
	if err := json.Unmarshal(msg, &m); err != nil {
		c.host.Log(rcpmod.LevelDebug, "ignored socket message")
		return
	}
	switch {
	case m.Type == "event" && m.Data.Action == "propertyValueChanged":
		c.applyJSON(m.Data.Property, m.Data.Value)
	case m.Type == "response" && m.ID == subscribeID:
		if !m.Data.Success {
			if c.ws != nil {
				c.ws.Close()
			}
			c.lost(rcpmod.StateError, "camera refused the event subscription")
			return
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
		c.gotMain = true
	case m.Type == "response" && m.ID == lensSubID:
		if !m.Data.Success {
			c.host.Log(rcpmod.LevelDebug, "camera has no lens descriptions")
		}
		for path, v := range m.Data.Values {
			c.applyJSON(path, v)
		}
		c.gotLens = true
	}
	if c.gotMain && c.gotLens && c.status != rcpmod.StateConnected {
		c.backoff, c.away = minBackoff, false
		c.setStatus(rcpmod.StateConnected, "")
	}
}

// applyJSON decodes one property object, updates the cache and emits the
// controls that changed. The object comes from the network: decoders only
// accept finite numbers and known shapes.
func (c *camera) applyJSON(path string, raw json.RawMessage) {
	dec, lensDec := decoders[path], lensDecoders[path]
	if dec == nil && lensDec == nil {
		return
	}
	var o map[string]any
	if err := json.Unmarshal(raw, &o); err != nil || o == nil {
		return
	}
	c.raw[path] = o
	if lensDec != nil || path == pathIris {
		old := c.lens
		if lensDec != nil {
			lensDec(&c.lens, o)
		}
		if path == pathIrisDesc {
			c.irisDesc = c.lens.IrisMin > 0
		}
		if path == pathIris {
			c.learnIrisEnds(o)
		}
		if c.lens != old {
			c.host.Emit(rcpmod.LensEvent(c.lens))
		}
	}
	if dec == nil {
		return
	}
	for id, v := range dec(c, o) {
		if !c.specs.Has(id) {
			continue
		}
		if old, ok := c.vals[id]; ok && old == v {
			continue
		}
		c.vals[id] = v
		c.host.Emit(rcpmod.ControlEvent(id, v))
	}
}

func (c *camera) Get(id string) (rcpmod.Value, error) {
	s, err := c.specs.Find(id)
	if err != nil {
		return rcpmod.Value{}, err
	}
	if s.Kind == rcpmod.KindAction {
		return rcpmod.Trigger(), nil
	}
	if c.status != rcpmod.StateConnected {
		return rcpmod.Value{}, fmt.Errorf("get %s: %w", id, rcpmod.ErrNotConnected)
	}
	if v, ok := c.vals[id]; ok {
		return v, nil
	}
	// Not reported on the socket yet: read the property directly.
	path, known := idPath[id]
	if !known {
		return rcpmod.Value{}, fmt.Errorf("get %s: %w", id, rcpmod.ErrNotImplemented)
	}
	body, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return rcpmod.Value{}, fmt.Errorf("get %s: %w", id, err)
	}
	c.applyJSON(path, body)
	if v, ok := c.vals[id]; ok {
		return v, nil
	}
	return rcpmod.Value{}, fmt.Errorf("get %s: the camera did not report it: %w", id, rcpmod.ErrNotConnected)
}

// Set validates, then sends one PUT. The new value comes back as an event
// on the socket.
func (c *camera) Set(id string, v rcpmod.Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	cmd, err := commandFor(id, v)
	if err != nil {
		return err
	}
	if c.status != rcpmod.StateConnected {
		return fmt.Errorf("set %s: %w", id, rcpmod.ErrNotConnected)
	}
	if cmd.Composite {
		// Send every key of the property, starting from the latest state we
		// know, and keep the merge so a quick second write builds on it.
		merged := maps.Clone(c.raw[cmd.Path])
		if merged == nil {
			merged = map[string]any{}
		}
		maps.Copy(merged, cmd.Body)
		c.raw[cmd.Path] = merged
		cmd.Body = maps.Clone(merged)
	}
	b, err := json.Marshal(cmd.Body)
	if err != nil {
		return err
	}
	if _, err := c.request(http.MethodPut, cmd.Path, b); err != nil {
		if cmd.Composite {
			c.resync(cmd.Path) // the merge base must be the camera's real state again
		}
		return fmt.Errorf("set %s: %w", id, err)
	}
	return nil
}

func (c *camera) resync(path string) {
	if body, err := c.request(http.MethodGet, path, nil); err == nil {
		c.applyJSON(path, body)
	}
}

var errRefused = errors.New("camera refused the request")

func (c *camera) request(method, path string, body []byte) ([]byte, error) {
	status, resp, err := c.host.HTTP(method, apiBase+path, body)
	if err != nil {
		return nil, err
	}
	if status/100 != 2 {
		return nil, fmt.Errorf("%s %s: HTTP %d: %w", method, path, status, errRefused)
	}
	return resp, nil
}
