package rcpmodtest_test

import (
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod/rcpmodtest"
)

// lamp is a small module: a socket, a timer, a custom block.
type lamp struct{}

func (lamp) Info() rcpmod.Info {
	return rcpmod.Info{ID: "acme-lamp", Brand: "Acme", Name: "Lamp", Transports: []string{rcpmod.TransportEthernet},
		DefaultPort: 80, Auth: rcpmod.AuthNone, Support: rcpmod.SupportPlanned}
}

func (lamp) New(cfg rcpmod.Config, host rcpmod.Host) (rcpmod.Camera, error) {
	return &lampCam{host: host, specs: rcpmod.NewSpecs([]rcpmod.ControlSpec{
		rcpmod.Toggle(rcpmod.Tally),
		rcpmod.Custom(rcpmod.Labeled(rcpmod.ReadOnly(rcpmod.ControlSpec{ID: "acme.firmware", Kind: rcpmod.KindText}), "Firmware", rcpmod.GroupSystem), "About", rcpmod.WidgetReadout),
	})}, nil
}

type lampCam struct {
	host  rcpmod.Host
	specs rcpmod.Specs
	msgs  int
}

func (c *lampCam) Controls() []rcpmod.ControlSpec { return c.specs.All() }
func (c *lampCam) Close()                         {}

func (c *lampCam) Connect() error {
	_, err := c.host.OpenWebSocket("/events", func([]byte) { c.msgs++ }, func(error) {
		c.host.Emit(rcpmod.StatusEvent(rcpmod.StateDisconnected, "dropped"))
	})
	if err != nil {
		return err
	}
	c.host.After(2*time.Second, func() { c.host.Emit(rcpmod.StatusEvent(rcpmod.StateConnected, "")) })
	return nil
}

func (c *lampCam) Get(id string) (rcpmod.Value, error) {
	if _, err := c.specs.Find(id); err != nil {
		return rcpmod.Value{}, err
	}
	if id == "acme.firmware" {
		return rcpmod.Text("1.2"), nil
	}
	return rcpmod.Bool(c.msgs > 0), nil
}

func (c *lampCam) Set(id string, v rcpmod.Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	return rcpmod.ErrNotConnected
}

func TestContract(t *testing.T) { rcpmodtest.Module(t, lamp{}) }

func TestHostRunsTimersOnAVirtualClock(t *testing.T) {
	h := rcpmodtest.NewHost()
	cam, _ := lamp{}.New(rcpmod.Config{}, h)
	_ = cam.Connect()
	h.Advance(time.Second)
	if h.Status().State == rcpmod.StateConnected {
		t.Fatal("connected before its timer")
	}
	h.Advance(time.Second)
	if h.Status().State != rcpmod.StateConnected {
		t.Errorf("status after 2 s = %+v, want connected", h.Status())
	}
}

func TestHostDrivesSockets(t *testing.T) {
	h := rcpmodtest.NewHost()
	cam, _ := lamp{}.New(rcpmod.Config{}, h)
	_ = cam.Connect()
	h.Socket("/events").Push([]byte("{}"))
	t.Run("a pushed message reaches the module", func(t *testing.T) {
		if v, _ := cam.Get(rcpmod.Tally); !v.On {
			t.Error("the module did not receive the message")
		}
	})
	t.Run("a dropped socket calls onClose", func(t *testing.T) {
		h.Socket("/events").Drop(nil)
		if h.Status().Detail != "dropped" {
			t.Errorf("status %+v, want the drop reported", h.Status())
		}
	})
}

func TestOnline(t *testing.T) {
	h := rcpmodtest.NewHost()
	cam, _ := lamp{}.New(rcpmod.Config{}, h)
	_ = cam.Connect()
	h.Advance(2 * time.Second)
	rcpmodtest.Online(t, cam)
}
