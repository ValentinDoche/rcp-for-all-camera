package plugins

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// Limits of the services lent to one camera.
const (
	requestTimeout = 3 * time.Second
	dialTimeout    = 5 * time.Second
	pingEvery      = 15 * time.Second
	idleTimeout    = 40 * time.Second
	maxSockets     = 4
	maxTimers      = 64
	maxTimerDelay  = time.Hour
)

// services are what one camera's instance may do outside its sandbox:
// talk to the configured address only, arm timers, emit events. They run
// inside a call into the instance (instance.mu held).
type services struct {
	cam      *camera
	cfg      core.CameraConfig
	hostport string
	dial     dialFunc
	hc       *http.Client

	sockets    map[int]*wsConn
	nextSocket int
	timers     map[int]*time.Timer
	nextTimer  int
	events     []core.Event // emitted during the current call
}

func newServices(cam *camera, cfg core.CameraConfig) *services {
	d := &net.Dialer{Timeout: dialTimeout}
	if ip := net.ParseIP(cfg.BindIP); ip != nil {
		d.LocalAddr = &net.TCPAddr{IP: ip}
	}
	return &services{
		cam: cam, cfg: cfg, hostport: net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)), dial: d.DialContext,
		hc: &http.Client{
			Timeout:   requestTimeout,
			Transport: &http.Transport{DialContext: d.DialContext, MaxIdleConnsPerHost: 2},
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse // a redirect could lead elsewhere
			},
		},
		sockets: map[int]*wsConn{}, timers: map[int]*time.Timer{},
	}
}

func (s *services) handle(ctx context.Context, req rcpmod.HostRequest) rcpmod.HostResponse {
	var resp rcpmod.HostResponse
	var err error
	switch req.Op {
	case rcpmod.HostHTTP:
		resp, err = s.http(ctx, req)
	case rcpmod.HostWSOpen:
		resp, err = s.wsOpen(ctx, req.Path)
	case rcpmod.HostWSSend:
		err = s.wsSend(req.Socket, req.Text)
	case rcpmod.HostWSClose:
		s.wsClose(req.Socket)
	case rcpmod.HostAfter:
		resp, err = s.after(req.Ms)
	case rcpmod.HostCancel:
		if t := s.timers[req.Timer]; t != nil {
			t.Stop()
			delete(s.timers, req.Timer)
		}
	case rcpmod.HostEmit:
		err = s.emit(req.Event)
	default:
		err = fmt.Errorf("unknown service %q", req.Op)
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return resp
}

var methods = map[string]bool{http.MethodGet: true, http.MethodPut: true, http.MethodPost: true, http.MethodPatch: true, http.MethodDelete: true}

// checkPath accepts an absolute path with an optional query: nothing that
// could change the host or split the request.
func checkPath(p string) error {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || len(p) > 2048 {
		return fmt.Errorf("path must be absolute and at most 2048 bytes")
	}
	for i := 0; i < len(p); i++ {
		if c := p[i]; c <= ' ' || c == 0x7f || c == '\\' || c == '#' {
			return fmt.Errorf("path has a forbidden character")
		}
	}
	return nil
}

func (s *services) noAddress() bool { return s.cfg.Host == "" }

func (s *services) http(ctx context.Context, req rcpmod.HostRequest) (rcpmod.HostResponse, error) {
	if s.noAddress() {
		return rcpmod.HostResponse{}, fmt.Errorf("this camera has no address")
	}
	if !methods[req.Method] {
		return rcpmod.HostResponse{}, fmt.Errorf("method not allowed")
	}
	if err := checkPath(req.Path); err != nil {
		return rcpmod.HostResponse{}, err
	}
	if len(req.Body) > maxMessage {
		return rcpmod.HostResponse{}, fmt.Errorf("body too large")
	}
	var body io.Reader
	if req.Body != nil {
		body = bytes.NewReader(req.Body)
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	hr, err := http.NewRequestWithContext(ctx, req.Method, "http://"+s.hostport+req.Path, body)
	if err != nil {
		return rcpmod.HostResponse{}, err
	}
	if req.Body != nil {
		hr.Header.Set("Content-Type", "application/json")
	}
	if s.cfg.Username != "" {
		hr.SetBasicAuth(s.cfg.Username, s.cfg.Password)
	}
	s.cam.log.Debug("command", "method", req.Method, "path", req.Path)
	r, err := s.hc.Do(hr)
	if err != nil {
		return rcpmod.HostResponse{}, err
	}
	defer r.Body.Close()
	data, err := io.ReadAll(io.LimitReader(r.Body, maxMessage+1))
	if err != nil {
		return rcpmod.HostResponse{}, err
	}
	if len(data) > maxMessage {
		return rcpmod.HostResponse{}, fmt.Errorf("answer larger than %d bytes", maxMessage)
	}
	return rcpmod.HostResponse{Status: r.StatusCode, Body: data}, nil
}

func (s *services) wsOpen(ctx context.Context, path string) (rcpmod.HostResponse, error) {
	if s.noAddress() {
		return rcpmod.HostResponse{}, fmt.Errorf("this camera has no address")
	}
	if err := checkPath(path); err != nil {
		return rcpmod.HostResponse{}, err
	}
	if len(s.sockets) >= maxSockets {
		return rcpmod.HostResponse{}, fmt.Errorf("at most %d sockets", maxSockets)
	}
	dctx, cancel := context.WithTimeout(ctx, dialTimeout)
	w, err := dialWS(dctx, s.dial, s.hostport, path)
	cancel()
	if err != nil {
		return rcpmod.HostResponse{}, err
	}
	w.idle = idleTimeout
	s.nextSocket++
	id := s.nextSocket
	s.sockets[id] = w
	go s.cam.readSocket(id, w)
	go keepAlive(w)
	return rcpmod.HostResponse{Socket: id}, nil
}

func (s *services) wsSend(id int, text string) error {
	w := s.sockets[id]
	if w == nil {
		return fmt.Errorf("socket %d is not open", id)
	}
	if len(text) > maxMessage {
		return fmt.Errorf("message too large")
	}
	return w.writeText([]byte(text))
}

// wsClose closes a socket the module no longer wants; its reader then
// stops without telling the module.
func (s *services) wsClose(id int) {
	if w := s.sockets[id]; w != nil {
		delete(s.sockets, id)
		_ = w.Close()
	}
}

func (s *services) after(ms int64) (rcpmod.HostResponse, error) {
	if len(s.timers) >= maxTimers {
		return rcpmod.HostResponse{}, fmt.Errorf("at most %d timers", maxTimers)
	}
	d := min(max(time.Duration(ms)*time.Millisecond, 0), maxTimerDelay)
	s.nextTimer++
	id := s.nextTimer
	s.timers[id] = time.AfterFunc(d, func() { s.cam.fireTimer(id) })
	return rcpmod.HostResponse{Timer: id}, nil
}

func (s *services) emit(e *rcpmod.Event) error {
	if e == nil {
		return fmt.Errorf("emit without event")
	}
	ce, err := eventToCore(*e, s.cam.specs)
	if err != nil {
		return err
	}
	s.events = append(s.events, ce)
	return nil
}

// shutdown closes every socket and stops every timer. It does not wait for
// the socket readers: they find the camera closed.
func (s *services) shutdown() {
	for id, w := range s.sockets {
		_ = w.Close()
		delete(s.sockets, id)
	}
	for id, t := range s.timers {
		t.Stop()
		delete(s.timers, id)
	}
	s.hc.CloseIdleConnections()
}

// keepAlive pings so a dead link fails the read timeout; it stops when the
// socket is closed.
func keepAlive(w *wsConn) {
	t := time.NewTicker(pingEvery)
	defer t.Stop()
	for range t.C {
		if w.writePing() != nil {
			_ = w.Close()
			return
		}
	}
}
