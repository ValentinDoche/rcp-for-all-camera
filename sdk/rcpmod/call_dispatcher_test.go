package rcpmod_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// echoModule is a tiny module that exercises every Host call.
type echoModule struct{}

func (echoModule) Info() rcpmod.Info { return rcpmod.Info{ID: "echo", Brand: "Test", Name: "Echo"} }

func (echoModule) New(cfg rcpmod.Config, host rcpmod.Host) (rcpmod.Camera, error) {
	return &echoCamera{host: host, specs: rcpmod.NewSpecs([]rcpmod.ControlSpec{rcpmod.Range(rcpmod.Gain, 0, 10, 1, "dB")})}, nil
}

type echoCamera struct {
	host  rcpmod.Host
	specs rcpmod.Specs
	got   []string
}

func (c *echoCamera) Controls() []rcpmod.ControlSpec { return c.specs.All() }
func (c *echoCamera) Close()                         {}

func (c *echoCamera) Connect() error {
	if _, err := c.host.OpenWebSocket("/events", func(b []byte) { c.got = append(c.got, string(b)) }, func(error) {}); err != nil {
		return err
	}
	c.host.After(time.Second, func() { c.host.Emit(rcpmod.StatusEvent(rcpmod.StateConnected, "")) })
	return nil
}

func (c *echoCamera) Get(id string) (rcpmod.Value, error) {
	if _, err := c.specs.Find(id); err != nil {
		return rcpmod.Value{}, err
	}
	return rcpmod.Number(float64(len(c.got))), nil
}

func (c *echoCamera) Set(id string, v rcpmod.Value) error {
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
	reqs []rcpmod.HostRequest
}

func (f *fakeTransport) call(r rcpmod.HostRequest) (rcpmod.HostResponse, error) {
	f.reqs = append(f.reqs, r)
	switch r.Op {
	case rcpmod.HostWSOpen:
		return rcpmod.HostResponse{Socket: 7}, nil
	case rcpmod.HostAfter:
		return rcpmod.HostResponse{Timer: 3}, nil
	case rcpmod.HostHTTP:
		return rcpmod.HostResponse{Status: 200}, nil
	}
	return rcpmod.HostResponse{}, nil
}

func call(t *testing.T, d *rcpmod.Dispatcher, req rcpmod.Request) rcpmod.Response {
	t.Helper()
	in, _ := json.Marshal(req)
	var resp rcpmod.Response
	if err := json.Unmarshal(d.Handle(in), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestDispatcherRunsAModule(t *testing.T) {
	tr := &fakeTransport{}
	d := rcpmod.NewDispatcher(echoModule{}, tr.call, func(rcpmod.Level, string) {})
	if r := call(t, d, rcpmod.Request{Op: rcpmod.OpNew, Config: &rcpmod.Config{Model: "echo"}}); r.Error != nil {
		t.Fatalf("new: %+v", r.Error)
	}
	call(t, d, rcpmod.Request{Op: rcpmod.OpConnect})
	call(t, d, rcpmod.Request{Op: rcpmod.OpWSMessage, Socket: 7, Text: "hello"})
	call(t, d, rcpmod.Request{Op: rcpmod.OpTimer, Timer: 3})

	t.Run("a socket message reaches its callback", func(t *testing.T) {
		r := call(t, d, rcpmod.Request{Op: rcpmod.OpGet, ID: rcpmod.Gain})
		if r.Result.(map[string]any)["number"] != 1.0 {
			t.Errorf("get = %+v, want one message received", r.Result)
		}
	})
	t.Run("a timer runs its function, which emits", func(t *testing.T) {
		last := tr.reqs[len(tr.reqs)-1]
		if last.Op != rcpmod.HostEmit || last.Event.Status.State != rcpmod.StateConnected {
			t.Errorf("last host call %+v, want the connected event", last)
		}
	})
	t.Run("errors cross as codes", func(t *testing.T) {
		v := rcpmod.Number(99)
		r := call(t, d, rcpmod.Request{Op: rcpmod.OpSet, ID: rcpmod.Gain, Value: &v})
		if r.Error == nil || r.Error.Code != rcpmod.CodeInvalidValue {
			t.Errorf("set out of range = %+v, want INVALID_VALUE", r.Error)
		}
	})
	t.Run("a camera without ui.go gets the standard page", func(t *testing.T) {
		r := call(t, d, rcpmod.Request{Op: rcpmod.OpPage})
		rows, _ := r.Result.(map[string]any)["rows"].([]any)
		if len(rows) != 2 {
			t.Errorf("page = %+v, want the standard page", r.Result)
		}
	})
}

func TestDispatcherNeedsNewFirst(t *testing.T) {
	d := rcpmod.NewDispatcher(echoModule{}, (&fakeTransport{}).call, func(rcpmod.Level, string) {})
	if r := call(t, d, rcpmod.Request{Op: rcpmod.OpConnect}); r.Error == nil {
		t.Error("connect before new must fail")
	}
}
