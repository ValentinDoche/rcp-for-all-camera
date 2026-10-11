package core

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeCamera struct{ cfg CameraConfig }

func (c *fakeCamera) Controls() []ControlSpec                       { return nil }
func (c *fakeCamera) Connect(context.Context) error                 { return nil }
func (c *fakeCamera) Close() error                                  { return nil }
func (c *fakeCamera) Status() Status                                { return Status{State: StateDisconnected} }
func (c *fakeCamera) Get(context.Context, ControlID) (Value, error) { return Value{}, ErrNotConnected }
func (c *fakeCamera) Set(context.Context, ControlID, Value) error   { return ErrNotConnected }
func (c *fakeCamera) Subscribe(func(Event)) (cancel func())         { return func() {} }

type fakeDriver struct {
	name   string
	models []ModelInfo
}

func (d fakeDriver) Name() string        { return d.name }
func (d fakeDriver) Models() []ModelInfo { return d.models }
func (d fakeDriver) New(_ ModelInfo, cfg CameraConfig) (Camera, error) {
	return &fakeCamera{cfg: cfg}, nil
}

func testModel(id ModelID) ModelInfo {
	return ModelInfo{
		ID: id, Brand: "Acme", Name: string(id), Driver: "acme",
		Transports: []Transport{TransportEthernet}, DefaultPort: 80,
		Auth: AuthNone, Support: SupportPlanned,
	}
}

func TestRegistryListsModelsInRegistrationOrder(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeDriver{"acme", []ModelInfo{testModel("acme.b"), testModel("acme.a")}}); err != nil {
		t.Fatal(err)
	}
	got := r.Models()
	if len(got) != 2 || got[0].ID != "acme.b" || got[1].ID != "acme.a" {
		t.Fatalf("Models() = %v, want [acme.b acme.a]", got)
	}
}

func TestRegistryRejectsDuplicateModel(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(fakeDriver{"acme", []ModelInfo{testModel("acme.a")}})
	err := r.Register(fakeDriver{"acme", []ModelInfo{testModel("acme.a")}})
	if !errors.Is(err, ErrDuplicateModel) {
		t.Fatalf("second Register = %v, want ErrDuplicateModel", err)
	}
}

func TestRegistryRejectsModelOwnedByAnotherDriver(t *testing.T) {
	r := NewRegistry()
	m := testModel("acme.a")
	m.Driver = "other"
	if err := r.Register(fakeDriver{"acme", []ModelInfo{m}}); err == nil {
		t.Fatal("Register with mismatched driver name = nil, want error")
	}
}

func TestRegistryRejectsModelIDWithoutDriverPrefix(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(fakeDriver{"acme", []ModelInfo{testModel("pocket")}}); err == nil {
		t.Fatal("Register(model id without prefix) = nil, want error")
	}
}

func TestRegistryNewBuildsCameraForValidConfig(t *testing.T) {
	r := NewRegistry()
	_ = r.Register(fakeDriver{"acme", []ModelInfo{testModel("acme.a")}})
	cam, err := r.New(CameraConfig{Model: "acme.a", Host: "192.168.1.20", Transport: TransportEthernet})
	if err != nil || cam == nil {
		t.Fatalf("New = %v, %v; want camera", cam, err)
	}
}

func TestRegistryNewRejectsUnknownModel(t *testing.T) {
	r := NewRegistry()
	_, err := r.New(CameraConfig{Model: "nope.x", Host: "10.0.0.1"})
	if !errors.Is(err, ErrUnknownModel) {
		t.Fatalf("New(unknown) = %v, want ErrUnknownModel", err)
	}
}

func TestValidateConfigAcceptsHostnamesAndIPs(t *testing.T) {
	m := testModel("acme.a")
	for _, host := range []string{"192.168.1.20", "cam-a.local", "fe80::1", "CAM1"} {
		c := CameraConfig{Model: m.ID, Host: host, Transport: TransportEthernet}
		if err := ValidateConfig(m, c); err != nil {
			t.Errorf("host %q: %v, want nil", host, err)
		}
	}
}

func TestValidateConfigRejectsBadHosts(t *testing.T) {
	m := testModel("acme.a")
	bad := []string{"", "http://10.0.0.1", "10.0.0.1/api", "a b", "host:80", strings.Repeat("a", 254), "-cam", "cam\x00"}
	for _, host := range bad {
		c := CameraConfig{Model: m.ID, Host: host, Transport: TransportEthernet}
		if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("host %q: %v, want ErrInvalidConfig", host, err)
		}
	}
}

func TestValidateConfigRejectsPortOutOfRange(t *testing.T) {
	m := testModel("acme.a")
	for _, p := range []int{-1, 65536} {
		c := CameraConfig{Model: m.ID, Host: "10.0.0.1", Port: p, Transport: TransportEthernet}
		if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("port %d: %v, want ErrInvalidConfig", p, err)
		}
	}
}

func TestValidateConfigRejectsTransportTheModelLacks(t *testing.T) {
	m := testModel("acme.a")
	c := CameraConfig{Model: m.ID, Host: "10.0.0.1", Transport: TransportWiFi}
	if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("wifi on ethernet-only model = %v, want ErrInvalidConfig", err)
	}
}

func TestValidateConfigDefaultsEmptyTransportToModelFirst(t *testing.T) {
	m := testModel("acme.a")
	c := CameraConfig{Model: m.ID, Host: "10.0.0.1"}
	if err := ValidateConfig(m, c); err != nil {
		t.Fatalf("empty transport = %v, want nil", err)
	}
}

func TestValidateConfigRequiresUsernameWhenModelNeedsAuth(t *testing.T) {
	m := testModel("acme.a")
	m.Auth = AuthRequired
	c := CameraConfig{Model: m.ID, Host: "10.0.0.1", Transport: TransportEthernet}
	if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("missing username = %v, want ErrInvalidConfig", err)
	}
}

func TestValidateConfigRejectsOverlongCredentials(t *testing.T) {
	m := testModel("acme.a")
	c := CameraConfig{Model: m.ID, Host: "10.0.0.1", Username: strings.Repeat("u", 200)}
	if err := ValidateConfig(m, c); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("long username = %v, want ErrInvalidConfig", err)
	}
}

func TestModelCheckRejectsEmptyTransports(t *testing.T) {
	m := testModel("acme.a")
	m.Transports = nil
	if err := m.Check(); err == nil {
		t.Fatal("Check(no transports) = nil, want error")
	}
}
