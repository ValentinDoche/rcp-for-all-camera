package rcpmod

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// dispatcher runs a module: it decodes each Request from the host, calls
// the module or its camera, and encodes the Response. It also implements
// Host for the camera, turning each Host call into a HostRequest through
// call. It is transport-agnostic: the WebAssembly glue gives it the host
// imports, tests give it a function.
type dispatcher struct {
	module  Module
	call    func(HostRequest) (HostResponse, error)
	log     func(Level, string)
	cam     Camera
	sockets map[int]*socket
	timers  map[int]func()
}

func newDispatcher(m Module, call func(HostRequest) (HostResponse, error), log func(Level, string)) *dispatcher {
	return &dispatcher{module: m, call: call, log: log, sockets: map[int]*socket{}, timers: map[int]func(){}}
}

// handle answers one encoded Request.
func (d *dispatcher) handle(in []byte) []byte {
	var req Request
	if err := json.Unmarshal(in, &req); err != nil {
		return encode(Response{Error: &WireError{Code: CodeInternal, Message: "malformed request"}})
	}
	result, err := d.run(req)
	if err != nil {
		return encode(Response{Error: &WireError{Code: CodeOf(err), Message: err.Error()}})
	}
	return encode(Response{Result: result})
}

func encode(r Response) []byte {
	b, err := json.Marshal(r)
	if err != nil {
		b, _ = json.Marshal(Response{Error: &WireError{Code: CodeInternal, Message: "unencodable result"}})
	}
	return b
}

var errNoCamera = errors.New("no camera: call new first")

func (d *dispatcher) run(req Request) (any, error) {
	if req.Op == OpInfo {
		return d.module.Info(), nil
	}
	if req.Op == OpNew {
		if req.Config == nil {
			return nil, fmt.Errorf("new without config: %w", ErrInvalidValue)
		}
		cam, err := d.module.New(*req.Config, hostAPI{d})
		if err != nil {
			return nil, err
		}
		d.cam = cam
		return nil, nil
	}
	if d.cam == nil {
		return nil, errNoCamera
	}
	switch req.Op {
	case OpControls:
		return d.cam.Controls(), nil
	case OpPage:
		if dz, ok := d.cam.(Designer); ok {
			return dz.Page(), nil
		}
		return StandardPage(), nil
	case OpConnect:
		return nil, d.cam.Connect()
	case OpClose:
		d.cam.Close()
		return nil, nil
	case OpGet:
		return d.cam.Get(req.ID)
	case OpSet:
		if req.Value == nil {
			return nil, fmt.Errorf("set without value: %w", ErrInvalidValue)
		}
		return nil, d.cam.Set(req.ID, *req.Value)
	case OpWSMessage:
		if s := d.sockets[req.Socket]; s != nil && s.onMessage != nil {
			s.onMessage([]byte(req.Text))
		}
		return nil, nil
	case OpWSClosed:
		if s := d.sockets[req.Socket]; s != nil {
			delete(d.sockets, req.Socket)
			if s.onClose != nil {
				var err error
				if req.Error != "" {
					err = errors.New(req.Error)
				}
				s.onClose(err)
			}
		}
		return nil, nil
	case OpTimer:
		if fn := d.timers[req.Timer]; fn != nil {
			delete(d.timers, req.Timer)
			fn()
		}
		return nil, nil
	}
	return nil, fmt.Errorf("unknown op %q", req.Op)
}

// hostAPI is the Host a camera receives.
type hostAPI struct{ d *dispatcher }

func (h hostAPI) HTTP(method, path string, body []byte) (int, []byte, error) {
	r, err := h.d.call(HostRequest{Op: HostHTTP, Method: method, Path: path, Body: body})
	if err != nil {
		return 0, nil, err
	}
	return r.Status, r.Body, nil
}

func (h hostAPI) OpenWebSocket(path string, onMessage func([]byte), onClose func(error)) (WebSocket, error) {
	r, err := h.d.call(HostRequest{Op: HostWSOpen, Path: path})
	if err != nil {
		return nil, err
	}
	s := &socket{d: h.d, id: r.Socket, onMessage: onMessage, onClose: onClose}
	h.d.sockets[r.Socket] = s
	return s, nil
}

func (h hostAPI) After(dur time.Duration, fn func()) (cancel func()) {
	r, err := h.d.call(HostRequest{Op: HostAfter, Ms: dur.Milliseconds()})
	if err != nil {
		h.d.log(LevelError, "timer refused: "+err.Error())
		return func() {}
	}
	h.d.timers[r.Timer] = fn
	return func() {
		if _, armed := h.d.timers[r.Timer]; armed {
			delete(h.d.timers, r.Timer)
			_, _ = h.d.call(HostRequest{Op: HostCancel, Timer: r.Timer})
		}
	}
}

func (h hostAPI) Emit(e Event) {
	if _, err := h.d.call(HostRequest{Op: HostEmit, Event: &e}); err != nil {
		h.d.log(LevelError, "event refused: "+err.Error())
	}
}

func (h hostAPI) Log(level Level, msg string) { h.d.log(level, msg) }

// socket is an open WebSocket, known to the dispatcher by its host id.
type socket struct {
	d         *dispatcher
	id        int
	onMessage func([]byte)
	onClose   func(error)
}

func (s *socket) Send(text []byte) error {
	_, err := s.d.call(HostRequest{Op: HostWSSend, Socket: s.id, Text: string(text)})
	return err
}

// Close closes the socket; onClose is not called for a socket the module
// closed itself.
func (s *socket) Close() {
	if _, open := s.d.sockets[s.id]; !open {
		return
	}
	delete(s.d.sockets, s.id)
	_, _ = s.d.call(HostRequest{Op: HostWSClose, Socket: s.id})
}
