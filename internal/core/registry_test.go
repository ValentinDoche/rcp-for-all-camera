package core_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// stubCamera is the camera the stub module builds; it remembers its config.
type stubCamera struct{ cfg core.CameraConfig }

func (c *stubCamera) Controls() []core.ControlSpec      { return nil }
func (c *stubCamera) Connect(context.Context) error     { return nil }
func (c *stubCamera) Close() error                      { return nil }
func (c *stubCamera) Status() core.Status               { return core.Status{State: core.StateDisconnected} }
func (c *stubCamera) Subscribe(func(core.Event)) func() { return func() {} }
func (c *stubCamera) Set(context.Context, core.ControlID, core.Value) error {
	return core.ErrNotConnected
}
func (c *stubCamera) Get(context.Context, core.ControlID) (core.Value, error) {
	return core.Value{}, core.ErrNotConnected
}

// stubModule declares info and builds stubCameras.
type stubModule struct{ info core.ModelInfo }

func (m stubModule) Info() core.ModelInfo { return m.info }
func (m stubModule) New(cfg core.CameraConfig) (core.Camera, error) {
	return &stubCamera{cfg: cfg}, nil
}

func testModel(id core.ModelID) core.ModelInfo {
	return core.ModelInfo{
		ID: id, Brand: "Acme", Name: string(id),
		Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 80,
		Auth: core.AuthNone, Support: core.SupportPlanned,
	}
}

func module(id core.ModelID) stubModule { return stubModule{testModel(id)} }

func TestRegistryListsModelsInRegistrationOrder(t *testing.T) {
	r := core.NewRegistry()
	if err := r.Register(module("acme-b"), module("acme-a")); err != nil {
		t.Fatal(err)
	}
	got := r.Models()
	if len(got) != 2 || got[0].ID != "acme-b" || got[1].ID != "acme-a" {
		t.Fatalf("Models() = %v, want [acme-b acme-a]", got)
	}
}

func TestRegistryRejectsDuplicateModel(t *testing.T) {
	r := core.NewRegistry()
	if err := r.Register(module("acme-a")); err != nil {
		t.Fatal(err)
	}
	if err := r.Register(module("acme-a")); !errors.Is(err, core.ErrDuplicateModel) {
		t.Fatalf("second Register = %v, want ErrDuplicateModel", err)
	}
}

func TestRegistryRegistersNothingWhenOneModuleIsBad(t *testing.T) {
	r := core.NewRegistry()
	if err := r.Register(module("acme-a"), module("Bad.ID")); err == nil {
		t.Fatal("Register(bad id) = nil, want error")
	}
	if n := len(r.Models()); n != 0 {
		t.Errorf("%d models registered, want none", n)
	}
}

func TestCheckModelID(t *testing.T) {
	for _, ok := range []core.ModelID{"demo", "bm-studio-6k-pro", "sony-fx6", "panasonic-aw-ue150"} {
		if err := core.CheckModelID(ok); err != nil {
			t.Errorf("CheckModelID(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []core.ModelID{"", "Demo", "bm.studio", "-bm", "bm-", "bm--studio", "bm studio", core.ModelID(strings.Repeat("a", 65))} {
		if err := core.CheckModelID(bad); !errors.Is(err, core.ErrUnknownModel) {
			t.Errorf("CheckModelID(%q) = %v, want ErrUnknownModel", bad, err)
		}
	}
}

func TestRegistryNewBuildsCameraForValidConfig(t *testing.T) {
	r := core.NewRegistry()
	_ = r.Register(module("acme-a"))
	cam, err := r.New(core.CameraConfig{Model: "acme-a", Host: "192.168.1.20"})
	if err != nil {
		t.Fatalf("New = %v", err)
	}
	if got := cam.(*stubCamera).cfg.Port; got != 80 {
		t.Errorf("port = %d, want the model default 80", got)
	}
}

func TestRegistryNewRejectsUnknownModel(t *testing.T) {
	r := core.NewRegistry()
	_, err := r.New(core.CameraConfig{Model: "nope", Host: "10.0.0.1"})
	if !errors.Is(err, core.ErrUnknownModel) {
		t.Fatalf("New(unknown) = %v, want ErrUnknownModel", err)
	}
}

func TestRegistryModelsAreCopies(t *testing.T) {
	r := core.NewRegistry()
	_ = r.Register(module("acme-a"))
	r.Models()[0].Transports[0] = core.TransportWiFi
	if got := r.Models()[0].Transports[0]; got != core.TransportEthernet {
		t.Errorf("Models() shares its slices with the registry: %s", got)
	}
}

func TestValidateConfigAcceptsHostnamesAndIPs(t *testing.T) {
	m := testModel("acme-a")
	for _, host := range []string{"192.168.1.20", "cam-a.local", "fe80::1", "CAM1"} {
		c := core.CameraConfig{Model: m.ID, Host: host}
		if err := core.ValidateConfig(m, c); err != nil {
			t.Errorf("host %q: %v, want nil", host, err)
		}
	}
}

func TestValidateConfigRejectsBadHosts(t *testing.T) {
	m := testModel("acme-a")
	bad := []string{"", "http://10.0.0.1", "10.0.0.1/api", "a b", "host:80", strings.Repeat("a", 254), "-cam", "cam\x00"}
	for _, host := range bad {
		c := core.CameraConfig{Model: m.ID, Host: host}
		if err := core.ValidateConfig(m, c); !errors.Is(err, core.ErrInvalidConfig) {
			t.Errorf("host %q: %v, want ErrInvalidConfig", host, err)
		}
	}
}

func TestValidateConfigRejectsPortOutOfRange(t *testing.T) {
	m := testModel("acme-a")
	for _, p := range []int{-1, 65536} {
		c := core.CameraConfig{Model: m.ID, Host: "10.0.0.1", Port: p}
		if err := core.ValidateConfig(m, c); !errors.Is(err, core.ErrInvalidConfig) {
			t.Errorf("port %d: %v, want ErrInvalidConfig", p, err)
		}
	}
}

func TestValidateConfigRequiresUsernameWhenModelNeedsAuth(t *testing.T) {
	m := testModel("acme-a")
	m.Auth = core.AuthRequired
	c := core.CameraConfig{Model: m.ID, Host: "10.0.0.1"}
	if err := core.ValidateConfig(m, c); !errors.Is(err, core.ErrInvalidConfig) {
		t.Fatalf("missing username = %v, want ErrInvalidConfig", err)
	}
}

func TestValidateConfigRejectsOverlongCredentials(t *testing.T) {
	m := testModel("acme-a")
	c := core.CameraConfig{Model: m.ID, Host: "10.0.0.1", Username: strings.Repeat("u", 200)}
	if err := core.ValidateConfig(m, c); !errors.Is(err, core.ErrInvalidConfig) {
		t.Fatalf("long username = %v, want ErrInvalidConfig", err)
	}
}

func TestModelCheckRejectsEmptyTransports(t *testing.T) {
	m := testModel("acme-a")
	m.Transports = nil
	if err := m.Check(); err == nil {
		t.Fatal("Check(no transports) = nil, want error")
	}
}
