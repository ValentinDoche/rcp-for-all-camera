package autostart

import (
	"testing"

	"golang.org/x/sys/windows/registry"
)

// The tests use a private key, never the real Run key.
const testKey = `Software\RCPForAllCameraTest\Run`

func testEntry(t *testing.T) *Entry {
	t.Helper()
	t.Cleanup(func() {
		_ = registry.DeleteKey(registry.CURRENT_USER, testKey)
		_ = registry.DeleteKey(registry.CURRENT_USER, `Software\RCPForAllCameraTest`)
	})
	return &Entry{KeyPath: testKey, Name: "RCPForAllCamera", Exe: `C:\Program Files\RCP\rcp.exe`}
}

func TestDisabledByDefault(t *testing.T) {
	e := testEntry(t)
	if on, err := e.Enabled(); err != nil || on {
		t.Fatalf("Enabled() = %v, %v; want false", on, err)
	}
}

func TestEnableWritesQuotedExePath(t *testing.T) {
	e := testEntry(t)
	if err := e.SetEnabled(true); err != nil {
		t.Fatal(err)
	}
	k, err := registry.OpenKey(registry.CURRENT_USER, testKey, registry.QUERY_VALUE)
	if err != nil {
		t.Fatal(err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue("RCPForAllCamera")
	if err != nil || v != `"C:\Program Files\RCP\rcp.exe"` {
		t.Fatalf("Run value = %q, %v", v, err)
	}
	if on, _ := e.Enabled(); !on {
		t.Fatal("Enabled() = false after enabling")
	}
}

func TestDisableIsIdempotent(t *testing.T) {
	e := testEntry(t)
	_ = e.SetEnabled(true)
	if err := e.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	if err := e.SetEnabled(false); err != nil {
		t.Fatalf("second disable = %v", err)
	}
	if on, _ := e.Enabled(); on {
		t.Fatal("still enabled")
	}
}

func TestEntryPointingToAnotherExeIsNotEnabled(t *testing.T) {
	e := testEntry(t)
	_ = e.SetEnabled(true)
	other := *e
	other.Exe = `D:\elsewhere\rcp.exe`
	if on, _ := other.Enabled(); on {
		t.Fatal("an entry for another exe must not count as enabled for this one")
	}
}
