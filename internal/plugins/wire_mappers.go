package plugins

import (
	"fmt"
	"math"
	"unicode"
	"unicode/utf8"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// The mappers below are the anti-corruption layer between the module wire
// DTOs (rcpmod) and the domain (core). A module is untrusted: everything it
// sends is checked before it reaches the application.

const maxNotesLen = 512

var transports = map[string]core.Transport{
	rcpmod.TransportEthernet:    core.TransportEthernet,
	rcpmod.TransportWiFi:        core.TransportWiFi,
	rcpmod.TransportUSBEthernet: core.TransportUSBEthernet,
}

// infoToCore converts and checks a module declaration.
func infoToCore(i rcpmod.Info) (core.ModelInfo, error) {
	m := core.ModelInfo{
		ID: core.ModelID(i.ID), Brand: i.Brand, Name: i.Name, DefaultPort: i.DefaultPort,
		Auth: core.AuthMode(i.Auth), Support: core.SupportLevel(i.Support), Notes: i.Notes,
		InterchangeableLens: i.InterchangeableLens, NoAddress: i.NoAddress,
	}
	for _, t := range i.Transports {
		ct, ok := transports[t]
		if !ok {
			return core.ModelInfo{}, fmt.Errorf("module %q: unknown transport", i.ID)
		}
		m.Transports = append(m.Transports, ct)
	}
	if !printable(i.Brand, core.MaxIDLen) || !printable(i.Name, core.MaxIDLen) || !printable(i.Notes, maxNotesLen) {
		return core.ModelInfo{}, fmt.Errorf("module %q: brand, name or notes too long or not printable", i.ID)
	}
	if err := m.Check(); err != nil {
		return core.ModelInfo{}, err
	}
	return m, nil
}

// specToCore converts and checks a control spec. A standard id with no
// label or group gets the standard ones.
func specToCore(s rcpmod.ControlSpec) (core.ControlSpec, error) {
	c := core.Std(core.ControlID(s.ID))
	if s.Label != "" {
		c.Label = s.Label
	}
	if s.Group != "" {
		c.Group = core.ControlGroup(s.Group)
	}
	c.Kind, c.Unit, c.Min, c.Max, c.Step = core.ControlKind(s.Kind), s.Unit, s.Min, s.Max, s.Step
	c.ReadOnly, c.Section, c.Hidden, c.Block, c.Widget = s.ReadOnly, s.Section, s.Hidden, s.Block, core.Widget(s.Widget)
	c.Guarded = c.Guarded || s.Guarded
	for _, o := range s.Options {
		c.Options = append(c.Options, core.Option{Value: o.Value, Label: o.Label})
	}
	if math.IsInf(c.Min, 0) || math.IsInf(c.Max, 0) || math.IsInf(c.Step, 0) {
		return core.ControlSpec{}, fmt.Errorf("control %q: infinite bound", s.ID)
	}
	if err := c.Check(); err != nil {
		return core.ControlSpec{}, err
	}
	return c, nil
}

func valueToCore(v rcpmod.Value) (core.Value, error) {
	c := core.Value{Kind: core.ControlKind(v.Kind), Number: v.Number, Choice: v.Choice, On: v.On, Text: v.Text}
	if math.IsNaN(c.Number) || math.IsInf(c.Number, 0) {
		return core.Value{}, fmt.Errorf("value is not a finite number: %w", core.ErrInvalidValue)
	}
	return c, c.Check()
}

func valueToWire(v core.Value) rcpmod.Value {
	return rcpmod.Value{Kind: string(v.Kind), Number: v.Number, Choice: v.Choice, On: v.On, Text: v.Text}
}

func configToWire(c core.CameraConfig) rcpmod.Config {
	return rcpmod.Config{Model: string(c.Model), Name: c.Name, Host: c.Host, Port: c.Port,
		Username: c.Username, Password: c.Password, PowerZoom: c.PowerZoom}
}

var states = map[string]core.ConnState{
	rcpmod.StateDisconnected: core.StateDisconnected,
	rcpmod.StateConnecting:   core.StateConnecting,
	rcpmod.StateConnected:    core.StateConnected,
	rcpmod.StateError:        core.StateError,
}

// maxDetailLen bounds a status detail shown in the GUI.
const maxDetailLen = 200

// eventToCore converts and checks an event: exactly one part set, a
// declared control with a value of its kind, a finite lens.
func eventToCore(e rcpmod.Event, specs interface {
	Find(core.ControlID) (core.ControlSpec, error)
}) (core.Event, error) {
	parts := 0
	var out core.Event
	if e.Status != nil {
		parts++
		st, ok := states[e.Status.State]
		if !ok || !printable(e.Status.Detail, maxDetailLen) {
			return core.Event{}, fmt.Errorf("bad status event")
		}
		out.Status = &core.Status{State: st, Detail: e.Status.Detail}
	}
	if e.Control != "" {
		parts++
		spec, err := specs.Find(core.ControlID(e.Control))
		if err != nil {
			return core.Event{}, err
		}
		v, err := valueToCore(e.Value)
		if err != nil || v.Kind != spec.Kind {
			return core.Event{}, fmt.Errorf("event for %s: bad value: %w", spec.ID, core.ErrInvalidValue)
		}
		out.Control, out.Value = spec.ID, v
	}
	if e.Lens != nil {
		parts++
		l := core.Lens(*e.Lens)
		for _, f := range []float64{l.IrisMin, l.IrisMax, l.FocalLength, l.FocalMin, l.FocalMax, l.FocusDistance, l.FocusNear, l.FocusFar} {
			if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
				return core.Event{}, fmt.Errorf("bad lens event")
			}
		}
		out.Lens = &l
	}
	if parts != 1 {
		return core.Event{}, fmt.Errorf("an event carries exactly one of status, control or lens")
	}
	return out, nil
}

// printable reports whether s is valid UTF-8 of at most max bytes without
// control or format characters.
func printable(s string, max int) bool {
	if len(s) > max || !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
}

// wireError maps a module error code to the domain error.
func wireError(e *rcpmod.WireError) error {
	msg := e.Message
	if !printable(msg, maxDetailLen) {
		msg = "module error"
	}
	if target := map[string]error{
		rcpmod.CodeUnknownControl: core.ErrUnknownControl,
		rcpmod.CodeInvalidValue:   core.ErrInvalidValue,
		rcpmod.CodeReadOnly:       core.ErrReadOnly,
		rcpmod.CodeNotConnected:   core.ErrNotConnected,
		rcpmod.CodeNotImplemented: core.ErrNotImplemented,
	}[e.Code]; target != nil {
		return fmt.Errorf("%s: %w", msg, target)
	}
	return fmt.Errorf("module: %s", msg)
}
