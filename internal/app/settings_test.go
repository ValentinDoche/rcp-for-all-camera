package app

import (
	"errors"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

type fakeAutoStart struct {
	on  bool
	err error
}

func (f *fakeAutoStart) Enabled() (bool, error) { return f.on, f.err }
func (f *fakeAutoStart) SetEnabled(on bool) error {
	if f.err != nil {
		return f.err
	}
	f.on = on
	return nil
}

var testNICs = []NetInterface{{Name: "Ethernet", IP: "192.168.1.110"}, {Name: "Wi-Fi", IP: "10.0.0.5"}}

func newSettingsService(t *testing.T, saved SavedRig, as AutoStarter) (*Service, *memStore, *bindRecorder) {
	t.Helper()
	reg := core.NewRegistry()
	rec := &bindRecorder{}
	if err := reg.Register(acme(rec)...); err != nil {
		t.Fatal(err)
	}
	st := &memStore{rig: saved}
	svc, err := NewService(Options{
		Registry: reg, Store: st, Secrets: newMemSecrets(),
		Interfaces: func() []NetInterface { return testNICs },
		AutoStart:  as,
	})
	if err != nil {
		t.Fatal(err)
	}
	return svc, st, rec
}

// bindRecorder is testDriver that remembers the BindIP each camera got.
type bindRecorder struct {
	testDriver
	last string
}

func (b *bindRecorder) New(cfg core.CameraConfig) (core.Camera, error) {
	b.last = cfg.BindIP
	return b.testDriver.New(cfg)
}

func TestSettingsDefaults(t *testing.T) {
	svc, _, _ := newSettingsService(t, SavedRig{}, &fakeAutoStart{})
	st := svc.Settings()
	if st.BindIP != "" || st.AutoStart {
		t.Fatalf("Settings() = %+v, want defaults", st)
	}
}

func TestNetworkInterfacesListsLocalAddresses(t *testing.T) {
	svc, _, _ := newSettingsService(t, SavedRig{}, nil)
	if got := svc.NetworkInterfaces(); len(got) != 2 || got[0].IP != "192.168.1.110" {
		t.Fatalf("NetworkInterfaces() = %v", got)
	}
}

func TestSetBindIPPersistsAndReachesCameras(t *testing.T) {
	svc, st, rec := newSettingsService(t, SavedRig{}, nil)
	if _, err := svc.AssignSlot(1, validInput()); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetBindIP("10.0.0.5"); err != nil {
		t.Fatal(err)
	}
	if st.rig.BindIP != "10.0.0.5" || svc.Settings().BindIP != "10.0.0.5" {
		t.Fatalf("bind IP not stored: rig %q", st.rig.BindIP)
	}
	if rec.last != "10.0.0.5" {
		t.Fatalf("camera rebuilt with BindIP %q, want 10.0.0.5", rec.last)
	}
	if err := svc.SetBindIP(""); err != nil || svc.Settings().BindIP != "" {
		t.Fatalf("SetBindIP(default) = %v", err)
	}
}

func TestSetBindIPRejectsAddressesNotOnThisPC(t *testing.T) {
	svc, st, _ := newSettingsService(t, SavedRig{}, nil)
	for _, ip := range []string{"192.168.9.9", "not-an-ip", "::1/64"} {
		if c := code(t, svc.SetBindIP(ip)); c != CodeInvalidConfig {
			t.Errorf("SetBindIP(%q) code = %s, want INVALID_CONFIG", ip, c)
		}
	}
	if st.rig.BindIP != "" {
		t.Fatal("rejected bind IP was stored")
	}
}

func TestSavedBindIPIsRestoredEvenIfInterfaceIsDown(t *testing.T) {
	saved := SavedRig{BindIP: "172.16.0.9"}
	svc, _, _ := newSettingsService(t, saved, nil)
	st := svc.Settings()
	if st.BindIP != "172.16.0.9" || st.BindIPAvailable {
		t.Fatalf("Settings() = %+v, want stored IP flagged unavailable", st)
	}
}

func TestAutoStartGoesThroughTheProvider(t *testing.T) {
	as := &fakeAutoStart{}
	svc, _, _ := newSettingsService(t, SavedRig{}, as)
	if err := svc.SetAutoStart(true); err != nil {
		t.Fatal(err)
	}
	if !as.on || !svc.Settings().AutoStart {
		t.Fatal("auto start not enabled")
	}
}

func TestAutoStartFailureIsInternalError(t *testing.T) {
	svc, _, _ := newSettingsService(t, SavedRig{},
		&fakeAutoStart{err: errors.New(`registry: access denied HKCU\...`)})
	if c := code(t, svc.SetAutoStart(true)); c != CodeInternal {
		t.Fatalf("code = %s, want INTERNAL", c)
	}
}

func TestAutoStartWithoutProviderIsNotImplemented(t *testing.T) {
	svc, _, _ := newSettingsService(t, SavedRig{}, nil)
	if c := code(t, svc.SetAutoStart(true)); c != CodeNotImplemented {
		t.Fatalf("code = %s, want NOT_IMPLEMENTED", c)
	}
}
