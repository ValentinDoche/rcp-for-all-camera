// Command hostile-module is a camera module that misbehaves on purpose,
// for the sandbox tests of internal/plugins. Each control triggers one
// attack; the tests check the host contains it.
package main

import (
	"math"
	"os"
	"strconv"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

func main() {}

func init() { rcpmod.Register(module{}) }

type module struct{}

func (module) Info() rcpmod.Info {
	return rcpmod.Info{ID: "hostile-module", Brand: "Test", Name: "Hostile module",
		Transports: []string{rcpmod.TransportEthernet}, DefaultPort: 80,
		Auth: rcpmod.AuthNone, Support: rcpmod.SupportPlanned}
}

func (module) New(cfg rcpmod.Config, host rcpmod.Host) (rcpmod.Camera, error) {
	return &camera{host: host}, nil
}

type camera struct {
	host rcpmod.Host
	hog  [][]byte
}

// Every attack is an action; read-only text controls report what the
// module could see.
func (c *camera) Controls() []rcpmod.ControlSpec {
	act := func(id string) rcpmod.ControlSpec {
		return rcpmod.Labeled(rcpmod.Action(id), id, rcpmod.GroupMisc)
	}
	txt := func(id string) rcpmod.ControlSpec {
		return rcpmod.Labeled(rcpmod.ReadOnly(rcpmod.ControlSpec{ID: id, Kind: rcpmod.KindText}), id, rcpmod.GroupMisc)
	}
	return []rcpmod.ControlSpec{
		act("hostile.loop"), act("hostile.memory"), act("hostile.badEvent"), act("hostile.nanEvent"),
		act("hostile.undeclaredEvent"), act("hostile.panic"),
		txt("hostile.readFile"), txt("hostile.env"), txt("hostile.httpOtherHost"), txt("hostile.httpPath"),
		txt("hostile.redirect"), txt("hostile.bigAnswer"),
	}
}

func (c *camera) Connect() error {
	c.host.Emit(rcpmod.StatusEvent(rcpmod.StateConnected, ""))
	return nil
}

func (c *camera) Close() {}

// Get runs a probe and reports what it got, as text.
func (c *camera) Get(id string) (rcpmod.Value, error) {
	switch id {
	case "hostile.readFile":
		// No file system is mounted: every path fails.
		for _, p := range []string{"C:/Windows/win.ini", "/etc/passwd", "rig.json", "."} {
			if _, err := os.ReadFile(p); err == nil {
				return rcpmod.Text("read " + p), nil
			}
		}
		return rcpmod.Text("no file"), nil
	case "hostile.env":
		return rcpmod.Text(strconv.Itoa(len(os.Environ()))), nil
	case "hostile.httpOtherHost":
		// The host decides the address; a full URL is not a path.
		_, _, err := c.host.HTTP("GET", "http://example.com/", nil)
		return rcpmod.Text(errText(err)), nil
	case "hostile.httpPath":
		_, _, err := c.host.HTTP("GET", "//example.com/x", nil)
		return rcpmod.Text(errText(err)), nil
	case "hostile.redirect":
		status, _, err := c.host.HTTP("GET", "/redirect", nil)
		return rcpmod.Text(strconv.Itoa(status) + " " + errText(err)), nil
	case "hostile.bigAnswer":
		_, _, err := c.host.HTTP("GET", "/big", nil)
		return rcpmod.Text(errText(err)), nil
	}
	return rcpmod.Value{}, rcpmod.ErrUnknownControl
}

func errText(err error) string {
	if err == nil {
		return "allowed"
	}
	return "refused"
}

// Set runs an attack.
func (c *camera) Set(id string, v rcpmod.Value) error {
	switch id {
	case "hostile.loop":
		for {
		}
	case "hostile.memory":
		for {
			c.hog = append(c.hog, make([]byte, 16<<20))
		}
	case "hostile.badEvent":
		c.host.Emit(rcpmod.Event{}) // carries nothing
	case "hostile.nanEvent":
		c.host.Emit(rcpmod.ControlEvent("hostile.readFile", rcpmod.Number(math.NaN())))
	case "hostile.undeclaredEvent":
		c.host.Emit(rcpmod.ControlEvent("iris", rcpmod.Number(4)))
	case "hostile.panic":
		panic("hostile module panics")
	}
	return nil
}
