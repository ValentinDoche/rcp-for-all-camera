package rcpmod

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// echoModule is a tiny module that exercises every Host call.
type echoModule struct{}

func (echoModule) Info() Info { return Info{ID: "echo", Brand: "Test", Name: "Echo"} }

func (echoModule) New(cfg Config, host Host) (Camera, error) {
	return &echoCamera{host: host, specs: NewSpecs([]ControlSpec{Range(Gain, 0, 10, 1, "dB")})}, nil
}

type echoCamera struct {
	host  Host
	specs Specs
	ws    WebSocket
	got   []string
}

func (c *echoCamera) Controls() []ControlSpec { return c.specs.All() }
func (c *echoCamera) Close()                  {}

func (c *echoCamera) Connect() error {
	ws, err := c.host.OpenWebSocket("/events", func(b []byte) { c.got = append(c.got, string(b)) }, func(error) {})
	if err != nil {
		return err
	}
	c.ws = ws
	c.host.After(time.Second, func() { c.host.Emit(StatusEvent(StateConnected, "")) })
	return nil
}

func (c *echoCamera) Get(id string) (Value, error) {
	if _, err := c.specs.Find(id); err != nil {
		return Value{}, err
	}
	return Number(float64(len(c.got))), nil
}

func (c *echoCamera) Set(id string, v Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	status, _, err := c.host.HTTP("PUT", "/gain", nil)
	if err != nil || status != 200 {
		return errors.New("refused")
	}
	return nil
}

// fakeTransport answers HostRequests like the application would.
type fakeTransport struct {
	reqs []HostRequest
}

func (f *fakeTransport) call(r HostRequest) (HostResponse, error) {
	f.reqs = append(f.reqs, r)
	switch r.Op {
	case HostWSOpen:
		return HostResponse{Socket: 7}, nil
	case HostAfter:
		return HostResponse{Timer: 3}, nil
	case HostHTTP:
		return HostResponse{Status: 200}, nil
	}
	return HostResponse{}, nil
}

func call(t *testing.T, d *dispatcher, req Request) Response {
	t.Helper()
	in, _ := json.Marshal(req)
	var resp Response
	if err := json.Unmarshal(d.handle(in), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestDispatcherRunsAModule(t *testing.T) {
	tr := &fakeTransport{}
	d := newDispatcher(echoModule{}, tr.call, func(Level, string) {})
	if r := call(t, d, Request{Op: OpNew, Config: &Config{Model: "echo"}}); r.Error != nil {
		t.Fatalf("new: %+v", r.Error)
	}
	call(t, d, Request{Op: OpConnect})
	call(t, d, Request{Op: OpWSMessage, Socket: 7, Text: "hello"})
	call(t, d, Request{Op: OpTimer, Timer: 3})

	t.Run("a socket message reaches its callback", func(t *testing.T) {
		r := call(t, d, Request{Op: OpGet, ID: Gain})
		if r.Result.(map[string]any)["number"] != 1.0 {
			t.Errorf("get = %+v, want one message received", r.Result)
		}
	})
	t.Run("a timer runs its function, which emits", func(t *testing.T) {
		last := tr.reqs[len(tr.reqs)-1]
		if last.Op != HostEmit || last.Event.Status.State != StateConnected {
			t.Errorf("last host call %+v, want the connected event", last)
		}
	})
	t.Run("errors cross as codes", func(t *testing.T) {
		v := Number(99)
		r := call(t, d, Request{Op: OpSet, ID: Gain, Value: &v})
		if r.Error == nil || r.Error.Code != CodeInvalidValue {
			t.Errorf("set out of range = %+v, want INVALID_VALUE", r.Error)
		}
	})
}

func TestDispatcherNeedsNewFirst(t *testing.T) {
	d := newDispatcher(echoModule{}, (&fakeTransport{}).call, func(Level, string) {})
	if r := call(t, d, Request{Op: OpConnect}); r.Error == nil {
		t.Error("connect before new must fail")
	}
}
