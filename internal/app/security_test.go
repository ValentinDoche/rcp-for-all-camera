package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestSetControlRejectsHostileIDWithoutEchoingIt(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(1, validInput())
	long := strings.Repeat("A", 10000)
	for _, id := range []string{long, "iris\x00", "<img src=x>"} {
		err := f.svc.SetControl(1, id, core.Number(4))
		if c := code(t, err); c != CodeUnknownControl {
			t.Errorf("SetControl(%.20q) code = %s, want UNKNOWN_CONTROL", id, c)
		}
		if len(err.Error()) > 200 {
			t.Errorf("error echoes the hostile id (%d bytes)", len(err.Error()))
		}
	}
}

func TestSetControlRejectsHostileChoice(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	_, _ = f.svc.AssignSlot(1, validInput())
	err := f.svc.SetControl(1, "iris", core.Choice(strings.Repeat("B", 10000)))
	if c := code(t, err); c != CodeInvalidValue {
		t.Fatalf("code = %s, want INVALID_VALUE", c)
	}
	if len(err.Error()) > 200 {
		t.Fatalf("error echoes the hostile choice (%d bytes)", len(err.Error()))
	}
}

func TestAssignSlotRejectsHugeModelIDWithoutEchoingIt(t *testing.T) {
	f := newFixture(t, config.Rig{Version: config.Version})
	in := validInput()
	in.Model = core.ModelID(strings.Repeat("m", 10000))
	_, err := f.svc.AssignSlot(1, in)
	if c := code(t, err); c != CodeUnknownModel {
		t.Fatalf("code = %s, want UNKNOWN_MODEL", c)
	}
	if len(err.Error()) > 200 {
		t.Fatalf("error echoes the model id (%d bytes)", len(err.Error()))
	}
}

func TestSlotInputNeverPrintsPassword(t *testing.T) {
	in := SlotInput{Model: "acme.secure", Host: "10.0.0.2", Username: "admin", Password: strPtr("hunter2")}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("in", "in", in, "ptr", &in)
	for _, verb := range []string{"%v", "%+v", "%#v"} {
		buf.WriteString(fmt.Sprintf(verb, in))
	}
	if strings.Contains(buf.String(), "hunter2") {
		t.Fatalf("SlotInput leaked the password: %s", buf.String())
	}
}

type failingStore struct{ memStore }

func (f *failingStore) Save(config.Rig) error { return errors.New("disk full") }

func TestAssignSlotRestoresSecretWhenSaveFails(t *testing.T) {
	reg := core.NewRegistry()
	_ = reg.Register(testDriver{})
	var saved config.Rig
	saved.Version = config.Version
	saved.Slots[0] = &core.CameraConfig{Model: "acme.secure", Host: "10.0.0.1", Username: "admin"}
	sec := config.NewMemorySecrets()
	_ = sec.Set(secretKey(1), "old")
	svc, err := NewService(Options{Registry: reg, Store: &failingStore{memStore{rig: saved}}, Secrets: sec})
	if err != nil {
		t.Fatal(err)
	}
	in := SlotInput{Model: "acme.secure", Host: "10.0.0.1", Username: "admin", Password: strPtr("new")}
	if _, err := svc.AssignSlot(1, in); code(t, err) != CodeInternal {
		t.Fatalf("AssignSlot with failing store = %v, want INTERNAL", err)
	}
	if v, _, _ := sec.Get(secretKey(1)); v != "old" {
		t.Fatalf("secret after failed save = %q, want the old one restored", v)
	}
	// Same on an empty slot: the new secret must not linger.
	if _, err := svc.AssignSlot(2, in); err == nil {
		t.Fatal("AssignSlot(2) succeeded with a failing store")
	}
	if _, ok, _ := sec.Get(secretKey(2)); ok {
		t.Fatal("secret of a never-saved slot was kept")
	}
}

// slowCamera claims to be connected and blocks every Get until ctx expires.
type slowCamera struct{ specs []core.ControlSpec }

func (c *slowCamera) Controls() []core.ControlSpec      { return c.specs }
func (c *slowCamera) Connect(context.Context) error     { return nil }
func (c *slowCamera) Close() error                      { return nil }
func (c *slowCamera) Status() core.Status               { return core.Status{State: core.StateConnected} }
func (c *slowCamera) Subscribe(func(core.Event)) func() { return func() {} }
func (c *slowCamera) Set(context.Context, core.ControlID, core.Value) error {
	return core.ErrNotImplemented
}
func (c *slowCamera) Get(ctx context.Context, _ core.ControlID) (core.Value, error) {
	<-ctx.Done()
	return core.Value{}, ctx.Err()
}

type slowDriver struct{}

func (slowDriver) Name() string { return "slow" }
func (slowDriver) Models() []core.ModelInfo {
	return []core.ModelInfo{{ID: "slow.cam", Brand: "Slow", Name: "Cam", Driver: "slow",
		Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 80,
		Auth: core.AuthNone, Support: core.SupportPlanned}}
}
func (slowDriver) New(core.ModelInfo, core.CameraConfig) (core.Camera, error) {
	var specs []core.ControlSpec
	for _, id := range []core.ControlID{core.Iris, core.Gain, core.Zoom, core.Focus, core.Tint} {
		s := core.Std(id)
		s.Kind, s.Max = core.KindRange, 1
		specs = append(specs, s)
	}
	return &slowCamera{specs: specs}, nil
}

func TestControlsIsBoundedByOneTimeoutNotOnePerControl(t *testing.T) {
	old := commandTimeout
	commandTimeout = 50 * time.Millisecond
	defer func() { commandTimeout = old }()

	reg := core.NewRegistry()
	_ = reg.Register(slowDriver{})
	svc, err := NewService(Options{Registry: reg, Store: &memStore{rig: config.Rig{Version: config.Version}}, Secrets: config.NewMemorySecrets()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AssignSlot(1, SlotInput{Model: "slow.cam", Host: "10.0.0.9"}); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	cv, err := svc.Controls(1)
	if err != nil || len(cv) != 5 {
		t.Fatalf("Controls = %d, %v", len(cv), err)
	}
	if d := time.Since(start); d > 3*commandTimeout {
		t.Fatalf("Controls took %v for 5 slow controls, want about one timeout (%v)", d, commandTimeout)
	}
}
